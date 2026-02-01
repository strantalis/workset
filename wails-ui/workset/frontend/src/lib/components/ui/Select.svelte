<script lang="ts">
	import { clickOutside } from '../../actions/clickOutside';

	interface Option {
		label: string;
		value: string;
	}

	interface Props {
		id?: string;
		value: string;
		options: Option[];
		placeholder?: string;
		disabled?: boolean;
		onchange?: (value: string) => void;
	}

	let {
		id, // eslint-disable-line prefer-const
		value = $bindable(),
		options, // eslint-disable-line prefer-const
		placeholder = 'Select...', // eslint-disable-line prefer-const
		disabled = false, // eslint-disable-line prefer-const
		onchange, // eslint-disable-line prefer-const
	}: Props = $props();

	let open = $state(false);
	let highlightedIndex = $state(-1);
	let triggerRef = $state<HTMLButtonElement | null>(null);
	let menuRef = $state<HTMLDivElement | null>(null);
	let menuStyle = $state('');

	const selectedOption = $derived(options.find((opt) => opt.value === value));
	const displayText = $derived(selectedOption?.label ?? placeholder);

	function updateMenuPosition() {
		if (!triggerRef || !open) return;
		const rect = triggerRef.getBoundingClientRect();
		const spaceBelow = window.innerHeight - rect.bottom - 20; // 20px margin from bottom
		const maxHeight = Math.max(120, Math.min(280, spaceBelow)); // Between 120-280px
		menuStyle = `
      position: fixed;
      top: ${rect.bottom + 4}px;
      left: ${rect.left}px;
      width: ${rect.width}px;
      max-height: ${maxHeight}px;
    `;
	}

	function toggle() {
		if (disabled) return;
		open = !open;
		if (open) {
			highlightedIndex = options.findIndex((opt) => opt.value === value);
			if (highlightedIndex === -1) highlightedIndex = 0;
			// Position menu after state update
			requestAnimationFrame(updateMenuPosition);
		}
	}

	function close(event?: MouseEvent) {
		// Don't close if clicking on the trigger button (handled by toggle)
		if (
			event &&
			triggerRef &&
			(event.target === triggerRef || triggerRef.contains(event.target as Node))
		) {
			return;
		}
		open = false;
		highlightedIndex = -1;
	}

	function selectOption(option: Option) {
		value = option.value;
		onchange?.(option.value);
		close();
		triggerRef?.focus();
	}

	function handleKeydown(event: KeyboardEvent) {
		if (disabled) return;

		switch (event.key) {
			case 'Enter':
			case ' ':
				event.preventDefault();
				if (open && highlightedIndex >= 0) {
					selectOption(options[highlightedIndex]);
				} else {
					toggle();
				}
				break;
			case 'Escape':
				event.preventDefault();
				close();
				triggerRef?.focus();
				break;
			case 'ArrowDown':
				event.preventDefault();
				if (!open) {
					open = true;
					highlightedIndex = options.findIndex((opt) => opt.value === value);
					if (highlightedIndex === -1) highlightedIndex = 0;
					requestAnimationFrame(updateMenuPosition);
				} else {
					highlightedIndex = Math.min(highlightedIndex + 1, options.length - 1);
				}
				break;
			case 'ArrowUp':
				event.preventDefault();
				if (open) {
					highlightedIndex = Math.max(highlightedIndex - 1, 0);
				}
				break;
			case 'Home':
				event.preventDefault();
				if (open) highlightedIndex = 0;
				break;
			case 'End':
				event.preventDefault();
				if (open) highlightedIndex = options.length - 1;
				break;
			case 'Tab':
				if (open) close();
				break;
		}
	}

	// Update position on scroll/resize
	$effect(() => {
		if (!open) return;

		const handleScroll = () => updateMenuPosition();
		const handleResize = () => updateMenuPosition();

		window.addEventListener('scroll', handleScroll, true);
		window.addEventListener('resize', handleResize);

		return () => {
			window.removeEventListener('scroll', handleScroll, true);
			window.removeEventListener('resize', handleResize);
		};
	});
</script>

