<script setup>
import { computed, nextTick, onBeforeUnmount, onMounted, ref, watch } from 'vue'
import {
  applyStyle,
  clearRuns,
  clampFindIndex,
  clipboardToLines,
  collapseBackward,
  findHits,
  listMarker,
  maxWordBlocks,
  mergeRuns,
  newId,
  plainText,
  replaceOnce,
  segmentsFromElement,
  sliceRuns,
  styleOf,
  textDiff,
} from '../editors/text'

const props = defineProps({
  modelValue: { type: Object, required: true },
  readonly: { type: Boolean, default: false },
})
const emit = defineEmits(['update:modelValue', 'save', 'versions'])

const active = ref(0)
const elements = ref([])
const menu = ref('')
const moreOpen = ref(false)
const findOpen = ref(false)
const findQuery = ref('')
const replaceQuery = ref('')
const findAt = ref(-1)
const showNav = ref(true)
const zoom = ref(1)
const past = ref([])
const future = ref([])
const typingStyle = ref(null)
const ui = ref(0)
const colorInput = ref(null)
const highlightInput = ref(null)
const findInput = ref(null)
let burst = false
let composing = false
let savedRange = null

const SIZES = [8, 9, 10, 11, 12, 14, 16, 18, 20, 24, 28, 36, 48, 72]
const ZOOMS = [50, 75, 100, 125, 150, 200]
const FONTS = [
  { id: '', label: 'Arial' },
  { id: 'serif', label: '宋体' },
  { id: 'sans', label: '黑体' },
  { id: 'mono', label: '等宽' },
]

const headings = computed(() => props.modelValue.blocks.flatMap((block, index) => {
  if (!['heading1', 'heading2', 'heading3'].includes(block.type)) return []
  const text = plainText(block.runs).trim()
  if (!text) return []
  return [{ index, text, level: Number(block.type.slice(-1)) }]
}))
const pages = computed(() => {
  const groups = [{ index: 0 }]
  props.modelValue.blocks.forEach((block, index) => {
    if (block.type === 'break' && index > 0) groups.push({ index })
  })
  return groups
})
const blank = computed(() => props.modelValue.blocks.every((block) => block.type !== 'break' && block.type !== 'image' && !plainText(block.runs).trim()))
const hits = computed(() => findHits(props.modelValue.blocks, findQuery.value))
const stats = computed(() => {
  const texts = props.modelValue.blocks.map((block) => plainText(block.runs))
  const all = texts.join('')
  return {
    chars: [...all.replace(/\s/g, '')].length,
    withSpace: [...all].length,
    paragraphs: texts.filter((text) => text.trim()).length,
  }
})
const caretStyle = computed(() => {
  ui.value
  const block = props.modelValue.blocks[active.value]
  if (!block) return styleOf({})
  if (typingStyle.value) return typingStyle.value
  const range = rangeOf(active.value)
  return styleAt(block.runs, range.start)
})
const fontSizeLabel = computed(() => caretStyle.value.size || defaultSize(props.modelValue.blocks[active.value]))

