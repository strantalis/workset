export type PaneKeydownEvent = {
  key: string
  currentTarget: unknown | null
  target: unknown | null
}

export const shouldHandlePaneKeydown = (event: PaneKeydownEvent): boolean => {
  if (!event.currentTarget || event.currentTarget !== event.target) {
    return false
  }
  return event.key === 'Enter' || event.key === ' '
}
