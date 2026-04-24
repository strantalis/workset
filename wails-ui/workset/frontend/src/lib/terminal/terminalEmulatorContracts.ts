export type TerminalDisposable = {
	dispose: () => void;
};

export type TerminalSnapshotLike = {
	version: number;
	nextOffset: number;
	cols: number;
	rows: number;
	activeBuffer: 'normal' | 'alternate';
	normalViewportY: number;
	cursor: {
		x: number;
		y: number;
		visible: boolean;
	};
	modes: {
		dec: number[];
		ansi: number[];
	};
	normalTail: string[];
	normalScreen?: string[];
	alternateScreen?: string[];
};

type BivariantHandler<T> = {
	bivarianceHack: (value: T) => void;
}['bivarianceHack'];

export type TerminalElementLike = {
	parentElement: Element | null;
};

export type FitDimensions = {
	cols: number;
	rows: number;
};

export type TerminalLinkRange = {
	start: { x: number; y: number };
	end: { x: number; y: number };
};

export type TerminalAttachOpenLike = {
	element?: TerminalElementLike | null;
	open: (container: HTMLElement) => void;
	focus: () => void;
};

export type TerminalViewportLike = {
	element?: TerminalElementLike | null;
	buffer: {
		active: {
			baseY: number;
			viewportY: number;
		};
	};
	clearSelection?: () => void;
	scrollToBottom: () => void;
	focus: () => void;
};

export type TerminalResettableLike = {
	reset: () => void;
	clear: () => void;
	scrollToBottom: () => void;
};

export type TerminalWritableLike = {
	write: (data: string | Uint8Array, callback?: () => void) => void;
};

export type TerminalEventLike = {
	onData: (callback: (data: string) => void) => TerminalDisposable;
	onResponse?: (callback: (data: string) => void) => TerminalDisposable;
	onResize?: (callback: (event: { cols: number; rows: number }) => void) => TerminalDisposable;
	onScroll?: (callback: (viewportY: number) => void) => TerminalDisposable;
};

export type TerminalAddonLike = TerminalDisposable;

export type TerminalLinkProviderLike = {
	provideLinks: (
		y: number,
		callback: (
			links:
				| {
						text: string;
						range: TerminalLinkRange;
						activate: (event: MouseEvent) => void;
				  }[]
				| undefined,
		) => void,
	) => void;
	dispose?: () => void;
};

export type FitAddonLike = TerminalAddonLike & {
	fit: () => void;
	proposeDimensions: () => FitDimensions | undefined;
};

export type TerminalLike = TerminalAttachOpenLike &
	TerminalViewportLike &
	TerminalResettableLike &
	TerminalWritableLike &
	TerminalEventLike &
	TerminalDisposable & {
		cols: number;
		rows: number;
		serializeState?: (options?: {
			nextOffset?: number;
			normalTailRows?: number;
		}) => TerminalSnapshotLike;
		restoreState?: (snapshot: TerminalSnapshotLike) => Promise<void> | void;
		options: {
			fontSize: number;
			cursorBlink?: boolean;
		};
		loadAddon?: BivariantHandler<TerminalAddonLike>;
		registerLinkProvider?: BivariantHandler<TerminalLinkProviderLike>;
	};

export type TerminalAttachOpenHandle = {
	terminal: TerminalAttachOpenLike;
	container: HTMLDivElement;
	opened?: boolean;
	openWindow?: Window | null;
};
