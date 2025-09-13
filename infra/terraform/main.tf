terraform {
  cloud {
    organization = "mikecbrant"
    workspaces {
      name = "vp-authorizer"
    }
  }

  required_providers {
    vpauthorizer = {
      source  = "mikecbrant/vpauthorizer"
      version = ">= 0.1.0"
    }
  }
}

# Provider uses AWS SDK default credentials chain; set AWS_REGION/AWS_PROFILE in your env.
provider "vpauthorizer" {}

resource "vpauthorizer_authorizer" "main" {
  description = "Example AVP stack via Terraform"

  verified_permissions {
    # Resolve assets from the shared infra/authorizer directory
    schema_file = "../authorizer/schema.yaml"
    policy_dir  = "../authorizer/policies"

    # Optional: run canaries after schema/policy ingestion
    canary_file = "../authorizer/canaries.yaml"

    # Enforce canonical action groups strictly
    action_group_enforcement = "error"
  }
}

output "policy_store_id" {
  value = vpauthorizer_authorizer.main.policy_store_id
}
output "lambda_authorizer_arn" {
  value = vpauthorizer_authorizer.main.lambda_authorizer_arn
}
