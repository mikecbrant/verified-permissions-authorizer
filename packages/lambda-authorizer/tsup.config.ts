import { defineConfig } from 'tsup';

export default defineConfig({
  entry: ['src/index.ts'],
  clean: true,
  format: ['cjs'],
  target: 'node20',
  platform: 'node',
  sourcemap: false,
  dts: true,
  minify: true,
  bundle: true,
  outDir: 'dist',
  shims: false,
});
