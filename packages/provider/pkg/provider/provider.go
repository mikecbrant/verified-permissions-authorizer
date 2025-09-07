package provider

import (
    "context"
    "embed"
    "encoding/json"
    "errors"
    "fmt"
    "io/fs"
    "net/mail"
    "os"
    "path/filepath"
    "sort"
    "strings"

    awsv2 "github.com/aws/aws-sdk-go-v2/aws"
    awsconfig "github.com/aws/aws-sdk-go-v2/config"
    vpapi "github.com/aws/aws-sdk-go-v2/service/verifiedpermissions"
    vpapiTypes "github.com/aws/aws-sdk-go-v2/service/verifiedpermissions/types"

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
    "github.com/pulumi/pulumi/sdk/v3/go/common/tokens"
    "github.com/pulumi/pulumi/sdk/v3/go/pulumi"
    ds "github.com/bmatcuk/doublestar/v4"
    "gopkg.in/yaml.v3"
)

//go:embed assets/index.mjs
var authorizerIndexMjs string
// Ensure the embed import is considered used by tools that don't honor //go:embed during analysis.
var _ embed.FS

// Note: The provider also includes a minimal Cognito trigger stub under
// packages/provider/assets/cognito-trigger-stub.mjs for future use.

// NewProvider wires up the multi-language component provider surface.
func NewProvider() (p.Provider, error) {
    return infer.NewProviderBuilder().
        WithComponents(infer.ComponentF(NewAuthorizerWithPolicyStore)).
        Build()
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
    // Optional: AVP schema & policy asset ingestion and validation settings.
    // When provided, the provider will:
    //  - Load a Cedar JSON Schema from assets (YAML or JSON file)
    //  - Validate single-namespace and required entity presence
    //  - Enforce action-group naming/mapping conventions in warn/error/off mode
    //  - Apply the schema to the created Policy Store (skip when unchanged)
    //  - Ingest static Cedar policies from assets/policies (required guardrails enforced)
    //  - Optionally run post-deploy canary authorization checks
    AvpAssets *AvpAssetsConfig `pulumi:"avpAssets,optional"`
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

    // Top-level outputs
    PolicyStoreId  pulumi.StringOutput `pulumi:"policyStoreId"`
    PolicyStoreArn pulumi.StringOutput `pulumi:"policyStoreArn"`
    Parameters     pulumi.StringMapOutput `pulumi:"parameters,optional"`

    // Grouped outputs
    Cognito *CognitoOutputs `pulumi:"cognito,optional"`
    Dynamo  DynamoOutputs  `pulumi:"dynamo"`
    Lambda  LambdaOutputs  `pulumi:"lambda"`
}

// CognitoOutputs groups optional Cognito-related outputs under the `cognito` object.
type CognitoOutputs struct {
    UserPoolArn       pulumi.StringPtrOutput   `pulumi:"userPoolArn,optional"`
    UserPoolClientIds pulumi.StringArrayOutput `pulumi:"userPoolClientIds,optional"`
    UserPoolId        pulumi.StringPtrOutput   `pulumi:"userPoolId,optional"`
}

// DynamoOutputs groups DynamoDB auth table outputs under the `dynamo` object.
type DynamoOutputs struct {
    AuthTableArn       pulumi.StringOutput    `pulumi:"AuthTableArn"`
    AuthTableStreamArn pulumi.StringOutput `pulumi:"AuthTableStreamArn,optional"`
}

// LambdaOutputs groups Lambda authorizer outputs under the `lambda` object.
type LambdaOutputs struct {
    AuthorizerFunctionArn pulumi.StringOutput `pulumi:"authorizerFunctionArn"`
    RoleArn               pulumi.StringOutput `pulumi:"roleArn"`
}

func (c *AuthorizerWithPolicyStore) Annotate(a infer.Annotator) {
    a.Describe(&c, "Provision an AWS Verified Permissions Policy Store and a bundled Lambda Request Authorizer.")
    a.SetToken(tokens.ModuleName("verified-permissions-authorizer"), tokens.TypeName("AuthorizerWithPolicyStore"))
}

