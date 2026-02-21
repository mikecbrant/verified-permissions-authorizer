# ADR 0003: Provider `aws-sdk` library (dynamo, verified-permissions)

Status: Approved

Date: 2025-09-10

Context
The provider needs reusable, well-tested helpers that wrap low-level AWS SDK calls so that component code and future resources are concise, correct, and consistent. Two areas are in scope now:
- DynamoDB single-table helpers (key construction, uniqueness guards, transactional writes, error categorization)
- Verified Permissions helpers (schema idempotence, thin client interface)

## Decisions
- Create an internal Go library under `internal/awssdk` with subpackages:
   - `dynamo`: helpers for key building and a single entrypoint `WriteTransaction(ctx, client, items, opts...)` that composes `TransactWriteItems` with appropriate condition expressions and error mapping.
   - `verifiedpermissions`: helpers for `PutSchema` idempotence and lightweight interfaces that can be mocked in tests.
- Keep functions short and single-purpose; expose small types and interfaces tailored to this provider.
- Provide a tiny leveled logger interface used by both subpackages; default to a no-op logger when callers do not pass one.
- Unit tests must cover 100% of this library; use interface-based fakes to avoid real AWS calls.

## Key patterns (DynamoDB)
- Key builders generate `PK/SK` and `GSI*` for the entities in ADR 0002. Callers pass typed inputs; helpers return `map[string]types.AttributeValue` ready for the AWS SDK.
- Uniqueness is enforced via `ConditionExpression` using `attribute_not_exists(PK) AND attribute_not_exists(SK)` for each `Put` in a transaction.
- Errors are categorized into `ConflictError` (non-retryable: conditional check failures) and `RetryableError` (throttling, throughput, transaction conflicts). A generic `OpError` wraps remaining cases.

## Verified Permissions patterns
- `PutSchemaIfChanged(ctx, api, storeID, cedarJSON)` fetches the current schema (`GetSchema`) and issues `PutSchema` only when the minified JSON differs. It accepts a minimal `API` interface so tests can stub it.
- Helpers are region-agnostic; the caller constructs the AWS config once and supplies a client.

## Logging
- The logger interface is intentionally tiny: `Debugf`, `Infof`, `Warnf`.
- Logging is contextual: messages include operation names and key identifiers but never log full item bodies or secrets.

## Consequences
- Provider code becomes simpler and safer to evolve.
- Tests exercise error handling and condition expression correctness without network calls.
- The library sets a precedent for future subpackages (e.g., S3, CloudWatch) should needs arise.
