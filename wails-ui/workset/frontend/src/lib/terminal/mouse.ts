export type MouseEncoding = 'sgr' | 'urxvt' | 'utf8' | 'x10'

type WheelParams = {
  button: number
  col: number
  row: number
  encoding: MouseEncoding
}

const clamp = (value: number, min: number, max: number): number => {
  if (value < min) return min
  if (value > max) return max
  return value
}

export const encodeWheel = ({button, col, row, encoding}: WheelParams): string => {
  if (encoding === 'sgr') {
    return `\x1b[<${button};${col};${row}M`
  }
  if (encoding === 'urxvt') {
    return `\x1b[${button};${col};${row}M`
  }
  if (encoding === 'utf8') {
    const cb = button + 32
    const cx = col + 32
    const cy = row + 32
    return `\x1b[M${String.fromCodePoint(cb, cx, cy)}`
  }
  const safeCol = clamp(col, 1, 223)
  const safeRow = clamp(row, 1, 223)
  const cb = button + 32
  const cx = safeCol + 32
  const cy = safeRow + 32
  return `\x1b[M${String.fromCharCode(cb, cx, cy)}`
}
