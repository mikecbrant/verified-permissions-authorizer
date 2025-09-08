import tsParser from '@typescript-eslint/parser';
import tsEslintPlugin from '@typescript-eslint/eslint-plugin';
import importPlugin from 'eslint-plugin-import';
import simpleImportSort from 'eslint-plugin-simple-import-sort';
import promisePlugin from 'eslint-plugin-promise';

// Optionally include shared config when available. We spread `shared` first so local
// rules/configs below take precedence when overlaps occur.
let shared = []
try {
  const m = await import('@mikecbrant/ts-tooling/eslint/flat')
  shared = Array.isArray(m.default) ? m.default : []
} catch {}

/** @type {import('eslint').Linter.FlatConfig[]} */
const config = [
  ...shared,
  {
    ignores: ['dist/**'],
  },
  {
    files: ['**/*.ts'] ,
    languageOptions: {
      parser: tsParser,
      parserOptions: {
        // Do not depend on a generated tsconfig; rely on defaults without a project file.
        project: null,
        tsconfigRootDir: new URL('.', import.meta.url).pathname,
        sourceType: 'module',
      },
    },
    plugins: {
      '@typescript-eslint': tsEslintPlugin,
      import: importPlugin,
      promise: promisePlugin,
      'simple-import-sort': simpleImportSort,
    },
    rules: {
      // Style requested by reviewer
      'func-style': ['error', 'expression'],
      'import/exports-last': 'error',
      'import/no-default-export': 'error',
      'simple-import-sort/imports': 'error',
      'simple-import-sort/exports': 'error',

      // TypeScript best practices (subset applicable via parser)
      '@typescript-eslint/explicit-function-return-type': 'error',
      '@typescript-eslint/explicit-module-boundary-types': 'error',
      // Disabled here because typed linting would require a generated tsconfig.
      '@typescript-eslint/no-floating-promises': 'off',
      '@typescript-eslint/consistent-type-definitions': ['error', 'type'],
      '@typescript-eslint/consistent-type-imports': ['error', { prefer: 'type-imports' }],

      // Prefer attaching catch handlers over try/catch in async flows
      'promise/prefer-await-to-then': 'off',
    },
  },
  {
    files: ['*.js', '*.mjs'],
    languageOptions: {
      parserOptions: { project: null },
    },
  },
];

export default config;
