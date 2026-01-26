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

export const stripMouseReports = (
  data: string,
  modes: {mouse: boolean},
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
