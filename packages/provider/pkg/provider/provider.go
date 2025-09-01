package provider

import (
    "embed"
    "fmt"
    "strings"
    "regexp"

    awscognito "github.com/pulumi/pulumi-aws/sdk/v6/go/aws/cognito"
    aws "github.com/pulumi/pulumi-aws/sdk/v6/go/aws"
    awscloudwatch "github.com/pulumi/pulumi-aws/sdk/v6/go/aws/cloudwatch"
    awsiam "github.com/pulumi/pulumi-aws/sdk/v6/go/aws/iam"
    awslambda "github.com/pulumi/pulumi-aws/sdk/v6/go/aws/lambda"
    awsvp "github.com/pulumi/pulumi-aws/sdk/v6/go/aws/verifiedpermissions"
    p "github.com/pulumi/pulumi-go-provider"
    "github.com/pulumi/pulumi-go-provider/infer"
    "github.com/pulumi/pulumi/sdk/v3/go/pulumi"
)

//go:embed ../../assets/index.mjs
var authorizerIndexMjs string

//go:embed ../../assets/cognito-trigger-stub.mjs
var cognitoTriggerStub string

// NewProvider wires up the multi-language component provider surface.
func NewProvider() (p.Provider, error) {
    return infer.NewProvider(infer.Options{
        Components: []infer.Component{
            infer.Component[*AuthorizerWithPolicyStore, AuthorizerArgs, AuthorizerResult](),
        },
    })
}

// AuthorizerArgs defines the inputs for the component resource.
type AuthorizerArgs struct {
    // Policy store description
    Description *string `pulumi:"description,optional"`
    // Validation mode for the policy store: STRICT | OFF (default STRICT)
    ValidationMode *string           `pulumi:"validationMode,optional"`
    LambdaEnv      map[string]string `pulumi:"lambdaEnvironment,optional"`

    // Global deployment behavior flag (mirrors VP-6 semantics):
    // when true, resources are created without retention and deletion protection.
    // when false, resources are retained on delete and user pools have deletion protection enabled.
    IsEphemeral *bool `pulumi:"isEphemeral,optional"`

    // Optional Cognito configuration. When provided, a Cognito User Pool will be provisioned
    // and configured as the Verified Permissions Identity Source for the created policy store.
    Cognito *CognitoConfig `pulumi:"cognito,optional"`
}

// AuthorizerResult defines the outputs for the component resource.
type AuthorizerResult struct {
    PolicyStoreId  string `pulumi:"policyStoreId"`
    PolicyStoreArn string `pulumi:"policyStoreArn"`
    FunctionArn    string `pulumi:"functionArn"`
    RoleArn        string `pulumi:"roleArn"`

    // Cognito-related outputs (present when Cognito is provisioned)
    UserPoolId       *string            `pulumi:"userPoolId,optional"`
    UserPoolArn      *string            `pulumi:"userPoolArn,optional"`
    UserPoolDomain   *string            `pulumi:"userPoolDomain,optional"`
    IdentityPoolId   *string            `pulumi:"identityPoolId,optional"`
    AuthRoleArn      *string            `pulumi:"authRoleArn,optional"`
    UnauthRoleArn    *string            `pulumi:"unauthRoleArn,optional"`
    UserPoolClientIds []string          `pulumi:"userPoolClientIds,optional"`
    Parameters       map[string]string  `pulumi:"parameters,optional"`
}

// AuthorizerWithPolicyStore is the component implementing the Construct.
type AuthorizerWithPolicyStore struct{}

func (c *AuthorizerWithPolicyStore) Annotate(a infer.Annotator) {
    a.Describe(&c, "Provision an AWS Verified Permissions Policy Store and a bundled Lambda Request Authorizer.")
    a.Token(&c, "verified-permissions-authorizer:index:AuthorizerWithPolicyStore")
}