// NewAuthorizerWithPolicyStore is the component constructor used by infer.Component.
func NewAuthorizerWithPolicyStore(
    ctx *pulumi.Context,
    name string,
    args AuthorizerArgs,
    opts ...pulumi.ResourceOption,
) (*AuthorizerWithPolicyStore, error) {
    comp := &AuthorizerWithPolicyStore{}
    const authorizerType = "verified-permissions-authorizer:index:AuthorizerWithPolicyStore"
    if err := ctx.RegisterComponentResource(authorizerType, name, comp, opts...); err != nil {
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
    // Derive common child options and (optionally) add RetainOnDelete.
    childOpts := append([]pulumi.ResourceOption{}, opts...)
    childOpts = append(childOpts, pulumi.Parent(comp))
    retOpts := append([]pulumi.ResourceOption{}, childOpts...)
    if *args.RetainOnDelete {
        retOpts = append(retOpts, pulumi.RetainOnDelete(true))
    }
    store, err := awsvp.NewPolicyStore(ctx, fmt.Sprintf("%s-store", name), storeArgs, retOpts...)
    if err != nil {
        return nil, err
    }

    // 1a) Optional: Apply schema and ingest policies from assets
    if args.AvpAssets != nil {
        if err := applySchemaAndPolicies(ctx, name, store, *args.AvpAssets); err != nil {
            return nil, err
        }
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
                ProjectionType: pulumi.String("ALL"),
            },
            awsdynamodb.TableGlobalSecondaryIndexArgs{
                Name:           pulumi.String("GSI2"),
                HashKey:        pulumi.String("GSI2PK"),
                RangeKey:       pulumi.StringPtr("GSI2SK"),
                ProjectionType: pulumi.String("ALL"),
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

    table, err := awsdynamodb.NewTable(ctx, fmt.Sprintf("%s-tenant", name), targs, tableOpt...)
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
    }, retOpts...)
    if err != nil {
        return nil, err
    }

    // Basic logs policy
    if _, err = awsiam.NewRolePolicyAttachment(ctx, fmt.Sprintf("%s-logs", name), &awsiam.RolePolicyAttachmentArgs{
        Role:      role.Name,
        PolicyArn: pulumi.String(awsiam.ManagedPolicyAWSLambdaBasicExecutionRole),
    }, retOpts...); err != nil {
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
    }, retOpts...)
    if err != nil {
        return nil, err
    }
    if _, err = awsiam.NewRolePolicyAttachment(ctx, fmt.Sprintf("%s-vp-attach", name), &awsiam.RolePolicyAttachmentArgs{
        Role:      role.Name,
        PolicyArn: vpPol.Arn,
    }, retOpts...); err != nil {
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
    }, retOpts...); err != nil {
        return nil, err
    }

    // 3) Lambda code: embed built authorizer
    code := pulumi.NewAssetArchive(map[string]interface{}{
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

    fnArgs := &awslambda.FunctionArgs{
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
    }
    if pc > 0 {
        // Ensure a version is created so we can attach provisioned concurrency to an alias.
        fnArgs.Publish = pulumi.BoolPtr(true)
    }
    fn, err := awslambda.NewFunction(ctx, fmt.Sprintf("%s-authorizer", name), fnArgs, retOpts...)
    if err != nil {
        return nil, err
    }

    // Optional provisioned concurrency (disabled by default when pc == 0)
    if pc > 0 {
        // Create an alias pointing to the published version. Derive the numeric version
        // from the function's QualifiedArn (which includes the version when Publish=true).
        version := fn.QualifiedArn.ApplyT(func(qarn string) (string, error) {
            // qarn format: arn:aws:lambda:<region>:<acct>:function:<name>:<version>
            idx := strings.LastIndex(qarn, ":")
            if idx == -1 || idx == len(qarn)-1 {
                return "", fmt.Errorf("unexpected QualifiedArn: %s", qarn)
            }
            return qarn[idx+1:], nil
        }).(pulumi.StringOutput)

        alias, err := awslambda.NewAlias(ctx, fmt.Sprintf("%s-authorizer-live", name), &awslambda.AliasArgs{
            Name:            pulumi.String("live"),
            FunctionName:    fn.Name,
            FunctionVersion: version,
        }, retOpts...)
        if err != nil {
            return nil, err
        }
        if _, err := awslambda.NewProvisionedConcurrencyConfig(ctx, fmt.Sprintf("%s-authorizer-pc", name), &awslambda.ProvisionedConcurrencyConfigArgs{
            FunctionName:                    fn.Name,
            Qualifier:                        alias.Name,
            ProvisionedConcurrentExecutions: pulumi.Int(pc),
        }, retOpts...); err != nil {
            return nil, err
        }
    }

    // 4) Log group
    if _, err = awscloudwatch.NewLogGroup(ctx, fmt.Sprintf("%s-lg", name), &awscloudwatch.LogGroupArgs{
        Name:            fn.Name.ApplyT(func(n string) string { return "/aws/lambda/" + n }).(pulumi.StringOutput).ToStringPtrOutput(),
        RetentionInDays: pulumi.IntPtr(14),
    }, retOpts...); err != nil {
        return nil, err
    }

    // Wire base outputs
    comp.PolicyStoreId = store.ID().ToStringOutput()
    comp.PolicyStoreArn = store.Arn
    // Grouped output assignments
    comp.Lambda.AuthorizerFunctionArn = fn.Arn
    comp.Lambda.RoleArn = role.Arn
    // Dynamo: StreamArn is only non-nil when streams are enabled on the table
    comp.Dynamo.AuthTableArn = table.Arn
    comp.Dynamo.AuthTableStreamArn = table.StreamArn

    // 5) Optional Cognito provisioning + Verified Permissions identity source
    if args.Cognito != nil {
        cog, err := provisionCognito(ctx, name, store, *args.Cognito, *args.RetainOnDelete, retOpts...)
        if err != nil {
            return nil, err
        }
        comp.Cognito = &CognitoOutputs{
            UserPoolId:        cog.UserPoolId.ToStringPtrOutput(),
            UserPoolArn:       cog.UserPoolArn.ToStringPtrOutput(),
            UserPoolClientIds: cog.ClientIds,
        }
        comp.Parameters = cog.Parameters
    }

    return comp, nil
}

// withRetention augments resource options with RetainOnDelete when retain==true.
func withRetention(opts []pulumi.ResourceOption, retain bool) []pulumi.ResourceOption {
    if retain {
        return append(opts, pulumi.RetainOnDelete(true))
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
    opts ...pulumi.ResourceOption,
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
        regionName        string
        sesIdentityName   string
        sesCallerAcct     string
        fromEmail         string
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
        _ = sesIdentityRegion // reference for clarity

        // Configure the user pool to use SES (DEVELOPER) with provided values
        emailCfg := &awscognito.UserPoolEmailConfigurationArgs{
            EmailSendingAccount: pulumi.String("DEVELOPER"),
            SourceArn:           pulumi.StringPtr(cfg.SesConfig.SourceArn),
            FromEmailAddress:    pulumi.StringPtr(cfg.SesConfig.From),
        }
        if cfg.SesConfig.ReplyToEmail != nil && *cfg.SesConfig.ReplyToEmail != "" {
            emailCfg.ReplyToEmailAddress = pulumi.StringPtr(*cfg.SesConfig.ReplyToEmail)
        }
        if cfg.SesConfig.ConfigurationSet != nil && *cfg.SesConfig.ConfigurationSet != "" {
            emailCfg.ConfigurationSet = pulumi.StringPtr(*cfg.SesConfig.ConfigurationSet)
        }
        upArgs.EmailConfiguration = emailCfg
    }

    userPool, err := awscognito.NewUserPool(ctx, fmt.Sprintf("%s-userpool", name), upArgs, opts...)
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
        }, append(withRetention(opts, retainOnDelete), pulumi.Provider(sesProv))...); err != nil {
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
        }, opts...)
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
    }, opts...)
    if err != nil {
        return nil, err
    }

    // Collect outputs (typed)
    res := &cognitoProvisionResult{
        UserPoolId:  userPool.ID().ToStringOutput(),
        UserPoolArn: userPool.Arn,
        ClientIds:   pulumi.StringArray(clientIds).ToStringArrayOutput(),
        Parameters:  (pulumi.StringMap{"USER_POOL_ID": userPool.ID().ToStringOutput()}).ToStringMapOutput(),
    }
    return res, nil
}
// Note: Identity Pools, lifecycle triggers, templates, and other advanced Cognito
// options were intentionally removed from the public configuration surface. The
// provider creates a minimal User Pool + client and binds it as the VP identity source.

