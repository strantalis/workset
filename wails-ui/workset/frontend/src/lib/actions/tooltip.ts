/**
 * Svelte action that renders a styled tooltip appended to document.body,
 * so it escapes any overflow:hidden ancestors.
 *
 * Usage: <button use:tooltip={"Save file"}>
 */
export function tooltip(node: HTMLElement, label?: string) {
	let tip: HTMLDivElement | null = null;
	let arrow: HTMLDivElement | null = null;
	let currentLabel = label;

	const show = () => {
		if (!currentLabel) return;

		tip = document.createElement('div');
		tip.className = 'ws-tooltip';
		tip.textContent = currentLabel;

		arrow = document.createElement('div');
		arrow.className = 'ws-tooltip-arrow';
		tip.appendChild(arrow);

		document.body.appendChild(tip);
		position();
	};

	const position = () => {
		if (!tip) return;
		const rect = node.getBoundingClientRect();
		const tipRect = tip.getBoundingClientRect();

		let left = rect.left + rect.width / 2 - tipRect.width / 2;
		const top = rect.bottom + 8;

		// Keep tooltip within viewport horizontally
		const pad = 6;
		if (left < pad) left = pad;
		if (left + tipRect.width > window.innerWidth - pad) {
			left = window.innerWidth - pad - tipRect.width;
		}

		tip.style.left = `${left}px`;
		tip.style.top = `${top}px`;

		// Position arrow centered on the trigger element
		if (arrow) {
			const arrowLeft = rect.left + rect.width / 2 - left;
			arrow.style.left = `${arrowLeft}px`;
		}
	};

	const hide = () => {
		tip?.remove();
		tip = null;
		arrow = null;
	};

	node.addEventListener('pointerenter', show);
	node.addEventListener('pointerleave', hide);

	return {
		update(newLabel?: string) {
			currentLabel = newLabel;
			if (tip && currentLabel) {
				const firstText = tip.childNodes[0];
				if (firstText?.nodeType === Node.TEXT_NODE) {
					firstText.textContent = currentLabel;
				}
				position();
			} else if (tip && !currentLabel) {
				hide();
			}
		},
		destroy() {
			hide();
			node.removeEventListener('pointerenter', show);
			node.removeEventListener('pointerleave', hide);
		},
	};
}