function defaultSize(block) {
  if (block?.type === 'heading1') return 20
  if (block?.type === 'heading2') return 16
  if (block?.type === 'heading3') return 14
  return 11
}
function emptyRuns() {
  return [{ text: '', bold: false, italic: false, underline: false }]
}
function tidyBlock(block) {
  if (!block.indent) delete block.indent
  if (!block.spacing) delete block.spacing
}
function touch() {
  props.modelValue.blocks.forEach(tidyBlock)
  emit('update:modelValue', props.modelValue)
}
function snapshot() {
  return JSON.stringify(props.modelValue.blocks)
}
function checkpoint() {
  const snap = snapshot()
  if (past.value[past.value.length - 1] === snap) return
  past.value.push(snap)
  if (past.value.length > 80) past.value.shift()
  future.value = []
}
function restore(json) {
  props.modelValue.blocks.splice(0, props.modelValue.blocks.length, ...JSON.parse(json))
  typingStyle.value = null
  burst = false
  touch()
  nextTick(refreshAll)
}
function undo() {
  if (props.readonly || !past.value.length) return
  future.value.push(snapshot())
  restore(past.value.pop())
}
function redo() {
  if (props.readonly || !future.value.length) return
  past.value.push(snapshot())
  restore(future.value.pop())
}
function fontFamily(font) {
  if (font === 'serif') return '"Songti SC", "STSong", serif'
  if (font === 'sans') return '"PingFang SC", "Noto Sans SC", "Microsoft YaHei", sans-serif'
  if (font === 'mono') return 'ui-monospace, "Courier New", monospace'
  return 'Arial, "PingFang SC", "Noto Sans SC", sans-serif'
}
function renderRuns(element, runs) {
  const source = runs?.length ? runs : emptyRuns()
  element.replaceChildren()
  if (source.length === 1 && !source[0].text) {
    element.append(document.createElement('br'))
    return
  }
  for (const run of source) {
    const style = styleOf(run)
    const plain = !style.bold && !style.italic && !style.underline && !style.strike && !style.color && !style.highlight && !style.size && !style.font
    if (plain) {
      element.append(document.createTextNode(run.text))
      continue
    }
    const span = document.createElement('span')
    span.textContent = run.text
    if (style.bold) span.style.fontWeight = '700'
    if (style.italic) span.style.fontStyle = 'italic'
    const deco = [style.underline ? 'underline' : '', style.strike ? 'line-through' : ''].filter(Boolean).join(' ')
    if (deco) span.style.textDecoration = deco
    if (style.color) span.style.color = style.color
    if (style.highlight) span.style.backgroundColor = style.highlight
    if (style.size) span.style.fontSize = `${style.size}pt`
    if (style.font) span.style.fontFamily = fontFamily(style.font)
    element.append(span)
  }
}
function setElement(index, element) {
  elements.value[index] = element
  if (!element || element.dataset.ready === '1') return
  renderRuns(element, props.modelValue.blocks[index].runs)
  element.dataset.ready = '1'
}
function refreshAll() {
  nextTick(() => {
    props.modelValue.blocks.forEach((block, index) => {
      const element = elements.value[index]
      if (!element || block.type === 'break' || block.type === 'image') return
      renderRuns(element, block.runs)
      element.dataset.ready = '1'
    })
  })
}
function focusPage(event) {
  if (props.readonly || event.target.closest('.word-block, .word-suggest, button')) return
  const index = Math.max(0, props.modelValue.blocks.findIndex((block) => block.type !== 'break' && block.type !== 'image'))
  focusBlock(index)
}
function focusBlock(index) {
  active.value = index
  typingStyle.value = null
  ui.value += 1
  nextTick(() => {
    const element = elements.value[index]
    if (!element) return
    element.focus()
    const end = plainText(props.modelValue.blocks[index].runs).length
    restoreSelection(element, end, end)
  })
}
function selectionRange(element) {
  const selection = window.getSelection()
  if (!selection?.rangeCount || !element.contains(selection.anchorNode)) return null
  const range = selection.getRangeAt(0)
  const start = offsetOf(element, range.startContainer, range.startOffset)
  const end = offsetOf(element, range.endContainer, range.endOffset)
  return { start: Math.min(start, end), end: Math.max(start, end) }
}
function offsetOf(root, node, offset) {
  const range = document.createRange()
  range.selectNodeContents(root)
  range.setEnd(node, offset)
  return range.toString().length
}
function restoreSelection(element, start, end) {
  const selection = window.getSelection()
  if (!selection) return
  const range = document.createRange()
  const walker = document.createTreeWalker(element, NodeFilter.SHOW_TEXT)
  let remainStart = start
  let remainEnd = end
  let startNode = null
  let endNode = null
  let startOffset = 0
  let endOffset = 0
  let current = walker.nextNode()
  while (current) {
    const size = current.nodeValue.length
    if (!startNode && remainStart <= size) {
      startNode = current
      startOffset = remainStart
    }
    if (!endNode && remainEnd <= size) {
      endNode = current
      endOffset = remainEnd
      break
    }
    remainStart -= size
    remainEnd -= size
    current = walker.nextNode()
  }
  if (!startNode) {
    element.focus()
    return
  }
  range.setStart(startNode, startOffset)
  range.setEnd(endNode || startNode, endNode ? endOffset : startOffset)
  selection.removeAllRanges()
  selection.addRange(range)
}
function rangeOf(index = active.value) {
  const element = elements.value[index]
  const live = element ? selectionRange(element) : null
  if (live) return live
  if (savedRange?.index === index) return savedRange
  const end = plainText(props.modelValue.blocks[index]?.runs).length
  return { start: end, end }
}
function rememberSelection() {
  const live = elements.value[active.value] ? selectionRange(elements.value[active.value]) : null
  if (live) savedRange = { ...live, index: active.value }
}
function styleAt(runs, offset) {
  const text = plainText(runs)
  if (!text) return styleOf({})
  const at = Math.max(0, Math.min(offset, text.length - 1))
  return styleOf(sliceRuns(runs, at, at + 1)[0] || {})
}
function paint(index, caret) {
  const element = elements.value[index]
  const block = props.modelValue.blocks[index]
  if (!element || !block) return
  renderRuns(element, block.runs)
  element.dataset.ready = '1'
  element.focus()
  if (caret != null) restoreSelection(element, caret, caret)
}
function applyPatch(patch) {
  if (props.readonly) return
  const block = props.modelValue.blocks[active.value]
  if (!block || block.type === 'break' || block.type === 'image') return
  const range = rangeOf()
  if (range.start !== range.end) {
    checkpoint()
    burst = false
    block.runs = applyStyle(block.runs, range.start, range.end, patch)
    typingStyle.value = null
    touch()
    nextTick(() => {
      paint(active.value, null)
      const element = elements.value[active.value]
      element?.focus()
      if (element) restoreSelection(element, range.start, range.end)
      ui.value += 1
    })
    return
  }
  typingStyle.value = { ...(typingStyle.value || styleAt(block.runs, range.start)), ...patch }
  ui.value += 1
}
function toggle(mark) {
  const block = props.modelValue.blocks[active.value]
  if (!block || props.readonly) return
  const range = rangeOf()
  if (range.start !== range.end) {
    const selected = sliceRuns(block.runs, range.start, range.end)
    applyPatch({ [mark]: !selected.every((run) => run[mark]) })
    return
  }
  const base = typingStyle.value || styleAt(block.runs, range.start)
  typingStyle.value = { ...base, [mark]: !base[mark] }
  ui.value += 1
}
function markOn(mark) {
  return !!caretStyle.value[mark]
}
function setType(type) {
  const block = props.modelValue.blocks[active.value]
  if (!block || props.readonly || block.type === 'break' || block.type === 'image') return
  checkpoint()
  block.type = type
  touch()
  menu.value = ''
  ui.value += 1
}
function setAlign(value) {
  const block = props.modelValue.blocks[active.value]
  if (!block || props.readonly || block.type === 'break' || block.type === 'image') return
  checkpoint()
  block.align = value
  touch()
  menu.value = ''
}
function setSpacing(value) {
  const block = props.modelValue.blocks[active.value]
  if (!block || props.readonly || block.type === 'break' || block.type === 'image') return
  checkpoint()
  if (value) block.spacing = value
  else delete block.spacing
  touch()
  menu.value = ''
}
function setFont(font) {
  applyPatch({ font })
  menu.value = ''
}
function setSize(size) {
  const next = Math.max(8, Math.min(72, Number(size) || 11))
  const block = props.modelValue.blocks[active.value]
  applyPatch({ size: next === defaultSize(block) ? 0 : next })
}
function stepSize(dir) {
  const current = fontSizeLabel.value
  const next = dir > 0
    ? (SIZES.find((size) => size > current) || SIZES[SIZES.length - 1])
    : ([...SIZES].reverse().find((size) => size < current) || SIZES[0])
  setSize(next)
}
function styleValue(type) {
  return ['heading1', 'heading2', 'heading3', 'quote'].includes(type) ? type : 'paragraph'
}
function printDoc() {
  window.print()
}
function changeIndent(delta) {
  const block = props.modelValue.blocks[active.value]
  if (!block || props.readonly || block.type === 'break' || block.type === 'image') return
  checkpoint()
  const indent = Math.max(0, Math.min(8, (block.indent || 0) + delta))
  if (indent) block.indent = indent
  else delete block.indent
  touch()
}
function clearFormat() {
  const block = props.modelValue.blocks[active.value]
  if (!block || props.readonly || block.type === 'break' || block.type === 'image') return
  const range = rangeOf()
  checkpoint()
  if (range.start !== range.end) {
    block.runs = applyStyle(block.runs, range.start, range.end, {
      bold: false, italic: false, underline: false, strike: false, color: '', highlight: '', size: 0, font: '',
    })
  } else {
    block.type = 'paragraph'
    block.align = 'left'
    delete block.indent
    delete block.spacing
    block.runs = clearRuns(block.runs)
    typingStyle.value = null
  }
  touch()
  nextTick(() => paint(active.value, range.start))
}
function onBeforeInput(index, event) {
  if (props.readonly) return
  if (event.inputType === 'historyUndo') {
    event.preventDefault()
    undo()
    return
  }
  if (event.inputType === 'historyRedo') {
    event.preventDefault()
    redo()
    return
  }
  if (event.inputType === 'insertLineBreak') {
    event.preventDefault()
    checkpoint()
    splitBlock(index)
    return
  }
  if (!burst && !String(event.inputType || '').includes('Composition')) {
    checkpoint()
    burst = true
  }
}
function onInput(index, event) {
  if (props.readonly || composing || event?.isComposing) return
  const block = props.modelValue.blocks[index]
  const previous = plainText(block.runs)
  const segments = segmentsFromElement(event.target)
  if (segments.length > 1) {
    burst = false
    typingStyle.value = null
    replaceWithSegments(index, segments)
    return
  }
  const nextText = plainText(segments[0])
  if (typingStyle.value && nextText !== previous) {
    const diff = textDiff(previous, nextText)
    const inserted = nextText.slice(diff.start, diff.start + diff.insert)
    block.runs = mergeRuns([
      ...sliceRuns(block.runs, 0, diff.start),
      ...(inserted ? [{ text: inserted, ...typingStyle.value }] : []),
      ...sliceRuns(block.runs, diff.start + diff.removed, previous.length),
    ])
    touch()
    const caret = diff.start + inserted.length
    nextTick(() => paint(index, caret))
    return
  }
  block.runs = segments[0] || emptyRuns()
  touch()
  ui.value += 1
}
function onPaste(index, event) {
  if (props.readonly) return
  const lines = clipboardToLines(event.clipboardData?.getData('text/plain'), event.clipboardData?.getData('text/html'))
  if (lines.length === 1 && lines[0] === '') return
  event.preventDefault()
  if (!burst) checkpoint()
  burst = false
  insertLines(index, lines)
}
function replaceWithSegments(index, segments, caret = null) {
  const block = props.modelValue.blocks[index]
  const room = maxWordBlocks - props.modelValue.blocks.length + 1
  const kept = segments.slice(0, Math.max(1, room))
  if (kept.length < segments.length) {
    const extra = segments.slice(kept.length).map((runs) => plainText(runs)).filter(Boolean).join(' ')
    if (extra) kept[kept.length - 1].push({ text: extra, bold: false, italic: false, underline: false })
  }
  block.runs = mergeRuns(kept[0])
  const created = kept.slice(1).map((runs) => ({
    id: newId(),
    type: 'paragraph',
    align: block.align,
    runs: mergeRuns(runs),
  }))
  if (created.length) props.modelValue.blocks.splice(index + 1, 0, ...created)
  touch()
  nextTick(() => {
    for (let offset = 0; offset < kept.length; offset += 1) {
      const element = elements.value[index + offset]
      const current = props.modelValue.blocks[index + offset]
      if (!element || !current) continue
      renderRuns(element, current.runs)
      element.dataset.ready = '1'
    }
    const last = elements.value[index + kept.length - 1]
    if (!last) return
    last.focus()
    active.value = index + kept.length - 1
    if (caret != null) restoreSelection(last, caret, caret)
  })
}
function insertLines(index, lines) {
  const element = elements.value[index]
  const block = props.modelValue.blocks[index]
  const selected = element ? selectionRange(element) : null
  const text = plainText(block.runs)
  const start = selected ? selected.start : text.length
  const end = selected ? selected.end : text.length
  const before = sliceRuns(block.runs, 0, start)
  const after = sliceRuns(block.runs, end, text.length)
  const first = mergeRuns([...before, { text: lines[0], ...(typingStyle.value || {}) }])
  if (lines.length === 1) {
    block.runs = mergeRuns([...first, ...after])
    touch()
    nextTick(() => paint(index, plainText(before).length + lines[0].length))
    return
  }
  const middle = lines.slice(1, -1).map((line) => (line ? [{ text: line, bold: false, italic: false, underline: false }] : emptyRuns()))
  const last = mergeRuns([{ text: lines[lines.length - 1], bold: false, italic: false, underline: false }, ...after])
  replaceWithSegments(index, [first.length ? first : emptyRuns(), ...middle, last.length ? last : emptyRuns()], lines[lines.length - 1].length)
}
function splitBlock(index) {
  if (props.modelValue.blocks.length >= maxWordBlocks) return
  const element = elements.value[index]
  const block = props.modelValue.blocks[index]
  if (!block || block.type === 'break' || block.type === 'image') return
  const selected = element ? selectionRange(element) : null
  const text = plainText(block.runs)
  const offset = selected ? selected.start : text.length
  const list = block.type === 'bullet' || block.type === 'numbered'
  if (list && text.length === 0) {
    if ((block.indent || 0) > 0) block.indent -= 1
    else {
      block.type = 'paragraph'
      delete block.indent
    }
    touch()
    ui.value += 1
    return
  }
  const left = sliceRuns(block.runs, 0, offset)
  const right = sliceRuns(block.runs, offset, text.length)
  block.runs = left.length ? left : emptyRuns()
  const heading = ['heading1', 'heading2', 'heading3', 'quote'].includes(block.type)
  const created = {
    id: newId(),
    type: list ? block.type : 'paragraph',
    align: heading ? 'left' : block.align,
    runs: right.length ? right : emptyRuns(),
  }
  if (list && block.indent) created.indent = block.indent
  if (!heading && block.spacing) created.spacing = block.spacing
  props.modelValue.blocks.splice(index + 1, 0, created)
  touch()
  nextTick(() => {
    ;[index, index + 1].forEach((at) => {
      const node = elements.value[at]
      const current = props.modelValue.blocks[at]
      if (!node || !current) return
      renderRuns(node, current.runs)
      node.dataset.ready = '1'
    })
    const next = elements.value[index + 1]
    next?.focus()
    active.value = index + 1
  })
}
function mergeBackward(index) {
  const result = collapseBackward(props.modelValue.blocks, index)
  if (!result) return
  touch()
  nextTick(() => paint(result.index, result.offset))
  active.value = result.index
}
function onImageKey(index, event) {
  if (props.readonly) return
  if (event.key === 'Backspace' || event.key === 'Delete') {
    event.preventDefault()
    checkpoint()
    if (props.modelValue.blocks.length <= 1) {
      props.modelValue.blocks.splice(0, 1, { id: newId(), type: 'paragraph', align: 'left', runs: emptyRuns() })
    } else {
      props.modelValue.blocks.splice(index, 1)
    }
    touch()
    return
  }
  if (event.key === 'Enter' && props.modelValue.blocks.length < maxWordBlocks) {
    event.preventDefault()
    checkpoint()
    props.modelValue.blocks.splice(index + 1, 0, { id: newId(), type: 'paragraph', align: 'left', runs: emptyRuns() })
    touch()
    nextTick(() => focusBlock(index + 1))
  }
}
function onKey(index, event) {
  if (props.readonly) return
  const key = event.key.toLowerCase()
  const command = event.metaKey || event.ctrlKey
  if (command && key === 'b') {
    event.preventDefault()
    toggle('bold')
    return
  }
  if (command && key === 'i') {
    event.preventDefault()
    toggle('italic')
    return
  }
  if (command && key === 'u') {
    event.preventDefault()
    toggle('underline')
    return
  }
  if (command && key === 'z' && !event.shiftKey) {
    event.preventDefault()
    undo()
    return
  }
  if (command && (key === 'y' || (key === 'z' && event.shiftKey))) {
    event.preventDefault()
    redo()
    return
  }
  if (command && (key === 'f' || key === 'h')) {
    event.preventDefault()
    openFind()
    return
  }
  if (command && key === 'p') {
    event.preventDefault()
    window.print()
    return
  }
  if (command && event.altKey && ['1', '2', '3'].includes(event.key)) {
    event.preventDefault()
    setType(event.key === '1' ? 'heading1' : event.key === '2' ? 'heading2' : 'heading3')
    return
  }
  if (event.key === 'Tab') {
    event.preventDefault()
    changeIndent(event.shiftKey ? -1 : 1)
    return
  }
  if (event.key === 'Enter' && !event.shiftKey) {
    event.preventDefault()
    checkpoint()
    burst = false
    splitBlock(index)
    return
  }
  if (event.key === 'Backspace') {
    const selected = selectionRange(elements.value[index])
    const block = props.modelValue.blocks[index]
    if (!selected || selected.start !== 0 || selected.end !== 0) return
    const list = block.type === 'bullet' || block.type === 'numbered'
    if (list && plainText(block.runs) === '') {
      event.preventDefault()
      checkpoint()
      if ((block.indent || 0) > 0) block.indent -= 1
      else {
        block.type = 'paragraph'
        delete block.indent
      }
      touch()
      return
    }
    if ((block.indent || 0) > 0) {
      event.preventDefault()
      changeIndent(-1)
      return
    }
    if (index > 0) {
      event.preventDefault()
      checkpoint()
      mergeBackward(index)
    }
  }
}
function insertBreak() {
  if (props.readonly || props.modelValue.blocks.length >= maxWordBlocks) return
  checkpoint()
  props.modelValue.blocks.splice(active.value + 1, 0, { id: newId(), type: 'break', align: 'left', runs: emptyRuns() })
  touch()
  menu.value = ''
}
function insertToday() {
  const block = props.modelValue.blocks[active.value]
  if (!block || props.readonly || block.type === 'break' || block.type === 'image') return
  const range = rangeOf()
  const text = new Intl.DateTimeFormat('zh-CN', { year: 'numeric', month: 'long', day: 'numeric' }).format(new Date())
  checkpoint()
  block.runs = mergeRuns([
    ...sliceRuns(block.runs, 0, range.start),
    { text, ...(typingStyle.value || styleAt(block.runs, range.start)) },
    ...sliceRuns(block.runs, range.end, plainText(block.runs).length),
  ])
  touch()
  menu.value = ''
  nextTick(() => paint(active.value, range.start + text.length))
}
function blockFrom(text, type = 'paragraph', extra = {}) {
  return { id: newId(), type, align: 'left', runs: text ? [{ text, bold: false, italic: false, underline: false }] : emptyRuns(), ...extra }
}
function applyTemplate(kind) {
  if (props.readonly) return
  const today = new Intl.DateTimeFormat('zh-CN', { year: 'numeric', month: 'long', day: 'numeric' }).format(new Date())
  const sets = {
    meeting: [
      blockFrom('会议记录', 'heading1'),
      blockFrom(`时间：${today}`),
      blockFrom('参会人：'),
      blockFrom('议题', 'heading2'),
      blockFrom('待讨论事项', 'bullet'),
      blockFrom('结论', 'heading2'),
      blockFrom('形成的决定', 'bullet'),
    ],
    mail: [
      blockFrom('收件人：'),
      blockFrom('主题：'),
      blockFrom(''),
      blockFrom('你好，'),
      blockFrom(''),
      blockFrom('此致'),
      blockFrom('敬礼'),
    ],
    weekly: [
      blockFrom('周报', 'heading1'),
      blockFrom('本周完成', 'heading2'),
      blockFrom('完成的事项', 'bullet'),
      blockFrom('下周计划', 'heading2'),
      blockFrom('计划推进的事项', 'bullet'),
    ],
    plan: [
      blockFrom('项目计划', 'heading1'),
      blockFrom('目标', 'heading2'),
      blockFrom('要达成的结果'),
      blockFrom('步骤', 'heading2'),
      blockFrom('第一步', 'numbered'),
      blockFrom('第二步', 'numbered'),
    ],
  }
  const blocks = sets[kind]
  if (!blocks) return
  checkpoint()
  if (blank.value) props.modelValue.blocks.splice(0, props.modelValue.blocks.length, ...blocks)
  else props.modelValue.blocks.splice(active.value + 1, 0, ...blocks)
  moreOpen.value = false
  touch()
  nextTick(() => refreshAll())
}
function openFind() {
  findOpen.value = true
  menu.value = ''
  nextTick(() => findInput.value?.focus())
}
function showHit(hit) {
  if (!hit) return
  active.value = hit.block
  nextTick(() => {
    const element = elements.value[hit.block]
    if (!element) return
    element.focus()
    restoreSelection(element, hit.start, hit.end)
  })
}
function findNext(step = 1) {
  const list = hits.value
  if (!list.length) {
    findAt.value = -1
    return
  }
  if (step === 0 || findAt.value < 0) findAt.value = clampFindIndex(findAt.value, list.length)
  else findAt.value = (findAt.value + step + list.length) % list.length
  showHit(list[findAt.value])
}
function replaceCurrent() {
  const list = hits.value
  if (!list.length || props.readonly) return
  const index = clampFindIndex(findAt.value, list.length)
  findAt.value = index
  const hit = list[index]
  checkpoint()
  const block = props.modelValue.blocks[hit.block]
  block.runs = replaceOnce(block.runs, hit.start, hit.end, replaceQuery.value)
  touch()
  nextTick(() => {
    refreshAll()
    findNext(0)
  })
}
function replaceAll() {
  const list = hits.value
  if (!list.length || props.readonly) return
  checkpoint()
  for (let index = list.length - 1; index >= 0; index -= 1) {
    const hit = list[index]
    const block = props.modelValue.blocks[hit.block]
    block.runs = replaceOnce(block.runs, hit.start, hit.end, replaceQuery.value)
  }
  touch()
  findAt.value = -1
  nextTick(refreshAll)
}
function toggleMenu(name) {
  menu.value = menu.value === name ? '' : name
  moreOpen.value = false
}
function onDocClick(event) {
  if (!event.target.closest('.word-menu, .word-more, .suggest-more')) {
    menu.value = ''
    moreOpen.value = false
  }
}
function chooseZoom(value) {
  zoom.value = value / 100
  menu.value = ''
}
function activePage(index) {
  let page = 0
  props.modelValue.blocks.forEach((block, at) => {
    if (block.type === 'break' && at > 0 && at <= index) page += 1
  })
  return page
}

