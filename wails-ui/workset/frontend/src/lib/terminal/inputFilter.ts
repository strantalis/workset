export type TerminalModeState = {
  altScreen: boolean
  mouse: boolean
}

export type StripResult = {
  filtered: string
  tail: string
}

const mouseReportPattern = /\x1b\[<\d+;\d+;\d+[mM]/g
const mouseReportBarePattern = /^(\d+;\d+;\d+[mM])+$/
const mouseReportPrefix = '\x1b[<'

const extractMouseTail = (value: string): {cleaned: string; tail: string} => {
  const idx = value.lastIndexOf(mouseReportPrefix)
  if (idx < 0) {
    return {cleaned: value, tail: ''}
  }
  const tail = value.slice(idx)
  if (/^\x1b\[<[\d;]*$/.test(tail)) {
    return {cleaned: value.slice(0, idx), tail}
  }
  return {cleaned: value, tail: ''}
}

export const isReportCSI = (seq: string): boolean => {
  if (!seq.startsWith('\x1b[')) {
    return false
  }
  const final = seq[seq.length - 1]
  if (final === 'R' || final === 'c') {
    return true
  }
  if (final === 'y' && seq.includes('$y')) {
    return true
  }
  return false
}

export const stripTerminalReports = (
  data: string,
  modes: TerminalModeState,
  tail: string
): StripResult => {
  if (modes.mouse || modes.altScreen) {
    return {filtered: data, tail: ''}
  }
  let combined = tail + data
  let nextTail = ''
  let out = ''
  for (let i = 0; i < combined.length; i++) {
    const ch = combined[i]
    if (ch === '\x1b') {
      const next = combined[i + 1]
      if (next === ']') {
        const belIndex = combined.indexOf('\x07', i + 2)
        const stIndex = combined.indexOf('\x1b\\', i + 2)
        let end = -1
        if (belIndex !== -1 && stIndex !== -1) {
          end = Math.min(belIndex + 1, stIndex + 2)
        } else if (belIndex !== -1) {
          end = belIndex + 1
        } else if (stIndex !== -1) {
          end = stIndex + 2
        }
        if (end === -1) {
          nextTail = combined.slice(i)
          break
        }
        i = end - 1
        continue
      }
      if (next === '[') {
        let end = -1
        for (let j = i + 2; j < combined.length; j++) {
          const code = combined.charCodeAt(j)
          if (code >= 0x40 && code <= 0x7e) {
            end = j + 1
            break
          }
        }
        if (end === -1) {
          nextTail = combined.slice(i)
          break
        }
        const seq = combined.slice(i, end)
        if (!isReportCSI(seq)) {
          out += seq
        }
        i = end - 1
        continue
      }
    }
    out += ch
  }
  return {filtered: out, tail: nextTail}
}

export const stripMouseReports = (
  data: string,
  modes: TerminalModeState,
  tail: string
): StripResult => {
  if (modes.mouse) {
    return {filtered: data, tail: ''}
  }
  const combined = tail + data
  const withoutReports = combined.replace(mouseReportPattern, '')
  if (mouseReportBarePattern.test(withoutReports)) {
    return {filtered: '', tail: ''}
  }
  const {cleaned, tail: nextTail} = extractMouseTail(withoutReports)
  return {filtered: cleaned, tail: nextTail}
}
