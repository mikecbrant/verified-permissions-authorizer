/* eslint-env node */
/** @type {import('eslint').Linter.Config} */
module.exports = {
  root: true,
  parser: '@typescript-eslint/parser',
  parserOptions: {
    project: ['./tsconfig.json'],
    tsconfigRootDir: __dirname,
    sourceType: 'module',
  },
  plugins: ['@typescript-eslint', 'import', 'promise'],
  extends: [
    'eslint:recommended',
    'plugin:@typescript-eslint/strict-type-checked',
    'plugin:@typescript-eslint/stylistic-type-checked',
    'plugin:import/recommended',
    'plugin:import/typescript',
    'plugin:promise/recommended',
    'eslint-config-prettier',
  ],
  env: {
    node: true,
    es2022: true,
  },
  ignorePatterns: ['dist/**'],
  rules: {
    'no-console': ['error', { allow: ['error'] }],
    'import/no-default-export': 'error',
    '@typescript-eslint/explicit-function-return-type': 'error',
    '@typescript-eslint/explicit-module-boundary-types': 'error',
    '@typescript-eslint/no-floating-promises': 'error',
    '@typescript-eslint/consistent-type-definitions': ['error', 'type'],
    '@typescript-eslint/consistent-type-imports': ['error', { prefer: 'type-imports' }],
    'promise/prefer-await-to-then': 'error',
  },
  overrides: [
    {
      files: ['*.js', '*.cjs'],
      parserOptions: { project: null },
    },
    {
      files: ['tsup.config.ts'],
      parserOptions: { project: null },
      rules: {
        '@typescript-eslint/no-require-imports': 'off',
      },
    },
  ],
};
