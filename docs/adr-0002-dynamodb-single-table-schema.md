# ADR 0002: DynamoDB single-table schema for authorization data

Status: Approved

Date: 2025-09-10

Decision drivers
- Support the Lambda authorizer’s read patterns without joins or multi-round trips.
- Enforce global and scoped uniqueness constraints in data using DynamoDB condition expressions.
- Keep the design implementation-neutral while documenting it “in terms of DynamoDB’s SDK.”

## Table and index layout
- Table name: provider-managed, created by the component. Keys and GSIs are fixed.
- Primary key: `PK` (partition key, String) + `SK` (sort key, String). The pair (PK, SK) MUST be unique.
- GSIs (both with ProjectionType ALL):
   - `GSI1` with keys `GSI1PK` (String) and `GSI1SK` (String)
   - `GSI2` with keys `GSI2PK` (String) and `GSI2SK` (String)

In AWS SDK terms (Go v2 or JS v3), items are maps of attribute names to AttributeValues. Only attributes listed above participate in keys/indexes.

## Item types (entities)
Every item includes a `Type` attribute (String) for diagnostics/introspection and must adhere to the key layouts below. Attributes listed are those relevant to authorization flows; additional attributes are allowed. Access patterns are colocated with each entity, grouped by index.

- Tenant
   - Keys (table): `PK = TENANT#{tenantId}`; `SK = TENANT#{tenantId}`
     - Access (table):
       - Get tenant by id: GetItem with full primary key `(PK, SK)`.
   - GSI1 (name): `GSI1PK = TENANT_NAME#{name}`; `GSI1SK = TENANT_NAME#{name}` (enforces unique tenant names)
     - Access (GSI1):
       - Get tenant by name: Query with equality on `GSI1PK` (exact match; limit 1).
   - Attributes: `tenantId` (ULID), `name`

- User
   - Keys (table): `PK = USER#{userId}`; `SK = USER#{userId}`
     - Access (table):
       - Get user by id: GetItem with full primary key `(PK, SK)`.
   - Attributes: `userId` (ULID), `email`, `phone`, `preferredUsername`, `givenName`, `familyName`, `roles` (array of global role IDs)

- UserEmail (internal uniqueness guard)
   - Keys (table): `PK = USER_EMAIL#{email}`; `SK = USER_EMAIL#{email}`
   - Access: no runtime reads; used only in write-time uniqueness transactions.
   - Attributes: `email`, `userId`

- UserPhone (internal uniqueness guard)
   - Keys (table): `PK = USER_PHONE#{phone}`; `SK = USER_PHONE#{phone}`
   - Access: no runtime reads; used only in write-time uniqueness transactions.
   - Attributes: `phone`, `userId`

- UserPreferredUsername (internal uniqueness guard)
   - Keys (table): `PK = USER_PREFERREDUSERNAME#{preferredUsername}`; `SK = USER_PREFERREDUSERNAME#{preferredUsername}`
   - Access: no runtime reads; used only in write-time uniqueness transactions.
   - Attributes: `preferredUsername`, `userId`

- Role
   - Keys (table; name by scope): `PK = ROLE_SCOPE#{scope}`; `SK = ROLE_NAME#{name}`
     - Access (table):
       - Get role by scope+name: GetItem with full primary key `(PK, SK)`.
   - GSI1 (lookup by id): `GSI1PK = ROLE#{roleId}`; `GSI1SK = ROLE#{roleId}`
     - Access (GSI1):
       - Resolve role definition by roleId: Query with equality on `GSI1PK` (exact match; limit 1).
       - Note: if the intent is to require GetItem by full primary key for this lookup, please confirm; current design resolves by roleId via GSI1 as we do not have `(scope,name)` when only `roleId` is provided.
   - Attributes: `roleId` (ULID), `name`, `scope` ("tenant" or "global")

- TenantGrant (per-tenant role membership for a user)
   - Keys (table; by-tenant, by-user): `PK = TENANT#{tenantId}`; `SK = USER#{userId}`
     - Access (table):
       - Check a user’s membership in a specific tenant: GetItem with full primary key `(PK, SK)`.
   - GSI1 (reverse lookup): `GSI1PK = USER#{userId}`; `GSI1SK = TENANT#{tenantId}`
     - Access (GSI1):
       - List a user’s tenant grants: Query with equality on `GSI1PK` (page to list all memberships and role IDs).
   - GSI2 (id): `GSI2PK = TENANT_GRANT#{tenantGrantId}`; `GSI2SK = TENANT_GRANT#{tenantGrantId}`
     - Access (GSI2):
       - Lookup by grant id: Query with equality on `GSI2PK` (exact match; limit 1).
   - Attributes: `tenantGrantId` (ULID), `tenantId`, `userId`, `roles` (array of role IDs)

