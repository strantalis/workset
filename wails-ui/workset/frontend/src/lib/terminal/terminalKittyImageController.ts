export type KittyImage = {
	id: string;
	format: string;
	width: number;
	height: number;
	data: Uint8Array;
	bitmap?: ImageBitmap;
	decoding?: Promise<void>;
};

export type KittyPlacement = {
	id: number;
	imageId: string;
	row: number;
	col: number;
	rows: number;
	cols: number;
	x: number;
	y: number;
	z: number;
};

export type KittyState = {
	images: Map<string, KittyImage>;
	placements: Map<string, KittyPlacement>;
};

export type KittyOverlay = {
	underlay: HTMLCanvasElement;
	overlay: HTMLCanvasElement;
	ctxUnder: CanvasRenderingContext2D;
	ctxOver: CanvasRenderingContext2D;
	cellWidth: number;
	cellHeight: number;
	dpr: number;
	renderScheduled: boolean;
};

type KittyEventImage = {
	id: string;
	format?: string;
	width?: number;
	height?: number;
	data?: string | number[] | Uint8Array;
};

type KittyEventPlacement = {
	id: number;
	imageId: string;
	row: number;
	col: number;
	rows: number;
	cols: number;
	x?: number;
	y?: number;
	z?: number;
};

export type KittyEventPayload = {
	kind: string;
	image?: KittyEventImage;
	placement?: KittyEventPlacement;
	delete?: {
		all?: boolean;
		imageId?: string;
		placementId?: number;
	};
	snapshot?: {
		images?: KittyEventImage[];
		placements?: KittyEventPlacement[];
	};
};

export type TerminalKittyHandle = {
	terminal: {
		cols: number;
		rows: number;
	};
	container: HTMLDivElement;
	kittyState?: KittyState;
	kittyOverlay?: KittyOverlay;
};

type TerminalKittyControllerDeps<THandle extends TerminalKittyHandle> = {
	getHandle: (id: string) => THandle | undefined;
	requestFrame?: (callback: FrameRequestCallback) => number;
	createBitmap?: (blob: Blob) => Promise<ImageBitmap>;
	getDevicePixelRatio?: () => number;
};

const decodeBase64 = (input: string | number[] | Uint8Array): Uint8Array => {
	if (!input) return new Uint8Array();
	if (input instanceof Uint8Array) {
		return input;
	}
	if (Array.isArray(input)) {
		return Uint8Array.from(input);
	}
	const binary = atob(input);
	const bytes = new Uint8Array(binary.length);
	for (let i = 0; i < binary.length; i += 1) {
		bytes[i] = binary.charCodeAt(i);
	}
	return bytes;
};

const clearOverlay = (handle: TerminalKittyHandle): void => {
	if (!handle.kittyOverlay) return;
	handle.kittyOverlay.ctxUnder.clearRect(
		0,
		0,
		handle.kittyOverlay.underlay.width,
		handle.kittyOverlay.underlay.height,
	);
	handle.kittyOverlay.ctxOver.clearRect(
		0,
		0,
		handle.kittyOverlay.overlay.width,
		handle.kittyOverlay.overlay.height,
	);
};

const createKittyOverlay = (getDevicePixelRatio: () => number): KittyOverlay => {
	const underlay = document.createElement('canvas');
	const overlay = document.createElement('canvas');
	const ctxUnder = underlay.getContext('2d');
	const ctxOver = overlay.getContext('2d');
	if (!ctxUnder || !ctxOver) {
		throw new Error('Unable to initialize kitty overlay canvas.');
	}
	underlay.className = 'kitty-underlay';
	overlay.className = 'kitty-overlay';
	return {
		underlay,
		overlay,
		ctxUnder,
		ctxOver,
		cellWidth: 0,
		cellHeight: 0,
		dpr: getDevicePixelRatio() || 1,
		renderScheduled: false,
	};
};

export const createKittyState = (): KittyState => ({
	images: new Map(),
	placements: new Map(),
});

