<script setup>
import { computed, nextTick, onBeforeUnmount, onMounted, ref, watch } from 'vue'
import { columnLabel, parseRef, sheetDisplay } from '../editors/formula'
import { useSessionStore } from '../stores/session'
import {
  MAX_COLS,
  MAX_ROWS,
  MAX_SHEETS,
  clearInputs,
  deleteCols,
  deleteRows,
  fillRange,
  findCell,
  growSheet,
  insertCols,
  insertRows,
  labelOf,
  normalizeWorkbook,
  parseTSV,
  pasteCells,
  pasteCut,
  pasteValues,
  putCell,
  renameSheetRefs,
  selectionTSV,
  setActive,
  setColumnWidth,
  sortRows,
  uniqueSheetName,
  usedBounds,
  writeInput,
} from '../editors/excel'

const props = defineProps({
  modelValue: { type: Object, required: true },
  readonly: { type: Boolean, default: false },
  title: { type: String, default: '' },
  status: { type: String, default: '' },
  pending: { type: Boolean, default: false },
  previewing: { type: Boolean, default: false },
})
const emit = defineEmits(['update:modelValue', 'update:title', 'save', 'versions'])
const session = useSessionStore()

const rootRef = ref(null)
const editorRef = ref(null)
const catcher = ref(null)
const composing = ref(false)
const anchor = ref({ row: 1, col: 1 })
const focusCell = ref({ row: 1, col: 1 })
const editing = ref(null)
const editorSource = ref('cell')
const previewActive = ref(0)
const renaming = ref(false)
const renameText = ref('')
const fillTarget = ref(null)
const localClip = ref(null)
const menu = ref('')
const ribbonOpen = ref(true)
const zoom = ref(1)
const starred = ref(false)
const painter = ref(null)
const filterOn = ref(false)
const filterText = ref('')
const nameDraft = ref('')
const linkDraft = ref('')
const noteDraft = ref('')
const commandQuery = ref('')
const colorInput = ref(null)
const fillInput = ref(null)
const undoStack = []
const redoStack = []
let editBaseline = ''
let dragListeners = null

const sheets = computed(() => (Array.isArray(props.modelValue?.sheets) ? props.modelValue.sheets : []))
const active = computed(() => {
  const requested = props.readonly ? previewActive.value : (props.modelValue?.active || 0)
  return Math.min(Math.max(0, requested), Math.max(0, sheets.value.length - 1))
})
const sheet = computed(() => sheets.value[active.value] || { name: 'Sheet1', row_count: 1, col_count: 1, cells: [] })
const rows = computed(() => Array.from({ length: sheet.value.row_count || 1 }, (_, index) => index + 1))
const cols = computed(() => Array.from({ length: sheet.value.col_count || 1 }, (_, index) => index + 1))
const range = computed(() => ({
  r1: Math.min(anchor.value.row, focusCell.value.row),
  r2: Math.max(anchor.value.row, focusCell.value.row),
  c1: Math.min(anchor.value.col, focusCell.value.col),
  c2: Math.max(anchor.value.col, focusCell.value.col),
}))
const rendered = computed(() => {
  for (const item of sheets.value) {
    void item.row_count
    void item.col_count
    for (const cell of item.cells || []) void cell.input
  }
  return sheetDisplay(sheet.value, props.modelValue)
})
const formulaText = computed(() => {
  if (editing.value) return editing.value.value
  return findCell(sheet.value, focusCell.value.row, focusCell.value.col)?.input || ''
})
const nameBox = computed(() => {
  const box = range.value
  const start = labelOf(box.r1, box.c1)
  if (box.r1 === box.r2 && box.c1 === box.c2) return start
  return `${start}:${labelOf(box.r2, box.c2)}`
})
const focusRecord = computed(() => findCell(sheet.value, focusCell.value.row, focusCell.value.col) || {})
const accountMark = computed(() => [...(session.account?.display_name || '我')][0] || '我')
const namedCells = computed(() => (sheet.value.cells || [])
  .filter((cell) => cell.input)
  .slice(0, 40)
  .map((cell) => ({ label: labelOf(cell.row, cell.col), input: cell.input })))
const chartBars = computed(() => {
  const box = range.value
  const bars = []
  for (let row = box.r1; row <= box.r2; row += 1) {
    for (let col = box.c1; col <= box.c2; col += 1) {
      const raw = String(shown(row, col)).replace(/[¥,%]/g, '').replace(/,/g, '')
      const value = Number(raw)
      if (!Number.isFinite(value)) continue
      bars.push({ label: labelOf(row, col), value })
      if (bars.length >= 8) return bars
    }
  }
  return bars
})
const chartMax = computed(() => Math.max(1, ...chartBars.value.map((item) => Math.abs(item.value))))
const ZOOMS = [50, 75, 100, 125, 150, 200]
const SIZES = [8, 9, 10, 11, 12, 14, 16, 18, 20, 24, 28, 36]
const FONTS = [
  { id: '', label: '默认' },
  { id: 'serif', label: '宋体' },
  { id: 'sans', label: '黑体' },
  { id: 'mono', label: '等宽' },
]
const commandNames = ['撤销', '重做', '打印', '加粗', '斜体', '删除线', '货币', '百分比', '数字', '插入行', '插入列', '升序', '降序', '求和', '保存']
const commandHits = computed(() => {
  const query = commandQuery.value.trim()
  if (!query) return []
  return commandNames.filter((name) => name.includes(query))
})

function emitUpdate() {
  emit('update:modelValue', props.modelValue)
}

function snapshot() {
  return JSON.stringify(props.modelValue)
}

function checkpoint() {
  undoStack.push(snapshot())
  if (undoStack.length > 80) undoStack.shift()
  redoStack.length = 0
}

function restore(raw) {
  const data = JSON.parse(raw)
  if (data.active) props.modelValue.active = data.active
  else delete props.modelValue.active
  props.modelValue.sheets = data.sheets
  emitUpdate()
}

function undo() {
  if (editing.value) {
    cancelEdit()
    return
  }
  if (!undoStack.length) return
  redoStack.push(snapshot())
  restore(undoStack.pop())
}

function redo() {
  if (editing.value || !redoStack.length) return
  undoStack.push(snapshot())
  restore(redoStack.pop())
}

function shown(row, col) {
  return rendered.value[`${row},${col}`] || ''
}

function inferAlign(text) {
  if (!text || text.startsWith('#')) return 'left'
  if (text === 'TRUE' || text === 'FALSE' || /^-?\d/.test(text) || text.endsWith('%') || /^\d{4}-\d{2}-\d{2}$/.test(text)) return 'right'
  return 'left'
}

function cellClass(row, col) {
  const cell = findCell(sheet.value, row, col)
  const text = shown(row, col)
  const align = cell?.align || inferAlign(text)
  const box = fillTarget.value
  return {
    selected: focusCell.value.row === row && focusCell.value.col === col,
    'in-range': row >= range.value.r1 && row <= range.value.r2 && col >= range.value.c1 && col <= range.value.c2,
    filling: box && row >= box.r1 && row <= box.r2 && col >= box.c1 && col <= box.c2,
    bold: Boolean(cell?.bold),
    italic: Boolean(cell?.italic),
    underline: Boolean(cell?.underline),
    strike: Boolean(cell?.strike),
    wrap: Boolean(cell?.wrap),
    rotated: cell?.rotate === 'v',
    error: text.startsWith('#'),
    [`align-${align}`]: true,
    [`valign-${cell?.valign || 'middle'}`]: true,
    [`border-${cell?.border}`]: Boolean(cell?.border),
  }
}

