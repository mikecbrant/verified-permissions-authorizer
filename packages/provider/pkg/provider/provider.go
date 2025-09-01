package provider

import (
    "embed"
    "encoding/json"
    "fmt"
    "net/mail"

    aws "github.com/pulumi/pulumi-aws/sdk/v6/go/aws"
    awscognito "github.com/pulumi/pulumi-aws/sdk/v6/go/aws/cognito"
    awscloudwatch "github.com/pulumi/pulumi-aws/sdk/v6/go/aws/cloudwatch"
    awsdynamodb "github.com/pulumi/pulumi-aws/sdk/v6/go/aws/dynamodb"
    awsiam "github.com/pulumi/pulumi-aws/sdk/v6/go/aws/iam"
    awslambda "github.com/pulumi/pulumi-aws/sdk/v6/go/aws/lambda"
    awssesv2 "github.com/pulumi/pulumi-aws/sdk/v6/go/aws/sesv2"
    awsvp "github.com/pulumi/pulumi-aws/sdk/v6/go/aws/verifiedpermissions"
    p "github.com/pulumi/pulumi-go-provider"
    "github.com/pulumi/pulumi-go-provider/infer"
    "github.com/pulumi/pulumi/sdk/v3/go/pulumi"
)

//go:embed assets/index.mjs
var authorizerIndexMjs string

// Note: The provider also includes a minimal Cognito trigger stub under
// packages/provider/assets/cognito-trigger-stub.mjs for future use.

// NewProvider wires up the multi-language component provider surface.
func NewProvider() (p.Provider, error) {
    return infer.NewProvider(infer.Options{
        Components: []infer.InferredComponent{
            infer.Component(NewAuthorizerWithPolicyStore),
        },
    })
}

// AuthorizerArgs defines the inputs for the component resource.
type AuthorizerArgs struct {
    // Policy store description
    Description *string `pulumi:"description,optional"`
    // When true, resources are retained on delete and protected from deletion (where supported).
    RetainOnDelete *bool `pulumi:"retainOnDelete,optional"`
    // DynamoDB-related options for the provider-managed auth table.
    Dynamo *DynamoConfig `pulumi:"dynamo,optional"`
    // Settings for the bundled Lambda authorizer
    Lambda *LambdaConfig `pulumi:"lambda,optional"`
    // Optional Cognito configuration. When provided, a Cognito User Pool will be provisioned
    // and configured as the Verified Permissions Identity Source for the created policy store.
    Cognito *CognitoConfig `pulumi:"cognito,optional"`
}

// LambdaConfig exposes a narrow set of tuning knobs for the Lambda authorizer.
type LambdaConfig struct {
    MemorySize             *int `pulumi:"memorySize,optional"`
    ReservedConcurrency    *int `pulumi:"reservedConcurrency,optional"`
    ProvisionedConcurrency *int `pulumi:"provisionedConcurrency,optional"`
}

// DynamoConfig groups DynamoDB table-related provider options.
type DynamoConfig struct {
    // If true, enable DynamoDB Streams on the auth table (NEW_AND_OLD_IMAGES).
    EnableDynamoDbStream *bool `pulumi:"enableDynamoDbStream,optional"`
}

// AuthorizerWithPolicyStore is the component implementing the Construct.
// It exposes the created resource ARNs as outputs.
type AuthorizerWithPolicyStore struct {
    pulumi.ResourceState

    PolicyStoreId  pulumi.StringOutput `pulumi:"policyStoreId"`
    PolicyStoreArn pulumi.StringOutput `pulumi:"policyStoreArn"`
    AuthorizerFunctionArn pulumi.StringOutput `pulumi:"authorizerFunctionArn"`
    RoleArn        pulumi.StringOutput `pulumi:"roleArn"`
    // DynamoDB table outputs (exported with PascalCase to match schema/docs)
    AuthTableArn       pulumi.StringOutput    `pulumi:"AuthTableArn"`
    AuthTableStreamArn pulumi.StringPtrOutput `pulumi:"AuthTableStreamArn,optional"`

    // Optional Cognito-related outputs
    UserPoolId        pulumi.StringPtrOutput   `pulumi:"userPoolId,optional"`
    UserPoolArn       pulumi.StringPtrOutput   `pulumi:"userPoolArn,optional"`
    UserPoolDomain    pulumi.StringPtrOutput   `pulumi:"userPoolDomain,optional"`
    IdentityPoolId    pulumi.StringPtrOutput   `pulumi:"identityPoolId,optional"`
    AuthRoleArn       pulumi.StringPtrOutput   `pulumi:"authRoleArn,optional"`
    UnauthRoleArn     pulumi.StringPtrOutput   `pulumi:"unauthRoleArn,optional"`
    UserPoolClientIds pulumi.StringArrayOutput `pulumi:"userPoolClientIds,optional"`
    Parameters        pulumi.StringMapOutput   `pulumi:"parameters,optional"`
}

