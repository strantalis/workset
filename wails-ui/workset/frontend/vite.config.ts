import { defineConfig } from 'vitest/config';
import { svelte } from '@sveltejs/vite-plugin-svelte';
import { svelteTesting } from '@testing-library/svelte/vite';
import tailwindcss from '@tailwindcss/vite';
import wails from '@wailsio/runtime/plugins/vite';

// https://vitejs.dev/config/
export default defineConfig(({ mode }) => ({
	plugins: [
		tailwindcss(),
		svelte({
			compilerOptions: {
				hmr: !mode?.includes('test') && !mode?.includes('production'),
			},
		}),
		...(mode === 'test' ? [] : [wails('./bindings')]),
		svelteTesting(),
	],
	resolve: {
		// Use browser conditions for client-side code in test mode
		conditions: mode === 'test' ? ['browser', 'svelte'] : ['browser'],
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
