# Terraform provider parity matrix

Status: Draft (updated for initial implementation PR)

- Provider source: `mikecbrant/vpauthorizer`
- Resource: `vpauthorizer_authorizer`

## Surface parity with Pulumi provider

- Policy store (STRICT): ✅
- Lambda authorizer (nodejs22.x): ✅ (no runtime override)
- DynamoDB auth table with GSIs: ✅ (stream optional)
- Cognito (User Pool + VP Identity Source): ⏳ planned (basic SES validation wired, creation to follow)
- AVP schema/policy ingestion: ✅ (same validation logic via shared Go)
- Guardrails: ⏳ provider-managed guardrails install deferred in TF v0.1 (schema/policy ingestion present)
- Canaries: ✅ (provider + consumer canaries)
- Transparency/exports: ✅ (IDs/ARNs provided as attributes)

## Tests coverage

- Unit: shared Go (SES config validation, action group enforcement) — ✅
- Acceptance: resource happy path (schema/policies only) — ✅ (gated by `TF_ACC` and AWS creds)
- Negative/validation: lambda concurrency ordering — ✅

## Notes

- Exact feature parity will land incrementally; both providers release in lockstep (same SemVer). Items marked ⏳ will land behind the same version bump once implemented.
