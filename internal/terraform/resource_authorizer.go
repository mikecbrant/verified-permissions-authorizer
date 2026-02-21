package provider

import (
	"archive/zip"
	"bytes"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"

	awscfg "github.com/aws/aws-sdk-go-v2/config"
	// "github.com/aws/aws-sdk-go-v2/service/cloudwatchlogs"
	// "github.com/aws/aws-sdk-go-v2/service/cognitoidentityprovider"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	dynamodbtypes "github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
	"github.com/aws/aws-sdk-go-v2/service/iam"
	"github.com/aws/aws-sdk-go-v2/service/lambda"
	lambdatypes "github.com/aws/aws-sdk-go-v2/service/lambda/types"
	"github.com/aws/aws-sdk-go-v2/service/verifiedpermissions"
	vptypes "github.com/aws/aws-sdk-go-v2/service/verifiedpermissions/types"
	sharedavp "github.com/mikecbrant/verified-permissions-authorizer/internal/common"
	sharedassets "github.com/mikecbrant/verified-permissions-authorizer/internal/common/assets"
)

var _ resource.Resource = (*authorizerResource)(nil)
var _ resource.ResourceWithImportState = (*authorizerResource)(nil)

func NewAuthorizerResource() resource.Resource { return &authorizerResource{} }

type authorizerResource struct{}

type authorizerModel struct {
	ID                  types.String              `tfsdk:"id"`
	Description         types.String              `tfsdk:"description"`
	RetainOnDelete      types.Bool                `tfsdk:"retain_on_delete"`
	Lambda              *LambdaBlock              `tfsdk:"lambda"`
	Dynamo              *DynamoBlock              `tfsdk:"dynamo"`
	Cognito             *CognitoBlock             `tfsdk:"cognito"`
	VerifiedPermissions *VerifiedPermissionsBlock `tfsdk:"verified_permissions"`

	// Outputs
	PolicyStoreId  types.String `tfsdk:"policy_store_id"`
	PolicyStoreArn types.String `tfsdk:"policy_store_arn"`
	Parameters     types.Map    `tfsdk:"parameters"`

	// Grouped outputs
	LambdaAuthorizerArn      types.String `tfsdk:"lambda_authorizer_arn"`
	LambdaRoleArn            types.String `tfsdk:"lambda_role_arn"`
	DynamoTableArn           types.String `tfsdk:"dynamo_table_arn"`
	DynamoStreamArn          types.String `tfsdk:"dynamo_stream_arn"`
	CognitoUserPoolId        types.String `tfsdk:"cognito_user_pool_id"`
	CognitoUserPoolArn       types.String `tfsdk:"cognito_user_pool_arn"`
	CognitoUserPoolClientIds types.List   `tfsdk:"cognito_user_pool_client_ids"`
}

func (r *authorizerResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_authorizer"
}

func (r *authorizerResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Provision an AWS Verified Permissions Policy Store and a bundled Lambda Request Authorizer; optionally Cognito and schema/policy ingestion.",
		Attributes: map[string]schema.Attribute{
			"id":               schema.StringAttribute{Computed: true, PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()}},
			"description":      schema.StringAttribute{Optional: true},
			"retain_on_delete": schema.BoolAttribute{Optional: true},
			// Outputs
			"policy_store_id":              schema.StringAttribute{Computed: true},
			"policy_store_arn":             schema.StringAttribute{Computed: true},
			"parameters":                   schema.MapAttribute{Computed: true, ElementType: types.StringType},
			"lambda_authorizer_arn":        schema.StringAttribute{Computed: true},
			"lambda_role_arn":              schema.StringAttribute{Computed: true},
			"dynamo_table_arn":             schema.StringAttribute{Computed: true},
			"dynamo_stream_arn":            schema.StringAttribute{Computed: true},
			"cognito_user_pool_id":         schema.StringAttribute{Computed: true},
			"cognito_user_pool_arn":        schema.StringAttribute{Computed: true},
			"cognito_user_pool_client_ids": schema.ListAttribute{Computed: true, ElementType: types.StringType},
		},
		Blocks: map[string]schema.Block{
			"lambda": schema.SingleNestedBlock{
				Attributes: map[string]schema.Attribute{
					"memory_size":             schema.Int64Attribute{Optional: true},
					"reserved_concurrency":    schema.Int64Attribute{Optional: true},
					"provisioned_concurrency": schema.Int64Attribute{Optional: true},
				},
			},
			"dynamo": schema.SingleNestedBlock{
				Attributes: map[string]schema.Attribute{
					"enable_dynamo_db_stream": schema.BoolAttribute{Optional: true},
				},
			},
			"cognito": schema.SingleNestedBlock{
				Attributes: map[string]schema.Attribute{
					"sign_in_aliases": schema.ListAttribute{Optional: true, ElementType: types.StringType},
				},
				Blocks: map[string]schema.Block{
					"ses_config": schema.SingleNestedBlock{
						Attributes: map[string]schema.Attribute{
							"source_arn":        schema.StringAttribute{Required: true},
							"from":              schema.StringAttribute{Required: true},
							"reply_to_email":    schema.StringAttribute{Optional: true},
							"configuration_set": schema.StringAttribute{Optional: true},
						},
					},
				},
			},
			"verified_permissions": schema.SingleNestedBlock{
				Attributes: map[string]schema.Attribute{
					"schema_file":              schema.StringAttribute{Optional: true},
					"policy_dir":               schema.StringAttribute{Optional: true},
					"action_group_enforcement": schema.StringAttribute{Optional: true},
					"disable_guardrails":       schema.BoolAttribute{Optional: true},
					"canary_file":              schema.StringAttribute{Optional: true},
				},
			},
		},
	}
}