// ------------- AVP schema/policy ingestion & validation -------------

// AvpAssetsConfig configures where the provider should find the AVP schema (YAML or JSON)
// and Cedar policy files, and how strictly to validate them.
type AvpAssetsConfig struct {
    // Directory containing a schema file and a policies/ subfolder.
    // Relative paths resolve from the Pulumi project root (working directory where `pulumi up` is run).
    Dir string `pulumi:"dir"`
    // Optional schema file name relative to Dir. When omitted, the provider searches for
    // schema.yaml, schema.yml, or schema.json within Dir.
    SchemaFile *string `pulumi:"schemaFile,optional"`
    // Glob (recursive) for policy files relative to Dir. Defaults to "policies/**/*.cedar".
    PoliciesGlob *string `pulumi:"policiesGlob,optional"`
    // Enforce action-group mapping convention in policies and schema actions: "off" | "warn" | "error" (default: warn).
    ActionGroupEnforcement *string `pulumi:"actionGroupEnforcement,optional"`
    // Require baseline guardrail deny policies to be present (default: true).
    RequireGuardrails *bool `pulumi:"requireGuardrails,optional"`
    // If true, run canary authorization checks after policy deployment using cases defined in canaryFile (default: false).
    PostDeployCanary *bool `pulumi:"postDeployCanary,optional"`
    // Optional canary tests file (YAML) relative to Dir. When present and postDeployCanary is true,
    // the provider runs IsAuthorized for each case and fails on mismatch.
    CanaryFile *string `pulumi:"canaryFile,optional"`
}

