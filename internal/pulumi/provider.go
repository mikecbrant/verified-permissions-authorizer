package provider

import (
	"encoding/json"
	"fmt"
	"strings"

	aws "github.com/pulumi/pulumi-aws/sdk/v6/go/aws"
	awscognito "github.com/pulumi/pulumi-aws/sdk/v6/go/aws/cognito"
	awsdynamodb "github.com/pulumi/pulumi-aws/sdk/v6/go/aws/dynamodb"
	awsiam "github.com/pulumi/pulumi-aws/sdk/v6/go/aws/iam"
	awslambda "github.com/pulumi/pulumi-aws/sdk/v6/go/aws/lambda"
	awssesv2 "github.com/pulumi/pulumi-aws/sdk/v6/go/aws/sesv2"
	awsvp "github.com/pulumi/pulumi-aws/sdk/v6/go/aws/verifiedpermissions"
	p "github.com/pulumi/pulumi-go-provider"
	"github.com/pulumi/pulumi-go-provider/infer"
	"github.com/pulumi/pulumi/sdk/v3/go/common/tokens"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"

	sharedassets "github.com/mikecbrant/verified-permissions-authorizer/internal/common/assets"
)

var authorizerIndexMjs = sharedassets.GetAuthorizerIndexMjs()

// NewProvider exposes construction to allow early sanity checks on embedded assets.
func NewProvider() (p.Provider, error) {
	if strings.TrimSpace(authorizerIndexMjs) == "" {
		var zero p.Provider
		return zero, fmt.Errorf("embedded authorizer lambda (index.mjs) not found; ensure CI populated internal/common/assets/lambda/index.mjs before building the provider")
	}
	return infer.NewProviderBuilder().
		WithComponents(infer.ComponentF(NewAuthorizerWithPolicyStore)).
		Build()
}

// Note: The provider also includes a minimal Cognito trigger stub for future use.

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
	// Verified Permissions schema & policy asset ingestion and validation settings.
	// See VerifiedPermissionsConfig for details.
	VerifiedPermissions *VerifiedPermissionsConfig `pulumi:"verifiedPermissions,optional"`
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
	PolicyStoreId  pulumi.StringOutput    `pulumi:"policyStoreId"`
	PolicyStoreArn pulumi.StringOutput    `pulumi:"policyStoreArn"`
	Parameters     pulumi.StringMapOutput `pulumi:"parameters,optional"`

	// Grouped outputs
	Cognito *CognitoOutputs `pulumi:"cognito,optional"`
	Dynamo  DynamoOutputs   `pulumi:"dynamo"`
	Lambda  LambdaOutputs   `pulumi:"lambda"`
}

// CognitoOutputs groups optional Cognito-related outputs under the `cognito` object.
type CognitoOutputs struct {
	UserPoolArn       pulumi.StringPtrOutput   `pulumi:"userPoolArn,optional"`
	UserPoolClientIDs pulumi.StringArrayOutput `pulumi:"userPoolClientIds,optional"`
	UserPoolId        pulumi.StringPtrOutput   `pulumi:"userPoolId,optional"`
}

// DynamoOutputs groups DynamoDB auth table outputs under the `dynamo` object.
type DynamoOutputs struct {
	AuthTableArn       pulumi.StringOutput `pulumi:"authTableArn"`
	AuthTableStreamArn pulumi.StringOutput `pulumi:"authTableStreamArn,optional"`
}

// LambdaOutputs groups Lambda authorizer outputs under the `lambda` object.
type LambdaOutputs struct {
	AuthorizerFunctionArn pulumi.StringOutput `pulumi:"authorizerFunctionArn"`
	RoleArn               pulumi.StringOutput `pulumi:"roleArn"`
}

// Annotate attaches schema metadata used for provider docs and code generation.
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

	normalizeAuthorizerArgs(&args)
	childOpts, retOpts := buildChildOptions(comp, opts, *args.RetainOnDelete)

	store, err := createPolicyStore(ctx, name, args, retOpts)
	if err != nil {
		return nil, err
	}

	table, err := createAuthTable(ctx, name, args, retOpts)
	if err != nil {
		return nil, err
	}

	role, fn, err := createAuthorizerLambda(ctx, name, store, childOpts)
	if err != nil {
		return nil, err
	}

	// Outputs
	comp.PolicyStoreId = store.ID().ToStringOutput()
	comp.PolicyStoreArn = store.Arn
	comp.Dynamo = DynamoOutputs{AuthTableArn: table.Arn, AuthTableStreamArn: table.StreamArn}
	comp.Lambda = LambdaOutputs{AuthorizerFunctionArn: fn.Arn, RoleArn: role.Arn}

	if args.Cognito != nil {
		cog, err := createCognito(ctx, name, *args.Cognito, childOpts)
		if err != nil {
			return nil, err
		}
		comp.Cognito = cog
	}

	// Verified Permissions schema and policy ingestion
	if args.VerifiedPermissions != nil {
		if err := applySchemaAndPolicies(ctx, name, store, *args.VerifiedPermissions); err != nil {
			return nil, err
		}
	}

	return comp, nil
}

