<script setup>
import { nextTick, onMounted, ref, watch } from 'vue'
import { applyMark, clipboardToLines, mergeRuns, newId, plainText, segmentsFromElement, sliceRuns } from '../editors/text'

const props = defineProps({
  modelValue: { type: Object, required: true },
  readonly: { type: Boolean, default: false },
})
const emit = defineEmits(['update:modelValue'])

const active = ref(0)
const elements = ref([])
const blockType = ref('paragraph')
const align = ref('left')

function touch() {
  emit('update:modelValue', props.modelValue)
}

function setElement(index, element) {
  if (!element) return
  elements.value[index] = element
  if (element.dataset.ready !== '1') {
    renderRuns(element, props.modelValue.blocks[index].runs)
    element.dataset.ready = '1'
  }
}

function renderRuns(element, runs) {
  const source = runs?.length ? runs : [{ text: '', bold: false, italic: false, underline: false }]
  element.replaceChildren()
  if (source.length === 1 && !source[0].text) {
    element.append(document.createElement('br'))
    return
  }
  for (const run of source) {
    if (run.bold || run.italic || run.underline) {
      const span = document.createElement('span')
      span.textContent = run.text
      if (run.bold) span.style.fontWeight = '700'
      if (run.italic) span.style.fontStyle = 'italic'
      if (run.underline) span.style.textDecoration = 'underline'
      element.append(span)
    } else {
      element.append(document.createTextNode(run.text))
    }
  }
}

function emptyRuns() {
  return [{ text: '', bold: false, italic: false, underline: false }]
}

function onInput(index, event) {
  const segments = segmentsFromElement(event.target)
  if (segments.length <= 1) {
    props.modelValue.blocks[index].runs = segments[0] || emptyRuns()
    touch()
    return
  }
  replaceWithSegments(index, segments)
}

function onPaste(index, event) {
  if (props.readonly) return
  const lines = clipboardToLines(event.clipboardData?.getData('text/plain'), event.clipboardData?.getData('text/html'))
  if (lines.length === 1 && lines[0] === '') return
  event.preventDefault()
  insertLines(index, lines)
}

function onBeforeInput(index, event) {
  if (props.readonly || event.inputType !== 'insertLineBreak') return
  event.preventDefault()
  splitBlock(index)
}

function replaceWithSegments(index, segments, caret = null) {
  const block = props.modelValue.blocks[index]
  const room = 400 - props.modelValue.blocks.length + 1
  const kept = segments.slice(0, Math.max(1, room))
  if (kept.length < segments.length) {
    const extra = segments.slice(kept.length).map((runs) => plainText(runs)).filter(Boolean).join(' ')
    if (extra) {
      const last = kept[kept.length - 1]
      last.splice(0, last.length, ...mergeRuns([...last, { text: extra, bold: false, italic: false, underline: false }]))
    }
  }
  block.runs = kept[0]
  const created = kept.slice(1).map((runs) => ({
    id: newId(),
    type: 'paragraph',
    align: block.align,
    runs,
  }))
  if (created.length) props.modelValue.blocks.splice(index + 1, 0, ...created)
  touch()
  nextTick(() => paintFrom(index, kept.length, caret))
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
  const first = mergeRuns([...before, { text: lines[0], bold: false, italic: false, underline: false }])
  if (lines.length === 1) {
    block.runs = mergeRuns([...first, ...after])
    touch()
    nextTick(() => {
      const current = elements.value[index]
      if (!current) return
      renderRuns(current, block.runs)
      current.dataset.ready = '1'
      current.focus()
      const offset = plainText(before).length + lines[0].length
      restoreSelection(current, offset, offset)
    })
    return
  }
  const middle = lines.slice(1, -1).map((line) => (line ? [{ text: line, bold: false, italic: false, underline: false }] : emptyRuns()))
  const last = mergeRuns([{ text: lines[lines.length - 1], bold: false, italic: false, underline: false }, ...after])
  replaceWithSegments(index, [first.length ? first : emptyRuns(), ...middle, last.length ? last : emptyRuns()], lines[lines.length - 1].length)
}

function focusBlock(index) {
  active.value = index
  const block = props.modelValue.blocks[index]
  blockType.value = block.type
  align.value = block.align
}