// canonical action group identifiers
var canonicalActionGroups = []string{
    "batchCreate", "create", "batchDelete", "delete", "find", "get", "batchUpdate", "update",
}

// applySchemaAndPolicies loads schema/policies from disk, performs validations, applies schema if changed,
// and creates static policies as Pulumi resources bound to the created policy store.
func applySchemaAndPolicies(ctx *pulumi.Context, name string, store *awsvp.PolicyStore, cfg AvpAssetsConfig) error {
    // Resolve base directory
    dir := strings.TrimSpace(cfg.Dir)
    if dir == "" {
        return fmt.Errorf("avpAssets.dir is required when avpAssets is provided")
    }
    if !filepath.IsAbs(dir) {
        cwd, _ := os.Getwd()
        dir = filepath.Join(cwd, dir)
    }
    if st, err := os.Stat(dir); err != nil || !st.IsDir() {
        return fmt.Errorf("avpAssets.dir %q not found or not a directory", cfg.Dir)
    }

    // Determine schema file path
    schemaPath := ""
    if cfg.SchemaFile != nil && strings.TrimSpace(*cfg.SchemaFile) != "" {
        schemaPath = *cfg.SchemaFile
        if !filepath.IsAbs(schemaPath) {
            schemaPath = filepath.Join(dir, schemaPath)
        }
    } else {
        // search for default names
        for _, f := range []string{"schema.yaml", "schema.yml", "schema.json"} {
            p := filepath.Join(dir, f)
            if _, err := os.Stat(p); err == nil {
                schemaPath = p
                break
            }
        }
    }
    if schemaPath == "" {
        return fmt.Errorf("no schema file found in %s (looked for schema.yaml|schema.yml|schema.json); set avpAssets.schemaFile to override", dir)
    }

    // Read and parse schema (YAML or JSON → JSON string)
    cedarJSON, ns, actions, err := loadAndValidateSchema(ctx, schemaPath)
    if err != nil {
        return err
    }

    // Action-group enforcement (schema-level, based on action names)
    agMode := strings.ToLower(valueOrDefault(cfg.ActionGroupEnforcement, "warn"))
    if violations, err := enforceActionGroups(actions, agMode); err != nil {
        return err
    } else if len(violations) > 0 && agMode == "warn" {
        ctx.Log.Warn(fmt.Sprintf("AVP: actions not aligned to canonical action groups %v: %s", canonicalActionGroups, strings.Join(violations, ", ")), &pulumi.LogArgs{})
    }

    // Apply schema if changed
    // Apply schema if changed, sequenced after the PolicyStore is created
    // Derive Region from Policy Store ARN to ensure we call the same Region even when a custom provider is used
    schemaApplied := pulumi.All(store.ID(), store.Arn).ApplyT(func(args []interface{}) (string, error) {
        id := args[0].(string)
        arn := args[1].(string)
        parts := strings.Split(arn, ":")
        if len(parts) < 4 {
            return "", fmt.Errorf("unexpected policy store ARN: %s", arn)
        }
        regionName := parts[3]
        if err := putSchemaIfChanged(ctx, id, cedarJSON, regionName); err != nil {
            return "", err
        }
        ctx.Log.Info(fmt.Sprintf("AVP: schema applied for namespace %q (no-op when unchanged)", ns), &pulumi.LogArgs{})
        return "ok", nil
    }).(pulumi.StringOutput)

    // Collect policy files
    policiesGlob := valueOrDefault(cfg.PoliciesGlob, "policies/**/*.cedar")
    files, err := globRecursive(dir, policiesGlob)
    if err != nil {
        return fmt.Errorf("failed to enumerate policies with glob %q: %w", policiesGlob, err)
    }
    if len(files) == 0 {
        ctx.Log.Warn(fmt.Sprintf("AVP: no policy files matched %q under %s", policiesGlob, dir), &pulumi.LogArgs{})
    }

    // Guardrails presence check
    reqGuardrails := true
    if cfg.RequireGuardrails != nil {
        reqGuardrails = *cfg.RequireGuardrails
    }
    if reqGuardrails {
        missing := missingGuardrailFiles(files)
        if len(missing) > 0 {
            return fmt.Errorf("required guardrail policy files missing: %s", strings.Join(missing, ", "))
        }
    }

    // Create static policies as child resources
    // Deterministic order for stable diffs
    sort.Strings(files)
    policyIDs := []pulumi.StringOutput{}
    for i, f := range files {
        b, err := os.ReadFile(f)
        if err != nil {
            return fmt.Errorf("failed to read policy %s: %w", f, err)
        }
        polName := fmt.Sprintf("%s-pol-%03d", name, i+1)
        // Gate the statement on schema application so policy creation occurs after PutSchema completes.
        stmt := pulumi.All(schemaApplied).ApplyT(func(_ []interface{}) string {
            return string(b)
        }).(pulumi.StringOutput)
        // AWS VP policy requires either static or template-linked definition; we use static.
        // The "statement" field is Cedar policy text.
        pol, err := awsvp.NewPolicy(ctx, polName, &awsvp.PolicyArgs{
            PolicyStoreId: store.ID(),
            Definition: &awsvp.PolicyDefinitionArgs{
                Static: &awsvp.PolicyDefinitionStaticArgs{
                    Statement: stmt,
                },
            },
        }, pulumi.Parent(store))
        if err != nil {
            return fmt.Errorf("failed to create policy for %s: %w", f, err)
        }
        // Capture policy IDs to sequence canary checks after all policies are created.
        policyIDs = append(policyIDs, pol.ID().ToStringOutput())
    }

    // Optional: post-deploy canary checks
    if cfg.PostDeployCanary != nil && *cfg.PostDeployCanary {
        // Chain canaries to run after policies (gated by schemaApplied and policy IDs) and export status so failures surface.
        canaryDeps := append([]pulumi.Output{schemaApplied}, toOutputs(policyIDs)...)
        canaryStatus := pulumi.All(canaryDeps...).ApplyT(func(_ []interface{}) (string, error) {
            if err := runCanaries(ctx, store, dir, cfg.CanaryFile); err != nil {
                return "", err
            }
            return "ok", nil
        }).(pulumi.StringOutput)
        ctx.Export(fmt.Sprintf("%s-avpCanary", name), canaryStatus)
    }

    return nil
}

