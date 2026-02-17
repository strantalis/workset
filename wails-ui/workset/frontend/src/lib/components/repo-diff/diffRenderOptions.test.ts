import { describe, expect, it } from 'vitest';

import { buildDiffRenderOptions, resolveDiffStyle } from './diffRenderOptions';

describe('diffRenderOptions', () => {
	it('uses unified layout when container is narrow', () => {
		expect(resolveDiffStyle(1099)).toBe('unified');
	});

	it('uses split layout for wide and unknown widths', () => {
		expect(resolveDiffStyle(1100)).toBe('split');
		expect(resolveDiffStyle(1600)).toBe('split');
		expect(resolveDiffStyle(null)).toBe('split');
		expect(resolveDiffStyle(undefined)).toBe('split');
	});

	it('builds wrapped diff options for renderer', () => {
		const options = buildDiffRenderOptions(900, undefined);
		expect(options.theme).toBe('pierre-dark');
		expect(options.themeType).toBe('dark');
		expect(options.diffStyle).toBe('unified');
		expect(options.diffIndicators).toBe('bars');
		expect(options.overflow).toBe('wrap');
	});
});