function cellStyle(row, col) {
  const cell = findCell(sheet.value, row, col)
  const width = sheet.value.col_widths?.[col - 1] || 0
  const family = cell?.font === 'serif'
    ? 'Georgia, "Songti SC", serif'
    : cell?.font === 'mono'
      ? 'Consolas, "Sarasa Mono SC", monospace'
      : cell?.font === 'sans'
        ? 'Arial, "PingFang SC", "Noto Sans SC", sans-serif'
        : undefined
  return {
    color: cell?.color || undefined,
    background: cell?.fill || undefined,
    fontFamily: family,
    fontSize: cell?.size ? `${cell.size}pt` : undefined,
    minWidth: width ? `${width}px` : undefined,
    width: width ? `${width}px` : undefined,
  }
}

function covered(row, col) {
  for (const cell of sheet.value.cells || []) {
    const rows = cell.merge_rows || 1
    const cols = cell.merge_cols || 1
    if (rows === 1 && cols === 1) continue
    if (row === cell.row && col === cell.col) continue
    if (row >= cell.row && row < cell.row + rows && col >= cell.col && col < cell.col + cols) return true
  }
  return false
}

function spanOf(row, col, axis) {
  const cell = findCell(sheet.value, row, col)
  return Math.max(1, Number(axis === 'row' ? cell?.merge_rows : cell?.merge_cols) || 1)
}

function colHot(col) {
  return col >= range.value.c1 && col <= range.value.c2
}

function rowHot(row) {
  return row >= range.value.r1 && row <= range.value.r2
}

function rowVisible(row) {
  const query = filterText.value.trim().toLowerCase()
  if (!filterOn.value || !query) return true
  for (let col = 1; col <= (sheet.value.col_count || 1); col += 1) {
    if (String(shown(row, col)).toLowerCase().includes(query)) return true
    if ((findCell(sheet.value, row, col)?.input || '').toLowerCase().includes(query)) return true
  }
  return false
}

function toggleMenu(name) {
  menu.value = menu.value === name ? '' : name
}

function closeMenus() {
  menu.value = ''
}

function focusGrid() {
  nextTick(() => {
    if (editing.value) return
    const el = catcher.value
    if (el && typeof el.focus === 'function') el.focus({ preventScroll: true })
    else rootRef.value?.focus({ preventScroll: true })
  })
}

function bindCatcher(el) {
  catcher.value = el || null
}

function onCatcherInput(event) {
  if (composing.value || props.readonly) return
  const value = event.target.value
  event.target.value = ''
  if (!value) return
  if (editing.value) {
    editing.value.value += value
    writeInput(sheet.value, editing.value.row, editing.value.col, editing.value.value)
    emitUpdate()
    return
  }
  beginEdit(value)
}

function onCompositionStart() {
  composing.value = true
}

function onCatcherKey(event) {
  const key = event.key.toLowerCase()
  if ((event.ctrlKey || event.metaKey) && key === 's') return
  event.stopPropagation()
  onWindowKey(event)
}

function onCompositionEnd(event) {
  composing.value = false
  const value = event.data || event.target.value || ''
  event.target.value = ''
  if (!value || props.readonly) return
  if (editing.value) {
    editing.value.value += value
    writeInput(sheet.value, editing.value.row, editing.value.col, editing.value.value)
    emitUpdate()
    return
  }
  beginEdit(value)
}

function move(dRow, dCol, extend = false) {
  const row = Math.min(sheet.value.row_count, Math.max(1, focusCell.value.row + dRow))
  const col = Math.min(sheet.value.col_count, Math.max(1, focusCell.value.col + dCol))
  if (!extend) anchor.value = { row, col }
  focusCell.value = { row, col }
  scrollIntoView(row, col)
}

function jump(dRow, dCol, extend = false) {
  let row = focusCell.value.row
  let col = focusCell.value.col
  const occupied = (r, c) => Boolean(shown(r, c))
  const step = (r, c) => ({ row: r + dRow, col: c + dCol })
  const inside = (r, c) => r >= 1 && c >= 1 && r <= sheet.value.row_count && c <= sheet.value.col_count
  if (occupied(row, col)) {
    const next = step(row, col)
    if (inside(next.row, next.col) && occupied(next.row, next.col)) {
      while (inside(row + dRow, col + dCol) && occupied(row + dRow, col + dCol)) {
        row += dRow
        col += dCol
      }
    } else {
      row += dRow
      col += dCol
      while (inside(row, col) && !occupied(row, col)) {
        row += dRow
        col += dCol
      }
      if (!inside(row, col)) {
        row = Math.min(sheet.value.row_count, Math.max(1, row))
        col = Math.min(sheet.value.col_count, Math.max(1, col))
      }
    }
  } else {
    row += dRow
    col += dCol
    while (inside(row, col) && !occupied(row, col)) {
      row += dRow
      col += dCol
    }
    if (!inside(row, col)) {
      row = dRow > 0 ? sheet.value.row_count : dRow < 0 ? 1 : row
      col = dCol > 0 ? sheet.value.col_count : dCol < 0 ? 1 : col
    }
  }
  if (!extend) anchor.value = { row, col }
  focusCell.value = { row, col }
  scrollIntoView(row, col)
}

function scrollIntoView(row, col) {
  nextTick(() => {
    rootRef.value?.querySelector(`[data-row="${row}"][data-col="${col}"]`)?.scrollIntoView({ block: 'nearest', inline: 'nearest' })
  })
}

function beginEdit(initial, source = 'cell') {
  if (props.readonly) return
  if (editing.value) {
    if (initial != null) {
      editing.value.value = initial
      writeInput(sheet.value, editing.value.row, editing.value.col, initial)
      emitUpdate()
    }
    return
  }
  checkpoint()
  editBaseline = undoStack[undoStack.length - 1]
  editorSource.value = source
  const row = focusCell.value.row
  const col = focusCell.value.col
  const value = initial != null ? initial : (findCell(sheet.value, row, col)?.input || '')
  editing.value = { row, col, value }
  writeInput(sheet.value, row, col, value)
  emitUpdate()
}

function commitEdit(dRow, dCol, keepFocus = false) {
  if (editing.value) {
    writeInput(sheet.value, editing.value.row, editing.value.col, editing.value.value)
    emitUpdate()
    if (editBaseline && snapshot() === editBaseline) undoStack.pop()
    editBaseline = ''
    editing.value = null
  }
  if (dRow || dCol) move(dRow, dCol)
  if (!keepFocus) focusGrid()
}

function cancelEdit() {
  if (!editing.value) return
  if (editBaseline) {
    restore(editBaseline)
    if (undoStack[undoStack.length - 1] === editBaseline) undoStack.pop()
  }
  editBaseline = ''
  editing.value = null
  focusGrid()
}

function onFormulaFocus() {
  if (!props.readonly) beginEdit(undefined, 'formula')
}

function onFormulaInput(event) {
  const value = event.target.value
  if (!editing.value) beginEdit(value, 'formula')
  else {
    editing.value.value = value
    writeInput(sheet.value, editing.value.row, editing.value.col, value)
    emitUpdate()
  }
}

function onCellInput(event) {
  const value = event.target.value
  editing.value.value = value
  writeInput(sheet.value, editing.value.row, editing.value.col, value)
  emitUpdate()
}

function insertNewline(event) {
  const el = event.target
  const start = el.selectionStart || 0
  const end = el.selectionEnd || 0
  const value = `${el.value.slice(0, start)}\n${el.value.slice(end)}`
  editing.value.value = value
  writeInput(sheet.value, editing.value.row, editing.value.col, value)
  emitUpdate()
  nextTick(() => {
    el.selectionStart = start + 1
    el.selectionEnd = start + 1
  })
}

