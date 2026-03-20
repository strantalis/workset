import { describe, expect, it } from 'vitest';
import { resolvePath, isAbsoluteUrl, hasImageExtension, parentDir } from './markdownImages';

describe('parentDir', () => {
	it('returns directory for nested paths', () => {
		expect(parentDir('docs/guide/README.md')).toBe('docs/guide');
	});

	it('returns empty string for root-level files', () => {
		expect(parentDir('README.md')).toBe('');
	});

	it('returns empty string for empty input', () => {
		expect(parentDir('')).toBe('');
	});
});

describe('isAbsoluteUrl', () => {
	it('detects http/https URLs', () => {
		expect(isAbsoluteUrl('https://example.com/img.png')).toBe(true);
		expect(isAbsoluteUrl('http://example.com/img.png')).toBe(true);
	});

	it('detects data URIs', () => {
		expect(isAbsoluteUrl('data:image/png;base64,abc')).toBe(true);
	});

	it('detects protocol-relative URLs', () => {
		expect(isAbsoluteUrl('//cdn.example.com/img.png')).toBe(true);
	});

	it('returns false for relative paths', () => {
		expect(isAbsoluteUrl('./images/logo.png')).toBe(false);
		expect(isAbsoluteUrl('../assets/img.png')).toBe(false);
		expect(isAbsoluteUrl('images/logo.png')).toBe(false);
	});
});

describe('hasImageExtension', () => {
	it('recognizes common image formats', () => {
		expect(hasImageExtension('logo.png')).toBe(true);
		expect(hasImageExtension('photo.jpg')).toBe(true);
		expect(hasImageExtension('photo.jpeg')).toBe(true);
		expect(hasImageExtension('anim.gif')).toBe(true);
		expect(hasImageExtension('modern.webp')).toBe(true);
		expect(hasImageExtension('icon.svg')).toBe(true);
		expect(hasImageExtension('favicon.ico')).toBe(true);
		expect(hasImageExtension('old.bmp')).toBe(true);
		expect(hasImageExtension('next.avif')).toBe(true);
	});

	it('is case-insensitive', () => {
		expect(hasImageExtension('LOGO.PNG')).toBe(true);
		expect(hasImageExtension('Photo.JPG')).toBe(true);
	});

	it('strips query strings and fragments', () => {
		expect(hasImageExtension('logo.png?v=2')).toBe(true);
		expect(hasImageExtension('logo.png#section')).toBe(true);
	});

	it('rejects non-image extensions', () => {
		expect(hasImageExtension('readme.md')).toBe(false);
		expect(hasImageExtension('code.ts')).toBe(false);
		expect(hasImageExtension('data.json')).toBe(false);
	});

	it('rejects files without extensions', () => {
		expect(hasImageExtension('Makefile')).toBe(false);
	});
});

describe('resolvePath', () => {
	it('resolves simple relative path from root', () => {
		expect(resolvePath('', 'images/logo.png')).toBe('images/logo.png');
	});

	it('resolves relative path against a base directory', () => {
		expect(resolvePath('docs', 'images/logo.png')).toBe('docs/images/logo.png');
	});

	it('resolves ./ prefixed paths', () => {
		expect(resolvePath('docs', './images/logo.png')).toBe('docs/images/logo.png');
	});

	it('resolves ../ within bounds', () => {
		expect(resolvePath('docs/guide', '../images/logo.png')).toBe('docs/images/logo.png');
	});

	it('resolves nested ../ within bounds', () => {
		expect(resolvePath('a/b/c', '../../img.png')).toBe('a/img.png');
	});

	it('returns null when ../ escapes root', () => {
		expect(resolvePath('', '../secret.png')).toBeNull();
		expect(resolvePath('docs', '../../secret.png')).toBeNull();
	});

	it('handles deeply nested base dirs', () => {
		expect(resolvePath('a/b/c/d', 'e.png')).toBe('a/b/c/d/e.png');
	});

	it('normalizes . segments', () => {
		expect(resolvePath('docs', './././img.png')).toBe('docs/img.png');
	});

	it('returns null for empty result', () => {
		expect(resolvePath('', '')).toBeNull();
	});
});
