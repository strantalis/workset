export const REPO_DIFF_SIDEBAR_WIDTH_KEY = 'workset:repoDiff:sidebarWidth';
export const REPO_DIFF_MIN_SIDEBAR_WIDTH = 200;
export const REPO_DIFF_DEFAULT_SIDEBAR_WIDTH = 280;

type StorageLike = Pick<Storage, 'getItem' | 'setItem'>;

type SidebarResizeControllerOptions = {
	document: Pick<Document, 'addEventListener' | 'removeEventListener'>;
	window: Pick<Window, 'addEventListener' | 'removeEventListener'>;
	storage: StorageLike | null;
	storageKey: string;
	minWidth: number;
	getSidebarWidth: () => number;
	setSidebarWidth: (value: number) => void;
	setIsResizing: (value: boolean) => void;
};

const parsePersistedWidth = (value: string | null, minWidth: number): number | null => {
	if (!value) return null;
	const parsed = Number.parseInt(value, 10);
	if (!Number.isFinite(parsed) || parsed < minWidth) return null;
	return parsed;
};

export const createSidebarResizeController = (options: SidebarResizeControllerOptions) => {
	let cleanupListeners: (() => void) | null = null;

	const clearListeners = (): void => {
		cleanupListeners?.();
		cleanupListeners = null;
	};

	const loadPersistedWidth = (): void => {
		try {
			const width = parsePersistedWidth(
				options.storage?.getItem(options.storageKey) ?? null,
				options.minWidth,
			);
			if (width !== null) {
				options.setSidebarWidth(width);
			}
		} catch {
			// storage unavailable, keep default width
		}
	};

	const persistWidth = (): void => {
		try {
			options.storage?.setItem(options.storageKey, String(options.getSidebarWidth()));
		} catch {
			// storage unavailable, width won't persist
		}
	};

	const startResize = (event: MouseEvent): void => {
		event.preventDefault();
		clearListeners();

		options.setIsResizing(true);
		const startX = event.clientX;
		const startWidth = options.getSidebarWidth();

		const handleMouseMove = (moveEvent: MouseEvent): void => {
			const diff = moveEvent.clientX - startX;
			options.setSidebarWidth(Math.max(options.minWidth, startWidth + diff));
		};

		const handleMouseUp = (): void => {
			options.setIsResizing(false);
			persistWidth();
			clearListeners();
		};

		const handleBlur = (): void => {
			options.setIsResizing(false);
			clearListeners();
		};

		options.document.addEventListener('mousemove', handleMouseMove);
		options.document.addEventListener('mouseup', handleMouseUp);
		options.window.addEventListener('blur', handleBlur);

		cleanupListeners = () => {
			options.document.removeEventListener('mousemove', handleMouseMove);
			options.document.removeEventListener('mouseup', handleMouseUp);
			options.window.removeEventListener('blur', handleBlur);
		};
	};

	const destroy = (): void => {
		clearListeners();
	};

	return {
		loadPersistedWidth,
		startResize,
		destroy,
	};
};