function onEditorKey(event) {
  if (event.key === 'Enter' && event.altKey) {
    event.preventDefault()
    insertNewline(event)
    return
  }
  if (event.key === 'Enter') {
    event.preventDefault()
    commitEdit(event.shiftKey ? -1 : 1, 0)
    return
  }
  if (event.key === 'Tab') {
    event.preventDefault()
    commitEdit(0, event.shiftKey ? -1 : 1)
    return
  }
  if (event.key === 'Escape') {
    event.preventDefault()
    cancelEdit()
    return
  }
  if ((event.ctrlKey || event.metaKey) && event.key.toLowerCase() === 'z' && !event.shiftKey) {
    event.preventDefault()
    cancelEdit()
  }
}

function setFocus(row, col, extend) {
  if (!extend) anchor.value = { row, col }
  focusCell.value = { row, col }
}

function onCellMouseDown(event, row, col) {
  if (event.button !== 0) return
  closeMenus()
  if (painter.value && !props.readonly && !event.shiftKey) {
    if (editing.value) commitEdit(0, 0)
    setFocus(row, col, false)
    const style = painter.value
    painter.value = null
    mutateStyle(() => style)
    focusGrid()
    return
  }
  if (editing.value && (editing.value.row !== row || editing.value.col !== col)) commitEdit(0, 0)
  setFocus(row, col, event.shiftKey)
  if (!event.shiftKey) startDrag(onSelectMove)
  focusGrid()
}

function onSelectMove(event) {
  const cell = cellFromPoint(event.clientX, event.clientY)
  if (cell) focusCell.value = cell
}

function selectRow(row, extend) {
  if (editing.value) commitEdit(0, 0)
  if (!extend) anchor.value = { row, col: 1 }
  focusCell.value = { row, col: sheet.value.col_count }
  focusGrid()
}

function selectCol(col, extend) {
  if (editing.value) commitEdit(0, 0)
  if (!extend) anchor.value = { row: 1, col }
  focusCell.value = { row: sheet.value.row_count, col }
  focusGrid()
}

function startDrag(move) {
  stopDrag()
  const up = () => stopDrag()
  dragListeners = { move, up }
  window.addEventListener('mousemove', move)
  window.addEventListener('mouseup', up)
}

function stopDrag() {
  if (!dragListeners) return
  window.removeEventListener('mousemove', dragListeners.move)
  window.removeEventListener('mouseup', dragListeners.up)
  dragListeners = null
}

function cellFromPoint(x, y) {
  const el = document.elementFromPoint(x, y)?.closest?.('[data-row][data-col]')
  if (!el || !rootRef.value?.contains(el)) return null
  const row = Number(el.dataset.row)
  const col = Number(el.dataset.col)
  if (!row || !col) return null
  return { row, col }
}

function onFillDown(event) {
  event.preventDefault()
  event.stopPropagation()
  fillTarget.value = { ...range.value }
  startDrag(onFillMove)
  window.addEventListener('mouseup', onFillUp, { once: true })
}

function onFillMove(event) {
  const cell = cellFromPoint(event.clientX, event.clientY)
  if (!cell) return
  const box = range.value
  const rowDelta = Math.abs(cell.row - focusCell.value.row)
  const colDelta = Math.abs(cell.col - focusCell.value.col)
  if (rowDelta >= colDelta) {
    fillTarget.value = {
      r1: Math.min(box.r1, cell.row),
      r2: Math.max(box.r2, cell.row),
      c1: box.c1,
      c2: box.c2,
    }
  } else {
    fillTarget.value = {
      r1: box.r1,
      r2: box.r2,
      c1: Math.min(box.c1, cell.col),
      c2: Math.max(box.c2, cell.col),
    }
  }
}

function onFillUp() {
  const target = fillTarget.value
  fillTarget.value = null
  stopDrag()
  if (!target || props.readonly) return
  const before = snapshot()
  checkpoint()
  fillRange(sheet.value, range.value, target)
  if (snapshot() === before) {
    undoStack.pop()
    return
  }
  anchor.value = { row: target.r1, col: target.c1 }
  focusCell.value = { row: target.r2, col: target.c2 }
  emitUpdate()
}

function mutateStyle(mutator) {
  if (props.readonly) return
  checkpoint()
  const box = range.value
  for (let row = box.r1; row <= box.r2; row += 1) {
    for (let col = box.c1; col <= box.c2; col += 1) {
      const existing = findCell(sheet.value, row, col) || { row, col, input: '' }
      putCell(sheet.value, { ...existing, row, col, ...mutator(existing) })
    }
  }
  emitUpdate()
}

function toggleFlag(flag) {
  const current = findCell(sheet.value, focusCell.value.row, focusCell.value.col)
  const next = !current?.[flag]
  mutateStyle(() => ({ [flag]: next }))
}

function setChoice(key, value) {
  mutateStyle(() => ({ [key]: value }))
  menu.value = ''
}

function stepDecimals(delta) {
  const cell = focusRecord.value
  const base = Number.isInteger(cell.decimals)
    ? cell.decimals
    : cell.format === 'percent' ? 0 : (cell.format === 'number' || cell.format === 'currency' ? 2 : 0)
  const next = Math.min(6, Math.max(0, base + delta))
  const format = cell.format && cell.format !== 'text' && cell.format !== 'date' ? cell.format : 'number'
  mutateStyle(() => ({ decimals: next, format }))
}

function stepSize(delta) {
  const current = focusRecord.value.size || 10
  let index = SIZES.findIndex((size) => size >= current)
  if (index < 0) index = SIZES.length - 1
  const next = SIZES[Math.min(SIZES.length - 1, Math.max(0, index + delta))]
  setChoice('size', next)
}

function setSize(value) {
  const size = Math.round(Number(value))
  if (!Number.isInteger(size)) return
  setChoice('size', Math.min(72, Math.max(6, size)))
}

function clearFormat() {
  mutateStyle(() => ({
    bold: false,
    italic: false,
    underline: false,
    strike: false,
    align: '',
    valign: '',
    format: '',
    color: '',
    fill: '',
    font: '',
    size: 0,
    wrap: false,
    border: '',
    rotate: '',
    decimals: undefined,
  }))
  menu.value = ''
}

function togglePainter() {
  if (painter.value) {
    painter.value = null
    return
  }
  const cell = focusRecord.value
  painter.value = {
    bold: Boolean(cell.bold),
    italic: Boolean(cell.italic),
    underline: Boolean(cell.underline),
    strike: Boolean(cell.strike),
    wrap: Boolean(cell.wrap),
    align: cell.align || '',
    valign: cell.valign || '',
    format: cell.format || '',
    color: cell.color || '',
    fill: cell.fill || '',
    font: cell.font || '',
    size: cell.size || 0,
    border: cell.border || '',
    rotate: cell.rotate || '',
    decimals: Number.isInteger(cell.decimals) ? cell.decimals : undefined,
  }
}

function insertFx() {
  const current = findCell(sheet.value, focusCell.value.row, focusCell.value.col)?.input || ''
  if (current.startsWith('=')) beginEdit(undefined, 'formula')
  else beginEdit(current ? `=${current}` : '=', 'formula')
  nextTick(() => rootRef.value?.querySelector('[data-testid="formula-input"]')?.focus())
}

