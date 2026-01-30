<script lang="ts">
  interface Props {
    activeSection: string;
    onSelectSection: (section: string) => void;
    aliasCount?: number;
    groupCount?: number;
  }

  let {
    activeSection,
    onSelectSection,
    aliasCount = 0,
    groupCount = 0
  }: Props = $props();

  type SidebarItem = {
    id: string
    label: string
    count?: number
  }

  type SidebarGroup = {
    title: string
    items: SidebarItem[]
  }

  let groups = $derived([
    {
      title: 'GENERAL',
      items: [
        {id: 'workspace', label: 'Workspace'},
        {id: 'agent', label: 'Agent'},
        {id: 'session', label: 'Terminal'}
      ]
    },
    {
      title: 'INTEGRATIONS',
      items: [
        {id: 'github', label: 'GitHub'}
      ]
    },
    {
      title: 'LIBRARY',
      items: [
        {id: 'aliases', label: 'Aliases', count: aliasCount},
        {id: 'groups', label: 'Groups', count: groupCount}
      ]
    }
  ] as SidebarGroup[])
</script>

<nav class="sidebar">
  {#each groups as group}
    <div class="group">
      <div class="group-title">{group.title}</div>
      {#each group.items as item}
        <button
          class="item"
          class:active={activeSection === item.id}
          type="button"
          onclick={() => onSelectSection(item.id)}
        >
          <span class="label">{item.label}</span>
          {#if item.count !== undefined && item.count > 0}
            <span class="badge">{item.count}</span>
          {/if}
        </button>
      {/each}
    </div>
  {/each}
</nav>

<style>
  .sidebar {
    display: flex;
    flex-direction: column;
    gap: 20px;
    min-width: 160px;
    padding-right: 16px;
    border-right: 1px solid var(--border);
  }

  .group {
    display: flex;
    flex-direction: column;
    gap: 4px;
  }

  .group-title {
    font-size: 10px;
    font-weight: 600;
    text-transform: uppercase;
    letter-spacing: 0.1em;
    color: var(--muted);
    padding: 4px 8px;
  }

  .item {
    display: flex;
    align-items: center;
    justify-content: space-between;
    gap: 8px;
    padding: 8px 12px;
    border: none;
    background: transparent;
    color: var(--text);
    font-size: 13px;
    text-align: left;
    border-radius: var(--radius-md);
    cursor: pointer;
    transition: background var(--transition-fast), color var(--transition-fast);
  }

  .item:hover {
    background: rgba(255, 255, 255, 0.04);
  }

  .item.active {
    background: rgba(255, 255, 255, 0.08);
    color: var(--accent);
  }

  .label {
    flex: 1;
  }

  .badge {
    background: rgba(255, 255, 255, 0.1);
    color: var(--muted);
    font-size: 11px;
    font-weight: 600;
    padding: 2px 6px;
    border-radius: 999px;
    min-width: 20px;
    text-align: center;
  }

  .item.active .badge {
    background: var(--accent-soft);
    color: var(--accent);
  }

  @media (max-width: 720px) {
    .sidebar {
      flex-direction: row;
      min-width: unset;
      padding-right: 0;
      padding-bottom: 12px;
      border-right: none;
      border-bottom: 1px solid var(--border);
      overflow-x: auto;
      gap: 12px;
    }

    .group {
      flex-direction: row;
      gap: 4px;
    }

    .group-title {
      display: none;
    }

    .item {
      white-space: nowrap;
      padding: 6px 10px;
    }
  }
</style>
