<script lang="ts">
  interface Props {
    title: string;
    body: string;
  }

  let { title, body }: Props = $props();
</script>

<section class="empty">
  <div class="title">{title}</div>
  <div class="body">{body}</div>
  <div class="actions">
    <button type="button">Create workspace</button>
  </div>
</section>

<style>
  .empty {
    display: flex;
    flex-direction: column;
    gap: 12px;
    align-items: flex-start;
    background: var(--panel);
    border: 1px solid var(--border);
    border-radius: 16px;
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
    gap: 12px;
  }

  button {
    background: var(--accent);
    border: none;
    color: #081018;
    padding: 8px 14px;
    border-radius: var(--radius-md);
    font-weight: 600;
    cursor: pointer;
    transition: background var(--transition-fast), transform var(--transition-fast);
  }

  button:hover:not(:disabled) {
    background: color-mix(in srgb, var(--accent) 85%, white);
  }

  button:active:not(:disabled) {
    transform: scale(0.98);
  }

  button:disabled {
    opacity: 0.5;
    cursor: not-allowed;
  }

</style>