function insertFormula(name) {
  if (props.readonly) return
  const box = range.value
  const single = box.r1 === box.r2 && box.c1 === box.c2
  let target = { row: focusCell.value.row, col: focusCell.value.col }
  let formula = `=${name}(${labelOf(target.row, target.col)})`
  if (single && target.row > 1) formula = `=${name}(${labelOf(1, target.col)}:${labelOf(target.row - 1, target.col)})`
  if (!single) {
    formula = `=${name}(${labelOf(box.r1, box.c1)}:${labelOf(box.r2, box.c2)})`
    target = { row: Math.min(MAX_ROWS, box.r2 + 1), col: box.c1 }
    if (target.row === box.r2) return
  }
  checkpoint()
  if (target.row > sheet.value.row_count) growSheet(sheet.value, target.row - sheet.value.row_count, 0)
  anchor.value = { ...target }
  focusCell.value = { ...target }
  writeInput(sheet.value, target.row, target.col, formula)
  emitUpdate()
  menu.value = ''
}

function jumpName() {
  const token = String(nameDraft.value || '').split(':')[0].trim()
  const ref = parseRef(token)
  if (!ref || ref.row > sheet.value.row_count || ref.col > sheet.value.col_count) return
  anchor.value = { row: ref.row, col: ref.col }
  focusCell.value = { row: ref.row, col: ref.col }
  menu.value = ''
  scrollIntoView(ref.row, ref.col)
  focusGrid()
}

function applyLink() {
  if (props.readonly) return
  const value = linkDraft.value.trim()
  checkpoint()
  writeInput(sheet.value, focusCell.value.row, focusCell.value.col, value)
  emitUpdate()
  menu.value = ''
}

function applyNote() {
  if (props.readonly) return
  const row = focusCell.value.row
  const col = focusCell.value.col
  checkpoint()
  const existing = findCell(sheet.value, row, col) || { row, col, input: '' }
  putCell(sheet.value, { ...existing, row, col, note: noteDraft.value })
  emitUpdate()
  menu.value = ''
}

function mergeSelection() {
  if (props.readonly) return
  const box = range.value
  const width = box.c2 - box.c1 + 1
  const height = box.r2 - box.r1 + 1
  const anchorCell = findCell(sheet.value, box.r1, box.c1)
  if (height === 1 && width === 1 && ((anchorCell?.merge_rows || 1) > 1 || (anchorCell?.merge_cols || 1) > 1)) {
    checkpoint()
    putCell(sheet.value, { ...anchorCell, merge_rows: 1, merge_cols: 1 })
    emitUpdate()
    menu.value = ''
    return
  }
  if (height === 1 && width === 1) return
  checkpoint()
  for (let row = box.r1; row <= box.r2; row += 1) {
    for (let col = box.c1; col <= box.c2; col += 1) {
      if (row === box.r1 && col === box.c1) continue
      const existing = findCell(sheet.value, row, col)
      if (!existing) continue
      putCell(sheet.value, { ...existing, input: '', merge_rows: 1, merge_cols: 1 })
    }
  }
  const kept = findCell(sheet.value, box.r1, box.c1) || { row: box.r1, col: box.c1, input: '' }
  putCell(sheet.value, { ...kept, row: box.r1, col: box.c1, merge_rows: height, merge_cols: width })
  anchor.value = { row: box.r1, col: box.c1 }
  focusCell.value = { row: box.r1, col: box.c1 }
  emitUpdate()
  menu.value = ''
}

function selectAll() {
  if (editing.value) commitEdit(0, 0)
  anchor.value = { row: 1, col: 1 }
  focusCell.value = { row: sheet.value.row_count, col: sheet.value.col_count }
  focusGrid()
}

function printSheet() {
  menu.value = ''
  window.print()
}

function runCommand(name) {
  commandQuery.value = ''
  menu.value = ''
  const commands = {
    撤销: undo,
    重做: redo,
    打印: printSheet,
    加粗: () => toggleFlag('bold'),
    斜体: () => toggleFlag('italic'),
    删除线: () => toggleFlag('strike'),
    货币: () => setChoice('format', 'currency'),
    百分比: () => setChoice('format', 'percent'),
    数字: () => setChoice('format', 'number'),
    插入行: insertRow,
    插入列: insertCol,
    升序: () => sortSheet('asc'),
    降序: () => sortSheet('desc'),
    求和: () => insertFormula('SUM'),
    保存: () => emit('save'),
  }
  commands[name]?.()
}

function clearSelection() {
  if (props.readonly) return
  checkpoint()
  clearInputs(sheet.value, range.value)
  emitUpdate()
}

function copySelection(cut) {
  const box = range.value
  const matrix = []
  for (let row = box.r1; row <= box.r2; row += 1) {
    const line = []
    for (let col = box.c1; col <= box.c2; col += 1) {
      const cell = findCell(sheet.value, row, col)
      line.push(cell ? { ...cell } : null)
    }
    matrix.push(line)
  }
  const tsv = selectionTSV(sheet.value, box)
  localClip.value = { matrix, tsv, cut, sheet: active.value }
  navigator.clipboard?.writeText(tsv).catch(() => {})
  if (cut) focusGrid()
}

function onPaste(event) {
  if (props.readonly || editing.value) return
  const text = event.clipboardData?.getData('text/plain') || ''
  if (!text && !localClip.value) return
  event.preventDefault()
  checkpoint()
  if (localClip.value && (!text || text === localClip.value.tsv)) pasteInternal()
  else pasteValues(sheet.value, focusCell.value.row, focusCell.value.col, parseTSV(text))
  emitUpdate()
}

function pasteInternal() {
  const clip = localClip.value
  if (clip.cut) {
    pasteCut(props.modelValue.sheets[clip.sheet], sheet.value, focusCell.value.row, focusCell.value.col, clip.matrix)
    localClip.value = null
    return
  }
  pasteCells(sheet.value, focusCell.value.row, focusCell.value.col, clip.matrix)
}

function insertRow() {
  if (props.readonly) return
  checkpoint()
  if (insertRows(sheet.value, focusCell.value.row, 1, props.modelValue)) emitUpdate()
  else undoStack.pop()
}

function removeRow() {
  if (props.readonly || sheet.value.row_count <= 1) return
  checkpoint()
  deleteRows(sheet.value, focusCell.value.row, 1, props.modelValue)
  focusCell.value = { row: Math.min(focusCell.value.row, sheet.value.row_count), col: focusCell.value.col }
  anchor.value = { ...focusCell.value }
  emitUpdate()
}

function insertCol() {
  if (props.readonly) return
  checkpoint()
  if (insertCols(sheet.value, focusCell.value.col, 1, props.modelValue)) emitUpdate()
  else undoStack.pop()
}

function removeCol() {
  if (props.readonly || sheet.value.col_count <= 1) return
  checkpoint()
  deleteCols(sheet.value, focusCell.value.col, 1, props.modelValue)
  focusCell.value = { row: focusCell.value.row, col: Math.min(focusCell.value.col, sheet.value.col_count) }
  anchor.value = { ...focusCell.value }
  emitUpdate()
}

function grow(kind) {
  if (props.readonly) return
  checkpoint()
  const before = snapshot()
  if (kind === 'row') growSheet(sheet.value, Math.min(5, MAX_ROWS - sheet.value.row_count), 0)
  else growSheet(sheet.value, 0, Math.min(2, MAX_COLS - sheet.value.col_count))
  if (snapshot() === before) undoStack.pop()
  else emitUpdate()
}

function sortSheet(direction) {
  if (props.readonly) return
  checkpoint()
  const single = range.value.r1 === range.value.r2 && range.value.c1 === range.value.c2
  const box = single ? usedBounds(sheet.value) : { ...range.value }
  if (single) box.c2 = Math.max(box.c2, focusCell.value.col)
  sortRows(sheet.value, box, focusCell.value.col, direction, props.modelValue)
  emitUpdate()
}

