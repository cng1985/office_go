const MAX_ROW = 120
const MAX_COL = 30

export function columnLabel(col) {
  let n = col
  let label = ''
  while (n > 0) {
    const rem = (n - 1) % 26
    label = String.fromCharCode(65 + rem) + label
    n = Math.floor((n - 1) / 26)
  }
  return label
}

export function columnIndex(label) {
  let col = 0
  for (const ch of String(label || '').toUpperCase()) {
    if (ch < 'A' || ch > 'Z') return 0
    col = col * 26 + (ch.charCodeAt(0) - 64)
  }
  return col
}

export function parseRef(ref) {
  const matched = /^(\$?)([A-Za-z]{1,3})(\$?)([1-9][0-9]*)$/.exec(String(ref || '').trim())
  if (!matched) return null
  const col = columnIndex(matched[2])
  const row = Number(matched[4])
  if (col < 1 || col > MAX_COL || row < 1 || row > MAX_ROW) return null
  return { col, row, absCol: matched[1] === '$', absRow: matched[3] === '$' }
}

export function formatRef(ref) {
  return `${ref.absCol ? '$' : ''}${columnLabel(ref.col)}${ref.absRow ? '$' : ''}${ref.row}`
}

export function mapFormulaRefs(input, mapper) {
  const source = String(input || '')
  if (!source.startsWith('=')) return source
  let out = '='
  let i = 1
  while (i < source.length) {
    if (source[i] === '"') {
      const start = i
      i += 1
      while (i < source.length) {
        if (source[i] === '"') {
          i += 1
          if (source[i] === '"') {
            i += 1
            continue
          }
          break
        }
        i += 1
      }
      out += source.slice(start, i)
      continue
    }
    const prev = source[i - 1] || ''
    const matched = /^(\$?)([A-Za-z]{1,3})(\$?)([1-9][0-9]*)/.exec(source.slice(i))
    const ref = matched && !/[A-Za-z0-9_]/.test(prev) ? parseRef(matched[0]) : null
    if (ref) {
      const next = mapper(ref, { sheet: sheetPrefix(source.slice(0, i)) })
      out += next ? formatRef(next) : '#REF!'
      i += matched[0].length
      continue
    }
    out += source[i]
    i += 1
  }
  return out
}

export function displayCell(model, row, col, workbook) {
  const result = evaluateCell(model, row, col, new Map(), new Set(), workbook)
  return formatResult(result, cellRecord(model, row, col))
}

export function sheetDisplay(sheet, workbook) {
  const cache = new Map()
  const shown = {}
  const rows = sheet?.row_count || 0
  const cols = sheet?.col_count || 0
  for (let row = 1; row <= rows; row += 1) {
    for (let col = 1; col <= cols; col += 1) {
      const result = evaluateCell(sheet, row, col, cache, new Set(), workbook)
      shown[`${row},${col}`] = formatResult(result, cellRecord(sheet, row, col))
    }
  }
  return shown
}

export function evaluateCell(model, row, col, cache = new Map(), stack = new Set(), workbook = null) {
  const sheetName = model?.name || ''
  const key = `${sheetName}!${row},${col}`
  if (cache.has(key)) return cache.get(key)
  if (stack.has(key)) return { error: '#CYCLE!' }
  const record = cellRecord(model, row, col)
  const input = record ? String(record.input || '') : ''
  stack.add(key)
  let result
  if (!input) result = { blank: true }
  else if (input.startsWith('=')) {
    try {
      result = parseFormula(input.slice(1), (sheet, nextRow, nextCol) => {
        const target = sheet ? findSheet(workbook, model, sheet) : model
        if (!target) return { error: '#REF!' }
        return evaluateCell(target, nextRow, nextCol, cache, stack, workbook)
      }, { row, col })
    } catch (error) {
      result = { error: error.message || '#VALUE!' }
    }
  } else if (record?.format === 'text') result = { text: input, textOnly: true }
  else if (isNumber(input)) result = { value: Number(input) }
  else result = { text: input, textOnly: true }
  stack.delete(key)
  cache.set(key, result)
  return result
}

