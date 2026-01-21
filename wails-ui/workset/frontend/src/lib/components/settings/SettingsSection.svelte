<script lang="ts">
  interface Props {
    title: string;
    description?: string;
    children?: import('svelte').Snippet;
  }

  let { title, description = '', children }: Props = $props();
</script>

<section class="section">
  <header class="section-header">
    <h2 class="section-title">{title}</h2>
    {#if description}
      <p class="section-description">{description}</p>
    {/if}
  </header>
  <div class="section-content">
    {@render children?.()}
  </div>
</section>

<style>
  .section {
    display: flex;
    flex-direction: column;
    gap: 16px;
    height: 100%;
  }

  .section-header {
    display: flex;
    flex-direction: column;
    gap: 4px;
    padding-bottom: 12px;
    border-bottom: 1px solid var(--border);
  }

  .section-title {
    font-size: 16px;
    font-weight: 600;
    margin: 0;
  }

  .section-description {
    font-size: 13px;
    color: var(--muted);
    margin: 0;
  }

  .section-content {
    flex: 1;
    overflow-y: auto;
  }
</style>
