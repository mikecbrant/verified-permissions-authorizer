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
    sharedassets "github.com/mikecbrant/verified-permissions-authorizer/providers/internal/assets"
    sharedavp "github.com/mikecbrant/verified-permissions-authorizer/providers/internal/avp"
    "github.com/hashicorp/terraform-plugin-framework/path"
)

var _ resource.Resource = (*authorizerResource)(nil)
var _ resource.ResourceWithImportState = (*authorizerResource)(nil)

func NewAuthorizerResource() resource.Resource { return &authorizerResource{} }

type authorizerResource struct{}

type authorizerModel struct {
    ID             types.String `tfsdk:"id"`
    Description    types.String `tfsdk:"description"`
    RetainOnDelete types.Bool   `tfsdk:"retain_on_delete"`
    Lambda         *LambdaBlock `tfsdk:"lambda"`
    Dynamo         *DynamoBlock `tfsdk:"dynamo"`
    Cognito        *CognitoBlock `tfsdk:"cognito"`
    VerifiedPermissions *VerifiedPermissionsBlock `tfsdk:"verified_permissions"`

    // Outputs
    PolicyStoreId  types.String `tfsdk:"policy_store_id"`
    PolicyStoreArn types.String `tfsdk:"policy_store_arn"`
    Parameters     types.Map    `tfsdk:"parameters"`

    // Grouped outputs
    LambdaAuthorizerArn types.String `tfsdk:"lambda_authorizer_arn"`
    LambdaRoleArn       types.String `tfsdk:"lambda_role_arn"`
    DynamoTableArn      types.String `tfsdk:"dynamo_table_arn"`
    DynamoStreamArn     types.String `tfsdk:"dynamo_stream_arn"`
    CognitoUserPoolId   types.String `tfsdk:"cognito_user_pool_id"`
    CognitoUserPoolArn  types.String `tfsdk:"cognito_user_pool_arn"`
    CognitoUserPoolClientIds types.List `tfsdk:"cognito_user_pool_client_ids"`
}

func (r *authorizerResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
    resp.TypeName = req.ProviderTypeName + "_authorizer"
}