<div class="select-wrapper" class:disabled>
	<button
		bind:this={triggerRef}
		type="button"
		{id}
		class="select-trigger"
		class:open
		class:placeholder={!selectedOption}
		aria-haspopup="listbox"
		aria-expanded={open}
		{disabled}
		onclick={toggle}
		onkeydown={handleKeydown}
	>
		<span class="select-value">{displayText}</span>
		<svg class="chevron" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
			<polyline points="6 9 12 15 18 9"></polyline>
		</svg>
	</button>
</div>

{#if open}
	<div
		bind:this={menuRef}
		class="select-menu"
		style={menuStyle}
		role="listbox"
		tabindex="-1"
		aria-activedescendant={highlightedIndex >= 0 ? `option-${highlightedIndex}` : undefined}
		use:clickOutside={close}
	>
		{#each options as option, index (option.value)}
			<button
				type="button"
				id="option-{index}"
				class="select-option"
				class:selected={option.value === value}
				class:highlighted={index === highlightedIndex}
				role="option"
				aria-selected={option.value === value}
				onclick={() => selectOption(option)}
				onmouseenter={() => (highlightedIndex = index)}
			>
				{option.label}
				{#if option.value === value}
					<svg
						class="check"
						viewBox="0 0 24 24"
						fill="none"
						stroke="currentColor"
						stroke-width="2.5"
					>
						<polyline points="20 6 9 17 4 12"></polyline>
					</svg>
				{/if}
			</button>
		{/each}
	</div>
{/if}

<style>
	.select-wrapper {
		position: relative;
		width: 100%;
	}

	.select-wrapper.disabled {
		opacity: 0.5;
		pointer-events: none;
	}

	.select-trigger {
		display: flex;
		align-items: center;
		justify-content: space-between;
		gap: 8px;
		width: 100%;
		background: var(--panel-strong);
		border: 1px solid rgba(255, 255, 255, 0.08);
		color: var(--text);
		border-radius: var(--radius-md);
		padding: 10px 12px;
		font-size: 13px;
		font-family: inherit;
		text-align: left;
		cursor: pointer;
		transition:
			border-color var(--transition-fast),
			box-shadow var(--transition-fast);
	}

	.select-trigger:focus {
		outline: none;
		border-color: var(--accent);
		box-shadow: 0 0 0 2px var(--accent-soft);
	}

	.select-trigger.open {
		border-color: var(--accent);
	}

	.select-trigger.placeholder .select-value {
		color: var(--muted);
	}

	.select-value {
		flex: 1;
		overflow: hidden;
		text-overflow: ellipsis;
		white-space: nowrap;
	}

	.chevron {
		width: 14px;
		height: 14px;
		flex-shrink: 0;
		color: var(--muted);
		transition: transform var(--transition-fast);
	}

	.select-trigger.open .chevron {
		transform: rotate(180deg);
	}

	.select-menu {
		background: #141f2e;
		border: 1px solid rgba(255, 255, 255, 0.1);
		border-radius: 10px;
		padding: 6px;
		z-index: 9999;
		box-shadow: 0 8px 24px rgba(0, 0, 0, 0.5);
		overflow-y: auto;
		scrollbar-width: thin;
		scrollbar-color: rgba(255, 255, 255, 0.2) transparent;
	}

	.select-menu::-webkit-scrollbar {
		width: 6px;
	}

	.select-menu::-webkit-scrollbar-track {
		background: transparent;
		margin: 6px 0;
	}

	.select-menu::-webkit-scrollbar-thumb {
		background: rgba(255, 255, 255, 0.2);
		border-radius: 3px;
	}

	.select-menu::-webkit-scrollbar-thumb:hover {
		background: rgba(255, 255, 255, 0.3);
	}

	.select-option {
		display: flex;
		align-items: center;
		justify-content: space-between;
		gap: 8px;
		width: 100%;
		background: none;
		border: none;
		color: var(--text);
		text-align: left;
		padding: 8px 12px;
		border-radius: var(--radius-sm);
		cursor: pointer;
		font-size: 13px;
		font-family: inherit;
		transition: background var(--transition-fast);
	}

	.select-option:hover,
	.select-option.highlighted {
		background: rgba(255, 255, 255, 0.06);
	}

	.select-option.selected {
		color: var(--accent);
	}

	.check {
		width: 14px;
		height: 14px;
		flex-shrink: 0;
		color: var(--accent);
	}
</style>