export function formatNumber(value) {
  if (!Number.isFinite(value)) return '#VALUE!'
  const rounded = Math.round(value * 1e10) / 1e10
  return String(rounded)
}

function sheetPrefix(prefix) {
  if (!prefix.endsWith('!')) return ''
  const named = /(?:'((?:[^']|'')*)'|([A-Za-z][A-Za-z0-9_.]*))!$/.exec(prefix)
  if (!named) return '*'
  return (named[1] || named[2] || '').replace(/''/g, "'")
}

function cellRecord(model, row, col) {
  return (model?.cells || []).find((item) => item.row === row && item.col === col) || null
}

function findSheet(workbook, current, name) {
  const sheets = workbook?.sheets || [current]
  return sheets.find((item) => String(item.name || '').toLowerCase() === String(name).toLowerCase()) || null
}

function formatResult(result, record) {
  if (!result || result.error) return result?.error || '#VALUE!'
  if (result.bool) return result.value ? 'TRUE' : 'FALSE'
  const format = record?.format || ''
  if (result.blank) return ''
  if (format === 'text') return result.textOnly ? result.text : formatNumber(result.value)
  if (result.textOnly) return result.text || ''
  if (typeof result.value !== 'number' || !Number.isFinite(result.value)) return result.text || ''
  if (format === 'percent') return `${formatNumber(result.value * 100)}%`
  if (format === 'number') return result.value.toFixed(2)
  if (format === 'date') return formatDate(result.value)
  return formatNumber(result.value)
}

function formatDate(serial) {
  const ms = Date.UTC(1899, 11, 30) + Math.round(serial) * 86400000
  const date = new Date(ms)
  if (Number.isNaN(date.getTime())) return '#VALUE!'
  const pad = (value) => String(value).padStart(2, '0')
  return `${date.getUTCFullYear()}-${pad(date.getUTCMonth() + 1)}-${pad(date.getUTCDate())}`
}

function isNumber(value) {
  return /^-?(?:\d+(?:\.\d+)?|\.\d+)(?:e[+-]?\d+)?$/i.test(String(value).trim())
}

function parseFormula(source, resolve, here) {
  const parser = new Parser(source, resolve, here)
  const value = parser.parseComparison()
  parser.skip()
  if (!parser.done()) throw new Error('#VALUE!')
  return value
}

class Parser {
  constructor(source, resolve, here) {
    this.source = source
    this.resolve = resolve
    this.here = here || { row: 1, col: 1 }
    this.index = 0
  }

  done() {
    return this.index >= this.source.length
  }

  peek() {
    return this.source[this.index] || ''
  }

  skip() {
    while (this.index < this.source.length && /[ \t]/.test(this.source[this.index])) this.index += 1
  }

  parseComparison() {
    const left = this.parseConcat()
    this.skip()
    const op = this.peekCompare()
    if (!op) return left
    this.index += op.length
    const right = this.parseConcat()
    const ok = compareValues(left, right, op)
    return { bool: true, value: ok ? 1 : 0 }
  }

  peekCompare() {
    const two = this.source.slice(this.index, this.index + 2)
    if (two === '<=' || two === '>=' || two === '<>') return two
    const one = this.peek()
    if (one === '<' || one === '>' || one === '=') return one
    return ''
  }

  parseConcat() {
    let value = this.parseAdd()
    for (;;) {
      this.skip()
      if (this.peek() !== '&') break
      this.index += 1
      const right = this.parseAdd()
      value = { text: asText(value) + asText(right), textOnly: true }
    }
    return value
  }

  parseAdd() {
    let value = this.parseMul()
    for (;;) {
      this.skip()
      const op = this.peek()
      if (op !== '+' && op !== '-') break
      this.index += 1
      const right = this.asNumber(this.parseMul())
      const left = this.asNumber(value)
      value = { value: op === '+' ? left + right : left - right }
    }
    return value
  }