func (r *authorizerResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
    resp.Schema = schema.Schema{
        Description: "Provision an AWS Verified Permissions Policy Store and a bundled Lambda Request Authorizer; optionally Cognito and schema/policy ingestion.",
        Attributes: map[string]schema.Attribute{
            "id": schema.StringAttribute{Computed: true, PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()}},
            "description": schema.StringAttribute{Optional: true},
            "retain_on_delete": schema.BoolAttribute{Optional: true},
            // Outputs
            "policy_store_id": schema.StringAttribute{Computed: true},
            "policy_store_arn": schema.StringAttribute{Computed: true},
            "parameters": schema.MapAttribute{Computed: true, ElementType: types.StringType},
            "lambda_authorizer_arn": schema.StringAttribute{Computed: true},
            "lambda_role_arn": schema.StringAttribute{Computed: true},
            "dynamo_table_arn": schema.StringAttribute{Computed: true},
            "dynamo_stream_arn": schema.StringAttribute{Computed: true},
            "cognito_user_pool_id": schema.StringAttribute{Computed: true},
            "cognito_user_pool_arn": schema.StringAttribute{Computed: true},
            "cognito_user_pool_client_ids": schema.ListAttribute{Computed: true, ElementType: types.StringType},
        },
        Blocks: map[string]schema.Block{
            "lambda": schema.SingleNestedBlock{
                Optional: true,
                Attributes: map[string]schema.Attribute{
                    "memory_size": schema.Int64Attribute{Optional: true},
                    "reserved_concurrency": schema.Int64Attribute{Optional: true},
                    "provisioned_concurrency": schema.Int64Attribute{Optional: true},
                },
            },
            "dynamo": schema.SingleNestedBlock{
                Optional: true,
                Attributes: map[string]schema.Attribute{
                    "enable_dynamo_db_stream": schema.BoolAttribute{Optional: true},
                },
            },
            "cognito": schema.SingleNestedBlock{
                Optional: true,
                Attributes: map[string]schema.Attribute{
                    "sign_in_aliases": schema.ListAttribute{Optional: true, ElementType: types.StringType},
                },
                Blocks: map[string]schema.Block{
                    "ses_config": schema.SingleNestedBlock{
                        Optional: true,
                        Attributes: map[string]schema.Attribute{
                            "source_arn": schema.StringAttribute{Required: true},
                            "from": schema.StringAttribute{Required: true},
                            "reply_to_email": schema.StringAttribute{Optional: true},
                            "configuration_set": schema.StringAttribute{Optional: true},
                        },
                    },
                },
            },
            "verified_permissions": schema.SingleNestedBlock{
                Optional: true,
                Attributes: map[string]schema.Attribute{
                    "schema_file": schema.StringAttribute{Optional: true},
                    "policy_dir": schema.StringAttribute{Optional: true},
                    "action_group_enforcement": schema.StringAttribute{Optional: true},
                    "disable_guardrails": schema.BoolAttribute{Optional: true},
                    "canary_file": schema.StringAttribute{Optional: true},
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
        if !plan.Lambda.MemorySize.IsNull() { mem = plan.Lambda.MemorySize.ValueInt64() }
        if !plan.Lambda.ReservedConcurrency.IsNull() { rc = plan.Lambda.ReservedConcurrency.ValueInt64() }
        if !plan.Lambda.ProvisionedConcurrency.IsNull() { pc = plan.Lambda.ProvisionedConcurrency.ValueInt64() }
    }
    if pc > 0 && rc < pc {
        resp.Diagnostics.AddError("Invalid lambda concurrency settings", fmt.Sprintf("provisioned_concurrency (%d) must be <= reserved_concurrency (%d)", pc, rc))
        return
    }

    cfg, err := awscfg.LoadDefaultConfig(ctx)
    if err != nil { resp.Diagnostics.AddError("AWS config error", err.Error()); return }
    region := cfg.Region

    // 1) Policy store
    vp := verifiedpermissions.NewFromConfig(cfg)
    psOut, err := vp.CreatePolicyStore(ctx, &verifiedpermissions.CreatePolicyStoreInput{ValidationSettings: &vptypes.ValidationSettings{Mode: vptypes.ValidationModeStrict}})
    if err != nil { resp.Diagnostics.AddError("Create policy store failed", err.Error()); return }
    psId := *psOut.PolicyStore.Id
    psArn := *psOut.PolicyStore.Arn

    // 2) DynamoDB table
    ddb := dynamodb.NewFromConfig(cfg)
    tableName := fmt.Sprintf("%s-tenant-%d", "vpa", time.Now().Unix())
    _, err = ddb.CreateTable(ctx, &dynamodb.CreateTableInput{
        TableName: &tableName,
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
    if err != nil { resp.Diagnostics.AddError("Create DynamoDB table failed", err.Error()); return }

    // 3) IAM role
    iamc := iam.NewFromConfig(cfg)
    assume := `{"Version":"2012-10-17","Statement":[{"Effect":"Allow","Principal":{"Service":["lambda.amazonaws.com"]},"Action":["sts:AssumeRole"]}]}`
    roleOut, err := iamc.CreateRole(ctx, &iam.CreateRoleInput{AssumeRolePolicyDocument: &assume, RoleName: awsString(fmt.Sprintf("vpa-role-%d", time.Now().Unix()))})
    if err != nil { resp.Diagnostics.AddError("Create IAM role failed", err.Error()); return }
    roleArn := *roleOut.Role.Arn

    // Attach basic execution role
    _, _ = iamc.AttachRolePolicy(ctx, &iam.AttachRolePolicyInput{RoleName: roleOut.Role.RoleName, PolicyArn: awsString("arn:aws:iam::aws:policy/service-role/AWSLambdaBasicExecutionRole")})

    // 4) Lambda function from embedded JS
    zbuf := new(bytes.Buffer)
    zw := zip.NewWriter(zbuf)
    f, _ := zw.Create("index.mjs")
    _, _ = f.Write([]byte(sharedassets.GetAuthorizerIndexMjs()))
    _ = zw.Close()

    lamb := lambda.NewFromConfig(cfg)
    fnName := fmt.Sprintf("vpa-authorizer-%d", time.Now().Unix())
    publish := pc > 0
    fnOut, err := lamb.CreateFunction(ctx, &lambda.CreateFunctionInput{
        FunctionName: &fnName,
        Role:         &roleArn,
        Runtime:      lambdatypes.RuntimeNodejs22x,
        Handler:      awsString("index.handler"),
        Code:         &lambdatypes.FunctionCode{ZipFile: zbuf.Bytes()},
        Architectures: []lambdatypes.Architecture{lambdatypes.ArchitectureArm64},
        Timeout:      awsInt32(10),
        MemorySize:   awsInt32(int32(mem)),
        Publish:      &publish,
        Environment:  &lambdatypes.Environment{Variables: map[string]string{"POLICY_STORE_ID": psId}},
    })
    if err != nil { resp.Diagnostics.AddError("Create Lambda failed", err.Error()); return }
    fnArn := *fnOut.FunctionArn

    // TODO: provisioned concurrency + alias when pc > 0 (deferred to follow-up to keep initial scope manageable)

    // 5) Apply schema and policies
    if plan.VerifiedPermissions != nil {
        schemaPath := strings.TrimSpace(defaultStr(plan.VerifiedPermissions.SchemaFile, "./authorizer/schema.yaml"))
        policyDir := strings.TrimSpace(defaultStr(plan.VerifiedPermissions.PolicyDir, "./authorizer/policies"))
        cedarJSON, ns, actions, _, err := sharedavp.LoadAndValidateSchema(absPath(schemaPath))
        if err != nil { resp.Diagnostics.AddError("Schema validation failed", err.Error()); return }
        mode := strings.ToLower(defaultStr(plan.VerifiedPermissions.ActionGroupEnforcement, "error"))
        if _, err := sharedavp.EnforceActionGroups(actions, mode); err != nil { resp.Diagnostics.AddError("Action group enforcement", err.Error()); return }
        if err := sharedavp.PutSchemaIfChanged(ctx, psId, cedarJSON, region); err != nil { resp.Diagnostics.AddError("Put schema failed", err.Error()); return }
        files, err := sharedavp.CollectPolicyFiles(absPath(policyDir))
        if err != nil { resp.Diagnostics.AddError("Collect policies failed", err.Error()); return }
        for i, p := range files {
            b, err := os.ReadFile(p)
            if err != nil { resp.Diagnostics.AddError("Read policy failed", err.Error()); return }
            _, err = vp.CreatePolicy(ctx, &verifiedpermissions.CreatePolicyInput{
                PolicyStoreId: &psId,
                Definition: &vptypes.PolicyDefinition{Static: &vptypes.StaticPolicyDefinition{Statement: awsString(string(b))}},
            })
            if err != nil { resp.Diagnostics.AddError("CreatePolicy failed", fmt.Sprintf("file %s: %v", p, err)); return }
        }
        // Optional canaries
        canaryFile := strings.TrimSpace(plan.VerifiedPermissions.CanaryFile.ValueString())
        if canaryFile == "" {
            def := "./authorizer/canaries.yaml"
            if _, err := os.Stat(def); err == nil { canaryFile = def }
        }
        if canaryFile != "" {
            if err := sharedavp.RunCombinedCanaries(ctx, region, psId, absPath(canaryFile), mode); err != nil {
                resp.Diagnostics.AddError("Canaries failed", err.Error()); return
            }
        }
        _ = ns
    }

    // Outputs
    plan.ID = types.StringValue(psId)
    plan.PolicyStoreId = types.StringValue(psId)
    plan.PolicyStoreArn = types.StringValue(psArn)
    plan.LambdaAuthorizerArn = types.StringValue(fnArn)
    plan.LambdaRoleArn = types.StringValue(roleArn)
    // NOTE: Dynamo outputs omitted in this first cut until DescribeTable available

    diags = resp.State.Set(ctx, &plan)
    resp.Diagnostics.Append(diags...)
}

func (r *authorizerResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
    // Best-effort no-op; full drift detection can be added later
    var state authorizerModel
    diags := req.State.Get(ctx, &state)
    resp.Diagnostics.Append(diags...)
    if resp.Diagnostics.HasError() { return }
    // Nothing to do; rely on stored state for now
    resp.State.Set(ctx, &state)
}

func (r *authorizerResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
    // Force recreation for now to keep behavior predictable
    var plan authorizerModel
    _ = req.Plan.Get(ctx, &plan)
    resp.Diagnostics.AddWarning("Recreate on update", "Changes require resource recreation; please taint to force replacement if needed.")
    resp.State.Set(ctx, &plan)
}

func (r *authorizerResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
    var state authorizerModel
    diags := req.State.Get(ctx, &state)
    resp.Diagnostics.Append(diags...)
    if resp.Diagnostics.HasError() { return }
    cfg, err := awscfg.LoadDefaultConfig(ctx)
    if err != nil { resp.Diagnostics.AddError("AWS config error", err.Error()); return }
    vp := verifiedpermissions.NewFromConfig(cfg)
    _, _ = vp.DeletePolicyStore(ctx, &verifiedpermissions.DeletePolicyStoreInput{PolicyStoreId: strPtr(state.PolicyStoreId.ValueString())})
    lamb := lambda.NewFromConfig(cfg)
    if !state.LambdaAuthorizerArn.IsNull() {
        // Name is last segment of ARN
        parts := strings.Split(state.LambdaAuthorizerArn.ValueString(), ":")
        name := parts[len(parts)-1]
        _, _ = lamb.DeleteFunction(ctx, &lambda.DeleteFunctionInput{FunctionName: &name})
    }
    // No further deletes; allow GC by customer if needed
}

func (r *authorizerResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
    resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}

// Helpers
func awsString(s string) *string { return &s }
func awsInt32(v int32) *int32 { return &v }
func strPtr(s string) *string { return &s }
func defaultStr(v types.String, def string) string { if v.IsNull() || v.ValueString() == "" { return def } ; return v.ValueString() }
func absPath(p string) string { if filepath.IsAbs(p) { return p }; cwd, _ := os.Getwd(); return filepath.Join(cwd, p) }
