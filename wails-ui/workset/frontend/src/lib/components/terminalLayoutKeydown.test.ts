import {describe, expect, it} from 'vitest'
import {shouldHandlePaneKeydown} from './terminalLayoutKeydown'

describe('shouldHandlePaneKeydown', () => {
  it('ignores key events from child targets', () => {
    const target = {}
    const currentTarget = {}

    const handled = shouldHandlePaneKeydown({
      key: ' ',
      currentTarget,
      target
    })

    expect(handled).toBe(false)
  })

  it('allows Enter when the pane itself is focused', () => {
    const pane = {}

    const handled = shouldHandlePaneKeydown({
      key: 'Enter',
      currentTarget: pane,
      target: pane
    })

    expect(handled).toBe(true)
  })

  it('allows Space when the pane itself is focused', () => {
    const pane = {}

    const handled = shouldHandlePaneKeydown({
      key: ' ',
      currentTarget: pane,
      target: pane
    })

    expect(handled).toBe(true)
  })
})
