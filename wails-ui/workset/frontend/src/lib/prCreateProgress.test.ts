import { describe, expect, it } from 'vitest';
import { getPrCreateStageCopy } from './prCreateProgress';

describe('getPrCreateStageCopy', () => {
	it('returns null for empty stage', () => {
		expect(getPrCreateStageCopy(null)).toBeNull();
	});

	it('returns copy for generating stage', () => {
		expect(getPrCreateStageCopy('generating')).toEqual({
			button: 'Generating title...',
			detail: 'Step 1/2: Generating title...',
		});
	});

	it('returns copy for creating stage', () => {
		expect(getPrCreateStageCopy('creating')).toEqual({
			button: 'Creating PR...',
			detail: 'Step 2/2: Creating PR...',
		});
	});
});
