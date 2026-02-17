export type WorksetLayoutMode = 'grid' | 'list';
export type WorksetGroupMode = 'all' | 'template' | 'repo' | 'active';

export const WORKSET_HUB_LAYOUT_MODE_KEY = 'workset:workset-hub-layout-mode';
export const WORKSET_HUB_GROUP_MODE_KEY = 'workset:workset-hub-group-mode';

const DEFAULT_LAYOUT_MODE: WorksetLayoutMode = 'grid';
const DEFAULT_GROUP_MODE: WorksetGroupMode = 'active';

export const parseWorksetHubLayoutMode = (value: string | null): WorksetLayoutMode =>
	value === 'list' || value === 'grid' ? value : DEFAULT_LAYOUT_MODE;

export const parseWorksetHubGroupMode = (value: string | null): WorksetGroupMode =>
	value === 'all' || value === 'template' || value === 'repo' || value === 'active'
		? value
		: DEFAULT_GROUP_MODE;

export const readWorksetHubLayoutMode = (): WorksetLayoutMode => {
	if (typeof localStorage === 'undefined') return DEFAULT_LAYOUT_MODE;
	return parseWorksetHubLayoutMode(localStorage.getItem(WORKSET_HUB_LAYOUT_MODE_KEY));
};

export const readWorksetHubGroupMode = (): WorksetGroupMode => {
	if (typeof localStorage === 'undefined') return DEFAULT_GROUP_MODE;
	return parseWorksetHubGroupMode(localStorage.getItem(WORKSET_HUB_GROUP_MODE_KEY));
};

export const persistWorksetHubLayoutMode = (value: WorksetLayoutMode): void => {
	if (typeof localStorage === 'undefined') return;
	localStorage.setItem(WORKSET_HUB_LAYOUT_MODE_KEY, value);
};

export const persistWorksetHubGroupMode = (value: WorksetGroupMode): void => {
	if (typeof localStorage === 'undefined') return;
	localStorage.setItem(WORKSET_HUB_GROUP_MODE_KEY, value);
};
