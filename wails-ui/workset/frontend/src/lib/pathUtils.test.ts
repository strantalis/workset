import { describe, expect, it } from 'vitest';
import { formatPath } from './pathUtils';

describe('formatPath - path display formatting', () => {
	describe('short paths (fit entirely)', () => {
		it('returns short paths unchanged', () => {
			expect(formatPath('main.go')).toBe('main.go');
			expect(formatPath('pkg/client.go')).toBe('pkg/client.go');
			expect(formatPath('cmd/app/main.go')).toBe('cmd/app/main.go');
		});

		it('handles paths at exactly max length', () => {
			const path = 'a'.repeat(40);
			expect(formatPath(path)).toBe(path);
		});
	});

	describe('medium paths (truncated from left)', () => {
		it('preserves filename and last directories', () => {
			const path = 'wails-ui/workset/frontend/src/lib/components/RepoDiff.svelte';
			const result = formatPath(path);
			expect(result).toMatch(/\.\.\.\/.*RepoDiff\.svelte$/);
			expect(result).toContain('RepoDiff.svelte');
		});

		it('shows last 2-3 directories with filename', () => {
			const path = 'very/deeply/nested/directory/structure/with/file.go';
			const result = formatPath(path, 30);
			expect(result).toContain('file.go');
			expect(result).toContain('...');
			// Should show some context directories
			expect(result.split('/').length).toBeGreaterThan(2);
		});

		it('handles different max lengths', () => {
			const path = 'a/b/c/d/e/f/g/h/i/j/file.txt';
			
			const shortResult = formatPath(path, 20);
			expect(shortResult.length).toBeLessThanOrEqual(20);
			expect(shortResult).toContain('file.txt');
			
			const longResult = formatPath(path, 50);
			expect(longResult.length).toBeLessThanOrEqual(50);
			expect(longResult).toContain('file.txt');
		});
	});

	describe('long filenames (always preserved)', () => {
		it('returns only filename when it exceeds maxChars', () => {
			const filename = 'very-long-component-name-that-is-excessive-and-needs-truncation.tsx';
			const path = `src/components/${filename}`;
			const result = formatPath(path, 40);
			
			expect(result).toBe(filename);
		});

		it('returns only filename when it equals maxChars', () => {
			const filename = 'a'.repeat(40);
			const path = `src/${filename}`;
			const result = formatPath(path);
			
			expect(result).toBe(filename);
		});

		it('handles filename just under maxChars', () => {
			const filename = 'a'.repeat(39);
			const path = `src/components/${filename}`;
			const result = formatPath(path);
			
			// When filename is 39 chars with maxChars=40, we don't have room
			// for ".../" (4 chars) plus a directory, so just return filename
			expect(result).toBe(filename);
			expect(result.length).toBeLessThanOrEqual(40);
		});
	});

	describe('edge cases', () => {
		it('handles empty path', () => {
			expect(formatPath('')).toBe('');
		});

		it('handles single directory', () => {
			expect(formatPath('file.go', 40)).toBe('file.go');
		});

		it('handles paths with no extension', () => {
			const path = 'some/very/long/path/to/a/file/that/has/no/extension';
			const result = formatPath(path);
			expect(result).toContain('extension');
			expect(result).toContain('...');
		});

		it('handles paths with dots in directories', () => {
			const path = 'node_modules/@types/react/index.d.ts';
			const result = formatPath(path);
			expect(result).toContain('index.d.ts');
		});

		it('handles deeply nested paths', () => {
			const path = 'a/b/c/d/e/f/g/h/i/j/k/l/m/n/o/p/q/r/s/t/u/v/w/x/y/z/file.go';
			const result = formatPath(path);
			expect(result).toContain('file.go');
			expect(result).toContain('...');
			// Result should never exceed maxChars
			expect(result.length).toBeLessThanOrEqual(40);
		});

		it('handles paths with special characters', () => {
			const path = 'src/components/[id]/page.tsx';
			expect(formatPath(path)).toBe(path); // Should fit entirely
		});

		it('handles very short maxChars', () => {
			const path = 'src/main.go';
			const result = formatPath(path, 10);
			// With maxChars=10 and filename="main.go" (7 chars), we should 
			// just return the filename since there's no room for prefix
			expect(result).toBe('main.go');
			expect(result.length).toBeLessThanOrEqual(10);
		});

		it('handles paths with spaces', () => {
			const path = 'My Documents/Projects/app.js';
			const result = formatPath(path, 25);
			expect(result).toContain('app.js');
		});
	});

	describe('common Go project paths', () => {
		it('handles typical Go package paths', () => {
			expect(formatPath('main.go')).toBe('main.go');
			expect(formatPath('cmd/workset/main.go')).toBe('cmd/workset/main.go');
			expect(formatPath('internal/workspace/repo.go')).toBe('internal/workspace/repo.go');
			expect(formatPath('pkg/worksetapi/client.go')).toBe('pkg/worksetapi/client.go');
		});

		it('handles long Go import paths', () => {
			const path = 'github.com/user/repo/pkg/subpackage/deep/nested/file.go';
			const result = formatPath(path);
			expect(result).toContain('file.go');
			expect(result).toContain('...');
		});
	});

	describe('frontend project paths', () => {
		it('handles typical frontend paths', () => {
			const paths = [
				'src/App.tsx',
				'src/components/Button.tsx',
				'src/lib/utils/format.ts',
				'wails-ui/workset/frontend/src/lib/components/RepoDiff.svelte',
			];

			paths.forEach(path => {
				const result = formatPath(path);
				const filename = path.split('/').pop() || '';
				expect(result).toContain(filename);
				expect(result.length).toBeLessThanOrEqual(40);
			});
		});
	});
});
