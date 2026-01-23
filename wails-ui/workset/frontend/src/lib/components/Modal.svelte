<script lang="ts">
  import type {Snippet} from 'svelte'
  import Button from './ui/Button.svelte'

  interface Props {
    title: string
    subtitle?: string
    size?: 'sm' | 'md' | 'lg' | 'xl' | 'full'
    headerAlign?: 'center' | 'left'
    onClose?: () => void
    children: Snippet
    footer?: Snippet
  }

  let {
    title,
    subtitle = '',
    size = 'md',
    headerAlign = 'center',
    onClose,
    children,
    footer
  }: Props = $props()

  const sizeMap = {
    sm: '360px',
    md: '420px',
    lg: '500px',
    xl: '480px',
    full: '1120px'
  }
</script>

<div class="modal" style="--modal-width: {sizeMap[size]}">
  <header class="modal-header" class:left={headerAlign === 'left'}>
    <div class="modal-header-text">
      <div class="modal-title">{title}</div>
      {#if subtitle}
        <div class="modal-subtitle">{subtitle}</div>
      {/if}
    </div>
    {#if onClose}
      <Button variant="ghost" size="sm" onclick={onClose}>Close</Button>
    {/if}
  </header>
  <div class="modal-body">
    {@render children()}
  </div>
  {#if footer}
    <div class="modal-footer">
      {@render footer()}
    </div>
  {/if}
</div>

<style>
  .modal {
    width: min(var(--modal-width, 420px), 90%);
    padding: 20px 22px;
    border-radius: 16px;
    border: 1px solid var(--border);
    background: var(--panel-strong);
    box-shadow: 0 24px 60px rgba(6, 10, 16, 0.6);
    display: flex;
    flex-direction: column;
    gap: 16px;
  }

  .modal-header {
    text-align: center;
  }

  .modal-header.left {
    display: flex;
    align-items: center;
    justify-content: space-between;
    gap: 16px;
    text-align: left;
  }

  .modal-header-text {
    flex: 1;
    min-width: 0;
  }

  .modal-title {
    font-size: 18px;
    font-weight: 600;
    color: var(--text);
  }

  .modal-subtitle {
    font-size: 12px;
    color: var(--muted);
    margin-top: 4px;
  }

  .modal-body {
    display: flex;
    flex-direction: column;
    gap: 12px;
  }

  .modal-footer {
    display: flex;
    flex-direction: column;
    align-items: center;
    gap: 10px;
    margin-top: 4px;
  }
</style>