func normalizeAuthorizerArgs(args *AuthorizerArgs) {
	if args.RetainOnDelete == nil {
		b := false
		args.RetainOnDelete = &b
	}
	if args.Dynamo == nil {
		args.Dynamo = &DynamoConfig{}
	}
}

func buildChildOptions(comp pulumi.Resource, opts []pulumi.ResourceOption, retainOnDelete bool) (childOpts []pulumi.ResourceOption, retainOpts []pulumi.ResourceOption) {
	childOpts = append([]pulumi.ResourceOption{}, opts...)
	childOpts = append(childOpts, pulumi.Parent(comp))
	retainOpts = append([]pulumi.ResourceOption{}, childOpts...)
	if retainOnDelete {
		retainOpts = append(retainOpts, pulumi.RetainOnDelete(true))
	}
	return childOpts, retainOpts
}

func createPolicyStore(ctx *pulumi.Context, name string, args AuthorizerArgs, opts []pulumi.ResourceOption) (*awsvp.PolicyStore, error) {
	storeArgs := &awsvp.PolicyStoreArgs{
		ValidationSettings: awsvp.PolicyStoreValidationSettingsArgs{Mode: pulumi.String("STRICT")},
	}
	if args.Description != nil {
		storeArgs.Description = pulumi.StringPtr(*args.Description)
	}
	return awsvp.NewPolicyStore(ctx, fmt.Sprintf("%s-store", name), storeArgs, opts...)
}

