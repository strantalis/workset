import type { FileDiffMetadata, FileDiffOptions as FileDiffOptionsBase } from '@pierre/diffs';
import type { DiffLineAnnotation } from './annotations';

export type FileDiffRenderOptions<TAnnotation = undefined> = FileDiffOptionsBase & {
	renderAnnotation?: (annotation: DiffLineAnnotation<TAnnotation>) => HTMLElement | undefined;
};

export type FileDiffRenderer<TAnnotation = undefined> = {
	setOptions(options: FileDiffRenderOptions<TAnnotation> | undefined): void;
	render(props: {
		fileDiff?: FileDiffMetadata;
		oldFile?: unknown;
		newFile?: unknown;
		forceRender?: boolean;
		fileContainer?: HTMLElement;
		containerWrapper?: HTMLElement;
		lineAnnotations?: DiffLineAnnotation<TAnnotation>[];
	}): void;
	cleanUp(): void;
};

export type FileDiffRendererModule<TAnnotation = undefined> = {
	FileDiff: new (options?: FileDiffRenderOptions<TAnnotation>) => FileDiffRenderer<TAnnotation>;
};

type DiffRenderControllerOptions<TAnnotation> = {
	getDiffModule: () => FileDiffRendererModule<TAnnotation> | null;
	getSelectedDiff: () => FileDiffMetadata | null;
	getDiffContainer: () => HTMLElement | null;
	buildOptions: () => FileDiffRenderOptions<TAnnotation>;
	getLineAnnotations: () => DiffLineAnnotation<TAnnotation>[];
	requestAnimationFrame: (callback: FrameRequestCallback) => number;
	setTimeout: (callback: () => void, milliseconds: number) => unknown;
};

const LINE_SELECTOR = (lineNumber: number): string =>
	`[data-line-number="${lineNumber}"], td.line-num[data-content="${lineNumber}"], .line-num[data-content="${lineNumber}"]`;

export const createDiffRenderController = <TAnnotation>(
	options: DiffRenderControllerOptions<TAnnotation>,
) => {
	let diffInstance: FileDiffRenderer<TAnnotation> | null = null;
	let pendingScrollLine: number | null = null;

	const renderDiff = (): void => {
		const diffModule = options.getDiffModule();
		const selectedDiff = options.getSelectedDiff();
		const diffContainer = options.getDiffContainer();
		if (!diffModule || !selectedDiff || !diffContainer) return;

		if (!diffInstance) {
			diffInstance = new diffModule.FileDiff(options.buildOptions());
		} else {
			diffInstance.setOptions(options.buildOptions());
		}

		diffInstance.render({
			fileDiff: selectedDiff,
			fileContainer: diffContainer,
			forceRender: true,
			lineAnnotations: options.getLineAnnotations(),
		});

		if (pendingScrollLine === null) return;

		const lineToScroll = pendingScrollLine;
		pendingScrollLine = null;
		options.requestAnimationFrame(() => {
			const lineEl = options.getDiffContainer()?.querySelector(LINE_SELECTOR(lineToScroll));
			if (!lineEl) return;

			lineEl.scrollIntoView({ behavior: 'smooth', block: 'center' });
			const row = lineEl.closest('tr');
			if (!row) return;

			row.classList.add('highlight-line');
			options.setTimeout(() => row.classList.remove('highlight-line'), 2000);
		});
	};

	const setPendingScrollLine = (line: number): void => {
		pendingScrollLine = line;
	};

	const cleanUp = (): void => {
		pendingScrollLine = null;
		diffInstance?.cleanUp();
		diffInstance = null;
	};

	return {
		renderDiff,
		setPendingScrollLine,
		cleanUp,
	};
};