function selectionRange(element) {
  const selection = window.getSelection()
  if (!selection || !selection.rangeCount || !element.contains(selection.anchorNode)) return null
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
  const range = document.createRange()
  let remainStart = start
  let remainEnd = end
  const walker = document.createTreeWalker(element, NodeFilter.SHOW_TEXT)
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

function format(mark) {
  if (props.readonly) return
  const element = elements.value[active.value]
  const block = props.modelValue.blocks[active.value]
  if (!element || !block) return
  const text = plainText(block.runs)
  const selected = selectionRange(element)
  const start = selected ? selected.start : 0
  const end = selected && selected.start !== selected.end ? selected.end : text.length
  block.runs = applyMark(block.runs, start, end, mark)
  renderRuns(element, block.runs)
  element.focus()
  restoreSelection(element, start, end === text.length && (!selected || selected.start === selected.end) ? plainText(block.runs).length : end)
  touch()
}

function setType(type) {
  const block = props.modelValue.blocks[active.value]
  if (!block || props.readonly) return
  block.type = type
  blockType.value = type
  touch()
}

function setAlign(value) {
  const block = props.modelValue.blocks[active.value]
  if (!block || props.readonly) return
  block.align = value
  align.value = value
  touch()
}

function onKey(index, event) {
  if (props.readonly) return
  if (event.key === 'Enter' && !event.shiftKey) {
    event.preventDefault()
    splitBlock(index)
  } else if (event.key === 'Backspace') {
    const element = elements.value[index]
    const selected = selectionRange(element)
    if (selected && selected.start === 0 && selected.end === 0 && index > 0) {
      event.preventDefault()
      mergeBackward(index)
    }
  }
}

function splitBlock(index) {
  if (props.modelValue.blocks.length >= 400) return
  const element = elements.value[index]
  const block = props.modelValue.blocks[index]
  const selected = selectionRange(element)
  const offset = selected ? selected.start : plainText(block.runs).length
  const left = sliceRuns(block.runs, 0, offset)
  const right = sliceRuns(block.runs, offset, plainText(block.runs).length)
  block.runs = left.length ? left : [{ text: '', bold: false, italic: false, underline: false }]
  const created = {
    id: newId(),
    type: 'paragraph',
    align: 'left',
    runs: right.length ? right : [{ text: '', bold: false, italic: false, underline: false }],
  }
  props.modelValue.blocks.splice(index + 1, 0, created)
  touch()
  nextTick(() => paintFrom(index, 2))
}

function paintFrom(index, count, caret = null) {
  for (let offset = 0; offset < count; offset += 1) {
    const element = elements.value[index + offset]
    const block = props.modelValue.blocks[index + offset]
    if (!element || !block) continue
    renderRuns(element, block.runs)
    element.dataset.ready = '1'
  }
  const last = elements.value[index + count - 1]
  if (last) {
    last.focus()
    active.value = index + count - 1
    if (caret != null) restoreSelection(last, caret, caret)
  }
}

function mergeBackward(index) {
  const previous = props.modelValue.blocks[index - 1]
  const current = props.modelValue.blocks[index]
  const offset = plainText(previous.runs).length
  previous.runs = [...previous.runs, ...current.runs].filter((run) => run.text)
  if (!previous.runs.length) previous.runs = [{ text: '', bold: false, italic: false, underline: false }]
  props.modelValue.blocks.splice(index, 1)
  touch()
  nextTick(() => {
    const element = elements.value[index - 1]
    renderRuns(element, previous.runs)
    element.focus()
    restoreSelection(element, offset, offset)
    active.value = index - 1
  })
}

function refreshAll() {
  nextTick(() => {
    props.modelValue.blocks.forEach((block, index) => {
      const element = elements.value[index]
      if (element) renderRuns(element, block.runs)
    })
  })
}

onMounted(refreshAll)
watch(() => props.modelValue, refreshAll)
</script>

<template>
  <div class="word-editor" data-testid="word-editor">
    <div class="toolbar" v-if="!readonly">
      <button type="button" class="ghost" :aria-pressed="blockType === 'paragraph'" @mousedown.prevent="setType('paragraph')">正文</button>
      <button type="button" class="ghost" :aria-pressed="blockType === 'heading1'" @mousedown.prevent="setType('heading1')">标题</button>
      <button type="button" class="ghost" :aria-pressed="blockType === 'heading2'" @mousedown.prevent="setType('heading2')">小标题</button>
      <button type="button" class="ghost" :aria-pressed="blockType === 'bullet'" @mousedown.prevent="setType('bullet')">项目符号</button>
      <button type="button" class="ghost" :aria-pressed="blockType === 'numbered'" @mousedown.prevent="setType('numbered')">编号</button>
      <button type="button" class="ghost" @mousedown.prevent="setAlign('left')">左齐</button>
      <button type="button" class="ghost" @mousedown.prevent="setAlign('center')">居中</button>
      <button type="button" class="ghost" @mousedown.prevent="setAlign('right')">右齐</button>
      <button type="button" class="ghost" data-testid="bold" @mousedown.prevent="format('bold')">加粗</button>
      <button type="button" class="ghost" @mousedown.prevent="format('italic')">斜体</button>
      <button type="button" class="ghost" @mousedown.prevent="format('underline')">下划线</button>
    </div>
    <article class="page">
      <div
        v-for="(block, index) in modelValue.blocks"
        :key="block.id"
        class="word-line"
        :data-align="block.align"
      >
        <span class="marker" v-if="block.type === 'bullet'">•</span>
        <span class="marker" v-else-if="block.type === 'numbered'">{{ index + 1 }}.</span>
        <div
          class="word-block"
          :class="block.type"
          :contenteditable="readonly ? 'false' : 'true'"
          spellcheck="false"
          data-testid="word-block"
          :ref="(element) => setElement(index, element)"
          @focus="focusBlock(index)"
          @beforeinput="onBeforeInput(index, $event)"
          @paste="onPaste(index, $event)"
          @input="onInput(index, $event)"
          @keydown="onKey(index, $event)"
        />
      </div>
    </article>
  </div>
</template>
