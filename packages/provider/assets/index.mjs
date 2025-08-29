// Placeholder embedded authorizer. CI replaces this file with the built output
// from packages/lambda-authorizer/dist/index.mjs before building the provider.

export const handler = async function () {
  return { allow: false }
}
