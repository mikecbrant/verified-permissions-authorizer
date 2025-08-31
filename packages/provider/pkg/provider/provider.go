package provider

import (
    "embed"
    "fmt"

    awscloudwatch "github.com/pulumi/pulumi-aws/sdk/v6/go/aws/cloudwatch"
    awsdynamodb "github.com/pulumi/pulumi-aws/sdk/v6/go/aws/dynamodb"
    awsiam "github.com/pulumi/pulumi-aws/sdk/v6/go/aws/iam"
    awslambda "github.com/pulumi/pulumi-aws/sdk/v6/go/aws/lambda"
    awsvp "github.com/pulumi/pulumi-aws/sdk/v6/go/aws/verifiedpermissions"
    p "github.com/pulumi/pulumi-go-provider"
    "github.com/pulumi/pulumi-go-provider/infer"
    "github.com/pulumi/pulumi/sdk/v3/go/pulumi"
)

//go:embed ../../assets/index.mjs
var authorizerIndexMjs string

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
    // If true, treat the stage as ephemeral: destroy resources on stack removal (no retention).
    EphemeralStage *bool `pulumi:"ephemeralStage,optional"`
    // If true, enable DynamoDB Streams on the tenant table (NEW_AND_OLD_IMAGES).
    EnableDynamoDbStreams *bool `pulumi:"enableDynamoDbStreams,optional"`
}

// AuthorizerResult defines the outputs for the component resource.
type AuthorizerResult struct {
    PolicyStoreId  string `pulumi:"policyStoreId"`
    PolicyStoreArn string `pulumi:"policyStoreArn"`
    FunctionArn    string `pulumi:"functionArn"`
    RoleArn        string `pulumi:"roleArn"`
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
    // Defaults for new provider-level options
    if args.EphemeralStage == nil {
        b := false
        args.EphemeralStage = &b
    }
    if args.EnableDynamoDbStreams == nil {
        b := false
        args.EnableDynamoDbStreams = &b
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
    store, err := awsvp.NewPolicyStore(ctx, fmt.Sprintf("%s-store", name), storeArgs, opts)
    if err != nil {
        return res, err
    }

    // 1b) DynamoDB single-table for tenants/users/roles
    // Removal policy (retain on delete) only when NOT ephemeral
    tableOpts := opts
    if !*args.EphemeralStage {
        tableOpts = pulumi.MergeResourceOptions(opts, pulumi.RetainOnDelete(true))
    }

    // Build base table args
    targs := &awsdynamodb.TableArgs{
        BillingMode: pulumi.String("PAY_PER_REQUEST"),
        // Only attributes participating in the primary index or GSIs may be declared here.
        // Items may still include GSI2PK/GSI2SK attributes; they are intentionally not part of AttributeDefinitions until GSI2 exists.
        Attributes: awsdynamodb.TableAttributeArray{
            awsdynamodb.TableAttributeArgs{ Name: pulumi.String("PK"), Type: pulumi.String("S") },
            awsdynamodb.TableAttributeArgs{ Name: pulumi.String("SK"), Type: pulumi.String("S") },
            awsdynamodb.TableAttributeArgs{ Name: pulumi.String("GSI1PK"), Type: pulumi.String("S") },
            awsdynamodb.TableAttributeArgs{ Name: pulumi.String("GSI1SK"), Type: pulumi.String("S") },
        },
        HashKey:  pulumi.String("PK"),
        RangeKey: pulumi.StringPtr("SK"),
        GlobalSecondaryIndexes: awsdynamodb.TableGlobalSecondaryIndexArray{
            awsdynamodb.TableGlobalSecondaryIndexArgs{
                Name:            pulumi.String("GSI1"),
                HashKey:         pulumi.String("GSI1PK"),
                RangeKey:        pulumi.StringPtr("GSI1SK"),
                ProjectionType:  pulumi.StringPtr("ALL"),
            },
        },
    }

    // For retained (non-ephemeral) stages enable deletion protection and PITR
    if !*args.EphemeralStage {
        targs.DeletionProtectionEnabled = pulumi.BoolPtr(true)
        targs.PointInTimeRecovery = &awsdynamodb.TablePointInTimeRecoveryArgs{ Enabled: pulumi.Bool(true) }
    }

    // Streams optional
    if *args.EnableDynamoDbStreams {
        targs.StreamEnabled = pulumi.BoolPtr(true)
        targs.StreamViewType = pulumi.StringPtr("NEW_AND_OLD_IMAGES")
    }

    table, err := awsdynamodb.NewTable(ctx, fmt.Sprintf("%s-tenant", name), targs, tableOpts)
    if err != nil {
        return res, err
    }

    // 2) IAM Role
    role, err := awsiam.NewRole(ctx, fmt.Sprintf("%s-role", name), &awsiam.RoleArgs{
        AssumeRolePolicy: pulumi.String(awsiam.GetPolicyDocumentOutput(ctx, awsiam.GetPolicyDocumentOutputArgs{
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
        }).Json().ToStringOutput()),
        Description: pulumi.StringPtr("Role for Verified Permissions Lambda Authorizer"),
    }, opts)
    if err != nil {
        return res, err
    }

    // Basic logs policy
    _, err = awsiam.NewRolePolicyAttachment(ctx, fmt.Sprintf("%s-logs", name), &awsiam.RolePolicyAttachmentArgs{
        Role:      role.Name,
        PolicyArn: pulumi.String(awsiam.ManagedPolicyAWSLambdaBasicExecutionRole),
    }, opts)
    if err != nil {
        return res, err
    }

    // Verified Permissions access policy: GetPolicyStore + IsAuthorized
    vpPol, err := awsiam.NewPolicy(ctx, fmt.Sprintf("%s-vp", name), &awsiam.PolicyArgs{
        Policy: pulumi.String(`{
  "Version": "2012-10-17",
  "Statement": [
    {
      "Sid": "VerifiedPermissionsAccess",
      "Effect": "Allow",
      "Action": [
        "verifiedpermissions:GetPolicyStore",
        "verifiedpermissions:IsAuthorized"
      ],
      "Resource": "*"
    }
  ]
}`),
    }, opts)
    if err != nil {
        return res, err
    }
    _, err = awsiam.NewRolePolicyAttachment(ctx, fmt.Sprintf("%s-vp-attach", name), &awsiam.RolePolicyAttachmentArgs{
        Role:      role.Name,
        PolicyArn: vpPol.Arn,
    }, opts)
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
    }, opts)
    if err != nil {
        return res, err
    }

    // 4) Log group
    _, err = awscloudwatch.NewLogGroup(ctx, fmt.Sprintf("%s-lg", name), &awscloudwatch.LogGroupArgs{
        Name:            fn.Name.ApplyT(func(n string) (string, error) { return "/aws/lambda/" + n, nil }).(pulumi.StringOutput),
        RetentionInDays: pulumi.IntPtr(14),
    }, opts)
    if err != nil {
        return res, err
    }

    // Return outputs via infer.SetOutputs
    outputs := map[string]any{
        "policyStoreId":  store.ID(),
        "policyStoreArn": store.Arn,
        "functionArn":    fn.Arn,
        "roleArn":        role.Arn,
        // Exports with exact names as required
        "TenantTableArn": table.Arn,
    }
    if *args.EnableDynamoDbStreams {
        outputs["TenantTableStreamArn"] = table.StreamArn
    }
    return AuthorizerResult{}, infer.SetOutputs(outputs)
}
