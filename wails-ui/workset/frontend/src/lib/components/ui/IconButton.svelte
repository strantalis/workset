<script lang="ts">
  import type {Snippet} from 'svelte'

  interface Props {
    size?: 'sm' | 'md' | 'lg'
    label: string
    disabled?: boolean
    onclick?: () => void
    children: Snippet
  }

  let {
    size = 'md',
    label,
    disabled = false,
    onclick,
    children
  }: Props = $props()

  const sizeMap = {
    sm: 24,
    md: 28,
    lg: 36
  }
</script>

<button
  class="icon-btn {size}"
  type="button"
  aria-label={label}
  {disabled}
  {onclick}
  style:--btn-size="{sizeMap[size]}px"
>
  {@render children()}
</button>

<style>
  .icon-btn {
    width: var(--btn-size);
    height: var(--btn-size);
    border-radius: var(--radius-sm);
    border: 1px solid var(--border);
    background: rgba(255, 255, 255, 0.02);
    color: var(--text);
    cursor: pointer;
    display: grid;
    place-items: center;
    transition:
      border-color var(--transition-fast),
      background var(--transition-fast),
      transform var(--transition-fast);
  }

  .icon-btn:hover:not(:disabled) {
    border-color: var(--accent);
    background: rgba(255, 255, 255, 0.04);
  }

  .icon-btn:active:not(:disabled) {
    transform: scale(0.95);
  }

  .icon-btn:disabled {
    opacity: 0.5;
    cursor: not-allowed;
  }

  .icon-btn :global(svg) {
    stroke: currentColor;
    stroke-width: 1.6;
    fill: none;
  }

  .icon-btn.sm :global(svg) {
    width: 14px;
    height: 14px;
  }

  .icon-btn.md :global(svg) {
    width: 16px;
    height: 16px;
  }

  .icon-btn.lg :global(svg) {
    width: 18px;
    height: 18px;
  }
</style>
