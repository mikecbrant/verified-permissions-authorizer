# Node.js SDK for Verified Permissions Authorizer

This package provides the Node.js SDK bindings for the `verified-permissions-authorizer` Pulumi component and ships a small CLI for validating AVP assets locally.

## CLI: avp-validate

Validate your Cedar JSON schema (YAML/JSON), enforce canonical action groups (PascalCase + Global\* variants), run a best‑effort policy syntax scan, and check canary structure – all without AWS calls.

Usage:

```
npx avp-validate --schema ./path/to/schema.yaml --policyDir ./path/to/policies --mode error
# optional canaries
npx avp-validate --schema ./schema.json --policyDir ./policies --canary ./canaries.yaml
```

- `--schema` (required): path to `schema.yaml`/`schema.yml` or `schema.json`
- `--policyDir` (required): directory containing `.cedar` files (recursively discovered)
- `--canary` (optional): YAML file with canary cases `{ principal, action, resource, expect }`
- `--mode` (optional): `off|warn|error` (default `error`)

The provider performs equivalent validations during `pulumi up` and, when a canary file is provided, executes authorization canaries post‑deploy.

## SDK

See `src/types.ts` for the full type surface. Example:

```ts
import { AuthorizerWithPolicyStore } from "pulumi-verified-permissions-authorizer";

new AuthorizerWithPolicyStore("authz", {
  verifiedPermissions: {
    schemaFile: "./avp/schema.yaml",
    policyDir: "./avp/policies",
    actionGroupEnforcement: "error",
  },
});
```

---

For Cedar language patterns (RBAC/ABAC/ReBAC), see https://docs.cedarpolicy.com/overview/patterns.html
