import { describe, expect, it } from 'vitest';
import { encodeWheel } from './mouse';

describe('encodeWheel', () => {
	it('encodes sgr wheel events', () => {
		const encoded = encodeWheel({ button: 64, col: 10, row: 20, encoding: 'sgr' });
		expect(encoded).toBe('\x1b[<64;10;20M');
	});

	it('encodes urxvt wheel events', () => {
		const encoded = encodeWheel({ button: 65, col: 5, row: 6, encoding: 'urxvt' });
		expect(encoded).toBe('\x1b[65;5;6M');
	});

	it('encodes utf8 wheel events', () => {
		const encoded = encodeWheel({ button: 64, col: 10, row: 20, encoding: 'utf8' });
		const cb = 96;
		const cx = 42;
		const cy = 52;
		expect(encoded).toBe(`\x1b[M${String.fromCodePoint(cb, cx, cy)}`);
	});

	it('clamps x10 wheel events', () => {
		const encoded = encodeWheel({ button: 64, col: 300, row: 400, encoding: 'x10' });
		const cb = 96;
		const cx = 223 + 32;
		const cy = 223 + 32;
		expect(encoded).toBe(`\x1b[M${String.fromCharCode(cb, cx, cy)}`);
	});
});
