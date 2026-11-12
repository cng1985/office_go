import { MAX_COL as MAX_COLS, MAX_ROW as MAX_ROWS, columnLabel, displayCell, mapFormulaRefs, parseRef, replaceSheetNames } from './formula.js'

export { MAX_COLS, MAX_ROWS }
export const MAX_SHEETS = 20
export const MAX_INPUT = 2000

const ALIGNS = new Set(['left', 'center', 'right'])
const VALIGNS = new Set(['top', 'middle', 'bottom'])
const FORMATS = new Set(['number', 'percent', 'currency', 'date', 'text'])
const FONTS = new Set(['serif', 'sans', 'mono'])
const BORDERS = new Set(['bottom', 'all', 'outer'])
export const STYLE_KEYS = ['bold', 'italic', 'underline', 'strike', 'align', 'valign', 'format', 'color', 'fill', 'font', 'size', 'wrap', 'border', 'rotate', 'decimals']

export function normalizeWorkbook(content) {
  const source = content && typeof content === 'object' ? content : {}
  const rawSheets = Array.isArray(source.sheets) && source.sheets.length
    ? source.sheets
    : [{
      name: source.sheet_name || source.name || 'Sheet1',
      row_count: source.row_count,
      col_count: source.col_count,
      col_widths: source.col_widths,
      cells: source.cells,
    }]
  const sheets = []
  const used = new Set()
  for (const raw of rawSheets.slice(0, MAX_SHEETS)) {
    const sheet = normalizeSheet(raw, sheets.length)
    sheet.name = uniqueSheetName(sheet.name, used)
    used.add(sheet.name.toLowerCase())
    sheets.push(sheet)
  }
  if (!sheets.length) sheets.push(normalizeSheet({}, 0))
  let active = Number.isInteger(source.active) ? source.active : 0
  if (active < 0 || active >= sheets.length) active = 0
  const workbook = {}
  if (active > 0) workbook.active = active
  workbook.sheets = sheets
  return workbook
}

const cellIndexes = new WeakMap()

function dropCellIndex(sheet) {
  if (sheet?.cells) cellIndexes.delete(sheet.cells)
}

export function findCell(sheet, row, col) {
  const cells = sheet?.cells
  if (!cells) return null
  let map = cellIndexes.get(cells)
  if (!map) {
    map = new Map()
    for (const item of cells) map.set(`${item.row},${item.col}`, item)
    cellIndexes.set(cells, map)
  }
  return map.get(`${row},${col}`) || null
}

export function rowWindow({ rowCount, scrollTop, viewHeight, rowHeight = 21, buffer = 6, merges = [] }) {
  const total = Math.max(1, rowCount || 1)
  const height = rowHeight > 0 ? rowHeight : 21
  const span = Math.max(1, Math.ceil(Math.max(height, viewHeight) / height) + buffer * 2)
  const maxScroll = Math.max(0, total * height - Math.max(height, viewHeight))
  const scrolled = Math.min(Math.max(0, scrollTop), maxScroll)
  const first = Math.min(total, Math.max(1, Math.floor(scrolled / height) + 1 - buffer))
  let start = first
  let end = Math.min(total, first + span - 1)
  const anchors = []
  for (const cell of merges) {
    const rows = Number(cell?.merge_rows) || 1
    if (rows > 1) anchors.push([Number(cell.row) || 1, (Number(cell.row) || 1) + rows - 1])
  }
  let grew = true
  while (grew) {
    grew = false
    for (const [top, bottom] of anchors) {
      if (bottom < start || top > end) continue
      const nextStart = Math.max(1, top)
      const nextEnd = Math.min(total, bottom)
      if (nextStart < start) {
        start = nextStart
        grew = true
      }
      if (nextEnd > end) {
        end = nextEnd
        grew = true
      }
    }
  }
  return {
    start,
    end,
    padTop: (start - 1) * height,
    padBottom: Math.max(0, total - end) * height,
  }
}

export function putCell(sheet, cell) {
  const next = normalizeStoredCell(cell)
  sheet.cells = (sheet.cells || []).filter((item) => !(item.row === cell.row && item.col === cell.col))
  if (next) sheet.cells.push(next)
  sheet.cells.sort((a, b) => a.row - b.row || a.col - b.col)
}

export function removeCell(sheet, row, col) {
  sheet.cells = (sheet.cells || []).filter((item) => !(item.row === row && item.col === col))
}