// loadAndValidateSchema parses YAML/JSON schema, enforces single namespace and required entities.
// Returns cedar JSON string, namespace name, and the set of action names.
func loadAndValidateSchema(ctx *pulumi.Context, schemaPath string) (string, string, []string, error) {
    raw, err := os.ReadFile(schemaPath)
    if err != nil {
        return "", "", nil, fmt.Errorf("failed to read schema file %s: %w", schemaPath, err)
    }
    var doc any
    switch strings.ToLower(filepath.Ext(schemaPath)) {
    case ".yaml", ".yml":
        if err := yaml.Unmarshal(raw, &doc); err != nil {
            return "", "", nil, fmt.Errorf("invalid YAML in %s: %w", schemaPath, err)
        }
    case ".json":
        if err := json.Unmarshal(raw, &doc); err != nil {
            return "", "", nil, fmt.Errorf("invalid JSON in %s: %w", schemaPath, err)
        }
    default:
        return "", "", nil, fmt.Errorf("unsupported schema extension %q; expected .yaml, .yml, or .json", filepath.Ext(schemaPath))
    }
    // Expect top-level object: { "<namespace>": { entityTypes: {...}, actions: {...} } }
    top, ok := doc.(map[string]any)
    if !ok {
        return "", "", nil, fmt.Errorf("schema must be a mapping of namespace → {entityTypes, actions}")
    }
    if len(top) != 1 {
        return "", "", nil, fmt.Errorf("AVP supports a single namespace per schema; found %d namespaces", len(top))
    }
    var ns string
    var body any
    for k, v := range top {
        ns = k
        body = v
        break
    }
    // Required entity presence checks
    bmap, ok := body.(map[string]any)
    if !ok {
        return "", "", nil, fmt.Errorf("schema namespace %q must map to an object", ns)
    }
    etRaw, ok := bmap["entityTypes"]
    if !ok {
        return "", "", nil, fmt.Errorf("schema namespace %q must define entityTypes", ns)
    }
    et, ok := etRaw.(map[string]any)
    if !ok {
        return "", "", nil, fmt.Errorf("entityTypes must be an object of entity type definitions")
    }
    requiredPrincipals := []string{"Tenant", "User", "Group", "Role", "GlobalRole", "TenantGrant"}
    requiredResources := []string{"Event", "Files", "Grant", "GlobalGrant", "Ticket"}
    missing := []string{}
    for _, r := range append(append([]string{}, requiredPrincipals...), requiredResources...) {
        if _, ok := et[r]; !ok {
            missing = append(missing, r)
        }
    }
    if len(missing) > 0 {
        return "", "", nil, fmt.Errorf("schema namespace %q missing required entity types: %s", ns, strings.Join(missing, ", "))
    }
    // Hierarchy expectations: Tenant & Group support trees → memberOfTypes includes self
    for _, hierarchical := range []string{"Tenant", "Group"} {
        if def, ok := et[hierarchical].(map[string]any); ok {
            if mot, ok := def["memberOfTypes"].([]any); ok {
                found := false
                for _, v := range mot {
                    if s, ok := v.(string); ok && s == hierarchical {
                        found = true
                        break
                    }
                }
                if !found {
                    ctx.Log.Warn(fmt.Sprintf("entity %s should include itself in memberOfTypes to enable hierarchical nesting", hierarchical), &pulumi.LogArgs{})
                }
            } else {
                ctx.Log.Warn(fmt.Sprintf("entity %s should define memberOfTypes including itself to enable hierarchical nesting", hierarchical), &pulumi.LogArgs{})
            }
        }
    }
    // Collect action names for action-group enforcement
    acts := []string{}
    if aRaw, ok := bmap["actions"]; ok {
        if amap, ok := aRaw.(map[string]any); ok {
            for name := range amap {
                acts = append(acts, name)
            }
        }
    }
    // Re-encode to canonical JSON (minified) for PutSchema
    b, err := json.Marshal(top)
    if err != nil {
        return "", "", nil, fmt.Errorf("failed to encode schema as JSON: %w", err)
    }
    return string(b), ns, acts, nil
}

