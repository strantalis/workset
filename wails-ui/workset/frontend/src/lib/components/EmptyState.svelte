<script lang="ts">
  import Button from './ui/Button.svelte'

  interface Props {
    title: string;
    body: string;
    actionLabel?: string;
    onAction?: () => void;
  }

  let {
    title,
    body,
    actionLabel = 'Create workspace',
    onAction
  }: Props = $props();
</script>

<section class="empty">
  <div class="title">{title}</div>
  <div class="body">{body}</div>
  {#if onAction}
    <div class="actions">
      <Button variant="primary" onclick={onAction}>{actionLabel}</Button>
    </div>
  {/if}
</section>

<style>
  .empty {
    display: flex;
    flex-direction: column;
    gap: var(--space-3);
    align-items: flex-start;
    background: var(--panel);
    border: 1px solid var(--border);
    border-radius: var(--radius-lg);
    padding: 32px;
  }

  .title {
    font-size: 20px;
    font-weight: 600;
  }

  .body {
    color: var(--muted);
    font-size: 14px;
  }

  .actions {
    display: flex;
    gap: var(--space-3);
    margin-top: var(--space-2);
  }
</style>