export function writeInput(sheet, row, col, input) {
  const existing = findCell(sheet, row, col)
  if ((existing?.input || '') === input) return false
  putCell(sheet, { ...(existing || { row, col }), row, col, input })
  return true
}

export function clearInputs(sheet, range) {
  for (let row = range.r1; row <= range.r2; row += 1) {
    for (let col = range.c1; col <= range.c2; col += 1) {
      const existing = findCell(sheet, row, col)
      if (!existing) continue
      putCell(sheet, { ...existing, input: '' })
    }
  }
}

export function shiftFormula(input, deltaRow, deltaCol) {
  return mapFormulaRefs(input, (ref) => {
    const row = ref.absRow ? ref.row : ref.row + deltaRow
    const col = ref.absCol ? ref.col : ref.col + deltaCol
    if (row < 1 || col < 1 || row > MAX_ROWS || col > MAX_COLS) return null
    return { ...ref, row, col }
  })
}

export function insertRows(sheet, at, count = 1, workbook = null) {
  if (sheet.row_count + count > MAX_ROWS) return false
  releaseMerges(sheet, 'row', at, count, 'insert')
  sheet.row_count += count
  for (const cell of sheet.cells) {
    if (cell.row >= at) cell.row += count
    cell.input = shiftStructural(cell.input, 'row', at, count, sheet.name, false, 'insert')
  }
  syncQualifiedRefs(workbook, sheet, 'row', at, count, 'insert')
  dropCellIndex(sheet)
  return true
}

export function deleteRows(sheet, at, count = 1, workbook = null) {
  if (sheet.row_count - count < 1) return false
  releaseMerges(sheet, 'row', at, count, 'delete')
  sheet.cells = sheet.cells.filter((cell) => cell.row < at || cell.row >= at + count)
  for (const cell of sheet.cells) {
    if (cell.row >= at + count) cell.row -= count
    cell.input = shiftStructural(cell.input, 'row', at, count, sheet.name, false, 'delete')
  }
  sheet.row_count -= count
  syncQualifiedRefs(workbook, sheet, 'row', at, count, 'delete')
  dropCellIndex(sheet)
  return true
}

export function insertCols(sheet, at, count = 1, workbook = null) {
  if (sheet.col_count + count > MAX_COLS) return false
  releaseMerges(sheet, 'col', at, count, 'insert')
  if (Array.isArray(sheet.col_widths) && sheet.col_widths.length) {
    const widths = sheet.col_widths.slice()
    while (widths.length < sheet.col_count) widths.push(0)
    widths.splice(at - 1, 0, ...Array(count).fill(0))
    sheet.col_widths = widths.slice(0, sheet.col_count + count)
  }
  sheet.col_count += count
  for (const cell of sheet.cells) {
    if (cell.col >= at) cell.col += count
    cell.input = shiftStructural(cell.input, 'col', at, count, sheet.name, false, 'insert')
  }
  syncQualifiedRefs(workbook, sheet, 'col', at, count, 'insert')
  dropCellIndex(sheet)
  return true
}

export function deleteCols(sheet, at, count = 1, workbook = null) {
  if (sheet.col_count - count < 1) return false
  releaseMerges(sheet, 'col', at, count, 'delete')
  if (Array.isArray(sheet.col_widths) && sheet.col_widths.length) {
    const widths = sheet.col_widths.slice()
    while (widths.length < sheet.col_count) widths.push(0)
    widths.splice(at - 1, count)
    sheet.col_widths = trimWidths(widths)
  }
  sheet.cells = sheet.cells.filter((cell) => cell.col < at || cell.col >= at + count)
  for (const cell of sheet.cells) {
    if (cell.col >= at + count) cell.col -= count
    cell.input = shiftStructural(cell.input, 'col', at, count, sheet.name, false, 'delete')
  }
  sheet.col_count -= count
  syncQualifiedRefs(workbook, sheet, 'col', at, count, 'delete')
  dropCellIndex(sheet)
  return true
}

export function growSheet(sheet, rows, cols) {
  const nextRows = Math.min(MAX_ROWS, sheet.row_count + rows)
  const nextCols = Math.min(MAX_COLS, sheet.col_count + cols)
  extendWidths(sheet, nextCols)
  sheet.row_count = nextRows
  sheet.col_count = nextCols
}

