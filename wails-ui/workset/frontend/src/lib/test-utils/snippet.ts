import type { Snippet } from 'svelte';

export const asSnippet = (value: string): Snippet => {
	return (() => value) as unknown as Snippet;
};
