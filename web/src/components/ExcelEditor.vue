<script setup>
import { computed, nextTick, onBeforeUnmount, onMounted, ref, watch } from 'vue'
import { columnLabel } from '../editors/formula'
import { sheetDisplay } from '../editors/formula'
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
  pasteValues,
  putCell,
  removeCell,
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
})
const emit = defineEmits(['update:modelValue'])

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
    error: text.startsWith('#'),
    [`align-${align}`]: true,
  }
}

function cellStyle(row, col) {
  const cell = findCell(sheet.value, row, col)
  const width = sheet.value.col_widths?.[col - 1] || 0
  return {
    color: cell?.color || undefined,
    background: cell?.fill || undefined,
    minWidth: width ? `${width}px` : undefined,
    width: width ? `${width}px` : undefined,
  }
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
  pasteCells(sheet.value, focusCell.value.row, focusCell.value.col, clip.matrix)
  if (!clip.cut) return
  const origin = props.modelValue.sheets[clip.sheet]
  if (origin) {
    clip.matrix.forEach((line) => line.forEach((cell) => {
      if (cell) removeCell(origin, cell.row, cell.col)
    }))
  }
  localClip.value = null
}

function insertRow() {
  if (props.readonly) return
  checkpoint()
  if (insertRows(sheet.value, focusCell.value.row, 1)) emitUpdate()
  else undoStack.pop()
}

function removeRow() {
  if (props.readonly || sheet.value.row_count <= 1) return
  checkpoint()
  deleteRows(sheet.value, focusCell.value.row, 1)
  focusCell.value = { row: Math.min(focusCell.value.row, sheet.value.row_count), col: focusCell.value.col }
  anchor.value = { ...focusCell.value }
  emitUpdate()
}

function insertCol() {
  if (props.readonly) return
  checkpoint()
  if (insertCols(sheet.value, focusCell.value.col, 1)) emitUpdate()
  else undoStack.pop()
}

function removeCol() {
  if (props.readonly || sheet.value.col_count <= 1) return
  checkpoint()
  deleteCols(sheet.value, focusCell.value.col, 1)
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
  const name = uniqueSheetName(`Sheet${sheets.value.length + 1}`, used)
  const created = normalizeWorkbook({ sheets: [{ name, row_count: 40, col_count: 16, cells: [] }] }).sheets[0]
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
  if (name && name !== sheet.value.name) {
    checkpoint()
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
  const start = sheet.value.col_widths?.[col - 1] || 96
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
  <div ref="rootRef" class="excel-editor" data-testid="excel-editor" tabindex="0" @paste="onPaste">
    <div class="toolbar" v-if="!readonly">
      <button type="button" class="ghost" data-testid="bold-button" :aria-pressed="Boolean(findCell(sheet, focusCell.row, focusCell.col)?.bold)" @click="toggleFlag('bold')">加粗</button>
      <button type="button" class="ghost" data-testid="italic-button" @click="toggleFlag('italic')">斜体</button>
      <button type="button" class="ghost" data-testid="underline-button" @click="toggleFlag('underline')">下划线</button>
      <button type="button" class="ghost" :aria-pressed="findCell(sheet, focusCell.row, focusCell.col)?.align === 'left'" @click="setChoice('align', 'left')">左对齐</button>
      <button type="button" class="ghost" @click="setChoice('align', 'center')">居中</button>
      <button type="button" class="ghost" @click="setChoice('align', 'right')">右对齐</button>
      <label>格式
        <select data-testid="number-format" :value="findCell(sheet, focusCell.row, focusCell.col)?.format || ''" @change="setChoice('format', $event.target.value)">
          <option value="">常规</option>
          <option value="number">数字</option>
          <option value="percent">百分比</option>
          <option value="date">日期</option>
          <option value="text">文本</option>
        </select>
      </label>
      <label>字体色 <input data-testid="font-color" type="color" :value="findCell(sheet, focusCell.row, focusCell.col)?.color || '#1d1a16'" @input="setChoice('color', $event.target.value)" /></label>
      <label>填充 <input data-testid="fill-color" type="color" :value="findCell(sheet, focusCell.row, focusCell.col)?.fill || '#fffaf3'" @input="setChoice('fill', $event.target.value)" /></label>
      <button type="button" class="ghost" data-testid="insert-row" @click="insertRow">插入行</button>
      <button type="button" class="ghost" @click="removeRow">删除行</button>
      <button type="button" class="ghost" data-testid="insert-col" @click="insertCol">插入列</button>
      <button type="button" class="ghost" @click="removeCol">删除列</button>
      <button type="button" class="ghost" @click="grow('row')">增加 5 行</button>
      <button type="button" class="ghost" @click="grow('col')">增加 2 列</button>
      <button type="button" class="ghost" data-testid="sort-asc" @click="sortSheet('asc')">升序</button>
      <button type="button" class="ghost" @click="sortSheet('desc')">降序</button>
      <button type="button" class="ghost" @click="undo">撤销</button>
      <button type="button" class="ghost" @click="redo">重做</button>
    </div>
    <div class="formula-bar">
      <span data-testid="name-box">{{ nameBox }}</span>
      <input
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
      <table class="sheet">
        <thead>
          <tr>
            <th class="corner"></th>
            <th v-for="col in cols" :key="col" :style="cellStyle(1, col)" @click="selectCol(col, $event.shiftKey)">
              {{ columnLabel(col) }}
              <i class="col-resizer" @mousedown="onResizeDown($event, col)"></i>
            </th>
          </tr>
        </thead>
        <tbody>
          <tr v-for="row in rows" :key="row">
            <th class="row-head" @click="selectRow(row, $event.shiftKey)">{{ row }}</th>
            <td
              v-for="col in cols"
              :key="col"
              data-testid="excel-cell"
              :data-row="row"
              :data-col="col"
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
              <i
                v-if="!readonly && focusCell.row === row && focusCell.col === col && !editing"
                class="fill-handle"
                data-testid="fill-handle"
                @mousedown="onFillDown"
              ></i>
            </td>
          </tr>
        </tbody>
      </table>
    </div>
    <div class="sheet-tabs">
      <button
        v-for="(item, index) in sheets"
        :key="`${item.name}-${index}`"
        type="button"
        class="sheet-tab"
        :class="{ active: index === active }"
        data-testid="sheet-tab"
        @click="switchSheet(index)"
        @dblclick.stop="startRename"
      >{{ item.name }}</button>
      <label v-if="renaming" class="sheet-rename">
        <input data-testid="sheet-name" v-model="renameText" maxlength="31" @keydown.enter.prevent="finishRename" @blur="finishRename" />
      </label>
      <button v-if="!readonly" type="button" class="ghost" data-testid="add-sheet" @click="addSheet">+</button>
      <button v-if="!readonly" type="button" class="ghost" @click="removeSheet">删除工作表</button>
    </div>
  </div>
</template>