  parseMul() {
    let value = this.parsePower()
    for (;;) {
      this.skip()
      const op = this.peek()
      if (op !== '*' && op !== '/') break
      this.index += 1
      const right = this.asNumber(this.parsePower())
      if (op === '/' && right === 0) throw new Error('#DIV/0!')
      const left = this.asNumber(value)
      value = { value: op === '*' ? left * right : left / right }
    }
    return value
  }

  parsePower() {
    const left = this.parseUnary()
    this.skip()
    if (this.peek() !== '^') return left
    this.index += 1
    const right = this.asNumber(this.parsePower())
    return { value: this.asNumber(left) ** right }
  }

  parseUnary() {
    this.skip()
    if (this.peek() === '+') {
      this.index += 1
      return this.parseUnary()
    }
    if (this.peek() === '-') {
      this.index += 1
      return { value: -this.asNumber(this.parseUnary()) }
    }
    return this.parsePercent()
  }

  parsePercent() {
    const value = this.parsePrimary()
    this.skip()
    if (this.peek() !== '%') return value
    let number = this.asNumber(value)
    while (this.peek() === '%') {
      this.index += 1
      number /= 100
    }
    return { value: number }
  }

  parsePrimary() {
    this.skip()
    if (this.peek() === '(') {
      this.index += 1
      const value = this.parseComparison()
      this.skip()
      if (this.peek() !== ')') throw new Error('#VALUE!')
      this.index += 1
      return value
    }
    if (this.peek() === '"') return { text: this.parseString(), textOnly: true }
    const sheet = this.peekSheet()
    if (sheet) {
      this.index += sheet.length
      if (this.peek() !== '!') throw new Error('#NAME?')
      this.index += 1
      const ref = this.readRef()
      if (!ref) throw new Error('#REF!')
      return this.mustCell(sheet.name, ref)
    }
    const letters = this.peekLetters()
    if (letters) {
      const upper = letters.toUpperCase()
      const after = this.source[this.index + letters.length] || ''
      if (after === '(') {
        this.index += letters.length
        return this.parseFunction(upper)
      }
      if (upper === 'TRUE' && !/[A-Za-z0-9_]/.test(after)) {
        this.index += letters.length
        return { bool: true, value: 1 }
      }
      if (upper === 'FALSE' && !/[A-Za-z0-9_]/.test(after)) {
        this.index += letters.length
        return { bool: true, value: 0 }
      }
    }
    const ref = this.readRef()
    if (ref) return this.mustCell('', ref)
    return { value: this.parseNumber() }
  }

  parseFunction(name) {
    this.skip()
    if (this.peek() !== '(') throw new Error('#NAME?')
    this.index += 1
    const args = []
    this.skip()
    if (this.peek() !== ')') {
      args.push(this.captureArgument())
      for (;;) {
        this.skip()
        if (this.peek() !== ',') break
        this.index += 1
        args.push(this.captureArgument())
      }
    }
    this.skip()
    if (this.peek() !== ')') throw new Error('#VALUE!')
    this.index += 1
    return callFunction(name, args, this.here)
  }

  captureArgument() {
    this.skip()
    const start = this.index
    let depth = 0
    let quote = false
    while (this.index < this.source.length) {
      const ch = this.peek()
      if (quote) {
        if (ch === '"') {
          if (this.source[this.index + 1] === '"') {
            this.index += 2
            continue
          }
          quote = false
        }
        this.index += 1
        continue
      }
      if (ch === '"') {
        quote = true
        this.index += 1
        continue
      }
      if (ch === '(') {
        depth += 1
        this.index += 1
        continue
      }
      if (ch === ')') {
        if (depth === 0) break
        depth -= 1
        this.index += 1
        continue
      }
      if (ch === ',' && depth === 0) break
      this.index += 1
    }
    return { raw: this.source.slice(start, this.index).trim(), host: this }
  }

  readPointed() {
    this.skip()
    const sheet = this.peekSheet()
    const before = this.index
    if (sheet) {
      this.index += sheet.length
      if (this.peek() !== '!') {
        this.index = before
        return null
      }
      this.index += 1
      const ref = this.readRef()
      if (!ref) throw new Error('#REF!')
      return { sheet: sheet.name, ref }
    }
    const ref = this.readRef()
    if (!ref) return null
    return { sheet: '', ref }
  }

