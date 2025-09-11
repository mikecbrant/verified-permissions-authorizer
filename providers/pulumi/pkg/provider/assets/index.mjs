// Placeholder authorizer code used for tests/build before the real Lambda is copied in CI.
// The release workflow replaces this file with the compiled Lambda (`packages/lambda-authorizer/dist/index.mjs`).
export async function handler() {
  return { statusCode: 200, body: 'ok' }
}
