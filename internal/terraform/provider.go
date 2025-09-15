package provider

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/provider"
	"github.com/hashicorp/terraform-plugin-framework/provider/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// Ensure implementation satisfies expected interfaces
var _ provider.Provider = (*vpaProvider)(nil)

type vpaProvider struct {
	version string
}

// New returns a provider factory closure with the given version string.
func New(version string) func() provider.Provider {
	return func() provider.Provider {
		return &vpaProvider{version: version}
	}
}

func (p *vpaProvider) Metadata(_ context.Context, _ provider.MetadataRequest, resp *provider.MetadataResponse) {
	resp.TypeName = "vpauthorizer"
	resp.Version = p.version
}

func (p *vpaProvider) Schema(_ context.Context, _ provider.SchemaRequest, resp *provider.SchemaResponse) {
	// No provider-level configuration; all config is on the resource per spec
	resp.Schema = schema.Schema{
		Attributes: map[string]schema.Attribute{},
	}
}

func (p *vpaProvider) Configure(ctx context.Context, req provider.ConfigureRequest, resp *provider.ConfigureResponse) {
	// Nothing; AWS config is discovered via default chain in the resource implementation
}

func (p *vpaProvider) Resources(_ context.Context) []func() resource.Resource {
	return []func() resource.Resource{
		NewAuthorizerResource,
	}
}

func (p *vpaProvider) DataSources(_ context.Context) []func() datasource.DataSource {
	return nil
}

// Common types used in the resource schema
type (
    // LambdaBlock captures optional Lambda configuration inputs.
    LambdaBlock struct {
        MemorySize             types.Int64 `tfsdk:"memory_size"`
        ReservedConcurrency    types.Int64 `tfsdk:"reserved_concurrency"`
        ProvisionedConcurrency types.Int64 `tfsdk:"provisioned_concurrency"`
    }
    // DynamoBlock captures DynamoDB options.
    DynamoBlock struct {
        EnableDynamoDbStream types.Bool `tfsdk:"enable_dynamo_db_stream"`
    }
    // CognitoSesBlock defines SES email options for Cognito.
    CognitoSesBlock struct {
        SourceArn        types.String `tfsdk:"source_arn"`
        From             types.String `tfsdk:"from"`
        ReplyToEmail     types.String `tfsdk:"reply_to_email"`
        ConfigurationSet types.String `tfsdk:"configuration_set"`
    }
    // CognitoBlock captures Cognito configuration.
    CognitoBlock struct {
        SignInAliases types.List       `tfsdk:"sign_in_aliases"`
        SesConfig     *CognitoSesBlock `tfsdk:"ses_config"`
    }
    // VerifiedPermissionsBlock configures AVP schema/policies/guardrails.
    VerifiedPermissionsBlock struct {
        SchemaFile             types.String `tfsdk:"schema_file"`
        PolicyDir              types.String `tfsdk:"policy_dir"`
        ActionGroupEnforcement types.String `tfsdk:"action_group_enforcement"`
        DisableGuardrails      types.Bool   `tfsdk:"disable_guardrails"`
        CanaryFile             types.String `tfsdk:"canary_file"`
    }
)