func (r *authorizerResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan authorizerModel
	diags := req.Plan.Get(ctx, &plan)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
	// Defaults
	mem := int64(128)
	rc := int64(1)
	pc := int64(0)
	if plan.Lambda != nil {
		if !plan.Lambda.MemorySize.IsNull() {
			mem = plan.Lambda.MemorySize.ValueInt64()
		}
		if !plan.Lambda.ReservedConcurrency.IsNull() {
			rc = plan.Lambda.ReservedConcurrency.ValueInt64()
		}
		if !plan.Lambda.ProvisionedConcurrency.IsNull() {
			pc = plan.Lambda.ProvisionedConcurrency.ValueInt64()
		}
	}
	if pc > 0 && rc < pc {
		resp.Diagnostics.AddError("Invalid lambda concurrency settings", fmt.Sprintf("provisioned_concurrency (%d) must be <= reserved_concurrency (%d)", pc, rc))
		return
	}

	cfg, err := awscfg.LoadDefaultConfig(ctx)
	if err != nil {
		resp.Diagnostics.AddError("AWS config error", err.Error())
		return
	}
	region := cfg.Region

	// 1) Policy store
	vp := verifiedpermissions.NewFromConfig(cfg)
	psOut, err := vp.CreatePolicyStore(ctx, &verifiedpermissions.CreatePolicyStoreInput{ValidationSettings: &vptypes.ValidationSettings{Mode: vptypes.ValidationModeStrict}})
	if err != nil {
		resp.Diagnostics.AddError("Create policy store failed", err.Error())
		return
	}
	psId := awsStringValue(psOut.PolicyStoreId)
	psArn := awsStringValue(psOut.Arn)
	if strings.TrimSpace(psId) == "" || strings.TrimSpace(psArn) == "" {
		resp.Diagnostics.AddError("Create policy store failed", "missing policy store identifiers from CreatePolicyStore response")
		return
	}

	// 2) DynamoDB table
	ddb := dynamodb.NewFromConfig(cfg)
	tableName := fmt.Sprintf("%s-tenant-%d", "vpa", time.Now().Unix())
	_, err = ddb.CreateTable(ctx, &dynamodb.CreateTableInput{
		TableName:   &tableName,
		BillingMode: dynamodbtypes.BillingModePayPerRequest,
		AttributeDefinitions: []dynamodbtypes.AttributeDefinition{
			{AttributeName: awsString("PK"), AttributeType: dynamodbtypes.ScalarAttributeTypeS},
			{AttributeName: awsString("SK"), AttributeType: dynamodbtypes.ScalarAttributeTypeS},
			{AttributeName: awsString("GSI1PK"), AttributeType: dynamodbtypes.ScalarAttributeTypeS},
			{AttributeName: awsString("GSI1SK"), AttributeType: dynamodbtypes.ScalarAttributeTypeS},
			{AttributeName: awsString("GSI2PK"), AttributeType: dynamodbtypes.ScalarAttributeTypeS},
			{AttributeName: awsString("GSI2SK"), AttributeType: dynamodbtypes.ScalarAttributeTypeS},
		},
		KeySchema: []dynamodbtypes.KeySchemaElement{{AttributeName: awsString("PK"), KeyType: dynamodbtypes.KeyTypeHash}, {AttributeName: awsString("SK"), KeyType: dynamodbtypes.KeyTypeRange}},
		GlobalSecondaryIndexes: []dynamodbtypes.GlobalSecondaryIndex{
			{IndexName: awsString("GSI1"), KeySchema: []dynamodbtypes.KeySchemaElement{{AttributeName: awsString("GSI1PK"), KeyType: dynamodbtypes.KeyTypeHash}, {AttributeName: awsString("GSI1SK"), KeyType: dynamodbtypes.KeyTypeRange}}, Projection: &dynamodbtypes.Projection{ProjectionType: dynamodbtypes.ProjectionTypeAll}},
			{IndexName: awsString("GSI2"), KeySchema: []dynamodbtypes.KeySchemaElement{{AttributeName: awsString("GSI2PK"), KeyType: dynamodbtypes.KeyTypeHash}, {AttributeName: awsString("GSI2SK"), KeyType: dynamodbtypes.KeyTypeRange}}, Projection: &dynamodbtypes.Projection{ProjectionType: dynamodbtypes.ProjectionTypeAll}},
		},
	})
	if err != nil {
		resp.Diagnostics.AddError("Create DynamoDB table failed", err.Error())
		return
	}

	// 3) IAM role
	iamc := iam.NewFromConfig(cfg)
	assume := `{"Version":"2012-10-17","Statement":[{"Effect":"Allow","Principal":{"Service":["lambda.amazonaws.com"]},"Action":["sts:AssumeRole"]}]}`
	roleOut, err := iamc.CreateRole(ctx, &iam.CreateRoleInput{AssumeRolePolicyDocument: &assume, RoleName: awsString(fmt.Sprintf("vpa-role-%d", time.Now().Unix()))})
	if err != nil {
		resp.Diagnostics.AddError("Create IAM role failed", err.Error())
		return
	}
	roleArn := *roleOut.Role.Arn

	// Attach basic execution role
	if _, err := iamc.AttachRolePolicy(ctx, &iam.AttachRolePolicyInput{RoleName: roleOut.Role.RoleName, PolicyArn: awsString("arn:aws:iam::aws:policy/service-role/AWSLambdaBasicExecutionRole")}); err != nil {
		resp.Diagnostics.AddError("Attach IAM role policy failed", fmt.Sprintf("policy=%s role=%s err=%v", "AWSLambdaBasicExecutionRole", *roleOut.Role.RoleName, err))
		return
	}

	// 4) Lambda function from embedded JS
	zbuf := new(bytes.Buffer)
	zw := zip.NewWriter(zbuf)
	f, err := zw.Create("index.mjs")
	if err != nil {
		resp.Diagnostics.AddError("zip create failed", err.Error())
		return
	}
	if _, err := f.Write([]byte(sharedassets.GetAuthorizerIndexMjs())); err != nil {
		resp.Diagnostics.AddError("zip write failed", err.Error())
		return
	}
	if err := zw.Close(); err != nil {
		resp.Diagnostics.AddError("zip close failed", err.Error())
		return
	}

	lamb := lambda.NewFromConfig(cfg)
	fnName := fmt.Sprintf("vpa-authorizer-%d", time.Now().Unix())
	publish := pc > 0
	fnOut, err := lamb.CreateFunction(ctx, &lambda.CreateFunctionInput{
		FunctionName:  &fnName,
		Role:          &roleArn,
		Runtime:       lambdatypes.RuntimeNodejs20x,
		Handler:       awsString("index.handler"),
		Code:          &lambdatypes.FunctionCode{ZipFile: zbuf.Bytes()},
		Architectures: []lambdatypes.Architecture{lambdatypes.ArchitectureArm64},
		Timeout:       awsInt32(10),
		MemorySize:    awsInt32(int32(mem)),
		Publish:       publish,
		Environment:   &lambdatypes.Environment{Variables: map[string]string{"POLICY_STORE_ID": psId}},
	})
	if err != nil {
		resp.Diagnostics.AddError("Create Lambda failed", err.Error())
		return
	}

	// 5) Optionally apply schema/policies and guardrails
	if plan.VerifiedPermissions != nil {
		vpPlan := plan.VerifiedPermissions
		// Resolve inputs (with defaults)
		schemaPath := strings.TrimSpace(strOrDefault(vpPlan.SchemaFile.ValueString(), "./authorizer/schema.yaml"))
		policyDir := strings.TrimSpace(strOrDefault(vpPlan.PolicyDir.ValueString(), "./authorizer/policies"))
		if !filepath.IsAbs(schemaPath) {
			cwd, err := os.Getwd()
			if err != nil {
				resp.Diagnostics.AddError("cwd error", err.Error())
				return
			}
			schemaPath = filepath.Join(cwd, schemaPath)
		}
		if !filepath.IsAbs(policyDir) {
			cwd, err := os.Getwd()
			if err != nil {
				resp.Diagnostics.AddError("cwd error", err.Error())
				return
			}
			policyDir = filepath.Join(cwd, policyDir)
		}
		if st, err := os.Stat(policyDir); err != nil || !st.IsDir() {
			resp.Diagnostics.AddError("Invalid verified_permissions.policy_dir", fmt.Sprintf("%q not found or not a directory", policyDir))
			return
		}

		cedarJSON, _, actions, warns, err := sharedavp.LoadAndValidateSchema(schemaPath)
		if err != nil {
			resp.Diagnostics.AddError("Schema error", err.Error())
			return
		}
		for _, w := range warns {
			resp.Diagnostics.AddWarning("AVP", w)
		}
		agMode := strings.ToLower(strings.TrimSpace(vpPlan.ActionGroupEnforcement.ValueString()))
		if agMode == "" {
			agMode = "error"
		}
		if violations, err := sharedavp.EnforceActionGroups(actions, agMode); err != nil {
			resp.Diagnostics.AddError("Action group enforcement", err.Error())
			return
		} else if len(violations) > 0 && agMode == "warn" {
			resp.Diagnostics.AddWarning("AVP", fmt.Sprintf("actions not aligned to canonical action groups: %s", strings.Join(violations, ", ")))
		}
		// Apply schema if changed
		if err := sharedavp.PutSchemaIfChanged(ctx, psId, cedarJSON, region); err != nil {
			resp.Diagnostics.AddError("Put schema failed", err.Error())
			return
		}
		files, err := sharedavp.CollectPolicyFiles(policyDir)
		if err != nil {
			resp.Diagnostics.AddError("Policy discovery failed", err.Error())
			return
		}
		// Policies are created through TF resources only in future iterations; here we just sanity check
		_ = files
	}

	// Outputs
	if di := resp.State.SetAttribute(ctx, path.Root("id"), types.StringValue(psId)); di.HasError() {
		resp.Diagnostics.Append(di...)
		return
	}
	if di := resp.State.SetAttribute(ctx, path.Root("policy_store_id"), types.StringValue(psId)); di.HasError() {
		resp.Diagnostics.Append(di...)
		return
	}
	if di := resp.State.SetAttribute(ctx, path.Root("policy_store_arn"), types.StringValue(psArn)); di.HasError() {
		resp.Diagnostics.Append(di...)
		return
	}
	if di := resp.State.SetAttribute(ctx, path.Root("lambda_authorizer_arn"), types.StringValue(*fnOut.FunctionArn)); di.HasError() {
		resp.Diagnostics.Append(di...)
		return
	}
	if di := resp.State.SetAttribute(ctx, path.Root("lambda_role_arn"), types.StringValue(roleArn)); di.HasError() {
		resp.Diagnostics.Append(di...)
		return
	}
	// Resolve and set DynamoDB table ARN from AWS to ensure correctness
	ddesc, err := ddb.DescribeTable(ctx, &dynamodb.DescribeTableInput{TableName: &tableName})
	if err != nil {
		resp.Diagnostics.AddError("Describe DynamoDB table failed", err.Error())
		return
	}
	if di := resp.State.SetAttribute(ctx, path.Root("dynamo_table_arn"), types.StringValue(*ddesc.Table.TableArn)); di.HasError() {
		resp.Diagnostics.Append(di...)
		return
	}
}

func (r *authorizerResource) Read(_ context.Context, _ resource.ReadRequest, _ *resource.ReadResponse) {
}
func (r *authorizerResource) Update(_ context.Context, _ resource.UpdateRequest, _ *resource.UpdateResponse) {
}
func (r *authorizerResource) Delete(_ context.Context, _ resource.DeleteRequest, _ *resource.DeleteResponse) {
}
func (r *authorizerResource) ImportState(_ context.Context, _ resource.ImportStateRequest, _ *resource.ImportStateResponse) {
}

func awsString(s string) *string { return &s }
func awsInt32(v int32) *int32    { return &v }

// awsStringValue safely dereferences an AWS SDK *string, returning an empty string when nil.
func awsStringValue(p *string) string {
	if p == nil {
		return ""
	}
	return *p
}

func strOrDefault(s string, def string) string {
	if strings.TrimSpace(s) == "" {
		return def
	}
	return s
}
