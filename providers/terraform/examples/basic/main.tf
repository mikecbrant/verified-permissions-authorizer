terraform {
  required_providers {
    vpauthorizer = {
      source  = "mikecbrant/vpauthorizer"
      version = ">= 0.1.0"
    }
  }
}

provider "vpauthorizer" {}

resource "vpauthorizer_authorizer" "example" {
  description = "Example authorizer"

  lambda {
    memory_size          = 256
    reserved_concurrency = 1
  }

  verified_permissions {
    schema_file  = "../../../../infra/authorizer/schema.yaml"
    policy_dir   = "../../../../infra/authorizer/policies"
    canary_file  = "../../../../infra/authorizer/canaries.yaml"
  }
}

output "policy_store_id" { value = vpauthorizer_authorizer.example.policy_store_id }