export function setColumnWidth(sheet, col, width) {
  const widths = Array.isArray(sheet.col_widths) ? sheet.col_widths.slice() : []
  while (widths.length < sheet.col_count) widths.push(0)
  widths[col - 1] = width
  const trimmed = trimWidths(widths)
  if (trimmed) sheet.col_widths = trimmed
  else delete sheet.col_widths
}

export function fillRange(sheet, source, target) {
  const vertical = target.c1 === source.c1 && target.c2 === source.c2 && (target.r1 !== source.r1 || target.r2 !== source.r2)
  const horizontal = target.r1 === source.r1 && target.r2 === source.r2 && (target.c1 !== source.c1 || target.c2 !== source.c2)
  if (vertical) {
    for (let col = source.c1; col <= source.c2; col += 1) fillLine(sheet, source.r1, source.r2, col, target.r1, target.r2, 'row')
    return true
  }
  if (horizontal) {
    for (let row = source.r1; row <= source.r2; row += 1) fillLine(sheet, source.c1, source.c2, row, target.c1, target.c2, 'col')
    return true
  }
  return false
}

export function parseTSV(text) {
  const lines = String(text || '').replace(/\r\n/g, '\n').replace(/\r/g, '\n').split('\n')
  if (lines.length && lines[lines.length - 1] === '') lines.pop()
  return lines.map((line) => line.split('\t'))
}

export function selectionTSV(sheet, range) {
  const lines = []
  for (let row = range.r1; row <= range.r2; row += 1) {
    const cols = []
    for (let col = range.c1; col <= range.c2; col += 1) cols.push(findCell(sheet, row, col)?.input || '')
    lines.push(cols.join('\t'))
  }
  return lines.join('\n')
}

export function pasteValues(sheet, row, col, matrix) {
  matrix.forEach((line, dr) => {
    line.forEach((value, dc) => {
      const destRow = row + dr
      const destCol = col + dc
      if (!ensureGrid(sheet, destRow, destCol)) return
      writeInput(sheet, destRow, destCol, value)
    })
  })
}

export function pasteCells(sheet, row, col, matrix) {
  matrix.forEach((line, dr) => {
    line.forEach((cell, dc) => {
      const destRow = row + dr
      const destCol = col + dc
      const span = mergeSpan(cell, destRow, destCol)
      if (!ensureGrid(sheet, destRow + span.rows - 1, destCol + span.cols - 1)) return
      if (!cell) {
        const existing = findCell(sheet, destRow, destCol)
        if (existing) putCell(sheet, { ...existing, input: '' })
        return
      }
      const next = {
        ...cell,
        row: destRow,
        col: destCol,
        input: shiftFormula(cell.input, destRow - cell.row, destCol - cell.col),
      }
      if (span.rows > 1) next.merge_rows = span.rows
      else delete next.merge_rows
      if (span.cols > 1) next.merge_cols = span.cols
      else delete next.merge_cols
      putCell(sheet, next)
    })
  })
}

export function visibleCell(sheet, row, col) {
  return mergeAnchor(sheet, row, col)
}

export function stepCell(sheet, row, col, dRow, dCol) {
  const origin = mergeAnchor(sheet, row, col)
  let nextRow = row
  let nextCol = col
  const limit = (sheet?.row_count || 1) + (sheet?.col_count || 1)
  for (let i = 0; i < limit; i += 1) {
    const candidateRow = nextRow + dRow
    const candidateCol = nextCol + dCol
    if (candidateRow < 1 || candidateCol < 1 || candidateRow > sheet.row_count || candidateCol > sheet.col_count) return origin
    nextRow = candidateRow
    nextCol = candidateCol
    const anchor = mergeAnchor(sheet, nextRow, nextCol)
    if (anchor.row === origin.row && anchor.col === origin.col && mergedAt(sheet, origin)) continue
    return anchor
  }
  return origin
}

export function sortRows(sheet, range, col, direction, workbook) {
  const rows = []
  for (let row = range.r1; row <= range.r2; row += 1) {
    rows.push({
      shown: displayCell(sheet, row, col, workbook),
      cells: sheet.cells
        .filter((cell) => cell.row === row && cell.col >= range.c1 && cell.col <= range.c2)
        .map((cell) => ({ ...cell })),
    })
  }
  rows.sort((a, b) => compareShown(a.shown, b.shown) * (direction === 'desc' ? -1 : 1))
  sheet.cells = sheet.cells.filter((cell) => cell.row < range.r1 || cell.row > range.r2 || cell.col < range.c1 || cell.col > range.c2)
  rows.forEach((row, index) => {
    const destRow = range.r1 + index
    for (const cell of row.cells) {
      sheet.cells.push({
        ...cell,
        row: destRow,
        input: shiftFormula(cell.input, destRow - cell.row, 0),
      })
    }
  })
  sheet.cells.sort((a, b) => a.row - b.row || a.col - b.col)
}

