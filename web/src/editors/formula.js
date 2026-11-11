const FUNCTIONS = new Set(['SUM', 'AVERAGE', 'MIN', 'MAX', 'COUNT'])

export function displayCell(model, row, col) {
  const result = evaluateCell(model, row, col, new Map(), new Set())
  if (result.error) return result.error
  if (result.textOnly) return result.text
  if (result.blank) return ''
  if (typeof result.value === 'number') return formatNumber(result.value)
  return result.text || ''
}

export function evaluateCell(model, row, col, cache = new Map(), stack = new Set()) {
  const key = `${row},${col}`
  if (cache.has(key)) return cache.get(key)
  if (stack.has(key)) return { error: '#CYCLE!' }
  const input = inputAt(model, row, col)
  stack.add(key)
  let result
  if (!input) {
    result = { blank: true, value: 0, text: '' }
  } else if (input.startsWith('=')) {
    try {
      result = parseFormula(input.slice(1), (nextRow, nextCol) => evaluateCell(model, nextRow, nextCol, cache, stack))
    } catch (error) {
      result = { error: error.message || '#VALUE!' }
    }
  } else if (isNumber(input)) {
    result = { value: Number(input), text: input }
  } else {
    result = { text: input, value: 0, textOnly: true }
  }
  stack.delete(key)
  cache.set(key, result)
  return result
}

function inputAt(model, row, col) {
  const cell = (model.cells || []).find((item) => item.row === row && item.col === col)
  return cell ? String(cell.input || '') : ''
}

export function formatNumber(value) {
  if (!Number.isFinite(value)) return '#VALUE!'
  const rounded = Math.round(value * 1e6) / 1e6
  return String(rounded)
}

export function columnLabel(col) {
  return String.fromCharCode(64 + col)
}

export function parseRef(ref) {
  const matched = /^([A-Z])([1-9][0-9]*)$/.exec(ref)
  if (!matched) return null
  return { col: matched[1].charCodeAt(0) - 64, row: Number(matched[2]) }
}

function isNumber(value) {
  return /^-?\d+(\.\d+)?$/.test(value.trim())
}

function parseFormula(source, resolve) {
  const parser = new Parser(source, resolve)
  const value = parser.parseExpression()
  parser.skip()
  if (!parser.done()) throw new Error('#VALUE!')
  return { value }
}

class Parser {
  constructor(source, resolve) {
    this.source = source
    this.resolve = resolve
    this.index = 0
  }

  done() {
    return this.index >= this.source.length
  }

  skip() {
    while (this.index < this.source.length && /\s/.test(this.source[this.index])) this.index += 1
  }

  parseExpression() {
    let value = this.parseTerm()
    for (;;) {
      this.skip()
      const op = this.source[this.index]
      if (op !== '+' && op !== '-') break
      this.index += 1
      const right = this.parseTerm()
      value = op === '+' ? value + right : value - right
    }
    return value
  }

  parseTerm() {
    let value = this.parseFactor()
    for (;;) {
      this.skip()
      const op = this.source[this.index]
      if (op !== '*' && op !== '/') break
      this.index += 1
      const right = this.parseFactor()
      if (op === '/' && right === 0) throw new Error('#DIV/0!')
      value = op === '*' ? value * right : value / right
    }
    return value
  }

  parseFactor() {
    this.skip()
    if (this.source[this.index] === '+') {
      this.index += 1
      return this.parseFactor()
    }
    if (this.source[this.index] === '-') {
      this.index += 1
      return -this.parseFactor()
    }
    if (this.source[this.index] === '(') {
      this.index += 1
      const value = this.parseExpression()
      this.skip()
      if (this.source[this.index] !== ')') throw new Error('#VALUE!')
      this.index += 1
      return value
    }
    const ident = this.peekIdent()
    if (ident && FUNCTIONS.has(ident)) {
      this.index += ident.length
      return this.parseFunction(ident)
    }
    if (ident) {
      this.index += ident.length
      const ref = parseRef(ident)
      if (!ref) throw new Error('#REF!')
      return numericValue(this.resolve(ref.row, ref.col))
    }
    return this.parseNumber()
  }

  parseFunction(name) {
    this.skip()
    if (this.source[this.index] !== '(') throw new Error('#VALUE!')
    this.index += 1
    const values = this.parseArguments()
    this.skip()
    if (this.source[this.index] !== ')') throw new Error('#VALUE!')
    this.index += 1
    if (name === 'COUNT') return values.filter((item) => item.kind === 'number').length
    const numbers = values.filter((item) => item.kind === 'number').map((item) => item.value)
    if (name === 'SUM') return numbers.reduce((sum, item) => sum + item, 0)
    if (name === 'MIN') return numbers.length ? Math.min(...numbers) : 0
    if (name === 'MAX') return numbers.length ? Math.max(...numbers) : 0
    if (!numbers.length) throw new Error('#DIV/0!')
    return numbers.reduce((sum, item) => sum + item, 0) / numbers.length
  }

  parseArguments() {
    const values = []
    this.skip()
    if (this.source[this.index] === ')') return values
    values.push(...this.parseArgument())
    for (;;) {
      this.skip()
      if (this.source[this.index] !== ',') break
      this.index += 1
      values.push(...this.parseArgument())
    }
    return values
  }

  parseArgument() {
    this.skip()
    const start = this.index
    const ident = this.peekIdent()
    if (ident && parseRef(ident)) {
      this.index += ident.length
      this.skip()
      if (this.source[this.index] === ':') {
        this.index += 1
        this.skip()
        const endIdent = this.peekIdent()
        const end = endIdent ? parseRef(endIdent) : null
        if (!end) throw new Error('#REF!')
        this.index += endIdent.length
        return cellsInRange(parseRef(ident), end).map((ref) => tagged(this.resolve(ref.row, ref.col)))
      }
      return [tagged(this.resolve(parseRef(ident).row, parseRef(ident).col))]
    }
    this.index = start
    return [{ kind: 'number', value: this.parseExpression() }]
  }

  peekIdent() {
    const matched = /^[A-Z]+[0-9]*/.exec(this.source.slice(this.index))
    return matched ? matched[0] : ''
  }

  parseNumber() {
    const matched = /^-?\d+(\.\d+)?/.exec(this.source.slice(this.index))
    if (!matched) throw new Error('#VALUE!')
    this.index += matched[0].length
    return Number(matched[0])
  }
}

function cellsInRange(start, end) {
  const row1 = Math.min(start.row, end.row)
  const row2 = Math.max(start.row, end.row)
  const col1 = Math.min(start.col, end.col)
  const col2 = Math.max(start.col, end.col)
  if (row2 - row1 > 100 || col2 - col1 > 26) throw new Error('#REF!')
  const cells = []
  for (let row = row1; row <= row2; row += 1) {
    for (let col = col1; col <= col2; col += 1) cells.push({ row, col })
  }
  return cells
}

function numericValue(result) {
  if (!result || result.error) throw new Error(result?.error || '#VALUE!')
  if (result.textOnly) throw new Error('#VALUE!')
  return result.value || 0
}

function tagged(result) {
  if (!result || result.error) throw new Error(result?.error || '#VALUE!')
  if (result.textOnly || result.blank) return { kind: 'text' }
  return { kind: 'number', value: result.value }
}