  peekSheet() {
    if (this.peek() === "'") {
      let i = this.index + 1
      let name = ''
      while (i < this.source.length) {
        if (this.source[i] === "'") {
          if (this.source[i + 1] === "'") {
            name += "'"
            i += 2
            continue
          }
          i += 1
          break
        }
        name += this.source[i]
        i += 1
      }
      return name ? { name, length: i - this.index } : null
    }
    const matched = /^[A-Za-z][A-Za-z0-9_.]*/.exec(this.source.slice(this.index))
    if (!matched || this.source[this.index + matched[0].length] !== '!') return null
    return { name: matched[0], length: matched[0].length }
  }

  peekLetters() {
    const matched = /^[A-Za-z]+/.exec(this.source.slice(this.index))
    return matched ? matched[0] : ''
  }

  readRef() {
    const matched = /^(\$?)([A-Za-z]{1,3})(\$?)([1-9][0-9]*)/.exec(this.source.slice(this.index))
    if (!matched) return null
    const ref = parseRef(matched[0])
    if (!ref) return null
    this.index += matched[0].length
    return ref
  }

  parseString() {
    this.index += 1
    let text = ''
    while (this.index < this.source.length) {
      if (this.peek() === '"') {
        this.index += 1
        if (this.peek() === '"') {
          text += '"'
          this.index += 1
          continue
        }
        return text
      }
      text += this.peek()
      this.index += 1
    }
    throw new Error('#VALUE!')
  }

  parseNumber() {
    const matched = /^(?:\d+(?:\.\d+)?|\.\d+)(?:e[+-]?\d+)?/i.exec(this.source.slice(this.index))
    if (!matched) throw new Error('#VALUE!')
    this.index += matched[0].length
    return Number(matched[0])
  }

  mustCell(sheet, ref) {
    const result = this.resolve(sheet, ref.row, ref.col)
    if (result?.error) throw new Error(result.error)
    return result || { blank: true }
  }

  asNumber(result) {
    if (!result || result.error) throw new Error(result?.error || '#VALUE!')
    if (result.blank) return 0
    if (result.bool) return result.value ? 1 : 0
    if (result.textOnly) {
      if (isNumber(result.text)) return Number(result.text)
      throw new Error('#VALUE!')
    }
    if (typeof result.value !== 'number' || !Number.isFinite(result.value)) throw new Error('#VALUE!')
    return result.value
  }
}

function cellsInRange(start, end) {
  const row1 = Math.min(start.row, end.row)
  const row2 = Math.max(start.row, end.row)
  const col1 = Math.min(start.col, end.col)
  const col2 = Math.max(start.col, end.col)
  if (row2 - row1 > MAX_ROW || col2 - col1 > MAX_COL) throw new Error('#REF!')
  const cells = []
  for (let row = row1; row <= row2; row += 1) {
    for (let col = col1; col <= col2; col += 1) cells.push({ row, col })
  }
  return cells
}

function callFunction(name, args, here, lookup) {
  const fn = FUNCTIONS[name]
  if (!fn) throw new Error('#NAME?')
  return fn(args, here, lookup)
}

function realize(arg) {
  if (!arg) throw new Error('#VALUE!')
  if (arg.kind) return arg
  const parser = new Parser(arg.raw, arg.host.resolve, arg.host.here)
  const pointed = parser.readPointed()
  parser.skip()
  if (pointed && parser.peek() === ':') {
    parser.index += 1
    const end = parser.readPointed()
    if (!end) throw new Error('#REF!')
    parser.skip()
    if (!parser.done()) throw new Error('#VALUE!')
    arg.kind = 'range'
    arg.sheet = pointed.sheet || end.sheet || ''
    arg.refs = cellsInRange(pointed.ref, end.ref)
    arg.parser = arg.host
    return arg
  }
  if (pointed && parser.done()) {
    arg.kind = 'scalar'
    arg.ref = pointed.ref
    arg.sheet = pointed.sheet || ''
    arg.result = arg.host.mustCell(arg.sheet, pointed.ref)
    return arg
  }
  const full = new Parser(arg.raw, arg.host.resolve, arg.host.here)
  const result = full.parseComparison()
  full.skip()
  if (!full.done()) throw new Error('#VALUE!')
  arg.kind = 'scalar'
  arg.result = result
  return arg
}