// Construct implements the component creation logic.
func (c *AuthorizerWithPolicyStore) Construct(ctx *pulumi.Context, name string, args AuthorizerArgs, opts pulumi.ResourceOption) (AuthorizerResult, error) {
    var res AuthorizerResult
    if args.ValidationMode == nil {
        def := "STRICT"
        args.ValidationMode = &def
    }
    ephemeral := true
    if args.IsEphemeral != nil {
        ephemeral = *args.IsEphemeral
    }

    // 1) Verified Permissions Policy Store
    storeArgs := &awsvp.PolicyStoreArgs{
        ValidationSettings: awsvp.PolicyStoreValidationSettingsArgs{
            Mode: pulumi.String(*args.ValidationMode),
        },
    }
    if args.Description != nil {
        storeArgs.Description = pulumi.StringPtr(*args.Description)
    }
    store, err := awsvp.NewPolicyStore(ctx, fmt.Sprintf("%s-store", name), storeArgs, withRetention(opts, !ephemeral))
    if err != nil {
        return res, err
    }

    // 2) IAM Role
    role, err := awsiam.NewRole(ctx, fmt.Sprintf("%s-role", name), &awsiam.RoleArgs{
        AssumeRolePolicy: awsiam.GetPolicyDocumentOutput(ctx, awsiam.GetPolicyDocumentOutputArgs{
            Statements: awsiam.GetPolicyDocumentStatementArray{
                awsiam.GetPolicyDocumentStatementArgs{
                    Actions: pulumi.StringArray{pulumi.String("sts:AssumeRole")},
                    Principals: awsiam.GetPolicyDocumentStatementPrincipalArray{
                        awsiam.GetPolicyDocumentStatementPrincipalArgs{
                            Type: pulumi.String("Service"),
                            Identifiers: pulumi.StringArray{pulumi.String("lambda.amazonaws.com")},
                        },
                    },
                },
            },
        }).Json(),
        Description: pulumi.StringPtr("Role for Verified Permissions Lambda Authorizer"),
    }, withRetention(opts, !ephemeral))
    if err != nil {
        return res, err
    }

    // Basic logs policy
    _, err = awsiam.NewRolePolicyAttachment(ctx, fmt.Sprintf("%s-logs", name), &awsiam.RolePolicyAttachmentArgs{
        Role:      role.Name,
        PolicyArn: pulumi.String(awsiam.ManagedPolicyAWSLambdaBasicExecutionRole),
    }, withRetention(opts, !ephemeral))
    if err != nil {
        return res, err
    }

    // Verified Permissions access policy: GetPolicyStore + IsAuthorized
    vpPol, err := awsiam.NewPolicy(ctx, fmt.Sprintf("%s-vp", name), &awsiam.PolicyArgs{
        Policy: pulumi.All(store.Arn).ApplyT(func(vals []interface{}) (string, error) {
            arn := vals[0].(string)
            return fmt.Sprintf(`{
  "Version": "2012-10-17",
  "Statement": [
    {
      "Sid": "VerifiedPermissionsAccess",
      "Effect": "Allow",
      "Action": [
        "verifiedpermissions:GetPolicyStore",
        "verifiedpermissions:IsAuthorized"
      ],
      "Resource": "%s"
    }
  ]
}`, arn), nil
        }).(pulumi.StringOutput),
    }, withRetention(opts, !ephemeral))
    if err != nil {
        return res, err
    }
    _, err = awsiam.NewRolePolicyAttachment(ctx, fmt.Sprintf("%s-vp-attach", name), &awsiam.RolePolicyAttachmentArgs{
        Role:      role.Name,
        PolicyArn: vpPol.Arn,
    }, withRetention(opts, !ephemeral))
    if err != nil {
        return res, err
    }

    // 3) Lambda code: embed built authorizer
    code := pulumi.NewAssetArchive(map[string]pulumi.AssetOrArchive{
        "index.mjs": pulumi.NewStringAsset(authorizerIndexMjs),
    })

    fn, err := awslambda.NewFunction(ctx, fmt.Sprintf("%s-authorizer", name), &awslambda.FunctionArgs{
        Role:    role.Arn,
        Runtime: pulumi.String("nodejs22.x"), // fixed; not configurable
        Handler: pulumi.String("index.handler"),
        Code:    code,
        Environment: &awslambda.FunctionEnvironmentArgs{
            Variables: pulumi.StringMap(func() map[string]pulumi.StringInput {
                m := map[string]pulumi.StringInput{
                    "POLICY_STORE_ID": store.ID().ToStringOutput(),
                }
                for k, v := range args.LambdaEnv {
                    m[k] = pulumi.String(v)
                }
                return m
            }()),
        },
        Architectures: pulumi.StringArray{pulumi.String("arm64")},
        Timeout:       pulumi.Int(10),
    }, withRetention(opts, !ephemeral))
    if err != nil {
        return res, err
    }

    // 4) Log group
    _, err = awscloudwatch.NewLogGroup(ctx, fmt.Sprintf("%s-lg", name), &awscloudwatch.LogGroupArgs{
        Name:            fn.Name.ApplyT(func(n string) (string, error) { return "/aws/lambda/" + n, nil }).(pulumi.StringOutput),
        RetentionInDays: pulumi.IntPtr(14),
    }, withRetention(opts, !ephemeral))
    if err != nil {
        return res, err
    }

    outputs := map[string]any{
        "policyStoreId":  store.ID(),
        "policyStoreArn": store.Arn,
        "functionArn":    fn.Arn,
        "roleArn":        role.Arn,
    }

    // 5) Optional Cognito provisioning + Verified Permissions identity source
    if args.Cognito != nil {
        // Create the user pool
        upRes, idpRes, err := provisionCognito(ctx, name, store, *args.Cognito, ephemeral, withRetention(opts, !ephemeral))
        if err != nil {
            return res, err
        }
        // Merge outputs
        for k, v := range upRes {
            outputs[k] = v
        }
        if idpRes != nil {
            for k, v := range idpRes {
                outputs[k] = v
            }
        }
    }

    // Return outputs via infer.SetOutputs
    return AuthorizerResult{}, infer.SetOutputs(outputs)
}

// withRetention augments resource options with RetainOnDelete when retain==true.
func withRetention(opts pulumi.ResourceOption, retain bool) pulumi.ResourceOption {
    if retain {
        return pulumi.Merge(opts, pulumi.RetainOnDelete(true))
    }
    return opts
}

// ---- Cognito configuration types ----

