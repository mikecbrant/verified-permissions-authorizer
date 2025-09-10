package provider

import (
    "os"
    "testing"

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
        ProtoV6ProviderFactories: map[string]func() (any, error){
            "vpauthorizer": func() (any, error) { return New("dev")(), nil },
        },
        Steps: []tftest.TestStep{{
            Config: cfg,
            Check: tftest.ComposeAggregateTestCheckFunc(
                tftest.TestCheckResourceAttrSet("vpauthorizer_authorizer.test", "policy_store_id"),
            ),
        }},
    })
}