export const createTerminalKittyController = <THandle extends TerminalKittyHandle>(
	deps: TerminalKittyControllerDeps<THandle>,
): {
	ensureOverlay: (id: string) => void;
	resizeOverlay: (handle: THandle) => void;
	applyEvent: (id: string, event: KittyEventPayload) => Promise<void>;
} => {
	const requestFrame = deps.requestFrame ?? requestAnimationFrame;
	const createBitmap = deps.createBitmap ?? ((blob: Blob) => createImageBitmap(blob));
	const getDevicePixelRatio = deps.getDevicePixelRatio ?? (() => window.devicePixelRatio || 1);

	const renderOverlay = (id: string): void => {
		const handle = deps.getHandle(id);
		if (!handle?.kittyOverlay) return;
		const overlay = handle.kittyOverlay;
		overlay.ctxUnder.clearRect(0, 0, overlay.underlay.width, overlay.underlay.height);
		overlay.ctxOver.clearRect(0, 0, overlay.overlay.width, overlay.overlay.height);
		if (!handle.kittyState) return;
		for (const placement of handle.kittyState.placements.values()) {
			const image = handle.kittyState.images.get(placement.imageId);
			if (!image || !image.bitmap) continue;
			const target = placement.z >= 0 ? overlay.ctxOver : overlay.ctxUnder;
			const x = (placement.col - 1) * overlay.cellWidth * overlay.dpr;
			const y = (placement.row - 1) * overlay.cellHeight * overlay.dpr;
			const w = placement.cols * overlay.cellWidth * overlay.dpr;
			const h = placement.rows * overlay.cellHeight * overlay.dpr;
			target.drawImage(image.bitmap, x, y, w, h);
		}
	};

	const scheduleRender = (id: string): void => {
		const handle = deps.getHandle(id);
		if (!handle?.kittyOverlay || handle.kittyOverlay.renderScheduled) return;
		handle.kittyOverlay.renderScheduled = true;
		requestFrame(() => {
			const current = deps.getHandle(id);
			if (!current?.kittyOverlay) return;
			current.kittyOverlay.renderScheduled = false;
			renderOverlay(id);
		});
	};

	const resizeOverlay = (handle: THandle): void => {
		if (!handle.kittyOverlay || !handle.container) return;
		const rect = handle.container.getBoundingClientRect();
		const dpr = getDevicePixelRatio();
		if (rect.width <= 0 || rect.height <= 0) return;
		handle.kittyOverlay.dpr = dpr;
		handle.kittyOverlay.underlay.width = rect.width * dpr;
		handle.kittyOverlay.underlay.height = rect.height * dpr;
		handle.kittyOverlay.overlay.width = rect.width * dpr;
		handle.kittyOverlay.overlay.height = rect.height * dpr;
		handle.kittyOverlay.underlay.style.width = `${rect.width}px`;
		handle.kittyOverlay.underlay.style.height = `${rect.height}px`;
		handle.kittyOverlay.overlay.style.width = `${rect.width}px`;
		handle.kittyOverlay.overlay.style.height = `${rect.height}px`;
		const cols = Math.max(handle.terminal.cols, 1);
		const rows = Math.max(handle.terminal.rows, 1);
		handle.kittyOverlay.cellWidth = rect.width / cols;
		handle.kittyOverlay.cellHeight = rect.height / rows;
	};

	const ensureOverlay = (id: string): void => {
		const handle = deps.getHandle(id);
		if (!handle) return;
		if (!handle.kittyOverlay) {
			try {
				handle.kittyOverlay = createKittyOverlay(getDevicePixelRatio);
				handle.container.append(handle.kittyOverlay.underlay, handle.kittyOverlay.overlay);
			} catch {
				handle.kittyOverlay = undefined;
				return;
			}
		}
		resizeOverlay(handle);
		scheduleRender(id);
	};

	const applyEvent = async (id: string, event: KittyEventPayload): Promise<void> => {
		const handle = deps.getHandle(id);
		if (!handle) return;
		handle.kittyState ??= createKittyState();
		if (event.kind === 'clear') {
			handle.kittyState.images.clear();
			handle.kittyState.placements.clear();
			clearOverlay(handle);
			return;
		}
		if (event.kind === 'snapshot' && event.snapshot) {
			handle.kittyState.images.clear();
			handle.kittyState.placements.clear();
			const images = event.snapshot.images ?? [];
			for (const image of images) {
				if (!image?.id || !image.data) continue;
				const data = decodeBase64(image.data);
				handle.kittyState.images.set(image.id, {
					id: image.id,
					format: image.format ?? 'png',
					width: image.width ?? 0,
					height: image.height ?? 0,
					data,
				});
			}
			const placements = event.snapshot.placements ?? [];
			for (const placement of placements) {
				if (!placement) continue;
				handle.kittyState.placements.set(String(placement.id), {
					id: placement.id ?? 0,
					imageId: placement.imageId ?? '',
					row: placement.row ?? 0,
					col: placement.col ?? 0,
					rows: placement.rows ?? 0,
					cols: placement.cols ?? 0,
					x: placement.x ?? 0,
					y: placement.y ?? 0,
					z: placement.z ?? 0,
				});
			}
		}
		if (event.kind === 'image' && event.image?.id && event.image.data) {
			const data = decodeBase64(event.image.data);
			handle.kittyState.images.set(event.image.id, {
				id: event.image.id,
				format: event.image.format ?? 'png',
				width: event.image.width ?? 0,
				height: event.image.height ?? 0,
				data,
			});
		}
		if (event.kind === 'placement' && event.placement) {
			handle.kittyState.placements.set(String(event.placement.id ?? 0), {
				id: event.placement.id ?? 0,
				imageId: event.placement.imageId ?? '',
				row: event.placement.row ?? 0,
				col: event.placement.col ?? 0,
				rows: event.placement.rows ?? 0,
				cols: event.placement.cols ?? 0,
				x: event.placement.x ?? 0,
				y: event.placement.y ?? 0,
				z: event.placement.z ?? 0,
			});
		}
		if (event.kind === 'delete' && event.delete) {
			if (event.delete.all) {
				handle.kittyState.images.clear();
				handle.kittyState.placements.clear();
			} else {
				if (event.delete.imageId) {
					handle.kittyState.images.delete(event.delete.imageId);
				}
				if (event.delete.placementId) {
					handle.kittyState.placements.delete(String(event.delete.placementId));
				}
			}
		}
		for (const image of handle.kittyState.images.values()) {
			if (image.bitmap || image.decoding) continue;
			if (!image.data || image.data.length === 0) continue;
			const blobData = image.data instanceof Uint8Array ? Uint8Array.from(image.data) : image.data;
			image.decoding = createBitmap(new Blob([blobData]))
				.then((bitmap) => {
					image.bitmap = bitmap;
				})
				.catch(() => undefined)
				.finally(() => {
					image.decoding = undefined;
				});
		}
		if (handle.kittyOverlay) {
			resizeOverlay(handle);
		}
		scheduleRender(id);
	};

	return {
		ensureOverlay,
		resizeOverlay,
		applyEvent,
	};
};