type SignInAliases struct {
    Username          *bool `pulumi:"username,optional"`
    Email             *bool `pulumi:"email,optional"`
    Phone             *bool `pulumi:"phone,optional"`
    PreferredUsername *bool `pulumi:"preferredUsername,optional"`
}

type AutoVerify struct {
    Email *bool `pulumi:"email,optional"`
    Phone *bool `pulumi:"phone,optional"`
}

type InviteTemplate struct {
    EmailSubject *string `pulumi:"emailSubject,optional"`
    EmailBody    *string `pulumi:"emailBody,optional"`
    SmsMessage   *string `pulumi:"smsMessage,optional"`
}

type VerificationTemplate struct {
    EmailSubject *string `pulumi:"emailSubject,optional"`
    EmailBody    *string `pulumi:"emailBody,optional"`
    SmsMessage   *string `pulumi:"smsMessage,optional"`
}

type CustomAttributes struct {
    GlobalRoles *bool `pulumi:"globalRoles,optional"`
    TenantId    *bool `pulumi:"tenantId,optional"`
    TenantName  *bool `pulumi:"tenantName,optional"`
    UserId      *bool `pulumi:"userId,optional"`
}

type DomainConfig struct {
    CertificateArn *string `pulumi:"certificateArn,optional"`
    DomainName     *string `pulumi:"domainName,optional"`
}

type TriggerConfig struct {
    Enabled     *bool             `pulumi:"enabled,optional"`
    Environment map[string]string `pulumi:"environment,optional"`
    Permissions []string          `pulumi:"permissions,optional"`
}

type CognitoConfig struct {
    // Whether to also create an Identity Pool and default authenticated/unauthenticated roles
    IdentityPoolFederation *bool `pulumi:"identityPoolFederation,optional"`

    // Sign-in aliases to enable (username is implicit). Maps to Cognito alias attributes.
    SignInAliases *SignInAliases `pulumi:"signInAliases,optional"`

    // Email sending account; 'COGNITO_DEFAULT' recommended when not using SES.
    EmailSendingAccount *string `pulumi:"emailSendingAccount,optional"`

    // MFA configuration (OFF | ON | OPTIONAL). OPTIONAL by default.
    Mfa *string `pulumi:"mfa,optional"`
    // SMS authentication/verification message template.
    MfaMessage *string `pulumi:"mfaMessage,optional"`

    // Account recovery setting strategy. Example: PHONE_WITHOUT_MFA_AND_EMAIL
    AccountRecovery *string `pulumi:"accountRecovery,optional"`

    // Auto-verify claims.
    AutoVerify *AutoVerify `pulumi:"autoVerify,optional"`

    // Advanced security mode (OFF | AUDIT | ENFORCED). Default ENFORCED.
    AdvancedSecurityMode *string `pulumi:"advancedSecurityMode,optional"`

    // Invitation and verification templates.
    UserInvitation   *InviteTemplate       `pulumi:"userInvitation,optional"`
    UserVerification *VerificationTemplate `pulumi:"userVerification,optional"`

    // Custom attributes to add to the pool schema; booleans indicate inclusion.
    CustomAttributes *CustomAttributes `pulumi:"customAttributes,optional"`

    // User pool domain configuration (custom for non-ephemeral; hosted for ephemeral).
    Domain *DomainConfig `pulumi:"domain,optional"`

    // Map of trigger name -> config. Names: createAuthChallenge, defineAuthChallenge, verifyAuthChallengeResponse,
    // postAuthentication, preSignUp, userMigration, preTokenGeneration.
    Triggers map[string]TriggerConfig `pulumi:"triggers,optional"`

    // Optional list of user pool client logical names to create. If empty, a single client named "default" is created.
    Clients []string `pulumi:"clients,optional"`
}

