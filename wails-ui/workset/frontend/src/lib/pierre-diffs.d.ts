declare module '@pierre/diffs' {
	export type FileDiffMetadata = {
		name: string;
		prevName?: string;
		type: 'change' | 'rename-pure' | 'rename-changed' | 'new' | 'deleted';
		hunks: Array<{
			additionCount: number;
			deletionCount: number;
		}>;
	};

	export type FileDiffOptions = {
		theme?: string | Record<'dark' | 'light', string>;
		themeType?: 'system' | 'light' | 'dark';
		diffStyle?: 'unified' | 'split';
		diffIndicators?: 'classic' | 'bars' | 'none';
		hunkSeparators?: 'simple' | 'metadata' | 'line-info' | 'custom';
		lineDiffType?: 'word-alt' | 'word' | 'char' | 'none';
		overflow?: 'scroll' | 'wrap';
		disableFileHeader?: boolean;
	};

	export type ParsedPatch = {
		files: FileDiffMetadata[];
		patchMetadata?: string;
	};

	export class FileDiff {
		constructor(options?: FileDiffOptions);
		setOptions(options?: FileDiffOptions): void;
		render(props: {
			fileDiff?: FileDiffMetadata;
			fileContainer?: HTMLElement;
			forceRender?: boolean;
		}): void;
		cleanUp(): void;
	}

	export function parsePatchFiles(data: string): ParsedPatch[];
}

declare module '@pierre/diffs/dist/components/web-components' {}
