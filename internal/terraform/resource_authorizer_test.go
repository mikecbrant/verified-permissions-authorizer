package provider

import (
	"os"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/providerserver"
	"github.com/hashicorp/terraform-plugin-go/tfprotov6"
	tftest "github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

func TestAcc_Authorizer_basic(t *testing.T) {
	if os.Getenv("TF_ACC") == "" {
		t.Skip("set TF_ACC to run acceptance tests (requires AWS credentials)")
	}
	cfg := `
provider "vpauthorizer" {}
resource "vpauthorizer_authorizer" "test" {
  verified_permissions {
    schema_file = "../../../../infra/authorizer/schema.yaml"
    policy_dir  = "../../../../infra/authorizer/policies"
  }
}
`
	tftest.Test(t, tftest.TestCase{
		ProtoV6ProviderFactories: map[string]func() (tfprotov6.ProviderServer, error){
			"vpauthorizer": providerserver.NewProtocol6WithError(New("dev")()),
		},
		Steps: []tftest.TestStep{{
			Config: cfg,
			Check: tftest.ComposeAggregateTestCheckFunc(
				tftest.TestCheckResourceAttrSet("vpauthorizer_authorizer.test", "policy_store_id"),
			),
		}},
	})
}
