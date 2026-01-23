<script lang="ts">
  import type {Snippet} from 'svelte'

  interface Props {
    variant?: 'primary' | 'ghost' | 'danger'
    size?: 'sm' | 'md'
    disabled?: boolean
    type?: 'button' | 'submit'
    class?: string
    onclick?: () => void
    children: Snippet
  }

  let {
    variant = 'ghost',
    size = 'md',
    disabled = false,
    type = 'button',
    class: className = '',
    onclick,
    children
  }: Props = $props()
</script>

<button
  class="btn {variant} {size} {className}"
  {type}
  {disabled}
  {onclick}
>
  {@render children()}
</button>

<style>
  .btn {
    border-radius: var(--radius-md);
    cursor: pointer;
    font-size: 13px;
    font-family: inherit;
    transition:
      background var(--transition-fast),
      border-color var(--transition-fast),
      transform var(--transition-fast);
  }

  .btn:active:not(:disabled) {
    transform: scale(0.98);
  }

  .btn:disabled {
    opacity: 0.5;
    cursor: not-allowed;
  }

  /* Size variants */
  .btn.md {
    padding: 8px 14px;
  }

  .btn.sm {
    padding: 6px 10px;
    font-size: 12px;
  }

  /* Ghost variant */
  .btn.ghost {
    background: rgba(255, 255, 255, 0.02);
    border: 1px solid var(--border);
    color: var(--text);
  }

  .btn.ghost:hover:not(:disabled) {
    border-color: var(--accent);
    background: rgba(255, 255, 255, 0.04);
  }

  /* Primary variant */
  .btn.primary {
    background: var(--accent);
    border: none;
    color: #081018;
    font-weight: 600;
  }

  .btn.primary:hover:not(:disabled) {
    background: color-mix(in srgb, var(--accent) 85%, white);
  }

  .btn.primary:disabled {
    opacity: 0.6;
  }

  /* Danger variant */
  .btn.danger {
    background: var(--danger-subtle);
    border: 1px solid var(--danger-soft);
    color: #ff9a9a;
    font-weight: 600;
  }

  .btn.danger:hover:not(:disabled) {
    background: var(--danger-soft);
    border-color: var(--danger);
  }
</style>
