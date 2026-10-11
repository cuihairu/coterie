import { defineConfig } from 'vitest/config'

// Kept separate from vite.config.ts so the router plugin does not
// regenerate routeTree.gen.ts mid-run. Tests run in node; the few
// browser globals under test are stubbed by the test file itself.
export default defineConfig({
  test: {
    environment: 'node',
  },
})