function scalars(args) {
  return args.flatMap((arg) => {
    const ready = realize(arg)
    return ready.kind === 'range' ? ready.refs.map((ref, index) => materialize(ready, index)) : [ready.result]
  })
}

function materialize(arg, index) {
  if (!arg.results) arg.results = []
  if (arg.results[index]) return arg.results[index]
  let result
  try {
    result = arg.parser.mustCell(arg.sheet, arg.refs[index])
  } catch (error) {
    result = { error: error.message || '#VALUE!' }
  }
  arg.results[index] = result
  return result
}

function rangeResults(arg) {
  const ready = realize(arg)
  if (ready.kind !== 'range') throw new Error('#VALUE!')
  return ready.refs.map((ref, index) => ({ ref, result: materialize(ready, index) }))
}

function numbersOf(args, { ignoreText = true, ignoreErrors = false } = {}) {
  const values = []
  for (const result of scalars(args)) {
    if (!result || result.blank) continue
    if (result.error) {
      if (ignoreErrors) continue
      throw new Error(result.error)
    }
    if (result.bool) {
      values.push(result.value ? 1 : 0)
      continue
    }
    if (result.textOnly) {
      if (!ignoreText) throw new Error('#VALUE!')
      if (isNumber(result.text)) values.push(Number(result.text))
      continue
    }
    if (typeof result.value === 'number') values.push(result.value)
  }
  return values
}

function asText(result) {
  if (!result || result.blank) return ''
  if (result.error) throw new Error(result.error)
  if (result.bool) return result.value ? 'TRUE' : 'FALSE'
  if (result.textOnly) return result.text || ''
  return formatNumber(result.value)
}

function compareValues(left, right, op) {
  const lv = compareToken(left)
  const rv = compareToken(right)
  if (typeof lv === 'number' && typeof rv === 'number') return compareOp(lv, rv, op)
  return compareOp(String(lv), String(rv), op)
}

function compareToken(result) {
  if (!result || result.blank) return 0
  if (result.error) throw new Error(result.error)
  if (result.bool) return result.value ? 1 : 0
  if (result.textOnly) return isNumber(result.text) ? Number(result.text) : result.text || ''
  return result.value
}

function compareOp(left, right, op) {
  if (op === '=') return left === right
  if (op === '<>') return left !== right
  if (op === '<') return left < right
  if (op === '>') return left > right
  if (op === '<=') return left <= right
  return left >= right
}

function criteriaMatches(result, criteria) {
  if (criteria?.textOnly) {
    const spec = String(criteria.text || '')
    const op = /^(<=|>=|<>|=|<|>)(.*)$/.exec(spec)
    if (op) {
      const raw = op[2]
      const target = isNumber(raw) ? Number(raw) : raw
      const left = result?.textOnly && !isNumber(result.text) ? result.text : compareToken(result)
      return compareOp(left, target, op[1])
    }
    return wildcard(asText(result), spec)
  }
  return compareValues(result, criteria, '=')
}

function wildcard(text, pattern) {
  let source = '^'
  for (let i = 0; i < pattern.length; i += 1) {
    const ch = pattern[i]
    if (ch === '~' && (pattern[i + 1] === '*' || pattern[i + 1] === '?' || pattern[i + 1] === '~')) {
      source += pattern[i + 1].replace(/[.*+?^${}()|[\]\\]/g, '\\$&')
      i += 1
    } else if (ch === '*') source += '.*'
    else if (ch === '?') source += '.'
    else source += ch.replace(/[.*+?^${}()|[\]\\]/g, '\\$&')
  }
  return new RegExp(`${source}$`, 'i').test(text)
}

