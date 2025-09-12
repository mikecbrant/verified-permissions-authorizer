terraform {
  required_providers {
    vpauthorizer = {
      source  = "mikecbrant/vpauthorizer"
      version = ">= 0.1.0"
    }
  }
}

provider "vpauthorizer" {}

resource "vpauthorizer_authorizer" "main" {
  verified_permissions {
    schema_file = "./authorizer/schema.yaml"
    policy_dir  = "./authorizer/policies"
  }
}