export function usedBounds(sheet) {
  let r2 = 1
  let c2 = 1
  for (const cell of sheet.cells || []) {
    if (!cellHasContent(cell)) continue
    r2 = Math.max(r2, cell.row)
    c2 = Math.max(c2, cell.col)
  }
  return { r1: 1, c1: 1, r2, c2 }
}

export function pasteCut(origin, dest, row, col, matrix) {
  for (const line of matrix || []) {
    for (const cell of line || []) {
      if (cell && origin) removeCell(origin, cell.row, cell.col)
    }
  }
  pasteCells(dest, row, col, matrix)
}

export function renameSheetRefs(workbook, oldName, newName) {
  if (!oldName || oldName === newName) return
  for (const item of workbook?.sheets || []) {
    for (const cell of item.cells || []) cell.input = replaceSheetNames(cell.input, oldName, newName)
  }
}

export function setActive(workbook, index) {
  if (index > 0) workbook.active = index
  else delete workbook.active
}

export function uniqueSheetName(name, used) {
  const base = cleanName(name) || 'Sheet1'
  if (!used.has(base.toLowerCase())) return base
  let n = 2
  while (n < 1000) {
    const suffix = ` ${n}`
    const next = `${base.slice(0, 31 - suffix.length)}${suffix}`
    if (!used.has(next.toLowerCase())) return next
    n += 1
  }
  return base
}

function normalizeSheet(raw, index) {
  const name = cleanName(raw?.name || raw?.sheet_name) || `Sheet${index + 1}`
  let rows = clamp(raw?.row_count, 40, 1, MAX_ROWS)
  let cols = clamp(raw?.col_count, 16, 1, MAX_COLS)
  const cells = []
  const seen = new Set()
  for (const item of raw?.cells || []) {
    const row = Number(item?.row)
    const col = Number(item?.col)
    if (!Number.isInteger(row) || !Number.isInteger(col) || row < 1 || col < 1 || row > MAX_ROWS || col > MAX_COLS) continue
    const key = `${row},${col}`
    if (seen.has(key)) continue
    seen.add(key)
    const cell = normalizeStoredCell({ ...item, row, col })
    if (!cell) continue
    rows = Math.max(rows, row)
    cols = Math.max(cols, col)
    cells.push(cell)
  }
  cells.sort((a, b) => a.row - b.row || a.col - b.col)
  const sheet = { name, row_count: rows, col_count: cols }
  const widths = normalizeWidths(raw?.col_widths, cols)
  if (widths) sheet.col_widths = widths
  sheet.cells = cells
  return sheet
}

function normalizeStoredCell(raw) {
  if (!raw) return null
  const input = String(raw.input ?? '').replace(/\r\n/g, '\n').replace(/\r/g, '\n')
  const clipped = [...input].slice(0, MAX_INPUT).join('')
  const cell = { row: raw.row, col: raw.col, input: clipped }
  if (raw.bold) cell.bold = true
  if (raw.italic) cell.italic = true
  if (raw.underline) cell.underline = true
  if (raw.strike) cell.strike = true
  if (ALIGNS.has(raw.align)) cell.align = raw.align
  if (VALIGNS.has(raw.valign)) cell.valign = raw.valign
  if (FORMATS.has(raw.format)) cell.format = raw.format
  if (FONTS.has(raw.font)) cell.font = raw.font
  if (isColor(raw.color)) cell.color = String(raw.color).toLowerCase()
  if (isColor(raw.fill)) cell.fill = String(raw.fill).toLowerCase()
  const size = Number(raw.size)
  if (Number.isInteger(size) && size >= 6 && size <= 72) cell.size = size
  if (raw.wrap) cell.wrap = true
  if (BORDERS.has(raw.border)) cell.border = raw.border
  if (raw.rotate === 'v') cell.rotate = 'v'
  const decimals = Number(raw.decimals)
  if (Number.isInteger(decimals) && decimals >= 0 && decimals <= 6 && raw.decimals !== '' && raw.decimals != null) cell.decimals = decimals
  const note = String(raw.note || '').replace(/\r\n/g, '\n').replace(/\r/g, '\n').replace(/[\u0000-\u0008\u000b\u000c\u000e-\u001f]/g, '').trim().slice(0, 500)
  if (note) cell.note = note
  const mergeRows = Number(raw.merge_rows) || 1
  const mergeCols = Number(raw.merge_cols) || 1
  if (mergeRows >= 1 && mergeCols >= 1 && (mergeRows > 1 || mergeCols > 1) && raw.row + mergeRows - 1 <= MAX_ROWS && raw.col + mergeCols - 1 <= MAX_COLS) {
    if (mergeRows > 1) cell.merge_rows = mergeRows
    if (mergeCols > 1) cell.merge_cols = mergeCols
  }
  if (!cellHasContent(cell)) return null
  return cell
}

