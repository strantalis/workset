import {describe, expect, it} from 'vitest'
import {stripMouseReports, stripTerminalReports} from './inputFilter'

describe('stripTerminalReports', () => {
  it('drops OSC color replies', () => {
    const input = '\x1b]11;rgb:1414/1f1f/2e2e\x07'
    const result = stripTerminalReports(input, {altScreen: false, mouse: false}, '')
    expect(result.filtered).toBe('')
    expect(result.tail).toBe('')
  })

  it('buffers split OSC sequences', () => {
    const first = stripTerminalReports('\x1b]11;rgb:1414/1f', {altScreen: false, mouse: false}, '')
    expect(first.filtered).toBe('')
    expect(first.tail).toBe('\x1b]11;rgb:1414/1f')
    const second = stripTerminalReports('1f/2e2e\x07', {altScreen: false, mouse: false}, first.tail)
    expect(second.filtered).toBe('')
    expect(second.tail).toBe('')
  })

  it('drops CSI report responses but keeps normal CSI', () => {
    const input = `A\x1b[12;34RB\x1b[2JC`
    const result = stripTerminalReports(input, {altScreen: false, mouse: false}, '')
    expect(result.filtered).toBe('AB\x1b[2JC')
  })

  it('drops DECRQM responses', () => {
    const input = '\x1b[?1004;2$y'
    const result = stripTerminalReports(input, {altScreen: false, mouse: false}, '')
    expect(result.filtered).toBe('')
  })

  it('does not filter when in alt screen', () => {
    const input = '\x1b]11;rgb:1414/1f1f/2e2e\x07'
    const result = stripTerminalReports(input, {altScreen: true, mouse: false}, '')
    expect(result.filtered).toBe(input)
  })
})

describe('stripMouseReports', () => {
  it('drops full mouse reports when mouse disabled', () => {
    const input = '\x1b[<64;10;20M'
    const result = stripMouseReports(input, {altScreen: false, mouse: false}, '')
    expect(result.filtered).toBe('')
    expect(result.tail).toBe('')
  })

  it('buffers split mouse report sequences', () => {
    const first = stripMouseReports('\x1b[<64;10', {altScreen: false, mouse: false}, '')
    expect(first.filtered).toBe('')
    expect(first.tail).toBe('\x1b[<64;10')
    const second = stripMouseReports(';20M', {altScreen: false, mouse: false}, first.tail)
    expect(second.filtered).toBe('')
    expect(second.tail).toBe('')
  })

  it('does not filter when mouse is enabled', () => {
    const input = '\x1b[<64;10;20M'
    const result = stripMouseReports(input, {altScreen: false, mouse: true}, '')
    expect(result.filtered).toBe(input)
    expect(result.tail).toBe('')
  })
})
