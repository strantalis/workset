<script lang="ts">
  import type {Snippet} from 'svelte'
  import {clickOutside} from '../../actions/clickOutside'

  interface Props {
    open: boolean
    onClose: () => void
    position?: 'left' | 'right'
    children: Snippet
  }

  let {
    open,
    onClose,
    position = 'right',
    children
  }: Props = $props()
</script>

{#if open}
  <div
    class="dropdown-menu {position}"
    use:clickOutside={onClose}
    role="menu"
  >
    {@render children()}
  </div>
{/if}

<style>
  .dropdown-menu {
    position: absolute;
    top: 28px;
    background: var(--panel-strong);
    border: 1px solid var(--border);
    border-radius: 10px;
    padding: 6px;
    display: grid;
    gap: 4px;
    z-index: 5;
    min-width: 140px;
    box-shadow: 0 4px 16px rgba(0, 0, 0, 0.3);
  }

  .dropdown-menu.right {
    right: 0;
  }

  .dropdown-menu.left {
    left: 0;
  }

  /* Menu item styles */
  .dropdown-menu :global(button) {
    display: flex;
    align-items: center;
    gap: 8px;
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

  .dropdown-menu :global(button svg) {
    width: 14px;
    height: 14px;
    stroke: currentColor;
    stroke-width: 1.6;
    fill: none;
    flex-shrink: 0;
  }

  .dropdown-menu :global(button:hover) {
    background: rgba(255, 255, 255, 0.06);
  }

  .dropdown-menu :global(button.danger) {
    color: var(--danger);
  }

  .dropdown-menu :global(button.danger:hover) {
    background: color-mix(in srgb, var(--danger) 15%, transparent);
  }
</style>
