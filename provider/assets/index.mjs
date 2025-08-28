// Placeholder embedded authorizer (ESM). CI replaces this file with the built output
// from packages/lambda-authorizer/dist/index.mjs before building the provider.
// The function signature must remain: export const handler = async (event) => { ... }.
export const handler = async () => {
  throw new Error(
    'Embedded authorizer not replaced during build. Ensure CI runs the lambda build and copies dist/index.mjs into provider/assets/index.mjs before building the provider.',
  );
};
