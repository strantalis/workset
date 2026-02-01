/**
 * Svelte action that fires a callback when clicking outside the element.
 * Usage: <div use:clickOutside={handleClickOutside}>
 */
export function clickOutside(node: HTMLElement, callback: (event: MouseEvent) => void) {
	const handleClick = (event: MouseEvent) => {
		if (!node.contains(event.target as Node)) {
			callback(event);
		}
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
