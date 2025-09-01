package provider

import (
    _ "embed"
    "fmt"

    awscloudwatch "github.com/pulumi/pulumi-aws/sdk/v6/go/aws/cloudwatch"
    awsiam "github.com/pulumi/pulumi-aws/sdk/v6/go/aws/iam"
    awslambda "github.com/pulumi/pulumi-aws/sdk/v6/go/aws/lambda"
    awsvp "github.com/pulumi/pulumi-aws/sdk/v6/go/aws/verifiedpermissions"
    p "github.com/pulumi/pulumi-go-provider"
    "github.com/pulumi/pulumi-go-provider/infer"
    "github.com/pulumi/pulumi/sdk/v3/go/pulumi"
)

//go:embed assets/index.mjs
var authorizerIndexMjs string

// NewProvider wires up the multi-language component provider surface.
func NewProvider() (p.Provider, error) {
    return infer.Provider(infer.Options{
        Components: []infer.InferredComponent{
            infer.Component(NewAuthorizerWithPolicyStore),
        },
    }), nil
}

// AuthorizerArgs defines the inputs for the component resource.
type AuthorizerArgs struct {
    // Policy store description
    Description *string `pulumi:"description,optional"`
    // Validation mode for the policy store: STRICT | OFF (default STRICT)
    ValidationMode *string           `pulumi:"validationMode,optional"`
    LambdaEnv      map[string]string `pulumi:"lambdaEnvironment,optional"`
}

// AuthorizerWithPolicyStore is the component implementing the Construct.
// It exposes the created resource ARNs as outputs.
type AuthorizerWithPolicyStore struct {
    pulumi.ResourceState

    PolicyStoreId  pulumi.StringOutput `pulumi:"policyStoreId"`
    PolicyStoreArn pulumi.StringOutput `pulumi:"policyStoreArn"`
    FunctionArn    pulumi.StringOutput `pulumi:"functionArn"`
    RoleArn        pulumi.StringOutput `pulumi:"roleArn"`
}

func (c *AuthorizerWithPolicyStore) Annotate(a infer.Annotator) {
    a.Describe(&c, "Provision an AWS Verified Permissions Policy Store and a bundled Lambda Request Authorizer.")
    a.SetToken("index", "AuthorizerWithPolicyStore")
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

    // Defaults
    if args.ValidationMode == nil {
        def := "STRICT"
        args.ValidationMode = &def
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
    childOpts := append(opts, pulumi.Parent(comp))
    store, err := awsvp.NewPolicyStore(ctx, fmt.Sprintf("%s-store", name), storeArgs, childOpts...)
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
    }, childOpts...)
    if err != nil {
        return nil, err
    }

    // Basic logs policy
    if _, err = awsiam.NewRolePolicyAttachment(ctx, fmt.Sprintf("%s-logs", name), &awsiam.RolePolicyAttachmentArgs{
        Role:      role.Name,
        PolicyArn: pulumi.String(awsiam.ManagedPolicyAWSLambdaBasicExecutionRole),
    }, childOpts...); err != nil {
        return nil, err
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
    }, childOpts...)
    if err != nil {
        return nil, err
    }
    if _, err = awsiam.NewRolePolicyAttachment(ctx, fmt.Sprintf("%s-vp-attach", name), &awsiam.RolePolicyAttachmentArgs{
        Role:      role.Name,
        PolicyArn: vpPol.Arn,
    }, childOpts...); err != nil {
        return nil, err
    }

    // 3) Lambda code: embed built authorizer
    code := pulumi.NewAssetArchive(map[string]interface{}{
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
    }, childOpts...)
    if err != nil {
        return nil, err
    }

    // 4) Log group
    if _, err = awscloudwatch.NewLogGroup(ctx, fmt.Sprintf("%s-lg", name), &awscloudwatch.LogGroupArgs{
        Name:            fn.Name.ApplyT(func(n string) (string, error) { return "/aws/lambda/" + n, nil }).(pulumi.StringOutput),
        RetentionInDays: pulumi.IntPtr(14),
    }, childOpts...); err != nil {
        return nil, err
    }

    // Wire outputs
    comp.PolicyStoreId = store.ID().ToStringOutput()
    comp.PolicyStoreArn = store.Arn
    comp.FunctionArn = fn.Arn
    comp.RoleArn = role.Arn

    return comp, nil
}