onMounted(() => {
  refreshAll()
  document.addEventListener('click', onDocClick)
  if (!props.readonly) nextTick(() => elements.value[0]?.focus())
})
onBeforeUnmount(() => document.removeEventListener('click', onDocClick))
watch(() => props.modelValue, refreshAll)
</script>

<template>
  <div class="word-editor" data-testid="word-editor" @mouseup="ui++" @keyup="ui++">
    <div class="word-menu" v-if="!readonly">
      <div class="menu-item" v-for="item in [
        ['file', '文件'], ['edit', '编辑'], ['view', '查看'], ['insert', '插入'], ['format', '格式'], ['tools', '工具'], ['help', '帮助'],
      ]" :key="item[0]">
        <button type="button" :aria-expanded="menu === item[0]" @click="toggleMenu(item[0])">{{ item[1] }}</button>
        <div class="menu-pop" v-if="menu === item[0]">
          <template v-if="item[0] === 'file'">
            <button type="button" @click="emit('save'); menu = ''">保存</button>
            <button type="button" @click="emit('versions'); menu = ''">版本历史</button>
            <button type="button" @click="printDoc(); menu = ''">打印</button>
          </template>
          <template v-else-if="item[0] === 'edit'">
            <button type="button" @click="undo(); menu = ''">撤销</button>
            <button type="button" @click="redo(); menu = ''">重做</button>
            <button type="button" @click="openFind">查找和替换</button>
          </template>
          <template v-else-if="item[0] === 'view'">
            <button type="button" @click="showNav = !showNav; menu = ''">{{ showNav ? '隐藏导航' : '显示导航' }}</button>
            <button type="button" v-for="value in ZOOMS" :key="value" @click="chooseZoom(value)">{{ value }}%</button>
          </template>
          <template v-else-if="item[0] === 'insert'">
            <button type="button" @click="insertBreak">分页符</button>
            <button type="button" @click="insertToday">日期</button>
          </template>
          <template v-else-if="item[0] === 'format'">
            <button type="button" @click="setType('paragraph')">普通文本</button>
            <button type="button" @click="setType('heading1')">标题 1</button>
            <button type="button" @click="setType('heading2')">标题 2</button>
            <button type="button" @click="setType('heading3')">标题 3</button>
            <button type="button" @click="setType('quote')">引用</button>
            <button type="button" @click="setAlign('left')">左对齐</button>
            <button type="button" @click="setAlign('center')">居中</button>
            <button type="button" @click="setAlign('right')">右对齐</button>
            <button type="button" @click="setAlign('justify')">两端对齐</button>
          </template>
          <template v-else-if="item[0] === 'tools'">
            <p>字数 {{ stats.chars }}</p>
            <p>字符数（计空格）{{ stats.withSpace }}</p>
            <p>段落 {{ stats.paragraphs }}</p>
          </template>
          <template v-else>
            <p>Ctrl+B 加粗，Ctrl+I 斜体，Ctrl+U 下划线</p>
            <p>Ctrl+Z 撤销，Ctrl+Y 重做，Ctrl+F 查找</p>
            <p>Tab 增加缩进，空列表回车退出列表</p>
          </template>
        </div>
      </div>
    </div>

    <div class="word-ribbon" v-if="!readonly" @pointerdown="rememberSelection">
      <button type="button" title="查找" data-testid="find-button" @click="openFind">
        <svg viewBox="0 0 24 24"><circle cx="10" cy="10" r="6" /><path d="M15 15l5 5" /></svg>
      </button>
      <button type="button" title="撤销" data-testid="undo" @mousedown.prevent="undo">
        <svg viewBox="0 0 24 24"><path d="M8 8H4v4" /><path d="M4 8a8 8 0 1 1-1 5" /></svg>
      </button>
      <button type="button" title="重做" data-testid="redo" @mousedown.prevent="redo">
        <svg viewBox="0 0 24 24"><path d="M16 8h4v4" /><path d="M20 8a8 8 0 1 0 1 5" /></svg>
      </button>
      <button type="button" title="打印" @mousedown.prevent="printDoc">
        <svg viewBox="0 0 24 24"><path d="M7 8V4h10v4" /><path d="M6 16H4a2 2 0 0 1-2-2v-4a2 2 0 0 1 2-2h16a2 2 0 0 1 2 2v4a2 2 0 0 1-2 2h-2" /><path d="M7 14h10v6H7z" /></svg>
      </button>
      <div class="menu-item">
        <button type="button" class="zoom-label" @click="toggleMenu('zoom')">{{ Math.round(zoom * 100) }}%</button>
        <div class="menu-pop" v-if="menu === 'zoom'">
          <button type="button" v-for="value in ZOOMS" :key="value" @click="chooseZoom(value)">{{ value }}%</button>
        </div>
      </div>
      <i class="ribbon-sep"></i>
      <select class="ribbon-select style-select" data-testid="style-select" :value="styleValue(modelValue.blocks[active]?.type)" @change="setType($event.target.value)">
        <option value="paragraph">普通文本</option>
        <option value="heading1">标题 1</option>
        <option value="heading2">标题 2</option>
        <option value="heading3">标题 3</option>
        <option value="quote">引用</option>
      </select>
      <select class="ribbon-select font-select" data-testid="font-select" :value="caretStyle.font || ''" @change="setFont($event.target.value)">
        <option v-for="item in FONTS" :key="item.label" :value="item.id">{{ item.label }}</option>
      </select>
      <button type="button" title="减小字号" @mousedown.prevent="stepSize(-1)">−</button>
      <input class="size-input" data-testid="font-size" :value="fontSizeLabel" @change="setSize($event.target.value)" />
      <button type="button" title="增大字号" @mousedown.prevent="stepSize(1)">+</button>
      <i class="ribbon-sep"></i>
      <button type="button" class="fmt b" data-testid="bold" title="加粗" :aria-pressed="markOn('bold')" @mousedown.prevent="toggle('bold')">B</button>
      <button type="button" class="fmt i" title="斜体" :aria-pressed="markOn('italic')" @mousedown.prevent="toggle('italic')">I</button>
      <button type="button" class="fmt u" title="下划线" :aria-pressed="markOn('underline')" @mousedown.prevent="toggle('underline')">U</button>
      <button type="button" class="fmt color-letter" title="字体颜色" @mousedown.prevent="colorInput?.click()">
        A<i :style="{ background: caretStyle.color || '#1a73e8' }"></i>
      </button>
      <input ref="colorInput" class="sr-color" type="color" data-testid="font-color" @input="applyPatch({ color: $event.target.value.toLowerCase() })" />
      <button type="button" class="fmt marker" title="突出显示" @mousedown.prevent="highlightInput?.click()">
        <svg viewBox="0 0 24 24"><path d="M4 16l8-8 4 4-8 8H4z" /><path d="M13 7l4 4" /></svg>
        <i :style="{ background: caretStyle.highlight || '#fff2a8' }"></i>
      </button>
      <input ref="highlightInput" class="sr-color" type="color" value="#fff2a8" data-testid="highlight-color" @input="applyPatch({ highlight: $event.target.value.toLowerCase() })" />
      <button type="button" title="清除格式" @mousedown.prevent="clearFormat">
        <svg viewBox="0 0 24 24"><path d="M4 20h8" /><path d="M14 5l5 5" /><path d="M5 14L14 5l4 4-9 9H5z" /></svg>
      </button>
      <i class="ribbon-sep"></i>
      <button type="button" title="左对齐" :aria-pressed="(modelValue.blocks[active]?.align || 'left') === 'left'" @mousedown.prevent="setAlign('left')">
        <svg viewBox="0 0 24 24"><path d="M4 6h16M4 10h10M4 14h16M4 18h10" /></svg>
      </button>
      <button type="button" title="居中" :aria-pressed="modelValue.blocks[active]?.align === 'center'" @mousedown.prevent="setAlign('center')">
        <svg viewBox="0 0 24 24"><path d="M4 6h16M7 10h10M4 14h16M7 18h10" /></svg>
      </button>
      <button type="button" title="右对齐" :aria-pressed="modelValue.blocks[active]?.align === 'right'" @mousedown.prevent="setAlign('right')">
        <svg viewBox="0 0 24 24"><path d="M4 6h16M10 10h10M4 14h16M10 18h10" /></svg>
      </button>
      <button type="button" title="两端对齐" data-testid="align-justify" :aria-pressed="modelValue.blocks[active]?.align === 'justify'" @mousedown.prevent="setAlign('justify')">
        <svg viewBox="0 0 24 24"><path d="M4 6h16M4 10h16M4 14h16M4 18h16" /></svg>
      </button>
      <select class="ribbon-select spacing-select" title="行距" data-testid="line-spacing" :value="modelValue.blocks[active]?.spacing || ''" @change="setSpacing($event.target.value)">
        <option value="">单倍</option>
        <option value="1.5">1.5 倍</option>
        <option value="2">双倍</option>
      </select>
      <button type="button" title="项目符号" data-testid="bullet" :aria-pressed="modelValue.blocks[active]?.type === 'bullet'" @mousedown.prevent="setType('bullet')">
        <svg viewBox="0 0 24 24"><circle cx="6" cy="7" r="1.4" /><circle cx="6" cy="12" r="1.4" /><circle cx="6" cy="17" r="1.4" /><path d="M10 7h9M10 12h9M10 17h9" /></svg>
      </button>
      <button type="button" title="编号" data-testid="numbered" :aria-pressed="modelValue.blocks[active]?.type === 'numbered'" @mousedown.prevent="setType('numbered')">
        <svg viewBox="0 0 24 24"><path d="M5 7h2M5 12h2M5 17h2M10 7h9M10 12h9M10 17h9" /></svg>
      </button>
      <button type="button" title="减少缩进" @mousedown.prevent="changeIndent(-1)">
        <svg viewBox="0 0 24 24"><path d="M4 6h16M10 10h10M10 14h10M4 18h16M8 12H4m0 0l2-2m-2 2l2 2" /></svg>
      </button>
      <button type="button" title="增加缩进" data-testid="indent" @mousedown.prevent="changeIndent(1)">
        <svg viewBox="0 0 24 24"><path d="M4 6h16M10 10h10M10 14h10M4 18h16M4 12h4m0 0l-2-2m2 2l-2 2" /></svg>
      </button>
      <button type="button" class="fmt s" title="删除线" :aria-pressed="markOn('strike')" @mousedown.prevent="toggle('strike')">S</button>
    </div>

    <div class="word-find" v-if="findOpen">
      <input ref="findInput" data-testid="find-input" v-model="findQuery" placeholder="查找" @keydown.enter.prevent="findNext(1)" />
      <span>{{ hits.length ? `${Math.max(findAt, 0) + 1}/${hits.length}` : '无匹配' }}</span>
      <button type="button" data-testid="find-next" @click="findNext(1)">下一个</button>
      <input data-testid="replace-input" v-model="replaceQuery" placeholder="替换为" :disabled="readonly" />
      <button type="button" data-testid="replace-one" :disabled="readonly" @click="replaceCurrent">替换</button>
      <button type="button" data-testid="replace-all" :disabled="readonly" @click="replaceAll">全部替换</button>
      <button type="button" @click="findOpen = false">关闭</button>
    </div>

    <div class="word-workspace">
      <aside class="word-nav" data-testid="word-nav" v-show="showNav">
        <div class="word-nav-head">
          <button type="button" title="收起导航" @click="showNav = false">‹</button>
          <strong>文档分页</strong>
          <button type="button" title="新增分页" data-testid="add-page" :disabled="readonly" @click="insertBreak">+</button>
        </div>
        <div v-for="(page, pageNo) in pages" :key="page.index" class="word-page-group">
          <button type="button" class="word-tab" :class="{ active: activePage(active) === pageNo }" @click="focusBlock(page.index)">
            <svg viewBox="0 0 24 24"><path d="M6 3h8l4 4v14H6z" /><path d="M14 3v5h5" /></svg>
            <span>标签页 {{ pageNo + 1 }}</span>
          </button>
          <button
            v-for="item in headings.filter((heading) => heading.index >= page.index && (pageNo === pages.length - 1 || heading.index < pages[pageNo + 1].index))"
            :key="item.index"
            type="button"
            class="word-outline"
            :class="{ active: active === item.index }"
            :data-level="item.level"
            @click="focusBlock(item.index)"
          >{{ item.text }}</button>
        </div>
        <p class="word-nav-hint" v-if="!headings.length">您添加到文档的标题会显示在此处。</p>
      </aside>
      <button v-if="!showNav" type="button" class="nav-reopen" @click="showNav = true">文档分页</button>
      <div class="word-canvas">
        <div class="word-spacer" :style="{ width: `${794 * zoom}px`, height: `${1180 * zoom}px` }">
          <div class="word-sheet" :style="{ transform: `scale(${zoom})` }">
            <div class="word-ruler" data-testid="word-ruler" aria-hidden="true">
              <span v-for="n in 20" :key="n" class="cm" :style="{ left: `${n * 37.8}px` }">{{ n }}</span>
              <i class="ruler-margin left"></i>
              <i class="ruler-margin right"></i>
            </div>
            <article class="word-page" data-testid="word-page" @mousedown="focusPage">
              <template v-for="(block, index) in modelValue.blocks" :key="block.id">
                <div v-if="block.type === 'break'" class="page-break-line"><span>分页符</span></div>
                <div
                  v-else-if="block.type === 'image'"
                  class="word-image"
                  :class="{ active: active === index }"
                  data-testid="pdf-image"
                  tabindex="0"
                  @focus="active = index"
                  @keydown="onImageKey(index, $event)"
                >
                  <img :src="block.image" alt="" />
                </div>
                <div
                  v-else
                  class="word-line"
                  :class="{ 'with-suggest': blank && index === 0 && !readonly }"
                  :data-align="block.align || 'left'"
                  :style="{ '--indent': block.indent || 0 }"
                >
                  <span class="marker" v-if="listMarker(modelValue.blocks, index)">{{ listMarker(modelValue.blocks, index) }}</span>
                  <div
                    class="word-block"
                    :class="block.type"
                    :data-spacing="block.spacing || '1'"
                    :contenteditable="readonly ? 'false' : 'true'"
                    spellcheck="false"
                    data-testid="word-block"
                    :ref="(element) => setElement(index, element)"
                    @focus="active = index"
                    @beforeinput="onBeforeInput(index, $event)"
                    @compositionstart="composing = true"
                    @compositionend="composing = false"
                    @paste="onPaste(index, $event)"
                    @input="onInput(index, $event)"
                    @keydown="onKey(index, $event)"
                  />
                  <div class="word-suggest" v-if="blank && index === 0 && !readonly">
                    <button type="button" data-testid="suggest-meeting" @mousedown.prevent="applyTemplate('meeting')">
                      <svg viewBox="0 0 24 24"><path d="M5 6h14v12H5z" /><path d="M8 3v4M16 3v4M5 10h14" /></svg>
                      会议记录
                    </button>
                    <button type="button" data-testid="suggest-mail" @mousedown.prevent="applyTemplate('mail')">
                      <svg viewBox="0 0 24 24"><path d="M4 6h16v12H4z" /><path d="M4 7l8 6 8-6" /></svg>
                      邮件草稿
                    </button>
                    <div class="suggest-more">
                      <button type="button" data-testid="suggest-more" @mousedown.prevent="moreOpen = !moreOpen">
                        <svg viewBox="0 0 24 24"><path d="M5 7h14M5 12h14M5 17h10" /></svg>
                        更多
                      </button>
                      <div class="menu-pop" v-if="moreOpen">
                        <button type="button" @mousedown.prevent="applyTemplate('weekly')">周报</button>
                        <button type="button" @mousedown.prevent="applyTemplate('plan')">项目计划</button>
                      </div>
                    </div>
                  </div>
                </div>
              </template>
            </article>
          </div>
        </div>
      </div>
    </div>
  </div>
</template>