// provisionCognito provisions a Cognito User Pool (and optional Identity Pool) and configures it
// as the Identity Source for the given Verified Permissions policy store.
// Returns: outputs, effective domain (if any), identity pool outputs (if any), error
func provisionCognito(
    ctx *pulumi.Context,
    name string,
    store *awsvp.PolicyStore,
    cfg CognitoConfig,
    ephemeral bool,
    opts pulumi.ResourceOption,
) (map[string]any, map[string]any, error) {
    // Defaults
    emailAcct := "COGNITO_DEFAULT"
    if cfg.EmailSendingAccount != nil {
        emailAcct = *cfg.EmailSendingAccount
    }
    mfa := "OPTIONAL"
    if cfg.Mfa != nil {
        mfa = *cfg.Mfa
    }
    advSec := "ENFORCED"
    if cfg.AdvancedSecurityMode != nil {
        advSec = *cfg.AdvancedSecurityMode
    }
    mfaMsg := "Your one time password: {####}"
    if cfg.MfaMessage != nil {
        mfaMsg = *cfg.MfaMessage
    }
    // Construct Cognito user pool args
    upArgs := &awscognito.UserPoolArgs{
        Name:                    pulumi.String(fmt.Sprintf("%s-up", name)),
        MfaConfiguration:        pulumi.String(mfa),
        SmsAuthenticationMessage: pulumi.StringPtr(mfaMsg),
        EmailConfiguration: &awscognito.UserPoolEmailConfigurationArgs{
            EmailSendingAccount: pulumi.StringPtr(emailAcct),
        },
        UsernameConfiguration: &awscognito.UserPoolUsernameConfigurationArgs{
            CaseSensitive: pulumi.Bool(false),
        },
        UserPoolAddOns: &awscognito.UserPoolUserPoolAddOnsArgs{
            AdvancedSecurityMode: pulumi.StringPtr(advSec),
        },
        AdminCreateUserConfig: &awscognito.UserPoolAdminCreateUserConfigArgs{
            AllowAdminCreateUserOnly: pulumi.Bool(true),
        },
        DeletionProtection: pulumi.String(func() string {
            if ephemeral {
                return "INACTIVE"
            }
            return "ACTIVE"
        }()),
    }
    // Sign-in aliases -> AliasAttributes
    aliasAttrs := []pulumi.StringInput{}
    if cfg.SignInAliases != nil {
        if cfg.SignInAliases.Email != nil && *cfg.SignInAliases.Email {
            aliasAttrs = append(aliasAttrs, pulumi.String("email"))
        }
        if cfg.SignInAliases.Phone != nil && *cfg.SignInAliases.Phone {
            aliasAttrs = append(aliasAttrs, pulumi.String("phone_number"))
        }
        if cfg.SignInAliases.PreferredUsername != nil && *cfg.SignInAliases.PreferredUsername {
            aliasAttrs = append(aliasAttrs, pulumi.String("preferred_username"))
        }
        // Username is implicit; no extra config required
    }
    if len(aliasAttrs) > 0 {
        upArgs.AliasAttributes = pulumi.StringArray(aliasAttrs)
    }
    // Auto-verify
    autoVerif := []pulumi.StringInput{}
    if cfg.AutoVerify != nil {
        if cfg.AutoVerify.Email != nil && *cfg.AutoVerify.Email {
            autoVerif = append(autoVerif, pulumi.String("email"))
        }
        if cfg.AutoVerify.Phone != nil && *cfg.AutoVerify.Phone {
            autoVerif = append(autoVerif, pulumi.String("phone_number"))
        }
    }
    if len(autoVerif) > 0 {
        upArgs.AutoVerifiedAttributes = pulumi.StringArray(autoVerif)
    }
    // Account recovery: map common preset to RecoveryMechanisms
    if cfg.AccountRecovery != nil {
        // Minimal mapping: if contains PHONE -> phone first, else email first
        if *cfg.AccountRecovery != "" {
            if contains(*cfg.AccountRecovery, "PHONE") {
                upArgs.AccountRecoverySetting = &awscognito.UserPoolAccountRecoverySettingArgs{
                    RecoveryMechanisms: awscognito.UserPoolAccountRecoverySettingRecoveryMechanismArray{
                        &awscognito.UserPoolAccountRecoverySettingRecoveryMechanismArgs{ Name: pulumi.String("verified_phone_number"), Priority: pulumi.Int(1) },
                        &awscognito.UserPoolAccountRecoverySettingRecoveryMechanismArgs{ Name: pulumi.String("verified_email"), Priority: pulumi.Int(2) },
                    },
                }
            } else {
                upArgs.AccountRecoverySetting = &awscognito.UserPoolAccountRecoverySettingArgs{
                    RecoveryMechanisms: awscognito.UserPoolAccountRecoverySettingRecoveryMechanismArray{
                        &awscognito.UserPoolAccountRecoverySettingRecoveryMechanismArgs{ Name: pulumi.String("verified_email"), Priority: pulumi.Int(1) },
                        &awscognito.UserPoolAccountRecoverySettingRecoveryMechanismArgs{ Name: pulumi.String("verified_phone_number"), Priority: pulumi.Int(2) },
                    },
                }
            }
        }
    }
    // Custom attributes
    schema := awscognito.UserPoolSchemaArray{}
    if cfg.CustomAttributes != nil {
        // string attributes
        if cfg.CustomAttributes.GlobalRoles != nil && *cfg.CustomAttributes.GlobalRoles {
            schema = append(schema, &awscognito.UserPoolSchemaArgs{ Name: pulumi.String("globalRoles"), AttributeDataType: pulumi.String("String"), Mutable: pulumi.BoolPtr(true) })
        }
        if cfg.CustomAttributes.TenantId != nil && *cfg.CustomAttributes.TenantId {
            schema = append(schema, &awscognito.UserPoolSchemaArgs{ Name: pulumi.String("tenantId"), AttributeDataType: pulumi.String("String"), Mutable: pulumi.BoolPtr(false) })
        }
        if cfg.CustomAttributes.TenantName != nil && *cfg.CustomAttributes.TenantName {
            schema = append(schema, &awscognito.UserPoolSchemaArgs{ Name: pulumi.String("tenantName"), AttributeDataType: pulumi.String("String"), Mutable: pulumi.BoolPtr(true) })
        }
        if cfg.CustomAttributes.UserId != nil && *cfg.CustomAttributes.UserId {
            schema = append(schema, &awscognito.UserPoolSchemaArgs{ Name: pulumi.String("userId"), AttributeDataType: pulumi.String("String"), Mutable: pulumi.BoolPtr(false) })
        }
    }
    if len(schema) > 0 {
        upArgs.Schemas = schema
    }
    // Verification template
    if cfg.UserVerification != nil {
        tmpl := &awscognito.UserPoolVerificationMessageTemplateArgs{
            DefaultEmailOption: pulumi.StringPtr("CONFIRM_WITH_CODE"),
        }
        if cfg.UserVerification.EmailBody != nil {
            tmpl.EmailMessage = pulumi.StringPtr(*cfg.UserVerification.EmailBody)
        }
        if cfg.UserVerification.EmailSubject != nil {
            tmpl.EmailSubject = pulumi.StringPtr(*cfg.UserVerification.EmailSubject)
        }
        if cfg.UserVerification.SmsMessage != nil {
            tmpl.SmsMessage = pulumi.StringPtr(*cfg.UserVerification.SmsMessage)
        }
        upArgs.VerificationMessageTemplate = tmpl
    }
    // Invitation template
    if cfg.UserInvitation != nil {
        upArgs.AdminCreateUserConfig = &awscognito.UserPoolAdminCreateUserConfigArgs{
            AllowAdminCreateUserOnly: pulumi.Bool(true),
            InviteMessageTemplate:    &awscognito.UserPoolAdminCreateUserConfigInviteMessageTemplateArgs{},
        }
        if cfg.UserInvitation.EmailBody != nil {
            upArgs.AdminCreateUserConfig.InviteMessageTemplate.EmailMessage = pulumi.StringPtr(*cfg.UserInvitation.EmailBody)
        }
        if cfg.UserInvitation.EmailSubject != nil {
            upArgs.AdminCreateUserConfig.InviteMessageTemplate.EmailSubject = pulumi.StringPtr(*cfg.UserInvitation.EmailSubject)
        }
        if cfg.UserInvitation.SmsMessage != nil {
            upArgs.AdminCreateUserConfig.InviteMessageTemplate.SmsMessage = pulumi.StringPtr(*cfg.UserInvitation.SmsMessage)
        }
    }

    // Optional SMS configuration role (for MFA via SMS). Create only when MFA not OFF.
    if mfa != "OFF" {
        smsRole, extId, err := ensureSmsRole(ctx, name, opts)
        if err != nil {
            return nil, nil, err
        }
        if smsRole != nil && extId != "" {
            upArgs.SmsConfiguration = &awscognito.UserPoolSmsConfigurationArgs{
                SnsCallerArn: smsRole.Arn,
                ExternalId:   pulumi.String(extId),
            }
        }
    }

    // Triggers (optional): create lightweight lambda per enabled trigger and wire LambdaConfig
    triggerFns := map[string]*awslambda.Function{}
    if len(cfg.Triggers) > 0 {
        lambdaConfig := &awscognito.UserPoolLambdaConfigArgs{}
        if cfg.Triggers != nil {
            for trigName, trig := range cfg.Triggers {
                if trig.Enabled != nil && !*trig.Enabled {
                    continue
                }
                fn, err := newCognitoTriggerLambda(ctx, fmt.Sprintf("%s-%s", name, trigName), trig, opts)
                if err != nil {
                    return nil, nil, err
                }
                triggerFns[trigName] = fn
                switch trigName {
                case "createAuthChallenge":
                    lambdaConfig.CreateAuthChallenge = fn.Arn
                case "defineAuthChallenge":
                    lambdaConfig.DefineAuthChallenge = fn.Arn
                case "verifyAuthChallengeResponse":
                    lambdaConfig.VerifyAuthChallengeResponse = fn.Arn
                case "postAuthentication":
                    lambdaConfig.PostAuthentication = fn.Arn
                case "preSignUp":
                    lambdaConfig.PreSignUp = fn.Arn
                case "userMigration":
                    lambdaConfig.UserMigration = fn.Arn
                case "preTokenGeneration":
                    lambdaConfig.PreTokenGeneration = fn.Arn
                }
            }
        }
        upArgs.LambdaConfig = lambdaConfig
    }

    userPool, err := awscognito.NewUserPool(ctx, fmt.Sprintf("%s-userpool", name), upArgs, opts)
    if err != nil {
        return nil, nil, err
    }
    // Grant Cognito permission to invoke trigger lambdas
    for trigName, fn := range triggerFns {
        _, err := awslambda.NewPermission(ctx, fmt.Sprintf("%s-%s-invoke", name, trigName), &awslambda.PermissionArgs{
            Action:    pulumi.String("lambda:InvokeFunction"),
            Function:  fn.Name,
            Principal: pulumi.String("cognito-idp.amazonaws.com"),
            SourceArn: userPool.Arn,
        }, opts)
        if err != nil {
            return nil, nil, err
        }
    }

    // Domain
    var domainOut pulumi.StringOutput
    if cfg.Domain != nil {
        // Determine region and stack for hosted domain composition
        region, _ := aws.GetRegion(ctx, &aws.GetRegionArgs{}, nil)
        if !ephemeral {
            if cfg.Domain.DomainName == nil || cfg.Domain.CertificateArn == nil {
                return nil, nil, fmt.Errorf("cognito.domain.domainName and certificateArn are required when isEphemeral=false")
            }
            d, err := awscognito.NewUserPoolDomain(ctx, fmt.Sprintf("%s-domain", name), &awscognito.UserPoolDomainArgs{
                Domain:        pulumi.String(*cfg.Domain.DomainName),
                UserPoolId:    userPool.ID(),
                CertificateArn: pulumi.StringPtr(*cfg.Domain.CertificateArn),
            }, opts)
            if err != nil {
                return nil, nil, err
            }
            domainOut = d.Domain
        } else {
            prefix := fmt.Sprintf("%s-%s-tenant", ctx.Stack(), name)
            // sanitize to match Cognito hosted domain requirements
            prefix = strings.ToLower(prefix)
            re := regexp.MustCompile(`[^a-z0-9-]`)
            prefix = re.ReplaceAllString(prefix, "-")
            if len(prefix) > 63 { prefix = prefix[:63] }
            d, err := awscognito.NewUserPoolDomain(ctx, fmt.Sprintf("%s-domain", name), &awscognito.UserPoolDomainArgs{
                Domain:     pulumi.String(prefix),
                UserPoolId: userPool.ID(),
            }, opts)
            if err != nil {
                return nil, nil, err
            }
            // Compose full hosted domain
            domainOut = d.Domain.ApplyT(func(p string) (string, error) { return fmt.Sprintf("%s.auth.%s.amazoncognito.com", p, region.Name), nil }).(pulumi.StringOutput)
        }
    }

    // Create one or more user pool clients (at least one is required to bind as VP identity source)
    clientNames := cfg.Clients
    if len(clientNames) == 0 {
        clientNames = []string{"default"}
    }
    clientIds := []pulumi.StringInput{}
    for _, cn := range clientNames {
        c, err := awscognito.NewUserPoolClient(ctx, fmt.Sprintf("%s-%s-client", name, cn), &awscognito.UserPoolClientArgs{
            Name:       pulumi.String(fmt.Sprintf("%s-%s", name, cn)),
            UserPoolId: userPool.ID(),
            // sensible defaults, can be expanded later via inputs
            PreventUserExistenceErrors: pulumi.StringPtr("ENABLED"),
            GenerateSecret:             pulumi.BoolPtr(false),
        }, opts)
        if err != nil {
            return nil, nil, err
        }
        clientIds = append(clientIds, c.ID())
    }

    // Identity Source referencing Cognito
    _, err = awsvp.NewIdentitySource(ctx, fmt.Sprintf("%s-id-src", name), &awsvp.IdentitySourceArgs{
        PolicyStoreId: store.ID(),
        Configuration: &awsvp.IdentitySourceConfigurationArgs{
            CognitoUserPoolConfiguration: &awsvp.IdentitySourceConfigurationCognitoUserPoolConfigurationArgs{
                UserPoolArn: userPool.Arn,
                ClientIds:   pulumi.StringArray(clientIds),
            },
        },
    }, opts)
    if err != nil {
        return nil, nil, err
    }

    // Optional: Identity Pool + roles
    var idpOutputs map[string]any
    if cfg.IdentityPoolFederation != nil && *cfg.IdentityPoolFederation {
        idpOutputs, err = provisionIdentityPool(ctx, name, userPool, clientIds, opts)
        if err != nil {
            return nil, nil, err
        }
    }

    // Collect outputs
    outs := map[string]any{
        "userPoolId":  userPool.ID(),
        "userPoolArn": userPool.Arn,
        "parameters":  pulumi.StringMap{ "USER_POOL_ID": userPool.ID().ToStringOutput() },
    }
    if domainOut != (pulumi.StringOutput{}) {
        outs["userPoolDomain"] = domainOut
    }
    // Convert clientIds []StringInput -> []string output using Apply
    outs["userPoolClientIds"] = pulumi.All(clientIds...).ApplyT(func(vals []interface{}) []string {
        res := make([]string, 0, len(vals))
        for _, v := range vals {
            if s, ok := v.(string); ok {
                res = append(res, s)
            }
        }
        return res
    })

    return outs, idpOutputs, nil
}