func (c *AuthorizerWithPolicyStore) Annotate(a infer.Annotator) {
    a.Describe(&c, "Provision an AWS Verified Permissions Policy Store and a bundled Lambda Request Authorizer.")
    a.Token(&c, "verified-permissions-authorizer:index:AuthorizerWithPolicyStore")
}

// NewAuthorizerWithPolicyStore is the component constructor used by infer.Component.
func NewAuthorizerWithPolicyStore(
    ctx *pulumi.Context,
    name string,
    args AuthorizerArgs,
    opts ...pulumi.ResourceOption,
) (*AuthorizerWithPolicyStore, error) {
    comp := &AuthorizerWithPolicyStore{}
    if err := ctx.RegisterComponentResource("verified-permissions-authorizer:index:AuthorizerWithPolicyStore", name, comp, opts...); err != nil {
        return nil, err
    }

    // Defaults for provider-level options
    if args.RetainOnDelete == nil {
        b := false
        args.RetainOnDelete = &b
    }
    // normalize nested config pointers
    if args.Dynamo == nil {
        args.Dynamo = &DynamoConfig{}
    }

    // 1) Verified Permissions Policy Store
    storeArgs := &awsvp.PolicyStoreArgs{
        ValidationSettings: awsvp.PolicyStoreValidationSettingsArgs{
            Mode: pulumi.String("STRICT"),
        },
    }
    if args.Description != nil {
        storeArgs.Description = pulumi.StringPtr(*args.Description)
    }
    childOpts := append(opts, pulumi.Parent(comp))
    // Apply RetainOnDelete to all child resources when retention is enabled
    retOpts := pulumi.MergeResourceOptions(childOpts...)
    if *args.RetainOnDelete {
        retOpts = pulumi.MergeResourceOptions(retOpts, pulumi.RetainOnDelete(true))
    }
    store, err := awsvp.NewPolicyStore(ctx, fmt.Sprintf("%s-store", name), storeArgs, retOpts)
    if err != nil {
        return nil, err
    }

    // 1b) DynamoDB single-table for auth/identity/roles data
    // Always parent to the component; retain on delete only when retention is enabled
    tableOpt := retOpts

    // Build base table args
    targs := &awsdynamodb.TableArgs{
        BillingMode: pulumi.String("PAY_PER_REQUEST"),
        // Only attributes participating in the primary index or GSIs may be declared here.
        Attributes: awsdynamodb.TableAttributeArray{
            awsdynamodb.TableAttributeArgs{ Name: pulumi.String("PK"), Type: pulumi.String("S") },
            awsdynamodb.TableAttributeArgs{ Name: pulumi.String("SK"), Type: pulumi.String("S") },
            awsdynamodb.TableAttributeArgs{ Name: pulumi.String("GSI1PK"), Type: pulumi.String("S") },
            awsdynamodb.TableAttributeArgs{ Name: pulumi.String("GSI1SK"), Type: pulumi.String("S") },
            // GSI2 attributes
            awsdynamodb.TableAttributeArgs{ Name: pulumi.String("GSI2PK"), Type: pulumi.String("S") },
            awsdynamodb.TableAttributeArgs{ Name: pulumi.String("GSI2SK"), Type: pulumi.String("S") },
        },
        HashKey:  pulumi.String("PK"),
        RangeKey: pulumi.StringPtr("SK"),
        GlobalSecondaryIndexes: awsdynamodb.TableGlobalSecondaryIndexArray{
            awsdynamodb.TableGlobalSecondaryIndexArgs{
                Name:           pulumi.String("GSI1"),
                HashKey:        pulumi.String("GSI1PK"),
                RangeKey:       pulumi.StringPtr("GSI1SK"),
                ProjectionType: pulumi.StringPtr("ALL"),
            },
            awsdynamodb.TableGlobalSecondaryIndexArgs{
                Name:           pulumi.String("GSI2"),
                HashKey:        pulumi.String("GSI2PK"),
                RangeKey:       pulumi.StringPtr("GSI2SK"),
                ProjectionType: pulumi.StringPtr("ALL"),
            },
        },
    }

    // When retention is enabled, turn on deletion protection and PITR
    if *args.RetainOnDelete {
        targs.DeletionProtectionEnabled = pulumi.BoolPtr(true)
        targs.PointInTimeRecovery = &awsdynamodb.TablePointInTimeRecoveryArgs{ Enabled: pulumi.Bool(true) }
    }

    // Streams optional
    enableStream := false
    if args.Dynamo != nil && args.Dynamo.EnableDynamoDbStream != nil {
        enableStream = *args.Dynamo.EnableDynamoDbStream
    }
    if enableStream {
        targs.StreamEnabled = pulumi.BoolPtr(true)
        targs.StreamViewType = pulumi.StringPtr("NEW_AND_OLD_IMAGES")
    }

    table, err := awsdynamodb.NewTable(ctx, fmt.Sprintf("%s-tenant", name), targs, tableOpt)
    if err != nil {
        return nil, err
    }

    // 2) IAM Role
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
        Description: pulumi.StringPtr("Role for Verified Permissions Lambda Authorizer"),
    }, retOpts)
    if err != nil {
        return nil, err
    }

    // Basic logs policy
    if _, err = awsiam.NewRolePolicyAttachment(ctx, fmt.Sprintf("%s-logs", name), &awsiam.RolePolicyAttachmentArgs{
        Role:      role.Name,
        PolicyArn: pulumi.String(awsiam.ManagedPolicyAWSLambdaBasicExecutionRole),
    }, retOpts); err != nil {
        return nil, err
    }

    // Verified Permissions access policy: GetPolicyStore + IsAuthorized (scoped to this policy store)
    vpDoc := awsiam.GetPolicyDocumentOutput(ctx, awsiam.GetPolicyDocumentOutputArgs{
        Statements: awsiam.GetPolicyDocumentStatementArray{
            awsiam.GetPolicyDocumentStatementArgs{
                Effect:    pulumi.StringPtr("Allow"),
                Actions:   pulumi.StringArray{pulumi.String("verifiedpermissions:GetPolicyStore"), pulumi.String("verifiedpermissions:IsAuthorized")},
                Resources: pulumi.StringArray{store.Arn},
            },
        },
    })
    vpPol, err := awsiam.NewPolicy(ctx, fmt.Sprintf("%s-vp", name), &awsiam.PolicyArgs{
        Policy: vpDoc.Json(),
    }, retOpts)
    if err != nil {
        return nil, err
    }
    if _, err = awsiam.NewRolePolicyAttachment(ctx, fmt.Sprintf("%s-vp-attach", name), &awsiam.RolePolicyAttachmentArgs{
        Role:      role.Name,
        PolicyArn: vpPol.Arn,
    }, retOpts); err != nil {
        return nil, err
    }

    // Grant the Lambda role read-only access to the provider-managed DynamoDB table
    // (table ARN and all index ARNs). Actions intentionally exclude any write or
    // streams consumer permissions.
    ddbReadDoc := awsiam.GetPolicyDocumentOutput(ctx, awsiam.GetPolicyDocumentOutputArgs{
        Statements: awsiam.GetPolicyDocumentStatementArray{
            // Table-only actions
            awsiam.GetPolicyDocumentStatementArgs{
                Effect: pulumi.StringPtr("Allow"),
                Actions: pulumi.StringArray{
                    pulumi.String("dynamodb:GetItem"),
                    pulumi.String("dynamodb:BatchGetItem"),
                    pulumi.String("dynamodb:DescribeTable"),
                },
                Resources: pulumi.StringArray{
                    table.Arn,
                },
            },
            // Actions that may target the table or its GSIs
            awsiam.GetPolicyDocumentStatementArgs{
                Effect: pulumi.StringPtr("Allow"),
                Actions: pulumi.StringArray{
                    pulumi.String("dynamodb:Query"),
                    pulumi.String("dynamodb:Scan"),
                },
                Resources: pulumi.StringArray{
                    table.Arn,
                    pulumi.Sprintf("%s/index/*", table.Arn),
                },
            },
        },
    })

    if _, err := awsiam.NewRolePolicy(ctx, fmt.Sprintf("%s-ddb-read", name), &awsiam.RolePolicyArgs{
        Role:   role.Name,
        Policy: ddbReadDoc.Json(),
    }, retOpts); err != nil {
        return nil, err
    }

    // 3) Lambda code: embed built authorizer
    code := pulumi.NewAssetArchive(map[string]pulumi.AssetOrArchive{
        "index.mjs": pulumi.NewStringAsset(authorizerIndexMjs),
    })

    // Defaults for authorizer Lambda config
    if args.Lambda == nil {
        args.Lambda = &LambdaConfig{}
    }
    mem := 128
    if args.Lambda.MemorySize != nil {
        mem = *args.Lambda.MemorySize
    }
    rc := 1
    if args.Lambda.ReservedConcurrency != nil {
        rc = *args.Lambda.ReservedConcurrency
    }
    pc := 0
    if args.Lambda.ProvisionedConcurrency != nil {
        pc = *args.Lambda.ProvisionedConcurrency
    }
    // Guard: when provisioned concurrency is enabled, ensure it does not exceed reserved concurrency
    if pc > 0 && rc < pc {
        return nil, fmt.Errorf("lambda.provisionedConcurrency (%d) must be less than or equal to reservedConcurrency (%d)", pc, rc)
    }

    fn, err := awslambda.NewFunction(ctx, fmt.Sprintf("%s-authorizer", name), &awslambda.FunctionArgs{
        Role:    role.Arn,
        Runtime: pulumi.String("nodejs22.x"), // fixed; not configurable
        Handler: pulumi.String("index.handler"),
        Code:    code,
        Environment: &awslambda.FunctionEnvironmentArgs{
            Variables: pulumi.StringMap(map[string]pulumi.StringInput{
                "POLICY_STORE_ID": store.ID().ToStringOutput(),
            }),
        },
        Architectures: pulumi.StringArray{pulumi.String("arm64")},
        Timeout:       pulumi.Int(10),
        MemorySize:    pulumi.Int(mem),
        ReservedConcurrentExecutions: pulumi.Int(rc),
    }, retOpts)
    if err != nil {
        return nil, err
    }

    // Optional provisioned concurrency (disabled by default when pc == 0)
    if pc > 0 {
        // Create a version and alias, then attach provisioned concurrency to the alias
        ver, err := awslambda.NewVersion(ctx, fmt.Sprintf("%s-authorizer-v", name), &awslambda.VersionArgs{
            FunctionName: fn.Name,
        }, retOpts)
        if err != nil {
            return nil, err
        }
        alias, err := awslambda.NewAlias(ctx, fmt.Sprintf("%s-authorizer-live", name), &awslambda.AliasArgs{
            Name:            pulumi.String("live"),
            FunctionName:    fn.Name,
            FunctionVersion: ver.Version,
        }, retOpts)
        if err != nil {
            return nil, err
        }
        _, err = awslambda.NewProvisionedConcurrencyConfig(ctx, fmt.Sprintf("%s-authorizer-pc", name), &awslambda.ProvisionedConcurrencyConfigArgs{
            FunctionName:                    fn.Name,
            Qualifier:                        alias.Name,
            ProvisionedConcurrentExecutions: pulumi.Int(pc),
        }, retOpts)
        if err != nil {
            return nil, err
        }
    }

    // 4) Log group
    if _, err = awscloudwatch.NewLogGroup(ctx, fmt.Sprintf("%s-lg", name), &awscloudwatch.LogGroupArgs{
        Name:            fn.Name.ApplyT(func(n string) string { return "/aws/lambda/" + n }).(pulumi.StringOutput).ToStringPtrOutput(),
        RetentionInDays: pulumi.IntPtr(14),
    }, retOpts); err != nil {
        return nil, err
    }

    // Wire base outputs
    comp.PolicyStoreId = store.ID().ToStringOutput()
    comp.PolicyStoreArn = store.Arn
    comp.AuthorizerFunctionArn = fn.Arn
    comp.RoleArn = role.Arn
    // StreamArn is only non-nil when streams are enabled on the table
    comp.AuthTableArn = table.Arn
    comp.AuthTableStreamArn = table.StreamArn

    // 5) Optional Cognito provisioning + Verified Permissions identity source
    if args.Cognito != nil {
        cog, err := provisionCognito(ctx, name, store, *args.Cognito, *args.RetainOnDelete, retOpts)
        if err != nil {
            return nil, err
        }
        comp.UserPoolId = cog.UserPoolId.ToStringPtrOutput()
        comp.UserPoolArn = cog.UserPoolArn.ToStringPtrOutput()
        comp.UserPoolClientIds = cog.ClientIds
        comp.Parameters = cog.Parameters
    }

    return comp, nil
}

