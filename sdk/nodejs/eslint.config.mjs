import tsParser from '@typescript-eslint/parser';
import tsEslintPlugin from '@typescript-eslint/eslint-plugin';
import importPlugin from 'eslint-plugin-import';
import simpleImportSort from 'eslint-plugin-simple-import-sort';
import promisePlugin from 'eslint-plugin-promise';

/** @type {import('eslint').Linter.FlatConfig[]} */
const config = [
  { ignores: ['dist/**'] },
  {
    files: ['**/*.ts'],
    languageOptions: {
      parser: tsParser,
      parserOptions: {
        project: ['./tsconfig.json'],
        tsconfigRootDir: new URL('.', import.meta.url).pathname,
        sourceType: 'module',
      },
    },
    plugins: { '@typescript-eslint': tsEslintPlugin, import: importPlugin, promise: promisePlugin, 'simple-import-sort': simpleImportSort },
    rules: {
      'func-style': ['error', 'expression'],
      'import/exports-last': 'error',
      'import/no-default-export': 'error',
      'simple-import-sort/imports': 'error',
      'simple-import-sort/exports': 'error',
      '@typescript-eslint/explicit-function-return-type': 'error',
      '@typescript-eslint/explicit-module-boundary-types': 'error',
      '@typescript-eslint/no-floating-promises': 'error',
      '@typescript-eslint/consistent-type-definitions': ['error', 'type'],
      '@typescript-eslint/consistent-type-imports': ['error', { prefer: 'type-imports' }],
      'promise/prefer-await-to-then': 'off',
    },
  },
  { files: ['*.js', '*.cjs', '*.mjs'], languageOptions: { parserOptions: { project: null } } },
];

export default config;