// ensureSmsRole creates an IAM role usable by Cognito to publish SMS via SNS.
func ensureSmsRole(ctx *pulumi.Context, name string, opts pulumi.ResourceOption) (*awsiam.Role, string, error) {
    extId := fmt.Sprintf("%s-sms-%s", name, ctx.Stack())
    role, err := awsiam.NewRole(ctx, fmt.Sprintf("%s-sms-role", name), &awsiam.RoleArgs{
        AssumeRolePolicy: awsiam.GetPolicyDocumentOutput(ctx, awsiam.GetPolicyDocumentOutputArgs{
            Statements: awsiam.GetPolicyDocumentStatementArray{
                awsiam.GetPolicyDocumentStatementArgs{
                    Actions: pulumi.StringArray{pulumi.String("sts:AssumeRole")},
                    Principals: awsiam.GetPolicyDocumentStatementPrincipalArray{
                        awsiam.GetPolicyDocumentStatementPrincipalArgs{
                            Type:        pulumi.String("Service"),
                            Identifiers: pulumi.StringArray{pulumi.String("cognito-idp.amazonaws.com")},
                        },
                    },
                    Conditions: awsiam.GetPolicyDocumentStatementConditionArray{
                        awsiam.GetPolicyDocumentStatementConditionArgs{
                            Test:     pulumi.String("StringEquals"),
                            Variable: pulumi.String("sts:ExternalId"),
                            Values:   pulumi.StringArray{pulumi.String(extId)},
                        },
                    },
                },
            },
        }).Json(),
        Description: pulumi.StringPtr("Cognito SMS publishing role"),
    }, opts)
    if err != nil {
        return nil, "", err
    }
    // Allow publishing SMS via SNS
    _, err = awsiam.NewRolePolicy(ctx, fmt.Sprintf("%s-sms-pol", name), &awsiam.RolePolicyArgs{
        Role: role.Name,
        Policy: pulumi.String(`{
  "Version": "2012-10-17",
  "Statement": [
    { "Effect": "Allow", "Action": ["sns:Publish"], "Resource": "*" }
  ]
}`),
    }, opts)
    if err != nil {
        return nil, "", err
    }
    return role, extId, nil
}

