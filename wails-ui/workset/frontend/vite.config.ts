import { defineConfig } from 'vite';
import { svelte } from '@sveltejs/vite-plugin-svelte';

// https://vitejs.dev/config/
export default defineConfig(({ mode }) => ({
	plugins: [
		svelte({
			compilerOptions: {
				hmr: !mode?.includes('test') && !mode?.includes('production'),
			},
		}),
	],
	resolve: {
		// Always use browser conditions for client-side app
		conditions: ['browser'],
	},
	test: {
		environment: 'jsdom',
		include: ['src/**/*.test.ts', 'src/**/*.spec.ts'],
		setupFiles: ['./src/test-setup.ts'],
		coverage: {
			provider: 'v8',
			reporter: ['text', 'html'],
			exclude: ['node_modules/', 'src/test-setup.ts', '**/*.d.ts', '**/*.spec.ts', '**/*.test.ts'],
		},
	},
}));