function numericResult(value) {
  if (!Number.isFinite(value)) throw new Error('#NUM!')
  return { value }
}

const FUNCTIONS = {
  SUM(args) {
    return { value: numbersOf(args).reduce((sum, item) => sum + item, 0) }
  },
  PRODUCT(args) {
    const numbers = numbersOf(args)
    return { value: numbers.reduce((product, item) => product * item, numbers.length ? 1 : 0) }
  },
  AVERAGE(args) {
    const numbers = numbersOf(args)
    if (!numbers.length) throw new Error('#DIV/0!')
    return { value: numbers.reduce((sum, item) => sum + item, 0) / numbers.length }
  },
  MIN(args) {
    const numbers = numbersOf(args)
    return { value: numbers.length ? Math.min(...numbers) : 0 }
  },
  MAX(args) {
    const numbers = numbersOf(args)
    return { value: numbers.length ? Math.max(...numbers) : 0 }
  },
  COUNT(args) {
    return { value: numbersOf(args, { ignoreErrors: true }).length }
  },
  COUNTA(args) {
    return { value: scalars(args).filter((item) => item && !item.blank && !item.error).length }
  },
  COUNTBLANK(args) {
    return { value: scalars(args).filter((item) => !item || item.blank).length }
  },
  ABS(args) {
    return numericResult(Math.abs(oneNumber(args)))
  },
  INT(args) {
    return numericResult(Math.floor(oneNumber(args)))
  },
  SQRT(args) {
    const value = oneNumber(args)
    if (value < 0) throw new Error('#NUM!')
    return numericResult(Math.sqrt(value))
  },
  SIGN(args) {
    const value = oneNumber(args)
    return { value: value > 0 ? 1 : value < 0 ? -1 : 0 }
  },
  POWER(args) {
    return numericResult(oneNumber(args, 0) ** oneNumber(args, 1))
  },
  MOD(args) {
    const divisor = oneNumber(args, 1)
    if (divisor === 0) throw new Error('#DIV/0!')
    const value = oneNumber(args, 0)
    return numericResult(value - divisor * Math.floor(value / divisor))
  },
  ROUND(args) {
    const digits = args[1] ? oneNumber(args, 1) : 0
    const factor = 10 ** digits
    return numericResult(Math.round(oneNumber(args, 0) * factor) / factor)
  },
  ROUNDUP(args) {
    return directionalRound(args, 1)
  },
  ROUNDDOWN(args) {
    return directionalRound(args, 0)
  },
  IF(args) {
    const chosen = args[truthNumber(args[0]) ? 1 : 2]
    if (!chosen) return { bool: true, value: 0 }
    const ready = realize(chosen)
    return ready.kind === 'range' ? materialize(ready, 0) : ready.result
  },
  AND(args) {
    for (const arg of args) {
      if (!allTrue(arg)) return { bool: true, value: 0 }
    }
    return { bool: true, value: 1 }
  },
  OR(args) {
    for (const arg of args) {
      if (anyTrue(arg)) return { bool: true, value: 1 }
    }
    return { bool: true, value: 0 }
  },
  NOT(args) {
    return { bool: true, value: allTrue(args[0]) ? 0 : 1 }
  },
  IFERROR(args) {
    try {
      const ready = realize(args[0])
      const value = ready.kind === 'range' ? materialize(ready, 0) : ready.result
      if (value?.error) throw new Error(value.error)
      return value
    } catch (error) {
      if (!args[1]) return { text: '', textOnly: true }
      const fallback = realize(args[1])
      return fallback.kind === 'range' ? materialize(fallback, 0) : fallback.result
    }
  },
  LEN(args) {
    return { value: [...asText(scalarArg(args, 0))].length }
  },
  LEFT(args) {
    const count = args[1] ? oneNumber(args, 1) : 1
    return { text: [...asText(scalarArg(args, 0))].slice(0, count).join(''), textOnly: true }
  },
  RIGHT(args) {
    const count = args[1] ? oneNumber(args, 1) : 1
    const chars = [...asText(scalarArg(args, 0))]
    return { text: chars.slice(Math.max(0, chars.length - count)).join(''), textOnly: true }
  },
  MID(args) {
    const start = Math.max(1, oneNumber(args, 1))
    const count = oneNumber(args, 2)
    return { text: [...asText(scalarArg(args, 0))].slice(start - 1, start - 1 + count).join(''), textOnly: true }
  },
  TRIM(args) {
    return { text: asText(scalarArg(args, 0)).trim().replace(/[ \t]+/g, ' '), textOnly: true }
  },
  UPPER(args) {
    return { text: asText(scalarArg(args, 0)).toUpperCase(), textOnly: true }
  },
  LOWER(args) {
    return { text: asText(scalarArg(args, 0)).toLowerCase(), textOnly: true }
  },
  CONCAT(args) {
    return { text: scalars(args).map((item) => asText(item)).join(''), textOnly: true }
  },
  CONCATENATE(args) {
    return FUNCTIONS.CONCAT(args)
  },
  VALUE(args) {
    const text = asText(scalarArg(args, 0)).trim().replace(/%$/, '')
    const percent = String(scalarArg(args, 0).text || '').trim().endsWith('%')
    if (!isNumber(text)) throw new Error('#VALUE!')
    return { value: Number(text) / (percent ? 100 : 1) }
  },
  TEXT(args) {
    const value = scalarArg(args, 0)
    const pattern = asText(scalarArg(args, 1))
    if (/y|m|d/i.test(pattern) && typeof value.value === 'number') return { text: formatDate(value.value), textOnly: true }
    if (pattern.includes('%') && typeof value.value === 'number') return { text: `${formatNumber(value.value * 100)}%`, textOnly: true }
    if (pattern.includes('0') && typeof value.value === 'number') {
      const decimals = (pattern.split('.')[1] || '').replace(/[^0]/g, '').length
      return { text: value.value.toFixed(decimals), textOnly: true }
    }
    return { text: asText(value), textOnly: true }
  },
  ISBLANK(args) {
    const value = scalarArg(args, 0)
    return { bool: true, value: !value || value.blank ? 1 : 0 }
  },
  ISNUMBER(args) {
    const value = scalarArg(args, 0)
    return { bool: true, value: value && typeof value.value === 'number' && !value.textOnly && !value.bool ? 1 : 0 }
  },
  ISTEXT(args) {
    const value = scalarArg(args, 0)
    return { bool: true, value: value?.textOnly ? 1 : 0 }
  },
  ROW(args, here) {
    if (!args.length) return { value: here.row }
    const ref = firstRef(args[0])
    return { value: ref.row }
  },
  COLUMN(args, here) {
    if (!args.length) return { value: here.col }
    const ref = firstRef(args[0])
    return { value: ref.col }
  },
  TODAY() {
    const now = new Date()
    const utc = Date.UTC(now.getFullYear(), now.getMonth(), now.getDate())
    return { value: Math.round((utc - Date.UTC(1899, 11, 30)) / 86400000) }
  },
  NOW() {
    const now = new Date()
    const utc = Date.UTC(now.getFullYear(), now.getMonth(), now.getDate(), now.getHours(), now.getMinutes(), now.getSeconds())
    return { value: (utc - Date.UTC(1899, 11, 30)) / 86400000 }
  },
  SUMIF(args) {
    return conditional(args, 'sum')
  },
  COUNTIF(args) {
    return conditional(args, 'count')
  },
  AVERAGEIF(args) {
    return conditional(args, 'average')
  },
  VLOOKUP(args) {
    const lookup = scalarArg(args, 0)
    const rows = rowsOf(args[1])
    const col = oneNumber(args, 2)
    if (col < 1 || !rows.length || col > rows[0].length) throw new Error('#REF!')
    const exact = args[3] ? oneNumber(args, 3) === 0 : false
    let found = null
    if (exact) {
      found = rows.find((row) => criteriaMatches(row[0].result, lookup))
    } else {
      for (const row of rows) {
        if (compareValues(row[0].result, lookup, '<=')) found = row
        else break
      }
    }
    if (!found) throw new Error('#N/A')
    return found[col - 1].result
  },
  INDEX(args) {
    const rows = rowsOf(args[0])
    const row = oneNumber(args, 1)
    const col = args[2] ? oneNumber(args, 2) : 1
    if (!rows[row - 1] || !rows[row - 1][col - 1]) throw new Error('#REF!')
    return rows[row - 1][col - 1].result
  },
  MATCH(args) {
    const lookup = scalarArg(args, 0)
    const items = rangeResults(args[1])
    const type = args[2] ? oneNumber(args, 2) : 1
    if (type === 0) {
      const index = items.findIndex((item) => criteriaMatches(item.result, lookup))
      if (index < 0) throw new Error('#N/A')
      return { value: index + 1 }
    }
    let found = -1
    items.forEach((item, index) => {
      const lessOrEqual = type === 1 ? compareValues(item.result, lookup, '<=') : compareValues(item.result, lookup, '>=')
      if (lessOrEqual) found = index
    })
    if (found < 0) throw new Error('#N/A')
    return { value: found + 1 }
  },
}