// newCognitoTriggerLambda creates a minimal Node.js22.x Lambda for a Cognito trigger and applies custom permissions.
func newCognitoTriggerLambda(ctx *pulumi.Context, name string, cfg TriggerConfig, opts pulumi.ResourceOption) (*awslambda.Function, error) {
    // IAM role per trigger
    role, err := awsiam.NewRole(ctx, fmt.Sprintf("%s-role", name), &awsiam.RoleArgs{
        AssumeRolePolicy: awsiam.GetPolicyDocumentOutput(ctx, awsiam.GetPolicyDocumentOutputArgs{
            Statements: awsiam.GetPolicyDocumentStatementArray{
                awsiam.GetPolicyDocumentStatementArgs{
                    Actions: pulumi.StringArray{pulumi.String("sts:AssumeRole")},
                    Principals: awsiam.GetPolicyDocumentStatementPrincipalArray{
                        awsiam.GetPolicyDocumentStatementPrincipalArgs{
                            Type:        pulumi.String("Service"),
                            Identifiers: pulumi.StringArray{pulumi.String("lambda.amazonaws.com")},
                        },
                    },
                },
            },
        }).Json(),
        Description: pulumi.StringPtr("Role for Cognito lifecycle trigger"),
    }, opts)
    if err != nil {
        return nil, err
    }
    // Basic logs
    _, err = awsiam.NewRolePolicyAttachment(ctx, fmt.Sprintf("%s-logs", name), &awsiam.RolePolicyAttachmentArgs{
        Role:      role.Name,
        PolicyArn: pulumi.String(awsiam.ManagedPolicyAWSLambdaBasicExecutionRole),
    }, opts)
    if err != nil {
        return nil, err
    }
    // Custom permissions (coarse-grained; user provided actions, resource *)
    if len(cfg.Permissions) > 0 {
        polDoc := "{\n  \"Version\": \"2012-10-17\",\n  \"Statement\": [{ \"Effect\": \"Allow\", \"Action\": ["
        for i, a := range cfg.Permissions {
            if i > 0 { polDoc += "," }
            polDoc += fmt.Sprintf("\"%s\"", a)
        }
        polDoc += "], \"Resource\": \"*\" }]\n}"
        _, err = awsiam.NewRolePolicy(ctx, fmt.Sprintf("%s-extra", name), &awsiam.RolePolicyArgs{
            Role:   role.Name,
            Policy: pulumi.String(polDoc),
        }, opts)
        if err != nil {
            return nil, err
        }
    }
    // Lambda code: embedded minimal stub that echoes/permits success for common triggers
    code := pulumi.NewAssetArchive(map[string]pulumi.AssetOrArchive{
        "index.mjs": pulumi.NewStringAsset(cognitoTriggerStub),
    })
    handler := "index.handler" // ignore cfg.Handler until external code assets are supported
    fn, err := awslambda.NewFunction(ctx, fmt.Sprintf("%s-fn", name), &awslambda.FunctionArgs{
        Role:         role.Arn,
        Runtime:      pulumi.String("nodejs22.x"),
        Handler:      pulumi.String(handler),
        Code:         code,
        Environment:  &awslambda.FunctionEnvironmentArgs{ Variables: stringMap(cfg.Environment) },
        Architectures: pulumi.StringArray{pulumi.String("arm64")},
        Timeout:      pulumi.Int(10),
    }, opts)
    if err != nil {
        return nil, err
    }
    // Log group for trigger
    _, _ = awscloudwatch.NewLogGroup(ctx, fmt.Sprintf("%s-lg", name), &awscloudwatch.LogGroupArgs{
        Name: fn.Name.ApplyT(func(n string) (string, error) { return "/aws/lambda/" + n, nil }).(pulumi.StringOutput),
        RetentionInDays: pulumi.IntPtr(14),
    }, opts)
    return fn, nil
}