func createAuthTable(ctx *pulumi.Context, name string, args AuthorizerArgs, opts []pulumi.ResourceOption) (*awsdynamodb.Table, error) {
	targs := &awsdynamodb.TableArgs{
		BillingMode: pulumi.String("PAY_PER_REQUEST"),
		// Only attributes participating in the primary index or GSIs may be declared here.
		Attributes: awsdynamodb.TableAttributeArray{
			awsdynamodb.TableAttributeArgs{Name: pulumi.String("PK"), Type: pulumi.String("S")},
			awsdynamodb.TableAttributeArgs{Name: pulumi.String("SK"), Type: pulumi.String("S")},
			awsdynamodb.TableAttributeArgs{Name: pulumi.String("GSI1PK"), Type: pulumi.String("S")},
			awsdynamodb.TableAttributeArgs{Name: pulumi.String("GSI1SK"), Type: pulumi.String("S")},
			// GSI2 attributes
			awsdynamodb.TableAttributeArgs{Name: pulumi.String("GSI2PK"), Type: pulumi.String("S")},
			awsdynamodb.TableAttributeArgs{Name: pulumi.String("GSI2SK"), Type: pulumi.String("S")},
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

	if args.RetainOnDelete != nil && *args.RetainOnDelete {
		targs.DeletionProtectionEnabled = pulumi.BoolPtr(true)
		targs.PointInTimeRecovery = &awsdynamodb.TablePointInTimeRecoveryArgs{Enabled: pulumi.Bool(true)}
	}

	if args.Dynamo != nil && args.Dynamo.EnableDynamoDbStream != nil && *args.Dynamo.EnableDynamoDbStream {
		targs.StreamEnabled = pulumi.BoolPtr(true)
		targs.StreamViewType = pulumi.StringPtr("NEW_AND_OLD_IMAGES")
	}

	return awsdynamodb.NewTable(ctx, fmt.Sprintf("%s-auth", name), targs, opts...)
}

func createAuthorizerLambda(ctx *pulumi.Context, name string, store *awsvp.PolicyStore, opts []pulumi.ResourceOption) (*awsiam.Role, *awslambda.Function, error) {
	role, err := awsiam.NewRole(ctx, fmt.Sprintf("%s-role", name), &awsiam.RoleArgs{
		AssumeRolePolicy: pulumi.String(`{"Version":"2012-10-17","Statement":[{"Effect":"Allow","Principal":{"Service":["lambda.amazonaws.com"]},"Action":["sts:AssumeRole"]}]}`),
	}, opts...)
	if err != nil {
		return nil, nil, err
	}

	_, _ = awsiam.NewRolePolicyAttachment(ctx, fmt.Sprintf("%s-role-basic", name), &awsiam.RolePolicyAttachmentArgs{
		PolicyArn: pulumi.String("arn:aws:iam::aws:policy/service-role/AWSLambdaBasicExecutionRole"),
		Role:      role.Name,
	}, pulumi.Parent(role))

	fn, err := awslambda.NewFunction(ctx, fmt.Sprintf("%s-authorizer", name), &awslambda.FunctionArgs{
		Role:          role.Arn,
		Runtime:       pulumi.String("nodejs22.x"),
		Handler:       pulumi.String("index.handler"),
		Architectures: pulumi.ToStringArray([]string{"arm64"}),
		Timeout:       pulumi.Int(10),
		MemorySize:    pulumi.Int(128),
		Environment:   &awslambda.FunctionEnvironmentArgs{Variables: pulumi.StringMap{"POLICY_STORE_ID": store.ID().ToStringOutput()}},
		Code: pulumi.NewAssetArchive(map[string]interface{}{
			"index.mjs": pulumi.NewStringAsset(authorizerIndexMjs),
		}),
		Publish: pulumi.Bool(true),
	}, opts...)
	if err != nil {
		return nil, nil, err
	}
	return role, fn, nil
}

func createCognito(ctx *pulumi.Context, name string, cfg CognitoConfig, opts []pulumi.ResourceOption) (*CognitoOutputs, error) {
	if cfg.SesConfig == nil {
		return createBareUserPool(ctx, name, opts)
	}
	reg, err := aws.GetRegion(ctx, nil)
	if err != nil {
		return nil, err
	}
	account, identity, identityRegion, err := validateSesConfig(*cfg.SesConfig, reg.Name)
	if err != nil {
		return nil, err
	}

	emailConf := &awscognito.UserPoolEmailConfigurationArgs{
		EmailSendingAccount: pulumi.String("DEVELOPER"),
		SourceArn:           pulumi.String(cfg.SesConfig.SourceArn),
		FromEmailAddress:    pulumi.String(cfg.SesConfig.From),
	}
	if cfg.SesConfig.ReplyToEmail != nil {
		emailConf.ReplyToEmailAddress = pulumi.StringPtr(*cfg.SesConfig.ReplyToEmail)
	}
	if cfg.SesConfig.ConfigurationSet != nil {
		emailConf.ConfigurationSet = pulumi.StringPtr(*cfg.SesConfig.ConfigurationSet)
	}

	up, err := awscognito.NewUserPool(ctx, fmt.Sprintf("%s-userpool", name), &awscognito.UserPoolArgs{EmailConfiguration: emailConf}, opts...)
	if err != nil {
		return nil, err
	}
	if err := createSesIdentityPolicy(ctx, name, up, account, identityRegion, identity, opts); err != nil {
		return nil, err
	}
	return &CognitoOutputs{UserPoolArn: up.Arn.ToStringPtrOutput(), UserPoolId: up.ID().ToStringPtrOutput(), UserPoolClientIDs: pulumi.ToStringArrayOutput([]pulumi.StringOutput{})}, nil
}

func createBareUserPool(ctx *pulumi.Context, name string, opts []pulumi.ResourceOption) (*CognitoOutputs, error) {
	up, err := awscognito.NewUserPool(ctx, fmt.Sprintf("%s-userpool", name), &awscognito.UserPoolArgs{}, opts...)
	if err != nil {
		return nil, err
	}
	return &CognitoOutputs{UserPoolArn: up.Arn.ToStringPtrOutput(), UserPoolId: up.ID().ToStringPtrOutput(), UserPoolClientIDs: pulumi.ToStringArrayOutput([]pulumi.StringOutput{})}, nil
}

func createSesIdentityPolicy(ctx *pulumi.Context, name string, up *awscognito.UserPool, account string, identityRegion string, identityName string, opts []pulumi.ResourceOption) error {
	policy := up.Arn.ApplyT(func(userPoolArn string) string {
		pol := map[string]any{
			"Version": "2012-10-17",
			"Statement": []map[string]any{{
				"Effect":    "Allow",
				"Action":    []string{"ses:SendEmail", "ses:SendRawEmail"},
				"Principal": map[string]any{"Service": "cognito-idp.amazonaws.com"},
				"Resource":  fmt.Sprintf("arn:%s:ses:%s:%s:identity/%s", partitionForRegion(identityRegion), identityRegion, account, identityName),
				"Condition": map[string]any{"StringEquals": map[string]any{"AWS:SourceArn": userPoolArn}},
			}},
		}
		b, _ := json.Marshal(pol)
		return string(b)
	}).(pulumi.StringOutput)
	_, err := awssesv2.NewEmailIdentityPolicy(ctx, fmt.Sprintf("%s-ses-policy", name), &awssesv2.EmailIdentityPolicyArgs{
		EmailIdentity: pulumi.String(identityName),
		Policy:        policy,
		PolicyName:    pulumi.String(fmt.Sprintf("%s-cognito-send", name)),
	}, opts...)
	return err
}
