export default {
  $schema: 'https://json.schemastore.org/tsconfig',
  compilerOptions: {
    target: 'ES2022',
    module: 'ES2022',
    moduleResolution: 'Bundler',
    strict: true,
    declaration: true,
    outDir: 'dist',
    skipLibCheck: true,
    verbatimModuleSyntax: true,
  },
  include: ['src/**/*'],
}