// enforceActionGroups checks that action names map cleanly to canonical groups per naming convention.
// Convention: action names start with one of: batchCreate|create|batchDelete|delete|find|get|batchUpdate|update
func enforceActionGroups(actions []string, mode string) ([]string, error) {
    if strings.EqualFold(mode, "off") {
        return nil, nil
    }
    groups := map[string]struct{}{}
    for _, g := range canonicalActionGroups {
        groups[g] = struct{}{}
    }
    bad := []string{}
    for _, a := range actions {
        g := leadingCamelSegment(a)
        if _, ok := groups[g]; !ok {
            bad = append(bad, a)
        }
    }
    if len(bad) == 0 {
        return nil, nil
    }
    if mode == "error" {
        return bad, fmt.Errorf("actions not aligned to canonical action groups %v: %s", canonicalActionGroups, strings.Join(bad, ", "))
    }
    // warn
    return bad, nil
}

// leadingCamelSegment returns the leading lowerCamelCase verb segment from an action name.
// Examples: createTicket -> create; batchDeleteFiles -> batchDelete; GetTicket -> get
func leadingCamelSegment(s string) string {
    if s == "" {
        return s
    }
    // Normalize to lower camel: ensure first rune is lowercased for comparison
    rs := []rune(s)
    rs[0] = []rune(strings.ToLower(string(rs[0])))[0]
    s = string(rs)
    // Identify transition from lower→upper (verb boundary to resource)
    for i := 1; i < len(s); i++ {
        if s[i] >= 'A' && s[i] <= 'Z' {
            return s[:i]
        }
    }
    return s
}

