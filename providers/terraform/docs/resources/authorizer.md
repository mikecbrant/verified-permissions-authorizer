# vpauthorizer_authorizer (resource)

Provision an AWS Verified Permissions Policy Store and a bundled Lambda Request Authorizer; optionally a Cognito User Pool and Verified Permissions identity source. Ingests AVP schema/policies with validations and optional canaries.

## Example

```hcl
resource "vpauthorizer_authorizer" "main" {
  description = "Authorizer"

  lambda {
    memory_size             = 256
    reserved_concurrency    = 2
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

## Argument Reference

- `description` (string) — optional policy store description.
- `retain_on_delete` (bool) — when true, enables deletion protection/PITR where supported.
- `lambda` (block)
  - `memory_size` (number, default 128)
  - `reserved_concurrency` (number, default 1)
  - `provisioned_concurrency` (number, default 0). Validation: when > 0, must be <= `reserved_concurrency`.
- `dynamo` (block)
  - `enable_dynamo_db_stream` (bool, default false)
- `cognito` (block)
  - `sign_in_aliases` (list(string), default ["email"]) — allowed: `email`, `phone`, `preferredUsername`
  - `ses_config` (block)
    - `source_arn` (string, required) — SES identity ARN `arn:aws:ses:<region>:<account-id>:identity/<email-or-domain>`
    - `from` (string, required)
    - `reply_to_email` (string, optional)
    - `configuration_set` (string, optional)
- `verified_permissions` (block)
  - `schema_file` (string, default `./authorizer/schema.yaml`)
  - `policy_dir` (string, default `./authorizer/policies`)
  - `action_group_enforcement` (string, `off|warn|error`, default `error`)
  - `disable_guardrails` (bool, default false)
  - `canary_file` (string, optional; default `./authorizer/canaries.yaml` when present)

## Attributes Reference

- `policy_store_id` (string)
- `policy_store_arn` (string)
- `lambda_authorizer_arn` (string)
- `lambda_role_arn` (string)
- `dynamo_table_arn` (string)
- `dynamo_stream_arn` (string)
- `cognito_user_pool_id` (string)
- `cognito_user_pool_arn` (string)
- `cognito_user_pool_client_ids` (list of string)
- `parameters` (map(string)) — additional parameters for downstream integration.

## Notes

- This resource does not expose raw underlying AWS resources as separate Terraform resources. Use the exported attributes in downstream configuration where needed.
