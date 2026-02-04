export interface ClickOutsideOptions {
	callback: (event: MouseEvent) => void;
	exclude?: HTMLElement | null;
}

/**
 * Svelte action that fires a callback when clicking outside the element.
 * Usage: <div use:clickOutside={{ callback: handleClickOutside, exclude: triggerElement }}>
 */
export function clickOutside(node: HTMLElement, options: ClickOutsideOptions) {
	const handleClick = (event: MouseEvent) => {
		const target = event.target as Node;
		// Don't trigger if clicking inside the node or on the excluded element
		if (node.contains(target) || (options.exclude && options.exclude.contains(target))) {
			return;
		}
		options.callback(event);
	};

	// Delay listener attachment to avoid triggering on the click that opened the element
	setTimeout(() => {
		document.addEventListener('click', handleClick, true);
	}, 0);

	return {
		destroy() {
			document.removeEventListener('click', handleClick, true);
		},
	};
}
