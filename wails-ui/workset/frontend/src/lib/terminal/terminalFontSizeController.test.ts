import { describe, expect, it, vi } from 'vitest';
import { createTerminalFontSizeController } from './terminalFontSizeController';

describe('createTerminalFontSizeController', () => {
	it('applies increased/decreased font sizes within bounds', () => {
		const onFontSizeChange = vi.fn();
		const onCursorBlinkChange = vi.fn();
		const controller = createTerminalFontSizeController({
			onFontSizeChange,
			onCursorBlinkChange,
			defaultFontSize: 12,
			minFontSize: 10,
			maxFontSize: 13,
			step: 1,
		});

		expect(controller.getCurrentFontSize()).toBe(12);

		controller.increaseFontSize();
		expect(controller.getCurrentFontSize()).toBe(13);
		expect(onFontSizeChange).toHaveBeenLastCalledWith(13);

		controller.increaseFontSize();
		expect(controller.getCurrentFontSize()).toBe(13);

		controller.decreaseFontSize();
		expect(controller.getCurrentFontSize()).toBe(12);
		expect(onFontSizeChange).toHaveBeenLastCalledWith(12);

		controller.decreaseFontSize();
		controller.decreaseFontSize();
		expect(controller.getCurrentFontSize()).toBe(10);
	});

	it('resets to default size', () => {
		const onFontSizeChange = vi.fn();
		const onCursorBlinkChange = vi.fn();
		const controller = createTerminalFontSizeController({
			onFontSizeChange,
			onCursorBlinkChange,
			defaultFontSize: 11,
			minFontSize: 9,
			maxFontSize: 13,
		});

		controller.increaseFontSize();
		expect(controller.getCurrentFontSize()).toBe(12);

		controller.resetFontSize();
		expect(controller.getCurrentFontSize()).toBe(11);
		expect(onFontSizeChange).toHaveBeenLastCalledWith(11);
	});

	it('updates cursor blink state', () => {
		const onFontSizeChange = vi.fn();
		const onCursorBlinkChange = vi.fn();
		const controller = createTerminalFontSizeController({
			onFontSizeChange,
			onCursorBlinkChange,
			defaultCursorBlink: true,
		});

		expect(controller.getCursorBlink()).toBe(true);

		controller.setCursorBlink(false);

		expect(controller.getCursorBlink()).toBe(false);
		expect(onCursorBlinkChange).toHaveBeenLastCalledWith(false);
	});
});
