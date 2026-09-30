import { fileURLToPath, URL } from 'node:url'
import { defineConfig } from 'vitest/config'

// Unit tests of the SPA's pure modules (no DOM, no Vuetify): `bun run test`,
// also part of `make front-check`.
export default defineConfig({
  resolve: {
    alias: {
      '@': fileURLToPath(new URL('src', import.meta.url)),
    },
  },
  test: {
    include: ['src/**/__tests__/*.test.ts'],
    environment: 'node',
  },
})
