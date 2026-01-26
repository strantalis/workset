import {defineConfig} from 'vite'
import {svelte} from '@sveltejs/vite-plugin-svelte'

// https://vitejs.dev/config/
export default defineConfig(({mode}) => ({
  plugins: mode === 'test' ? [] : [svelte()],
  test: {
    environment: 'node',
    include: ['src/**/*.test.ts']
  }
}))
