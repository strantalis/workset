import { defineConfig } from 'vitest/config';
import { svelte } from '@sveltejs/vite-plugin-svelte';
import { svelteTesting } from '@testing-library/svelte/vite';
import tailwindcss from '@tailwindcss/vite';
import wails from '@wailsio/runtime/plugins/vite';

// Wails dev mode runs a background Vite dev server and a foreground Vite build at
// the same time. Sharing node_modules/.vite causes cache cleanup races.
const resolveCacheDir = (command: 'serve' | 'build', mode?: string): string => {
	if (mode === 'test') return 'node_modules/.vite-test';
	if (command === 'serve') return 'node_modules/.vite-dev';
	if (mode === 'development') return 'node_modules/.vite-build-dev';
	return 'node_modules/.vite-build';
};

// https://vitejs.dev/config/
export default defineConfig(({ command, mode }) => ({
	cacheDir: resolveCacheDir(command, mode),
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
