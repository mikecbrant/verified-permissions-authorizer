# vpauthorizer provider

Source: `mikecbrant/vpauthorizer`

The vpauthorizer provider provisions an opinionated AWS Verified Permissions authorizer stack as a single high-level resource. It creates a Verified Permissions Policy Store (STRICT validation), a bundled Lambda Request Authorizer, a provider‑managed DynamoDB table, optional Cognito User Pool + Verified Permissions Identity Source, and ingests AVP schema/policies with validations and optional canaries.

Use the resource `vpauthorizer_authorizer`.

## Example usage

```hcl
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
  description      = "Authorizer for demo"

  lambda {
    memory_size          = 256
    reserved_concurrency = 2
    provisioned_concurrency = 1
  }

  dynamo {
    enable_dynamo_db_stream = true
  }

  verified_permissions {
    schema_file  = "./authorizer/schema.yaml"
    policy_dir   = "./authorizer/policies"
    action_group_enforcement = "error"
    canary_file  = "./authorizer/canaries.yaml"
  }
}
```

## Authentication

This provider uses the standard AWS SDK default credential chain for AWS API calls (environment, shared config/credentials files, IAM role). There is no provider‑specific authentication or configuration.

## Resources

- `vpauthorizer_authorizer` — provisions the authorizer stack.
