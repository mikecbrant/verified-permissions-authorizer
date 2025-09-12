Engineering guidelines (provider repository)

Scope: internal modules under `packages/provider` and related tests.

- Common utilities first (DRY)
  - Prefer small shared packages for cross-cutting helpers used by more than one subpackage.
  - Examples in this repo:
    - `pkg/logging`: tiny leveled `Logger` interface and `NopLogger` for reuse across internal libs and tests.
    - `internal/testutil`: shared fakes and helpers (e.g., buffer-backed logger, fake DynamoDB transact client, string helpers).
  - Do not duplicate small fakes or helpers inside individual test files; move them to `internal/testutil` and import.

- Logging standard (Go)
  - Use message + context everywhere: `Logger.Debug("op.name", logging.Fields{"key": value})`.
  - Prefer short, lowerCamelCase keys; ensure values are JSON‑serializable.
  - Avoid printf‑style `*f` logging APIs. If formatting is needed, compute values separately and place in context.
  - Never log secrets or full record payloads; include only identifiers and counts needed for troubleshooting.

- Error handling policy (Go)
  - Log at the point of failure when the code has the most context (include what was attempted and key identifiers), then return the error to the caller.
  - Avoid swallowing errors or converting them to logs-only outcomes. Let the higher level decide how to handle/translate errors.
  - Prefer wrapping with `%w` to preserve the chain. For known classes, wrap in a typed error (e.g., `ConflictError`, `RetryableError`).

- Neutral placement for generic utilities
  - Avoid defining generic/shared utilities in AWS-specific packages. For example, the `Logger` interface lives in `pkg/logging` (not under `pkg/aws-sdk`).
  - AWS service–specific formatting helpers may live alongside the service code, but keep the logging surface generic.

- Package/file organization
  - Prefer smaller, single‑purpose Go files. Split entity‑specific helpers (e.g., DynamoDB key builders) into per‑entity files with matching tests.
  - Keep generic helpers (e.g., DynamoDB AttributeValue constructors, JSON canonicalization) in common utility packages.

- Tests
  - Reuse `internal/testutil` fakes in unit tests (e.g., DynamoDB transact client).
  - Keep assertions focused; prefer stdlib helpers (e.g., `strings.Contains`) or thin wrappers in `internal/testutil` when repeated.

- Pre‑PR checklist (must run locally before requesting review)
  - JavaScript/TypeScript packages affected by your change:
    - `pnpm -r typecheck` (or scope with `--filter {pkg}` when appropriate) — no TypeScript errors.
    - `pnpm -r lint` — no ESLint errors or warnings (rules use `--max-warnings 0`).
    - `pnpm -r test` — all unit tests green; `packages/lambda-authorizer` enforces 100% coverage on `src/**`.
  - Go provider (`packages/provider`):
    - `go vet ./...` — no vet findings.
    - `go test ./... -cover` — tests green with coverage output.
  - If a toolchain is unavailable locally (e.g., Go not installed), call this out in your PR body and rely on CI for that portion.

- CI and merge policy
  - All GitHub Actions checks must be green on the PR before merging. Do not merge with red or pending checks.
  - If your change breaks a check, include the fix in the same PR whenever possible.
  - Mark a PR “Ready for review” only after the local checklist above passes and CI is green or clearly isolated to a missing local toolchain.