// putSchemaIfChanged retrieves existing schema and applies only when content differs.
func putSchemaIfChanged(ctx *pulumi.Context, policyStoreId string, cedarJSON string, region string) error {
    cfg, err := loadAwsConfig(ctx.Context(), region)
    if err != nil {
        return err
    }
    client := vpapi.NewFromConfig(cfg)

    // Fetch current schema; treat NotFound as empty
    var current string
    pulumiCtx := ctx.Context()
    getOut, err := client.GetSchema(pulumiCtx, &vpapi.GetSchemaInput{PolicyStoreId: &policyStoreId})
    if err != nil {
        var notFound *vpapiTypes.ResourceNotFoundException
        if !errors.As(err, &notFound) {
            // Other errors: continue to attempt PutSchema but report context
            ctx.Log.Warn(fmt.Sprintf("AVP GetSchema warning: %v", err), &pulumi.LogArgs{})
        }
    } else if getOut.Definition != nil && getOut.Definition.CedarJson != nil {
        current = *getOut.Definition.CedarJson
    }
    if normalizeJSON(current) == normalizeJSON(cedarJSON) {
        ctx.Log.Info("AVP: schema unchanged; skipping PutSchema", &pulumi.LogArgs{})
        return nil
    }
    // Apply
    _, err = client.PutSchema(pulumiCtx, &vpapi.PutSchemaInput{
        PolicyStoreId: &policyStoreId,
        Definition:    &vpapiTypes.SchemaDefinition{CedarJson: &cedarJSON},
    })
    if err != nil {
        return fmt.Errorf("failed to put schema: %w", err)
    }
    return nil
}

