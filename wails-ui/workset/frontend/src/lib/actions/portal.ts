/**
 * Svelte action that teleports element to document body.
 * Usage: <div use:portal>
 */
export function portal(node: HTMLElement) {
	const target = document.body;
	target.appendChild(node);

	return {
		destroy() {
			if (node.parentNode) {
				node.parentNode.removeChild(node);
			}
		},
	};
}