func stringMap(m map[string]string) pulumi.StringMapInput {
    out := pulumi.StringMap{}
    for k, v := range m {
        out[k] = pulumi.String(v)
    }
    return out
}

// provisionIdentityPool creates a Cognito Identity Pool with default authenticated/unauthenticated roles.
func provisionIdentityPool(
    ctx *pulumi.Context,
    name string,
    userPool *awscognito.UserPool,
    clientIds []pulumi.StringInput,
    opts pulumi.ResourceOption,
) (map[string]any, error) {
    region, _ := aws.GetRegion(ctx, &aws.GetRegionArgs{}, nil)
    // Build provider name of user pool for identity pool mapping
    providerName := pulumi.Sprintf("cognito-idp.%s.amazonaws.com/%s", region.Name, userPool.ID())
    ip, err := awscognito.NewIdentityPool(ctx, fmt.Sprintf("%s-idp", name), &awscognito.IdentityPoolArgs{
        IdentityPoolName:               pulumi.String(fmt.Sprintf("%s-identity-pool", name)),
        AllowUnauthenticatedIdentities: pulumi.Bool(false),
        CognitoIdentityProviders: awscognito.IdentityPoolCognitoIdentityProviderArray{
            &awscognito.IdentityPoolCognitoIdentityProviderArgs{
                ClientId:     pulumi.ToOutput(clientIds[0]).(pulumi.StringOutput).ToStringPtrOutput(),
                ProviderName: providerName,
            },
        },
    }, opts)
    if err != nil {
        return nil, err
    }

    // Authenticated role
    authRole, err := awsiam.NewRole(ctx, fmt.Sprintf("%s-auth-role", name), &awsiam.RoleArgs{
        AssumeRolePolicy: awsiam.GetPolicyDocumentOutput(ctx, awsiam.GetPolicyDocumentOutputArgs{
            Statements: awsiam.GetPolicyDocumentStatementArray{
                awsiam.GetPolicyDocumentStatementArgs{
                    Actions: pulumi.StringArray{pulumi.String("sts:AssumeRole")},
                    Principals: awsiam.GetPolicyDocumentStatementPrincipalArray{
                        awsiam.GetPolicyDocumentStatementPrincipalArgs{
                            Type:        pulumi.String("Federated"),
                            Identifiers: pulumi.StringArray{pulumi.String("cognito-identity.amazonaws.com")},
                        },
                    },
                    Conditions: awsiam.GetPolicyDocumentStatementConditionArray{
                        awsiam.GetPolicyDocumentStatementConditionArgs{
                            Test:     pulumi.String("StringEquals"),
                            Variable: pulumi.String("cognito-identity.amazonaws.com:aud"),
                            Values:   pulumi.StringArray{ip.ID().ToStringOutput()},
                        },
                        awsiam.GetPolicyDocumentStatementConditionArgs{
                            Test:     pulumi.String("ForAnyValue:StringLike"),
                            Variable: pulumi.String("cognito-identity.amazonaws.com:amr"),
                            Values:   pulumi.StringArray{pulumi.String("authenticated")},
                        },
                    },
                },
            },
        }).Json(),
    }, opts)
    if err != nil {
        return nil, err
    }
    // Unauthenticated role
    unauthRole, err := awsiam.NewRole(ctx, fmt.Sprintf("%s-unauth-role", name), &awsiam.RoleArgs{
        AssumeRolePolicy: awsiam.GetPolicyDocumentOutput(ctx, awsiam.GetPolicyDocumentOutputArgs{
            Statements: awsiam.GetPolicyDocumentStatementArray{
                awsiam.GetPolicyDocumentStatementArgs{
                    Actions: pulumi.StringArray{pulumi.String("sts:AssumeRole")},
                    Principals: awsiam.GetPolicyDocumentStatementPrincipalArray{
                        awsiam.GetPolicyDocumentStatementPrincipalArgs{
                            Type:        pulumi.String("Federated"),
                            Identifiers: pulumi.StringArray{pulumi.String("cognito-identity.amazonaws.com")},
                        },
                    },
                    Conditions: awsiam.GetPolicyDocumentStatementConditionArray{
                        awsiam.GetPolicyDocumentStatementConditionArgs{
                            Test:     pulumi.String("StringEquals"),
                            Variable: pulumi.String("cognito-identity.amazonaws.com:aud"),
                            Values:   pulumi.StringArray{ip.ID().ToStringOutput()},
                        },
                        awsiam.GetPolicyDocumentStatementConditionArgs{
                            Test:     pulumi.String("ForAnyValue:StringLike"),
                            Variable: pulumi.String("cognito-identity.amazonaws.com:amr"),
                            Values:   pulumi.StringArray{pulumi.String("unauthenticated")},
                        },
                    },
                },
            },
        }).Json(),
    }, opts)
    if err != nil {
        return nil, err
    }

    // Attach roles to identity pool
    _, err = awscognito.NewIdentityPoolRoleAttachment(ctx, fmt.Sprintf("%s-roles", name), &awscognito.IdentityPoolRoleAttachmentArgs{
        IdentityPoolId: ip.ID(),
        Roles: pulumi.StringMap{
            "authenticated":   authRole.Arn,
            "unauthenticated": unauthRole.Arn,
        },
    }, opts)
    if err != nil {
        return nil, err
    }

    return map[string]any{
        "identityPoolId": ip.ID(),
        "authRoleArn":    authRole.Arn,
        "unauthRoleArn":  unauthRole.Arn,
    }, nil
}

func contains(s, substr string) bool { return strings.Contains(s, substr) }
