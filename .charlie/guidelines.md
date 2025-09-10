Engineering guidelines (provider repository)

Scope: internal modules under `packages/provider` and related tests.

- Common utilities first (DRY)
  - Prefer small shared packages for cross-cutting helpers used by more than one subpackage.
  - Examples in this repo:
    - `pkg/logging`: tiny leveled `Logger` interface and `NopLogger` for reuse across internal libs and tests.
    - `internal/testutil`: shared fakes and helpers (e.g., buffer-backed logger, fake DynamoDB transact client, string helpers).
  - Do not duplicate small fakes or helpers inside individual test files; move them to `internal/testutil` and import.

- Error handling policy (Go)
  - Log at the point of failure when the code has the most context (include what was attempted and key identifiers), then return the error to the caller.
  - Avoid swallowing errors or converting them to logs-only outcomes. Let the higher level decide how to handle/translate errors.
  - Prefer wrapping with `%w` to preserve the chain. For known classes, wrap in a typed error (e.g., `ConflictError`, `RetryableError`).

- Neutral placement for generic utilities
  - Avoid defining generic/shared utilities in AWS-specific packages. For example, the `Logger` interface lives in `pkg/logging` (not under `pkg/aws-sdk`).
  - AWS service–specific formatting helpers may live alongside the service code, but keep the logging surface generic.

- Tests
  - Reuse `internal/testutil` fakes in unit tests (e.g., DynamoDB transact client).
  - Keep assertions focused; prefer stdlib helpers (e.g., `strings.Contains`) or thin wrappers in `internal/testutil` when repeated.
