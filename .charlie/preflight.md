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
