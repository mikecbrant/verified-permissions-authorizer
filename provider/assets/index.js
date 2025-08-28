// Placeholder embedded authorizer. CI replaces this file with the built output
// from packages/lambda-authorizer/dist/index.js before building the provider.
// The function signature must remain: exports.handler = async (event) => { ... }.
exports.handler = async function () {
  throw new Error(
    'Embedded authorizer not replaced during build. Ensure CI runs the lambda build and copies dist/index.js into provider/assets/index.js before building the provider.',
  );
};