// runCanaries executes authorization checks defined in a YAML file under dir (defaults to canaries.yaml).
// File shape:
// cases:
//   - principal: { entityType: "User", entityId: "user-1" }
//     action: "getTicket"
//     resource: { entityType: "Ticket", entityId: "t-1" }
//     expect: "ALLOW" | "DENY"
func runCanaries(ctx *pulumi.Context, store *awsvp.PolicyStore, dir string, canaryFile *string) error {
    if ctx.DryRun() {
        ctx.Log.Info("AVP canary: preview mode; skipping canary execution", &pulumi.LogArgs{})
        return nil
    }
    path := filepath.Join(dir, "canaries.yaml")
    if canaryFile != nil && *canaryFile != "" {
        p := *canaryFile
        if !filepath.IsAbs(p) {
            p = filepath.Join(dir, p)
        }
        path = p
    }
    b, err := os.ReadFile(path)
    if err != nil {
        return fmt.Errorf("failed to read canary file %s: %w", path, err)
    }
    var doc struct{
        Cases []struct{
            Principal map[string]string `yaml:"principal"`
            Action    string            `yaml:"action"`
            Resource  map[string]string `yaml:"resource"`
            Expect    string            `yaml:"expect"`
        } `yaml:"cases"`
    }
    if err := yaml.Unmarshal(b, &doc); err != nil {
        return fmt.Errorf("invalid canary YAML %s: %w", path, err)
    }
    if len(doc.Cases) == 0 {
        ctx.Log.Warn("AVP canary: no cases defined; skipping", &pulumi.LogArgs{})
        return nil
    }
    // Execute inside an ApplyT so the store ID is resolved and failures are surfaced
    _ = store.ID().ToStringOutput().ApplyT(func(id string) (string, error) {
        region, err := aws.GetRegion(ctx, nil)
        if err != nil {
            return "", fmt.Errorf("failed to get AWS region: %w", err)
        }
        cfg, err := loadAwsConfig(ctx.Context(), region.Name)
        if err != nil {
            return "", err
        }
        client := vpapi.NewFromConfig(cfg)
        for i, c := range doc.Cases {
            ptype := c.Principal["entityType"]
            pid := c.Principal["entityId"]
            rtype := c.Resource["entityType"]
            rid := c.Resource["entityId"]
            act := c.Action
            p := vpapiTypes.EntityIdentifier{EntityType: &ptype, EntityId: &pid}
            r := vpapiTypes.EntityIdentifier{EntityType: &rtype, EntityId: &rid}
            out, err := client.IsAuthorized(ctx.Context(), &vpapi.IsAuthorizedInput{
                PolicyStoreId: &id,
                Principal:     &p,
                Resource:      &r,
                Action:        &vpapiTypes.ActionIdentifier{ActionType: vpapiTypes.ActionTypeAction, ActionId: &act},
            })
            if err != nil {
                return "", fmt.Errorf("canary #%d failed to execute: %v", i+1, err)
            }
            got := string(out.Decision)
            if !strings.EqualFold(got, c.Expect) {
                return "", fmt.Errorf("canary #%d unexpected decision: got %s, want %s (principal=%v, action=%s, resource=%v)", i+1, got, c.Expect, c.Principal, c.Action, c.Resource)
            }
        }
        ctx.Log.Info(fmt.Sprintf("AVP canary: %d checks passed", len(doc.Cases)), &pulumi.LogArgs{})
        return id, nil
    })
    return nil
}

// missingGuardrailFiles returns a list of required guardrail policy basenames that are not present.
func missingGuardrailFiles(paths []string) []string {
    required := []string{
        "01-deny-tenant-mismatch.cedar",
        "02-deny-tenant-role-global-admin.cedar",
    }
    present := map[string]struct{}{}
    for _, p := range paths {
        present[filepath.Base(p)] = struct{}{}
    }
    missing := []string{}
    for _, r := range required {
        if _, ok := present[r]; !ok {
            missing = append(missing, r)
        }
    }
    return missing
}

// globRecursive implements a simple recursive glob: base + pattern (supports **).
func globRecursive(base, pattern string) ([]string, error) {
    // Translate a subset of ** glob to filepath.WalkDir
    matches := []string{}
    err := filepath.WalkDir(base, func(path string, d fs.DirEntry, err error) error {
        if err != nil {
            return err
        }
        if d.IsDir() {
            return nil
        }
        rel, _ := filepath.Rel(base, path)
        ok, err := ds.PathMatch(pattern, rel)
        if err != nil {
            return err
        }
        if ok {
            matches = append(matches, path)
        }
        return nil
    })
    return matches, err
}

// loadAwsConfig loads the default AWS configuration for the given region using the standard
// environment/credentials chain used by the Pulumi AWS provider.
func loadAwsConfig(ctx context.Context, region string) (awsv2.Config, error) {
    return awsconfig.LoadDefaultConfig(ctx, awsconfig.WithRegion(region))
}

// normalizeJSON minifies JSON text for stable equality comparison; when input is empty returns empty string.
func normalizeJSON(s string) string {
    if strings.TrimSpace(s) == "" {
        return ""
    }
    var v any
    if err := json.Unmarshal([]byte(s), &v); err != nil {
        // Not JSON? return original
        return s
    }
    b, err := json.Marshal(v)
    if err != nil {
        return s
    }
    return string(b)
}

func valueOrDefault[T ~string](ptr *T, def T) string { // generic-ish helper for *string
    if ptr == nil {
        return string(def)
    }
    return string(*ptr)
}

func toOutputs(ins []pulumi.StringOutput) []pulumi.Output {
    outs := make([]pulumi.Output, 0, len(ins))
    for _, in := range ins {
        outs = append(outs, in)
    }
    return outs
}

