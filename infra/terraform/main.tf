terraform {
  cloud {
    organization = "mikecbrant" # also set in GHA as vars.TFC_ORG
    workspaces { name = "vp-authorizer" } # also set in GHA as vars.TFC_WORKSPACE
  }

  required_providers {
    vpauthorizer = {
      source  = "mikecbrant/vpauthorizer"
      version = ">= 0.1.0"
    }
  }
}

provider "vpauthorizer" {}

resource "vpauthorizer_authorizer" "example" {
  description = "Example authorizer (Terraform)"

  lambda {
    memory_size          = 256
    reserved_concurrency = 1
  }

  # Use the same example assets as the Pulumi stack (../shared)
  verified_permissions {
    schema_file = "../shared/schema.yaml"
    policy_dir  = "../shared/policies"
    canary_file = "../shared/canaries.yaml"
  }
}

output "policy_store_id" { value = vpauthorizer_authorizer.example.policy_store_id }
