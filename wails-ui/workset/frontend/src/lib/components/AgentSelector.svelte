<script lang="ts">
  import type {AgentOption} from '../types'
  import AgentCard from './AgentCard.svelte'

  interface Props {
    agents: AgentOption[]
    selected: string
    availability: Record<string, boolean>
    availabilityStatus: 'idle' | 'loading' | 'ready' | 'error'
    onSelect: (id: string) => void
  }

  let {agents, selected, availability, availabilityStatus, onSelect}: Props = $props()

  const isAvailable = (id: string): boolean => {
    if (availabilityStatus !== 'ready') return true
    return availability[id] ?? false
  }
</script>

<div class="agent-grid">
  {#each agents as agent}
    <AgentCard
      {agent}
      available={isAvailable(agent.id)}
      selected={selected === agent.id}
      onclick={() => {
        if (isAvailable(agent.id)) {
          onSelect(agent.id)
        }
      }}
    />
  {/each}
</div>

<style>
  .agent-grid {
    display: grid;
    grid-template-columns: repeat(2, 1fr);
    gap: 8px;
  }

  @media (min-width: 440px) {
    .agent-grid {
      grid-template-columns: repeat(2, 1fr);
    }
  }
</style>