function switchSheet(index) {
  if (editing.value) commitEdit(0, 0)
  if (props.readonly) {
    previewActive.value = index
    anchor.value = { row: 1, col: 1 }
    focusCell.value = { row: 1, col: 1 }
    return
  }
  if (index === active.value) return
  setActive(props.modelValue, index)
  anchor.value = { row: 1, col: 1 }
  focusCell.value = { row: 1, col: 1 }
  emitUpdate()
}

function addSheet() {
  if (props.readonly || sheets.value.length >= MAX_SHEETS) return
  checkpoint()
  const used = new Set(sheets.value.map((item) => item.name.toLowerCase()))
  const name = uniqueSheetName(`工作表${sheets.value.length + 1}`, used)
  const created = normalizeWorkbook({ sheets: [{ name, row_count: 50, col_count: 26, cells: [] }] }).sheets[0]
  props.modelValue.sheets.push(created)
  setActive(props.modelValue, props.modelValue.sheets.length - 1)
  anchor.value = { row: 1, col: 1 }
  focusCell.value = { row: 1, col: 1 }
  emitUpdate()
}

function removeSheet() {
  if (props.readonly || sheets.value.length <= 1) return
  checkpoint()
  const index = active.value
  props.modelValue.sheets.splice(index, 1)
  setActive(props.modelValue, Math.min(index, props.modelValue.sheets.length - 1))
  emitUpdate()
}

function startRename() {
  if (props.readonly) return
  renaming.value = true
  renameText.value = sheet.value.name
}

function finishRename() {
  if (!renaming.value) return
  const used = new Set(sheets.value.filter((_, index) => index !== active.value).map((item) => item.name.toLowerCase()))
  const name = uniqueSheetName(renameText.value, used)
  const oldName = sheet.value.name
  if (name && name !== oldName) {
    checkpoint()
    renameSheetRefs(props.modelValue, oldName, name)
    sheet.value.name = name
    emitUpdate()
  }
  renaming.value = false
}

function onResizeDown(event, col) {
  if (props.readonly) return
  event.preventDefault()
  event.stopPropagation()
  const startX = event.clientX
  const start = sheet.value.col_widths?.[col - 1] || 100
  checkpoint()
  const move = (ev) => {
    setColumnWidth(sheet.value, col, Math.min(360, Math.max(48, Math.round(start + ev.clientX - startX))))
    emitUpdate()
  }
  const up = () => {
    window.removeEventListener('mousemove', move)
    window.removeEventListener('mouseup', up)
  }
  window.addEventListener('mousemove', move)
  window.addEventListener('mouseup', up)
}

function onWindowKey(event) {
  if (!rootRef.value) return
  const target = event.target
  if (target !== document.body && !rootRef.value.contains(target)) return
  const key = event.key
  const command = event.ctrlKey || event.metaKey
  const inFormula = target?.dataset?.testid === 'formula-input'
  const inEditor = target === editorRef.value || target?.dataset?.testid === 'cell-editor'
  const inCatcher = target?.dataset?.testid === 'grid-catcher'
  if (editing.value && !inEditor && !inFormula && !props.readonly) {
    if (key === 'Enter') {
      event.preventDefault()
      commitEdit(event.shiftKey ? -1 : 1, 0)
      return
    }
    if (key === 'Tab') {
      event.preventDefault()
      commitEdit(0, event.shiftKey ? -1 : 1)
      return
    }
    if (key === 'Escape') {
      event.preventDefault()
      cancelEdit()
      return
    }
    if (key === 'Backspace') {
      event.preventDefault()
      const value = editing.value.value.slice(0, -1)
      editing.value.value = value
      writeInput(sheet.value, editing.value.row, editing.value.col, value)
      emitUpdate()
      return
    }
    if (key.length === 1 && !command && !event.altKey) {
      event.preventDefault()
      const value = `${editing.value.value}${key}`
      editing.value.value = value
      writeInput(sheet.value, editing.value.row, editing.value.col, value)
      emitUpdate()
    }
    return
  }
  if (inEditor || inFormula) return
  if ((target?.tagName === 'INPUT' || target?.tagName === 'TEXTAREA') && !inCatcher) return
  if (props.readonly && !['ArrowUp', 'ArrowDown', 'ArrowLeft', 'ArrowRight', 'Tab', 'Enter', 'Home'].includes(key)) return
  if (command && key.toLowerCase() === 'z') {
    event.preventDefault()
    if (event.shiftKey) redo()
    else undo()
    return
  }
  if (command && key.toLowerCase() === 'y') {
    event.preventDefault()
    redo()
    return
  }
  if (command && key.toLowerCase() === 'b') {
    event.preventDefault()
    toggleFlag('bold')
    return
  }
  if (command && key.toLowerCase() === 'i') {
    event.preventDefault()
    toggleFlag('italic')
    return
  }
  if (command && key.toLowerCase() === 'u') {
    event.preventDefault()
    toggleFlag('underline')
    return
  }
  if (command && key.toLowerCase() === 'c') {
    event.preventDefault()
    copySelection(false)
    return
  }
  if (command && key.toLowerCase() === 'x') {
    event.preventDefault()
    copySelection(true)
    return
  }
  if (key === 'F2') {
    event.preventDefault()
    beginEdit()
    return
  }
  if (key === 'Delete' || key === 'Backspace') {
    event.preventDefault()
    clearSelection()
    return
  }
  if (key === 'Enter') {
    event.preventDefault()
    move(event.shiftKey ? -1 : 1, 0)
    return
  }
  if (key === 'Tab') {
    event.preventDefault()
    move(0, event.shiftKey ? -1 : 1)
    return
  }
  if (key === 'ArrowUp') {
    event.preventDefault()
    if (command) jump(-1, 0, event.shiftKey)
    else move(-1, 0, event.shiftKey)
    return
  }
  if (key === 'ArrowDown') {
    event.preventDefault()
    if (command) jump(1, 0, event.shiftKey)
    else move(1, 0, event.shiftKey)
    return
  }
  if (key === 'ArrowLeft') {
    event.preventDefault()
    if (command) jump(0, -1, event.shiftKey)
    else move(0, -1, event.shiftKey)
    return
  }
  if (key === 'ArrowRight') {
    event.preventDefault()
    if (command) jump(0, 1, event.shiftKey)
    else move(0, 1, event.shiftKey)
    return
  }
  if (key === 'Home') {
    event.preventDefault()
    if (command) setFocus(1, 1, false)
    else setFocus(focusCell.value.row, 1, event.shiftKey)
    return
  }
  if (key.length === 1 && !command && !event.altKey) {
    if (inCatcher) return
    event.preventDefault()
    beginEdit(key)
  }
}

function bindEditor(el) {
  editorRef.value = el || null
}

watch(editing, async (value) => {
  if (!value || editorSource.value === 'formula') return
  await nextTick()
  const el = editorRef.value
  if (!el || typeof el.focus !== 'function') return
  el.focus()
  const length = el.value.length
  el.setSelectionRange(length, length)
})

watch(() => props.modelValue, () => {
  if (focusCell.value.row > (sheet.value.row_count || 1)) focusCell.value = { row: 1, col: 1 }
  if (focusCell.value.col > (sheet.value.col_count || 1)) focusCell.value = { row: 1, col: 1 }
})

onMounted(() => window.addEventListener('keydown', onWindowKey))
onBeforeUnmount(() => {
  window.removeEventListener('keydown', onWindowKey)
  stopDrag()
})
</script>