function directionalRound(args, away) {
  const digits = args[1] ? oneNumber(args, 1) : 0
  const factor = 10 ** digits
  const value = oneNumber(args, 0) * factor
  const rounded = away ? (value < 0 ? Math.floor(value) : Math.ceil(value)) : (value < 0 ? Math.ceil(value) : Math.floor(value))
  return numericResult(rounded / factor)
}

function scalarArg(args, index) {
  const ready = realize(args[index])
  const value = ready.kind === 'range' ? materialize(ready, 0) : ready.result
  if (value?.error) throw new Error(value.error)
  return value
}

function oneNumber(args, index = 0) {
  const ready = realize(args[index])
  const parser = new Parser('', () => ({ blank: true }), { row: 1, col: 1 })
  if (ready.kind === 'range') return parser.asNumber(materialize(ready, 0))
  return parser.asNumber(ready.result)
}

function firstRef(arg) {
  const ready = realize(arg)
  if (ready.ref) return ready.ref
  if (ready.kind === 'range') return ready.refs[0]
  throw new Error('#VALUE!')
}

function allTrue(arg) {
  return truthValues(arg).every((item) => item !== 0)
}

function anyTrue(arg) {
  return truthValues(arg).some((item) => item !== 0)
}

function truthNumber(arg) {
  return allTrue(arg) ? 1 : 0
}

