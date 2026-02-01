import { describe, expect, it } from 'vitest';
import { stripMouseReports } from './inputFilter';

describe('stripMouseReports', () => {
	it('drops full mouse reports when mouse disabled', () => {
		const input = '\x1b[<64;10;20M';
		const result = stripMouseReports(input, { mouse: false }, '');
		expect(result.filtered).toBe('');
		expect(result.tail).toBe('');
	});

	it('buffers split mouse report sequences', () => {
		const first = stripMouseReports('\x1b[<64;10', { mouse: false }, '');
		expect(first.filtered).toBe('');
		expect(first.tail).toBe('\x1b[<64;10');
		const second = stripMouseReports(';20M', { mouse: false }, first.tail);
		expect(second.filtered).toBe('');
		expect(second.tail).toBe('');
	});

	it('does not filter when mouse is enabled', () => {
		const input = '\x1b[<64;10;20M';
		const result = stripMouseReports(input, { mouse: true }, '');
		expect(result.filtered).toBe(input);
		expect(result.tail).toBe('');
	});
});