// withRetention augments resource options with RetainOnDelete when retain==true.
func withRetention(opts pulumi.ResourceOption, retain bool) pulumi.ResourceOption {
    if retain {
        return pulumi.Merge(opts, pulumi.RetainOnDelete(true))
    }
    return opts
}

// ---- Cognito configuration types ----

type CognitoConfig struct {
    // Allowed sign-in aliases; defaults to ["email"]. Allowed values: email, phone, preferredUsername.
    SignInAliases []string `pulumi:"signInAliases,optional"`
    // Optional Amazon SES configuration for Cognito User Pool email sending.
    SesConfig *CognitoSesConfig `pulumi:"sesConfig,optional"`
}

type cognitoProvisionResult struct {
    UserPoolId  pulumi.StringOutput
    UserPoolArn pulumi.StringOutput
    ClientIds   pulumi.StringArrayOutput
    Parameters  pulumi.StringMapOutput
}

// provisionCognito provisions a Cognito User Pool (and optional Identity Pool) and configures it
// as the Identity Source for the given Verified Permissions policy store.
func provisionCognito(
    ctx *pulumi.Context,
    name string,
    store *awsvp.PolicyStore,
    cfg CognitoConfig,
    retainOnDelete bool,
    opts pulumi.ResourceOption,
) (*cognitoProvisionResult, error) {
    // Construct minimal Cognito user pool args
    upArgs := &awscognito.UserPoolArgs{
        Name: pulumi.String(fmt.Sprintf("%s-up", name)),
        UsernameConfiguration: &awscognito.UserPoolUsernameConfigurationArgs{
            CaseSensitive: pulumi.Bool(false),
        },
        DeletionProtection: pulumi.String(func() string {
            if retainOnDelete {
                return "ACTIVE"
            }
            return "INACTIVE"
        }()),
    }
    // Map sign-in aliases (default to email when none provided)
    aliases := cfg.SignInAliases
    if len(aliases) == 0 {
        aliases = []string{"email"}
    }
    aliasAttrs := []pulumi.StringInput{}
    for _, a := range aliases {
        switch a {
        case "email":
            aliasAttrs = append(aliasAttrs, pulumi.String("email"))
        case "phone":
            aliasAttrs = append(aliasAttrs, pulumi.String("phone_number"))
        case "preferredUsername":
            aliasAttrs = append(aliasAttrs, pulumi.String("preferred_username"))
        }
    }
    if len(aliasAttrs) > 0 {
        upArgs.AliasAttributes = pulumi.StringArray(aliasAttrs)
    }

    // Optional: SES-backed email sending
    var (
        regionName      string
        sesIdentityName string
        sesCallerAcct   string
        fromEmail       string
        sesIdentityRegion string
    )
    if cfg.SesConfig != nil {
        // Look up current AWS region and caller (for validation + policy construction)
        region, err := aws.GetRegion(ctx, nil)
        if err != nil {
            return nil, fmt.Errorf("failed to get AWS region: %w", err)
        }
        regionName = region.Name

        caller, err := aws.GetCallerIdentity(ctx, nil)
        if err != nil {
            return nil, fmt.Errorf("failed to get AWS caller identity: %w", err)
        }
        sesCallerAcct = caller.AccountId

        // Validate config + extract SES identity account/name/region
        acc, ident, identRegion, err := validateSesConfig(*cfg.SesConfig, regionName)
        if err != nil {
            return nil, err
        }
        sesIdentityName = ident
        sesIdentityRegion = identRegion
        if acc != sesCallerAcct {
            return nil, fmt.Errorf("cognito.sesConfig.sourceArn account (%s) must match the current AWS account (%s); cross-account identities are not supported", acc, sesCallerAcct)
        }
        if addr, err := mail.ParseAddress(cfg.SesConfig.From); err == nil && addr.Address != "" {
            fromEmail = addr.Address
        } else {
            fromEmail = cfg.SesConfig.From
        }

        // Create a region-scoped AWS provider for SES (when identity region differs)
        _ = identRegion // currently unused outside resource option; keep for clarity

        // Configure the user pool to use SES (DEVELOPER) with provided values
        upArgs.EmailConfiguration = &awscognito.UserPoolEmailConfigurationArgs{
            EmailSendingAccount: pulumi.String("DEVELOPER"),
            SourceArn:           pulumi.StringPtr(cfg.SesConfig.SourceArn),
            From:                pulumi.StringPtr(cfg.SesConfig.From),
        }
        if cfg.SesConfig.ReplyToEmail != nil && *cfg.SesConfig.ReplyToEmail != "" {
            upArgs.EmailConfiguration.ReplyToEmailAddress = pulumi.StringPtr(*cfg.SesConfig.ReplyToEmail)
        }
        if cfg.SesConfig.ConfigurationSet != nil && *cfg.SesConfig.ConfigurationSet != "" {
            upArgs.EmailConfiguration.ConfigurationSet = pulumi.StringPtr(*cfg.SesConfig.ConfigurationSet)
        }
    }

    userPool, err := awscognito.NewUserPool(ctx, fmt.Sprintf("%s-userpool", name), upArgs, opts)
    if err != nil {
        return nil, err
    }

    // If SES config was provided, attach the SES identity policy now that we have a pool ID
    if cfg.SesConfig != nil {
        // Build policy JSON using user pool ID -> ARN with proper JSON escaping
        policy := userPool.ID().ApplyT(func(id string) (string, error) {
            upArn := fmt.Sprintf("arn:%s:cognito-idp:%s:%s:userpool/%s", partitionForRegion(regionName), regionName, sesCallerAcct, id)
            principalService := "cognito-idp.amazonaws.com"
            if partitionForRegion(regionName) == "aws-cn" {
                principalService = "cognito-idp.amazonaws.com.cn"
            }
            cond := map[string]any{
                "aws:SourceAccount": sesCallerAcct,
                "aws:SourceArn":     upArn,
            }
            if fromEmail != "" {
                cond["ses:FromAddress"] = fromEmail
            }
            doc := map[string]any{
                "Version": "2012-10-17",
                "Statement": []any{
                    map[string]any{
                        "Sid":      "AllowCognitoUserPool",
                        "Effect":   "Allow",
                        "Resource": cfg.SesConfig.SourceArn,
                        "Principal": map[string]any{"Service": []string{principalService}},
                        "Action":   []string{"ses:SendEmail", "ses:SendRawEmail"},
                        "Condition": map[string]any{
                            "StringEquals": cond,
                        },
                    },
                },
            }
            b, err := json.Marshal(doc)
            if err != nil {
                return "", err
            }
            return string(b), nil
        }).(pulumi.StringOutput)

        // Create a region-scoped AWS provider for SES in the identity's region
        sesProv, err := aws.NewProvider(ctx, fmt.Sprintf("%s-ses-%s", name, sesIdentityRegion), &aws.ProviderArgs{Region: pulumi.String(sesIdentityRegion)})
        if err != nil {
            return nil, err
        }
        if _, err := awssesv2.NewEmailIdentityPolicy(ctx, fmt.Sprintf("%s-ses-identity-policy", name), &awssesv2.EmailIdentityPolicyArgs{
            EmailIdentity: pulumi.String(sesIdentityName),
            PolicyName:    pulumi.String(fmt.Sprintf("%s-cognito", name)),
            Policy:        policy,
        }, pulumi.Merge(withRetention(opts, retainOnDelete), pulumi.Provider(sesProv))); err != nil {
            return nil, err
        }
    }

    // Create one or more user pool clients (at least one is required to bind as VP identity source)
    clientNames := []string{"default"}
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
            return nil, err
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
        return nil, err
    }

    // Collect outputs (typed)
    res := &cognitoProvisionResult{
        UserPoolId:  userPool.ID().ToStringOutput(),
        UserPoolArn: userPool.Arn,
        ClientIds:   pulumi.ToStringArrayOutput(pulumi.StringArray(clientIds)),
        Parameters:  (pulumi.StringMap{"USER_POOL_ID": userPool.ID().ToStringOutput()}).ToStringMapOutput(),
    }
    return res, nil
}
// Note: Identity Pools, lifecycle triggers, templates, and other advanced Cognito
// options were intentionally removed from the public configuration surface. The
// provider creates a minimal User Pool + client and binds it as the VP identity source.

