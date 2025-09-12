Local preflight checks (Go + Node)

Run these before opening a PR to keep CI green.

- Prerequisites
  - Go toolchain installed (version per packages/provider/go.mod; currently Go 1.24)
  - Node 20+ and pnpm installed
  - Workspace install completed: pnpm install

- Go (provider)
  - Build: `go build ./packages/provider/...`
  - Vet: `go vet ./packages/provider/...`
  - Test: `go test ./packages/provider/...`
    - Optional coverage: `go test ./packages/provider/... -coverprofile=coverage.out -covermode=count`

- Node (workspace)
  - Lint: `pnpm -r lint`
  - Typecheck: `pnpm -r typecheck`
  - Test: `pnpm -r test`

All of the above should pass locally before you open or update a PR.

## Provider release & versioning policy (Pulumi and Terraform)

- Release determination: Use comment-based Changesets to decide the next SemVer. The same Changeset drives both providers.
- Version parity: Pulumi and Terraform providers must publish in lockstep with identical versions (`X.Y.Z`). No mismatched or out-of-band versions.
- CI/CD flow:
  - Run Pulumi and Terraform publish jobs in parallel.
  - After both succeed, run a single common step that updates repository version information (e.g., changelogs, tags) and commits back to the default branch.
- Repository layout:
  - Terraform provider source lives under `providers/terraform`.
  - Pulumi provider source lives under `providers/pulumi`.
  - Shared Go logic used by both providers lives in common packages within `providers/` (e.g., `providers/internal`), so both providers use the exact same implementation wherever possible.
