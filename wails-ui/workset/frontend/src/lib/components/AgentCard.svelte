<script lang="ts">
  import type {AgentOption} from '../types'

  interface Props {
    agent: AgentOption
    available: boolean
    selected: boolean
    onclick: () => void
  }

  let {agent, available, selected, onclick}: Props = $props()

  const iconPaths: Record<string, {d: string; viewBox?: string}> = {
    codex: {
      d: 'M4 17l6-6-6-6M12 19h8',
      viewBox: '0 0 24 24'
    },
    claude: {
      d: 'M18 3a3 3 0 0 0-3 3v12a3 3 0 0 0 3 3 3 3 0 0 0 3-3 3 3 0 0 0-3-3H6a3 3 0 0 0-3 3 3 3 0 0 0 3 3 3 3 0 0 0 3-3V6a3 3 0 0 0-3-3 3 3 0 0 0-3 3 3 3 0 0 0 3 3h12a3 3 0 0 0 3-3 3 3 0 0 0-3-3z',
      viewBox: '0 0 24 24'
    },
    opencode: {
      d: 'M16 18l6-6-6-6M8 6l-6 6 6 6',
      viewBox: '0 0 24 24'
    },
    pi: {
      d: 'M4 7h16M7 7v10M17 7v10',
      viewBox: '0 0 24 24'
    },
    cursor: {
      d: 'M3 3l7.07 16.97 2.51-7.39 7.39-2.51L3 3zM13 13l6 6',
      viewBox: '0 0 24 24'
    }
  }

  const icon = $derived(iconPaths[agent.id] ?? iconPaths.codex)
</script>

<button
  type="button"
  class="agent-card"
  class:selected
  class:unavailable={!available}
  {onclick}
  disabled={!available}
>
  <div class="agent-icon">
    <svg viewBox={icon.viewBox ?? '0 0 24 24'} aria-hidden="true">
      <path d={icon.d} />
    </svg>
  </div>
  <div class="agent-info">
    <span class="agent-name">{agent.label}</span>
    {#if !available}
      <span class="agent-badge">Not installed</span>
    {/if}
  </div>
  {#if selected && available}
    <div class="agent-check">
      <svg viewBox="0 0 24 24" aria-hidden="true">
        <polyline points="20 6 9 17 4 12" />
      </svg>
    </div>
  {/if}
</button>

<style>
  .agent-card {
    display: flex;
    align-items: center;
    gap: 10px;
    padding: 10px 12px;
    background: rgba(255, 255, 255, 0.03);
    border: 1px solid var(--border);
    border-radius: var(--radius-md);
    cursor: pointer;
    transition:
      border-color var(--transition-fast),
      background var(--transition-fast),
      opacity var(--transition-fast);
    text-align: left;
  }

  .agent-card:hover:not(:disabled) {
    background: rgba(255, 255, 255, 0.06);
    border-color: rgba(255, 255, 255, 0.15);
  }

  .agent-card.selected {
    border-color: var(--accent);
    background: rgba(45, 140, 255, 0.08);
  }

  .agent-card.unavailable {
    opacity: 0.5;
    cursor: not-allowed;
  }

  .agent-icon {
    width: 32px;
    height: 32px;
    border-radius: var(--radius-sm);
    background: rgba(255, 255, 255, 0.06);
    display: grid;
    place-items: center;
    flex-shrink: 0;
  }

  .agent-icon svg {
    width: 18px;
    height: 18px;
    stroke: currentColor;
    stroke-width: 2;
    stroke-linecap: round;
    stroke-linejoin: round;
    fill: none;
    color: var(--text);
  }

  .agent-card.selected .agent-icon {
    background: rgba(45, 140, 255, 0.15);
  }

  .agent-card.selected .agent-icon svg {
    color: var(--accent);
  }

  .agent-info {
    flex: 1;
    min-width: 0;
    display: flex;
    flex-direction: column;
    gap: 2px;
  }

  .agent-name {
    font-size: 13px;
    font-weight: 500;
    color: var(--text);
  }

  .agent-badge {
    font-size: 10px;
    color: var(--warning);
    text-transform: uppercase;
    letter-spacing: 0.04em;
  }

  .agent-check {
    width: 20px;
    height: 20px;
    flex-shrink: 0;
  }

  .agent-check svg {
    width: 20px;
    height: 20px;
    stroke: var(--accent);
    stroke-width: 2.5;
    stroke-linecap: round;
    stroke-linejoin: round;
    fill: none;
  }
</style>