function cellHasContent(cell) {
  return Boolean(
    cell.input || cell.bold || cell.italic || cell.underline || cell.strike || cell.align || cell.valign
    || cell.format || cell.color || cell.fill || cell.font || cell.size || cell.wrap || cell.border
    || cell.rotate || cell.note || Number.isInteger(cell.decimals) || cell.merge_rows || cell.merge_cols,
  )
}

function releaseMerges(sheet, axis, at, count, mode) {
  for (const cell of sheet.cells || []) {
    const spanRow = cell.merge_rows || 1
    const spanCol = cell.merge_cols || 1
    if (spanRow === 1 && spanCol === 1) continue
    const start = axis === 'row' ? cell.row : cell.col
    const span = axis === 'row' ? spanRow : spanCol
    const end = start + span - 1
    const hit = mode === 'insert'
      ? at > start && at <= end
      : !(end < at || start > at + count - 1)
    if (!hit) continue
    delete cell.merge_rows
    delete cell.merge_cols
  }
}

function normalizeWidths(widths, cols) {
  if (!Array.isArray(widths) || !widths.length) return null
  const next = []
  for (let i = 0; i < cols; i += 1) {
    const value = Number(widths[i]) || 0
    next.push(value >= 48 && value <= 360 ? Math.round(value) : 0)
  }
  return trimWidths(next)
}

function trimWidths(widths) {
  if (!widths.some((value) => value)) return null
  return widths
}

function cleanName(name) {
  return String(name || '').replace(/[\u0000-\u001f:\\/?*[\]]/g, '').trim().slice(0, 31)
}

function clamp(value, fallback, min, max) {
  const number = Number(value)
  if (!Number.isInteger(number)) return fallback
  return Math.min(max, Math.max(min, number))
}

function isColor(value) {
  return /^#[0-9a-fA-F]{6}$/.test(String(value || ''))
}

function mergeSpan(cell, destRow, destCol) {
  let rows = Math.max(1, Number(cell?.merge_rows) || 1)
  let cols = Math.max(1, Number(cell?.merge_cols) || 1)
  if (destRow < 1 || destCol < 1 || destRow > MAX_ROWS || destCol > MAX_COLS) return { rows: 1, cols: 1 }
  rows = Math.min(rows, MAX_ROWS - destRow + 1)
  cols = Math.min(cols, MAX_COLS - destCol + 1)
  return { rows, cols }
}

function mergeAnchor(sheet, row, col) {
  for (const cell of sheet?.cells || []) {
    const rows = cell.merge_rows || 1
    const cols = cell.merge_cols || 1
    if (rows === 1 && cols === 1) continue
    if (row >= cell.row && row < cell.row + rows && col >= cell.col && col < cell.col + cols) {
      return { row: cell.row, col: cell.col }
    }
  }
  return { row, col }
}

function mergedAt(sheet, anchor) {
  const cell = findCell(sheet, anchor.row, anchor.col)
  return (cell?.merge_rows || 1) > 1 || (cell?.merge_cols || 1) > 1
}

function ensureGrid(sheet, row, col) {
  if (row > MAX_ROWS || col > MAX_COLS || row < 1 || col < 1) return false
  if (row > sheet.row_count) sheet.row_count = row
  if (col > sheet.col_count) {
    extendWidths(sheet, col)
    sheet.col_count = col
  }
  return true
}

function extendWidths(sheet, cols) {
  if (!Array.isArray(sheet.col_widths) || !sheet.col_widths.length) return
  while (sheet.col_widths.length < cols) sheet.col_widths.push(0)
}