function truthValues(arg) {
  const ready = realize(arg)
  const values = ready.kind === 'range' ? ready.refs.map((ref, index) => materialize(ready, index)) : [ready.result]
  const parser = new Parser('', () => ({ blank: true }), { row: 1, col: 1 })
  return values.map((item) => parser.asNumber(item))
}

function rowsOf(arg) {
  const items = rangeResults(arg)
  const map = new Map()
  for (const item of items) {
    if (!map.has(item.ref.row)) map.set(item.ref.row, [])
    map.get(item.ref.row).push(item)
  }
  return [...map.keys()].sort((a, b) => a - b).map((row) => map.get(row).sort((a, b) => a.ref.col - b.ref.col))
}

function conditional(args, mode) {
  const source = rangeResults(args[0])
  const criteria = scalarArg(args, 1)
  const target = args[2] ? rangeResults(args[2]) : source
  let total = 0
  let count = 0
  source.forEach((item, index) => {
    if (!criteriaMatches(item.result, criteria)) return
    const value = target[index]?.result
    const numeric = value && typeof value.value === 'number' && !value.textOnly && !value.bool
    if (mode === 'average') {
      if (!numeric) return
      count += 1
      total += value.value
      return
    }
    count += 1
    if (numeric) total += value.value
  })
  if (mode === 'count') return { value: count }
  if (mode === 'average') {
    if (!count) throw new Error('#DIV/0!')
    return { value: total / count }
  }
  return { value: total }
}