<template>
  <div ref="rootRef" class="excel-editor" :class="{ painting: painter }" data-testid="excel-editor" tabindex="0" @paste="onPaste">
    <div class="sheets-top">
      <router-link class="sheets-mark" to="/" aria-label="返回文库">
        <svg viewBox="0 0 24 24" aria-hidden="true"><rect x="4" y="3" width="16" height="18" rx="1.5" /><path d="M4 9h16M4 15h16M10 3v18M16 3v18" /></svg>
      </router-link>
      <input class="sheets-title" data-testid="doc-title" :value="title" :disabled="previewing" @input="emit('update:title', $event.target.value)" />
      <button type="button" class="star-btn" :aria-pressed="starred" title="星标" @click="starred = !starred">
        <svg viewBox="0 0 24 24"><path d="M12 3.6l2.4 4.9 5.4.8-3.9 3.8.9 5.4L12 16l-4.8 2.5.9-5.4L4.2 9.3l5.4-.8z" /></svg>
      </button>
      <nav class="sheets-menus" v-if="!readonly">
        <div class="menu-item" v-for="item in [
          ['file', '文件'], ['edit', '编辑'], ['view', '查看'], ['insert', '插入'], ['format', '格式'], ['data', '数据'], ['tools', '工具'], ['extensions', '扩展程序'], ['help', '帮助'],
        ]" :key="item[0]">
          <button type="button" :aria-expanded="menu === item[0]" @click="toggleMenu(item[0])">{{ item[1] }}</button>
          <div class="menu-pop" v-if="menu === item[0]">
            <template v-if="item[0] === 'file'">
              <button type="button" @click="emit('save'); menu = ''">保存到云端</button>
              <button type="button" @click="emit('versions'); menu = ''">版本历史</button>
              <button type="button" @click="printSheet">打印</button>
            </template>
            <template v-else-if="item[0] === 'edit'">
              <button type="button" @click="undo(); menu = ''">撤销</button>
              <button type="button" @click="redo(); menu = ''">重做</button>
              <button type="button" @click="copySelection(true); menu = ''">剪切</button>
              <button type="button" @click="copySelection(false); menu = ''">复制</button>
              <button type="button" @click="clearSelection(); menu = ''">清除内容</button>
            </template>
            <template v-else-if="item[0] === 'view'">
              <button type="button" v-for="value in ZOOMS" :key="value" @click="zoom = value / 100; menu = ''">{{ value }}%</button>
              <button type="button" @click="ribbonOpen = !ribbonOpen; menu = ''">{{ ribbonOpen ? '收起工具栏' : '展开工具栏' }}</button>
            </template>
            <template v-else-if="item[0] === 'insert'">
              <button type="button" data-testid="insert-row" @click="insertRow(); menu = ''">插入行</button>
              <button type="button" @click="removeRow(); menu = ''">删除行</button>
              <button type="button" data-testid="insert-col" @click="insertCol(); menu = ''">插入列</button>
              <button type="button" @click="removeCol(); menu = ''">删除列</button>
              <button type="button" @click="addSheet(); menu = ''">工作表</button>
              <button type="button" @click="insertFormula('SUM')">求和</button>
            </template>
            <template v-else-if="item[0] === 'format'">
              <button type="button" data-testid="underline-button" @click="toggleFlag('underline'); menu = ''">下划线</button>
              <button type="button" @click="setChoice('format', '')">常规</button>
              <button type="button" @click="setChoice('format', 'number')">数字</button>
              <button type="button" @click="setChoice('format', 'currency')">货币</button>
              <button type="button" @click="setChoice('format', 'percent')">百分比</button>
              <button type="button" @click="setChoice('format', 'date')">日期</button>
              <button type="button" @click="setChoice('format', 'text')">文本</button>
              <button type="button" @click="clearFormat">清除格式</button>
            </template>
            <template v-else-if="item[0] === 'data'">
              <button type="button" data-testid="sort-asc" @click="sortSheet('asc'); menu = ''">升序</button>
              <button type="button" @click="sortSheet('desc'); menu = ''">降序</button>
              <button type="button" @click="filterOn = !filterOn; menu = ''">{{ filterOn ? '取消筛选' : '创建筛选' }}</button>
            </template>
            <template v-else-if="item[0] === 'tools'">
              <button type="button" @click="grow('row'); menu = ''">增加 5 行</button>
              <button type="button" @click="grow('col'); menu = ''">增加 2 列</button>
            </template>
            <template v-else-if="item[0] === 'extensions'">
              <p>当前没有已安装的扩展程序。</p>
            </template>
            <template v-else>
              <p>Ctrl+B 加粗，Ctrl+I 斜体，Ctrl+U 下划线</p>
              <p>Ctrl+Z 撤销，Ctrl+Y 重做，Ctrl+S 保存</p>
              <p>方向键移动，Shift 扩展选区，F2 编辑</p>
            </template>
          </div>
        </div>
      </nav>
      <div class="sheets-actions">
        <span class="sheets-status" :class="{ quiet: status && !/未|冲突|更新|失败/.test(status) }" data-testid="save-status">{{ status }}</span>
        <button v-if="!readonly" type="button" class="history-btn" data-testid="version-button" title="版本历史" @click="emit('versions')">
          <svg viewBox="0 0 24 24"><circle cx="12" cy="12" r="8" /><path d="M12 8v5l3 2" /></svg>
        </button>
        <div class="share-wrap menu-item" v-if="!readonly">
          <button type="button" class="share-btn" data-testid="save-button" :disabled="pending || previewing" @click="emit('save')">
            <svg viewBox="0 0 24 24"><rect x="6" y="11" width="12" height="8" rx="1" /><path d="M8 11V8a4 4 0 0 1 8 0v3" /></svg>
            共享
          </button>
          <button type="button" class="share-caret" aria-label="保存选项" @click="toggleMenu('share')">▾</button>
          <div class="menu-pop share-pop" v-if="menu === 'share'">
            <button type="button" @click="emit('save'); menu = ''">保存到云端</button>
            <button type="button" @click="emit('versions'); menu = ''">版本历史</button>
          </div>
        </div>
        <router-link class="sheets-avatar" to="/" :title="session.account?.display_name || '账号'">{{ accountMark }}</router-link>
      </div>
    </div>

    <div class="sheets-ribbon" v-if="!readonly && ribbonOpen">
      <label class="menu-search menu-item">
        <svg viewBox="0 0 24 24"><circle cx="10" cy="10" r="6" /><path d="M15 15l5 5" /></svg>
        <input v-model="commandQuery" placeholder="菜单" @focus="menu = 'commands'" @keydown.enter.prevent="commandHits[0] && runCommand(commandHits[0])" />
        <div class="menu-pop" v-if="menu === 'commands' && commandHits.length">
          <button type="button" v-for="name in commandHits" :key="name" @click="runCommand(name)">{{ name }}</button>
        </div>
      </label>
      <button type="button" title="撤销" @click="undo"><svg viewBox="0 0 24 24"><path d="M8 8H4v4" /><path d="M4 8a8 8 0 1 1-1 5" /></svg></button>
      <button type="button" title="重做" @click="redo"><svg viewBox="0 0 24 24"><path d="M16 8h4v4" /><path d="M20 8a8 8 0 1 0 1 5" /></svg></button>
      <button type="button" title="打印" @click="printSheet"><svg viewBox="0 0 24 24"><path d="M7 8V4h10v4" /><path d="M6 16H4a2 2 0 0 1-2-2v-4a2 2 0 0 1 2-2h16a2 2 0 0 1 2 2v4a2 2 0 0 1-2 2h-2" /><path d="M7 14h10v6H7z" /></svg></button>
      <button type="button" title="格式刷" :aria-pressed="Boolean(painter)" @click="togglePainter"><svg viewBox="0 0 24 24"><path d="M4 16l6-6 4 4-6 6H4z" /><path d="M14 6l4 4" /><path d="M15 4l5 5" /></svg></button>
      <div class="menu-item">
        <button type="button" class="zoom-label" @click="toggleMenu('zoom')">{{ Math.round(zoom * 100) }}%</button>
        <div class="menu-pop" v-if="menu === 'zoom'">
          <button type="button" v-for="value in ZOOMS" :key="value" @click="zoom = value / 100; menu = ''">{{ value }}%</button>
        </div>
      </div>
      <i class="ribbon-sep"></i>
      <button type="button" class="fmt" title="货币" :aria-pressed="focusRecord.format === 'currency'" @click="setChoice('format', 'currency')">¥</button>
      <button type="button" class="fmt" title="百分比" :aria-pressed="focusRecord.format === 'percent'" @click="setChoice('format', 'percent')">%</button>
      <button type="button" class="fmt dec" title="减少小数位数" @click="stepDecimals(-1)">.0←</button>
      <button type="button" class="fmt dec" title="增加小数位数" @click="stepDecimals(1)">.00→</button>
      <div class="menu-item">
        <button type="button" class="fmt" data-testid="number-format" title="数字格式" @click="toggleMenu('number')">123</button>
        <div class="menu-pop" v-if="menu === 'number'">
          <button type="button" @click="setChoice('format', '')">常规</button>
          <button type="button" @click="setChoice('format', 'number')">数字</button>
          <button type="button" @click="setChoice('format', 'currency')">货币</button>
          <button type="button" @click="setChoice('format', 'percent')">百分比</button>
          <button type="button" @click="setChoice('format', 'date')">日期</button>
          <button type="button" @click="setChoice('format', 'text')">文本</button>
        </div>
      </div>
      <i class="ribbon-sep"></i>
      <select class="font-select" data-testid="sheet-font" :value="focusRecord.font || ''" @change="setChoice('font', $event.target.value)">
        <option v-for="item in FONTS" :key="item.label" :value="item.id">{{ item.label }}</option>
      </select>
      <button type="button" title="减小字号" @click="stepSize(-1)">−</button>
      <input class="size-input" data-testid="sheet-font-size" :value="focusRecord.size || 10" @change="setSize($event.target.value)" />
      <button type="button" title="增大字号" @click="stepSize(1)">+</button>
      <i class="ribbon-sep"></i>
      <button type="button" class="fmt b" data-testid="bold-button" title="加粗" :aria-pressed="Boolean(focusRecord.bold)" @click="toggleFlag('bold')">B</button>
      <button type="button" class="fmt i" data-testid="italic-button" title="斜体" :aria-pressed="Boolean(focusRecord.italic)" @click="toggleFlag('italic')">I</button>
      <button type="button" class="fmt s" title="删除线" :aria-pressed="Boolean(focusRecord.strike)" @click="toggleFlag('strike')">S</button>
      <button type="button" class="fmt color-letter" title="文本颜色" @click="colorInput?.click()">A<i :style="{ background: focusRecord.color || '#202124' }"></i></button>
      <input ref="colorInput" class="sr-color" type="color" data-testid="font-color" :value="focusRecord.color || '#202124'" @input="setChoice('color', $event.target.value.toLowerCase())" />
      <button type="button" class="fmt color-letter" title="填充颜色" @click="fillInput?.click()">
        <svg viewBox="0 0 24 24"><path d="M7 13l7-7 4 4-7 7H7z" /><path d="M5 19h8" /></svg>
        <i :style="{ background: focusRecord.fill || '#fff3b0' }"></i>
      </button>
      <input ref="fillInput" class="sr-color" type="color" data-testid="fill-color" :value="focusRecord.fill || '#fff3b0'" @input="setChoice('fill', $event.target.value.toLowerCase())" />
      <div class="menu-item">
        <button type="button" title="边框" @click="toggleMenu('border')"><svg viewBox="0 0 24 24"><rect x="5" y="5" width="14" height="14" /><path d="M5 12h14M12 5v14" /></svg></button>
        <div class="menu-pop" v-if="menu === 'border'">
          <button type="button" @click="setChoice('border', '')">无边框</button>
          <button type="button" @click="setChoice('border', 'bottom')">下框线</button>
          <button type="button" @click="setChoice('border', 'all')">所有框线</button>
          <button type="button" @click="setChoice('border', 'outer')">外框线</button>
        </div>
      </div>
      <button type="button" title="合并单元格" data-testid="merge-cells" @click="mergeSelection"><svg viewBox="0 0 24 24"><rect x="4" y="6" width="16" height="12" /><path d="M12 6v12M4 12h16" /></svg></button>
      <div class="menu-item">
        <button type="button" title="水平对齐" @click="toggleMenu('align')"><svg viewBox="0 0 24 24"><path d="M4 7h16M7 12h10M4 17h16" /></svg></button>
        <div class="menu-pop" v-if="menu === 'align'">
          <button type="button" @click="setChoice('align', 'left')">左对齐</button>
          <button type="button" @click="setChoice('align', 'center')">居中</button>
          <button type="button" @click="setChoice('align', 'right')">右对齐</button>
        </div>
      </div>
      <div class="menu-item">
        <button type="button" title="垂直对齐" @click="toggleMenu('valign')"><svg viewBox="0 0 24 24"><path d="M6 4v16M12 7v10M18 4v16" /></svg></button>
        <div class="menu-pop" v-if="menu === 'valign'">
          <button type="button" @click="setChoice('valign', 'top')">顶端对齐</button>
          <button type="button" @click="setChoice('valign', 'middle')">垂直居中</button>
          <button type="button" @click="setChoice('valign', 'bottom')">底端对齐</button>
        </div>
      </div>
      <button type="button" title="自动换行" :aria-pressed="Boolean(focusRecord.wrap)" @click="setChoice('wrap', !focusRecord.wrap)"><svg viewBox="0 0 24 24"><path d="M4 7h16M4 12h10a4 4 0 0 1 0 8H8" /><path d="M8 17l-3 3 3 3" /></svg></button>
      <button type="button" title="文字方向" :aria-pressed="focusRecord.rotate === 'v'" @click="setChoice('rotate', focusRecord.rotate === 'v' ? '' : 'v')"><svg viewBox="0 0 24 24"><path d="M8 19V7M8 7l-3 3M8 7l3 3M14 5h6M14 10h6M14 15h4" /></svg></button>
      <i class="ribbon-sep"></i>
      <div class="menu-item">
        <button type="button" title="插入链接" @click="linkDraft = focusRecord.input || 'https://'; toggleMenu('link')"><svg viewBox="0 0 24 24"><path d="M10 14a5 5 0 0 0 7 0l2-2a5 5 0 0 0-7-7l-1 1" /><path d="M14 10a5 5 0 0 0-7 0l-2 2a5 5 0 0 0 7 7l1-1" /></svg></button>
        <div class="menu-pop link-pop" v-if="menu === 'link'">
          <input v-model="linkDraft" placeholder="https://" @keydown.enter.prevent="applyLink" />
          <button type="button" @click="applyLink">插入链接</button>
        </div>
      </div>
      <div class="menu-item">
        <button type="button" title="插入批注" @click="noteDraft = focusRecord.note || ''; toggleMenu('note')"><svg viewBox="0 0 24 24"><path d="M6 5h12v10H9l-3 3z" /></svg></button>
        <div class="menu-pop link-pop" v-if="menu === 'note'">
          <textarea v-model="noteDraft" maxlength="500" placeholder="批注"></textarea>
          <button type="button" @click="applyNote">保存批注</button>
        </div>
      </div>
      <div class="menu-item">
        <button type="button" title="图表" @click="toggleMenu('chart')"><svg viewBox="0 0 24 24"><path d="M5 19V9M12 19V5M19 19v-7" /></svg></button>
        <div class="menu-pop chart-pop" v-if="menu === 'chart'">
          <p v-if="!chartBars.length">选区里还没有可绘制的数字。</p>
          <div v-for="bar in chartBars" :key="bar.label" class="chart-row">
            <span>{{ bar.label }}</span>
            <i :style="{ width: `${Math.max(8, Math.abs(bar.value) / chartMax * 120)}px` }"></i>
          </div>
        </div>
      </div>
      <button type="button" title="筛选" :aria-pressed="filterOn" @click="filterOn = !filterOn"><svg viewBox="0 0 24 24"><path d="M4 6h16l-6 7v5l-4 2v-7z" /></svg></button>
      <input v-if="filterOn" class="filter-input" data-testid="sheet-filter" v-model="filterText" placeholder="筛选" />
      <div class="menu-item">
        <button type="button" title="函数" data-testid="insert-sum" @click="toggleMenu('fn')">Σ</button>
        <div class="menu-pop" v-if="menu === 'fn'">
          <button type="button" v-for="name in ['SUM', 'AVERAGE', 'COUNT', 'MIN', 'MAX']" :key="name" @click="insertFormula(name)">{{ name }}</button>
        </div>
      </div>
      <div class="menu-item">
        <button type="button" title="更多" @click="toggleMenu('more')"><svg viewBox="0 0 24 24"><path d="M6 12h.01M12 12h.01M18 12h.01" /></svg></button>
        <div class="menu-pop" v-if="menu === 'more'">
          <button type="button" @click="grow('row'); menu = ''">增加 5 行</button>
          <button type="button" @click="grow('col'); menu = ''">增加 2 列</button>
          <button type="button" @click="clearFormat">清除格式</button>
        </div>
      </div>
      <span class="ribbon-gap"></span>
      <div class="menu-item">
        <button type="button" class="spell-btn" title="标出公式错误" @click="toggleMenu('spell')">拼 ▾</button>
        <div class="menu-pop" v-if="menu === 'spell'">
          <p>以 # 开头的公式错误会标成红色。</p>
        </div>
      </div>
      <button type="button" class="fold-btn" title="收起工具栏" @click="ribbonOpen = false">⌃</button>
    </div>
    <div class="ribbon-fold" v-if="!readonly && !ribbonOpen">
      <button type="button" title="展开工具栏" @click="ribbonOpen = true">⌄</button>
    </div>

    <div class="formula-bar">
      <div class="name-box menu-item">
        <input
          data-testid="name-box"
          :value="menu === 'name' ? nameDraft : nameBox"
          @focus="nameDraft = nameBox; menu = 'name'"
          @input="nameDraft = $event.target.value"
          @keydown.enter.prevent="jumpName"
        />
        <button type="button" class="name-caret" aria-label="已用单元格" @click="toggleMenu('names')">▾</button>
        <div class="menu-pop" v-if="menu === 'names'">
          <button type="button" v-for="item in namedCells" :key="item.label" @click="nameDraft = item.label; jumpName()">{{ item.label }} {{ item.input }}</button>
          <p v-if="!namedCells.length">还没有内容</p>
        </div>
      </div>
      <button type="button" class="fx-btn" title="函数" @click="insertFx">fx</button>
      <input
        class="formula-input"
        data-testid="formula-input"
        :value="formulaText"
        :readonly="readonly"
        @focus="onFormulaFocus"
        @input="onFormulaInput"
        @keydown.enter.prevent="commitEdit(1, 0)"
        @keydown.tab.prevent="commitEdit(0, $event.shiftKey ? -1 : 1)"
        @keydown.esc.prevent="cancelEdit"
      />
    </div>
    <div class="sheet-wrap">
      <div class="sheet-zoom" :style="{ zoom }">
      <table class="sheet">
        <thead>
          <tr>
            <th class="corner" @click="selectAll"></th>
            <th v-for="col in cols" :key="col" :class="{ hot: colHot(col) }" :style="cellStyle(1, col)" @click="selectCol(col, $event.shiftKey)">
              {{ columnLabel(col) }}
              <i class="col-resizer" @mousedown="onResizeDown($event, col)"></i>
            </th>
          </tr>
        </thead>
        <tbody>
          <tr v-for="row in rows" :key="row" v-show="rowVisible(row)">
            <th class="row-head" :class="{ hot: rowHot(row) }" @click="selectRow(row, $event.shiftKey)">{{ row }}</th>
            <template v-for="col in cols" :key="col">
            <td
              v-if="!covered(row, col)"
              data-testid="excel-cell"
              :data-row="row"
              :data-col="col"
              :rowspan="spanOf(row, col, 'row')"
              :colspan="spanOf(row, col, 'col')"
              :class="cellClass(row, col)"
              :style="cellStyle(row, col)"
              @mousedown="onCellMouseDown($event, row, col)"
              @dblclick.stop="beginEdit()"
            >
              <input
                v-if="!readonly && !editing && focusCell.row === row && focusCell.col === col"
                :ref="bindCatcher"
                data-testid="grid-catcher"
                class="key-catcher"
                :class="{ composing }"
                @input="onCatcherInput"
                @keydown="onCatcherKey"
                @compositionstart="onCompositionStart"
                @compositionend="onCompositionEnd"
              />
              <textarea
                v-if="editing && editing.row === row && editing.col === col"
                :ref="bindEditor"
                data-testid="cell-editor"
                class="cell-editor"
                :value="editing.value"
                @input="onCellInput"
                @keydown="onEditorKey"
                @blur="commitEdit(0, 0, true)"
              ></textarea>
              <span v-else>{{ shown(row, col) }}</span>
              <i v-if="findCell(sheet, row, col)?.note" class="cell-note" :title="findCell(sheet, row, col).note"></i>
              <i
                v-if="!readonly && focusCell.row === row && focusCell.col === col && !editing"
                class="fill-handle"
                data-testid="fill-handle"
                @mousedown="onFillDown"
              ></i>
            </td>
            </template>
          </tr>
        </tbody>
      </table>
      </div>
    </div>
    <div class="sheet-bar">
      <button v-if="!readonly" type="button" class="sheet-add" data-testid="add-sheet" title="添加工作表" @click="addSheet">+</button>
      <div class="menu-item">
        <button type="button" class="sheet-list" title="所有工作表" @click="toggleMenu('sheets')">
          <svg viewBox="0 0 24 24"><path d="M5 7h14M5 12h14M5 17h14" /></svg>
        </button>
        <div class="menu-pop up" v-if="menu === 'sheets'">
          <button type="button" v-for="(item, index) in sheets" :key="`${item.name}-${index}`" @click="switchSheet(index); menu = ''">{{ item.name }}</button>
        </div>
      </div>
      <div class="sheet-tabs">
        <div class="sheet-tab-wrap" v-for="(item, index) in sheets" :key="`${item.name}-${index}`">
          <input
            v-if="renaming && index === active"
            data-testid="sheet-name"
            v-model="renameText"
            maxlength="31"
            @keydown.enter.prevent="finishRename"
            @blur="finishRename"
          />
          <button
            v-else
            type="button"
            class="sheet-tab"
            :class="{ active: index === active }"
            data-testid="sheet-tab"
            @click="switchSheet(index)"
            @dblclick.stop="startRename"
          >
            <span>{{ item.name }}</span>
            <i v-if="index === active" @click.stop="toggleMenu('tab')">▾</i>
          </button>
        </div>
      </div>
      <div class="menu-pop up tab-menu" v-if="menu === 'tab'">
        <button type="button" @click="startRename(); menu = ''">重命名</button>
        <button type="button" @click="removeSheet(); menu = ''">删除工作表</button>
      </div>
    </div>
  </div>
</template>