function syncQualifiedRefs(workbook, sheet, axis, at, count, mode) {
  if (!workbook?.sheets) return
  for (const other of workbook.sheets) {
    if (other === sheet) continue
    for (const cell of other.cells || []) {
      cell.input = shiftStructural(cell.input, axis, at, count, sheet.name, true, mode)
    }
  }
}

function shiftStructural(input, axis, at, count, sheetName, qualifiedOnly, mode) {
  const name = String(sheetName || '').toLowerCase()
  return mapFormulaRefs(input, (ref, ctx) => {
    const target = String(ctx.sheet || '').toLowerCase()
    if (qualifiedOnly) {
      if (!target || target !== name) return ref
    } else if (target && target !== name) return ref
    if (mode === 'insert') {
      if (ref[axis] < at) return ref
      const next = { ...ref, [axis]: ref[axis] + count }
      if (next.row > MAX_ROWS || next.col > MAX_COLS) return null
      return next
    }
    if (ref[axis] >= at && ref[axis] < at + count) return null
    if (ref[axis] >= at + count) return { ...ref, [axis]: ref[axis] - count }
    return ref
  })
}

function fillLine(sheet, start, end, fixed, targetStart, targetEnd, axis) {
  const from = Math.min(start, end)
  const to = Math.max(start, end)
  const span = to - from + 1
  const destFrom = Math.min(targetStart, targetEnd)
  const destTo = Math.max(targetStart, targetEnd)
  const series = axis === 'row' ? numericSeries(sheet, from, to, fixed, 'row') : numericSeries(sheet, from, to, fixed, 'col')
  for (let cursor = destFrom; cursor <= destTo; cursor += 1) {
    if (cursor >= from && cursor <= to) continue
    const offset = cursor - from
    if (axis === 'row') {
      const src = findCell(sheet, from + mod(offset, span), fixed)
      if (series) putNumber(sheet, cursor, fixed, series.start + series.step * offset, src)
      else copyAlong(sheet, from + mod(offset, span), fixed, cursor, fixed)
    } else {
      const src = findCell(sheet, fixed, from + mod(offset, span))
      if (series) putNumber(sheet, fixed, cursor, series.start + series.step * offset, src)
      else copyAlong(sheet, fixed, from + mod(offset, span), fixed, cursor)
    }
  }
}

function numericSeries(sheet, from, to, fixed, axis) {
  const values = []
  for (let cursor = from; cursor <= to; cursor += 1) {
    const cell = axis === 'row' ? findCell(sheet, cursor, fixed) : findCell(sheet, fixed, cursor)
    if (!cell || cell.input.startsWith('=') || !/^-?\d+(?:\.\d+)?$/.test(cell.input)) return null
    values.push(Number(cell.input))
  }
  if (values.length < 2) return null
  const step = values[1] - values[0]
  for (let i = 2; i < values.length; i += 1) {
    if (Math.abs(values[i] - values[i - 1] - step) > 1e-9) return null
  }
  return { start: values[0], step }
}

function putNumber(sheet, row, col, value, style) {
  const input = String(Math.round(value * 1e10) / 1e10)
  const next = { row, col, input }
  for (const key of STYLE_KEYS) {
    if (key === 'decimals') {
      if (Number.isInteger(style?.decimals)) next.decimals = style.decimals
      continue
    }
    if (style?.[key]) next[key] = style[key]
  }
  putCell(sheet, next)
}

function copyAlong(sheet, srcRow, srcCol, destRow, destCol) {
  const src = findCell(sheet, srcRow, srcCol)
  if (!src) {
    const existing = findCell(sheet, destRow, destCol)
    if (existing) putCell(sheet, { ...existing, input: '' })
    return
  }
  putCell(sheet, {
    ...src,
    row: destRow,
    col: destCol,
    input: shiftFormula(src.input, destRow - srcRow, destCol - srcCol),
  })
}

function mod(value, span) {
  return ((value % span) + span) % span
}

function compareShown(a, b) {
  const an = /^-?\d+(?:\.\d+)?$/.test(a)
  const bn = /^-?\d+(?:\.\d+)?$/.test(b)
  if (an && bn) return Number(a) - Number(b)
  if (an) return -1
  if (bn) return 1
  return String(a).localeCompare(String(b), 'zh')
}

export function refAt(label) {
  return parseRef(label)
}

export function labelOf(row, col) {
  return `${columnLabel(col)}${row}`
}
