// Stubbed Lambda authorizer entrypoint for local builds and CI linting.
// The release workflows overwrite this file with the real built output
// from packages/lambda-authorizer before building provider binaries.
// Keeping a minimal, valid handler here ensures `go build` works in a
// clean workspace without extra steps.

export async function handler() {
  // Always deny by default in the stub to avoid accidental reliance.
  return { isAuthorized: false }
}
