import type { FileDiffMetadata } from '@pierre/diffs';
import { describe, expect, it, vi } from 'vitest';
import type { DiffLineAnnotation } from './annotations';
import {
	createDiffRenderController,
	type FileDiffRenderOptions,
	type FileDiffRenderer,
	type FileDiffRendererModule,
} from './diffRenderController';

type Annotation = { id: string };

type Setup = ReturnType<typeof createSetup>;

const createRendererInstance = (): FileDiffRenderer<Annotation> => ({
	setOptions: vi.fn(),
	render: vi.fn(),
	cleanUp: vi.fn(),
});

const createSetup = () => {
	let diffModule: FileDiffRendererModule<Annotation> | null = null;
	let selectedDiff: FileDiffMetadata | null = {
		path: 'src/main.ts',
	} as unknown as FileDiffMetadata;
	let diffContainer: HTMLElement | null = document.createElement('div');

	const annotations: DiffLineAnnotation<Annotation>[] = [
		{ side: 'additions', lineNumber: 19, metadata: { id: 'annotation-1' } },
	];
	const buildOptions = vi.fn(() => ({ diffStyle: 'split' }) as FileDiffRenderOptions<Annotation>);
	const getLineAnnotations = vi.fn(() => annotations);

	const instances: FileDiffRenderer<Annotation>[] = [];
	const fileDiffConstructor = vi.fn().mockImplementation(function (
		_options?: FileDiffRenderOptions<Annotation>,
	) {
		const instance = createRendererInstance();
		instances.push(instance);
		return instance;
	});
	diffModule = {
		FileDiff: fileDiffConstructor as unknown as FileDiffRendererModule<Annotation>['FileDiff'],
	};

	const frameCallbacks: FrameRequestCallback[] = [];
	const requestAnimationFrame = vi.fn((callback: FrameRequestCallback) => {
		frameCallbacks.push(callback);
		callback(0);
		return frameCallbacks.length;
	});

	const timeoutCallbacks: Array<() => void> = [];
	const setTimeout = vi.fn((callback: () => void) => {
		timeoutCallbacks.push(callback);
		return timeoutCallbacks.length;
	});

	const controller = createDiffRenderController<Annotation>({
		getDiffModule: () => diffModule,
		getSelectedDiff: () => selectedDiff,
		getDiffContainer: () => diffContainer,
		buildOptions,
		getLineAnnotations,
		requestAnimationFrame,
		setTimeout,
	});

	return {
		controller,
		instances,
		fileDiffConstructor,
		buildOptions,
		getLineAnnotations,
		requestAnimationFrame,
		timeoutCallbacks,
		setTimeout,
		setDiffModule: (value: FileDiffRendererModule<Annotation> | null) => {
			diffModule = value;
		},
		setSelectedDiff: (value: FileDiffMetadata | null) => {
			selectedDiff = value;
		},
		setDiffContainer: (value: HTMLElement | null) => {
			diffContainer = value;
		},
	};
};

const addLineCell = (
	setup: Setup,
	lineNumber: number,
): { row: HTMLTableRowElement; cell: HTMLTableCellElement } => {
	const table = document.createElement('table');
	const row = document.createElement('tr');
	const cell = document.createElement('td');
	cell.className = 'line-num';
	cell.dataset.content = String(lineNumber);
	row.appendChild(cell);
	table.appendChild(row);
	setup.setDiffContainer(table);
	return { row, cell };
};

describe('diffRenderController', () => {
	it('creates renderer once and updates options on subsequent renders', () => {
		const setup = createSetup();

		setup.controller.renderDiff();
		expect(setup.fileDiffConstructor).toHaveBeenCalledTimes(1);
		expect(setup.buildOptions).toHaveBeenCalledTimes(1);
		expect(setup.instances).toHaveLength(1);
		expect(setup.instances[0].render).toHaveBeenCalledWith({
			fileDiff: expect.any(Object),
			fileContainer: expect.any(HTMLElement),
			forceRender: true,
			lineAnnotations: [{ side: 'additions', lineNumber: 19, metadata: { id: 'annotation-1' } }],
		});

		setup.controller.renderDiff();
		expect(setup.fileDiffConstructor).toHaveBeenCalledTimes(1);
		expect(setup.buildOptions).toHaveBeenCalledTimes(2);
		expect(setup.instances[0].setOptions).toHaveBeenCalledTimes(1);
		expect(setup.instances[0].render).toHaveBeenCalledTimes(2);
		expect(setup.getLineAnnotations).toHaveBeenCalledTimes(2);
	});

	it('applies pending line scroll and highlight exactly once per pending request', () => {
		const setup = createSetup();
		const { row, cell } = addLineCell(setup, 19);
		const scrollIntoView = vi.fn();
		cell.scrollIntoView = scrollIntoView;

		setup.controller.setPendingScrollLine(19);
		setup.controller.renderDiff();

		expect(setup.requestAnimationFrame).toHaveBeenCalledTimes(1);
		expect(scrollIntoView).toHaveBeenCalledWith({ behavior: 'smooth', block: 'center' });
		expect(row.classList.contains('highlight-line')).toBe(true);
		expect(setup.setTimeout).toHaveBeenCalledTimes(1);

		setup.timeoutCallbacks[0]();
		expect(row.classList.contains('highlight-line')).toBe(false);

		setup.controller.renderDiff();
		expect(scrollIntoView).toHaveBeenCalledTimes(1);
	});

	it('noops when renderer prerequisites are unavailable and cleanup resets renderer', () => {
		const setup = createSetup();

		setup.setDiffModule(null);
		setup.controller.renderDiff();
		expect(setup.fileDiffConstructor).not.toHaveBeenCalled();

		setup.setDiffModule({
			FileDiff:
				setup.fileDiffConstructor as unknown as FileDiffRendererModule<Annotation>['FileDiff'],
		});
		setup.setSelectedDiff(null);
		setup.controller.renderDiff();
		expect(setup.fileDiffConstructor).not.toHaveBeenCalled();

		setup.setSelectedDiff({ path: 'src/main.ts' } as unknown as FileDiffMetadata);
		setup.setDiffContainer(null);
		setup.controller.renderDiff();
		expect(setup.fileDiffConstructor).not.toHaveBeenCalled();

		const { cell } = addLineCell(setup, 19);
		const scrollIntoView = vi.fn();
		cell.scrollIntoView = scrollIntoView;
		setup.controller.renderDiff();
		expect(setup.fileDiffConstructor).toHaveBeenCalledTimes(1);

		setup.controller.setPendingScrollLine(19);
		setup.controller.cleanUp();
		expect(setup.instances[0].cleanUp).toHaveBeenCalledTimes(1);

		setup.controller.renderDiff();
		expect(setup.fileDiffConstructor).toHaveBeenCalledTimes(2);
		expect(scrollIntoView).not.toHaveBeenCalled();
	});
});
