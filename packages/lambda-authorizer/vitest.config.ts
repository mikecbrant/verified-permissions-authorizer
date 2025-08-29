import { defineConfig } from 'vitest/config'

export default defineConfig({
  test: {
    testTimeout: 30000,
    coverage: {
      provider: 'v8',
      all: false,
      include: ['src/**'],
      thresholds: {
        lines: 100,
        functions: 100,
        branches: 100,
        statements: 100,
      },
    },
  },
})