- Policy (tracks AVP static policy metadata)
   - Keys (table; name index): `PK = GLOBAL`; `SK = POLICY_NAME#{name}`
     - Access (table):
       - Resolve policy metadata by name prefix: Query with `PK = GLOBAL` and `begins_with(SK, "POLICY_NAME#prefix")`.
       - Get policy metadata by exact name: GetItem with full primary key `(PK, SK)`.
   - GSI1 (id): `GSI1PK = POLICY#{policyId}`; `GSI1SK = POLICY#{policyId}`
     - Access (GSI1):
       - Resolve policy metadata by id: Query with equality on `GSI1PK` (exact match; limit 1).
   - Attributes: `policyId` (from Verified Permissions), `name`

These patterns avoid cross-partition fan-out and align with DynamoDB single-table best practices, documenting precisely which reads are GetItem (full primary key) vs Query on GSIs.

## Uniqueness and transactions
Use DynamoDB condition expressions within `TransactWriteItems` to enforce uniqueness atomically:
- For create of items with unique (PK, SK): add a `Put` with `ConditionExpression = attribute_not_exists(PK) AND attribute_not_exists(SK)`.
- For uniqueness guards (UserEmail, UserPhone, UserPreferredUsername): include additional `Put` operations in the same transaction with the same not-exists condition. If any exists, the transaction fails cleanly.
- For Role names unique per scope: condition on the role name key pair `(PK = ROLE_SCOPE#{scope}, SK = ROLE_NAME#{name})`.

Error handling guidance (AWS SDK error types):
- `ConditionalCheckFailedException` or transaction cancellation with a `ConditionalCheckFailed` reason is a conflict (non-retryable by default).
- Throughput/capacity and transient network errors are retryable with backoff (`ProvisionedThroughputExceededException`, `ThrottlingException`, `RequestLimitExceeded`).

## Representative examples
Tenant
```json
{
   "PK": {"S": "TENANT#01J8Z0E2Z8D2A3J7A7Y2H9GQ9C"},
   "SK": {"S": "TENANT#01J8Z0E2Z8D2A3J7A7Y2H9GQ9C"},
   "GSI1PK": {"S": "TENANT_NAME#acme"},
   "GSI1SK": {"S": "TENANT_NAME#acme"},
   "Type": {"S": "Tenant"},
   "tenantId": {"S": "01J8Z0E2Z8D2A3J7A7Y2H9GQ9C"},
   "name": {"S": "acme"}
}
```

TenantGrant (user in a tenant with two roles)
```json
{
   "PK": {"S": "TENANT#01J8Z0E2Z8D2A3J7A7Y2H9GQ9C"},
   "SK": {"S": "USER#01J8YZZQ3V8PZKQ0ZKX4C2M7FM"},
   "GSI1PK": {"S": "USER#01J8YZZQ3V8PZKQ0ZKX4C2M7FM"},
   "GSI1SK": {"S": "TENANT#01J8Z0E2Z8D2A3J7A7Y2H9GQ9C"},
   "GSI2PK": {"S": "TENANT_GRANT#01J8Z3Q3TQ6T0C9J2W0G7N2B6V"},
   "GSI2SK": {"S": "TENANT_GRANT#01J8Z3Q3TQ6T0C9J2W0G7N2B6V"},
   "Type": {"S": "TenantGrant"},
   "tenantId": {"S": "01J8Z0E2Z8D2A3J7A7Y2H9GQ9C"},
   "userId": {"S": "01J8YZZQ3V8PZKQ0ZKX4C2M7FM"},
   "roles": {"L": [{"S": "01J8X2W3Y4Z5A6B7C8D9E0F1G2"}, {"S": "01J8X2W3Y4Z5A6B7C8D9E0F1H3"}]}
}
```

Role
```json
{
   "PK": {"S": "ROLE_SCOPE#tenant"},
   "SK": {"S": "ROLE_NAME#admin"},
   "GSI1PK": {"S": "ROLE#01J8X2W3Y4Z5A6B7C8D9E0F1G2"},
   "GSI1SK": {"S": "ROLE#01J8X2W3Y4Z5A6B7C8D9E0F1G2"},
   "Type": {"S": "Role"},
   "roleId": {"S": "01J8X2W3Y4Z5A6B7C8D9E0F1G2"},
   "name": {"S": "admin"},
   "scope": {"S": "tenant"}
}
```

Policy (metadata)
```json
{
   "PK": {"S": "GLOBAL"},
   "SK": {"S": "POLICY_NAME#ticket-tenant-enforce"},
   "GSI1PK": {"S": "POLICY#p-abc123"},
   "GSI1SK": {"S": "POLICY#p-abc123"},
   "Type": {"S": "Policy"},
   "name": {"S": "ticket-tenant-enforce"},
   "policyId": {"S": "p-abc123"}
}
```

## Referencing from code
- The provider’s Go library `pkg/aws-sdk/dynamo` builds keys and condition expressions and wraps `TransactWriteItems` with error categorization.
- Authorizer and provisioning code should rely on the library helpers for consistent key construction and uniqueness enforcement, not duplicate string templates.

Consequences
- Clear, migration-friendly patterns for adding new entities and access patterns.
- Encodes uniqueness and lookup patterns into indexes so the authorizer never needs multi-item joins during evaluation.
