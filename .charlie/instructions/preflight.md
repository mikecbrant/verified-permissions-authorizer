Local preflight checks (Go + Node)

Run these before opening a PR to keep CI green.

- Prerequisites
  - Go toolchain installed (version per `go.mod`; currently `go 1.25`)
  - Node 20+ and pnpm installed
  - Workspace install completed: `pnpm install`

- Go (root module)
  - Tidy: `go mod tidy`
  - Build: `go build ./...`
  - Vet: `go vet ./...`
  - Lint: `golangci-lint run ./...`
  - Test: `go test ./... -coverprofile=coverage.out -covermode=count`

- Node (workspace)
  - Lint: `pnpm -r lint`
  - Typecheck: `pnpm -r typecheck`
  - Test: `pnpm -r test`

All of the above should pass locally before you open or update a PR.

## Provider release & versioning policy (Pulumi and Terraform)

- Release determination: Use Changesets to decide the next SemVer. The same Changeset drives both Pulumi and Terraform artifacts.
- Version parity: Pulumi and Terraform providers must publish in lockstep with identical versions (`X.Y.Z`).
- CI/CD flow:
  - Publish Pulumi and Terraform provider artifacts in parallel.
  - After both succeed, update repository version metadata (changelog, tags) and commit to the default branch.
- Repository layout:
  - Pulumi provider entrypoint: `cmd/pulumi-resource-verified-permissions-authorizer`
  - Terraform provider entrypoint: `cmd/terraform-provider-vpauthorizer`
  - Shared implementation: packages under `internal/**`
