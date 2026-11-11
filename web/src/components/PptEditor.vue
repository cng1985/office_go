<script setup>
import { computed, nextTick, onBeforeUnmount, onMounted, ref } from 'vue'
import { newId } from '../editors/text'
import {
  CANVAS_H,
  CANVAS_W,
  clamp,
  duplicateElement,
  moveItem,
  resizeBox,
  shapeElement,
  slideLayout,
  textElement,
} from '../editors/ppt'
import { useSessionStore } from '../stores/session'

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

const HANDLES = ['nw', 'n', 'ne', 'e', 'se', 's', 'sw', 'w']
const slideIndex = ref(0)
const selectedId = ref('')
const editingId = ref('')
const frame = ref(null)
const showEl = ref(null)
const scale = ref(1)
const showScale = ref(1)
const showing = ref(false)
const past = ref([])
const future = ref([])
const menu = ref('')
const query = ref('')
const ribbonOpen = ref(true)
const panel = ref('theme')
const zoomChoice = ref('fit')
const starred = ref(false)
const painter = ref(null)
const transition = ref('none')
const activeTheme = ref('light')
const colorInput = ref(null)
const rulerX = Array.from({ length: 26 }, (_, index) => index + 1)
const rulerY = Array.from({ length: 14 }, (_, index) => index + 1)
const THEMES = [
  { id: 'light', name: '纯浅色', background: '#ffffff', ink: '#202124', sub: '#5f6368' },
  { id: 'dark', name: '纯深色', background: '#000000', ink: '#ffffff', sub: '#e8eaed' },
  { id: 'simple', name: '精简', background: '#f1f3f4', ink: '#202124', sub: '#5f6368' },
]
const PLACEHOLDERS = new Set(['点击可添加标题', '点击可添加副标题', '点击可添加正文', '单击编辑标题', '单击编辑副标题', '单击编辑正文'])
let observer
let clip = null
const viewScale = computed(() => (zoomChoice.value === 'fit' ? scale.value : Number(zoomChoice.value)))
const sheetWidth = computed(() => 22 + CANVAS_W * viewScale.value)
const sheetHeight = computed(() => 22 + CANVAS_H * viewScale.value)
const initial = computed(() => (session.account?.display_name || session.account?.username || '云').slice(0, 1))
const statusQuiet = computed(() => !/未|冲突|更新|失败/.test(props.status || ''))

const slide = computed(() => props.modelValue.slides[slideIndex.value])
const selected = computed(() => slide.value?.elements.find((item) => item.id === selectedId.value))

function touch() {
  emit('update:modelValue', props.modelValue)
}
function snapshot() {
  return JSON.stringify(props.modelValue.slides)
}
function checkpoint() {
  const snap = snapshot()
  if (past.value[past.value.length - 1] === snap) return
  past.value.push(snap)
  if (past.value.length > 80) past.value.shift()
  future.value = []
}
function restore(json) {
  const slides = JSON.parse(json)
  props.modelValue.slides.splice(0, props.modelValue.slides.length, ...slides)
  if (slideIndex.value >= slides.length) slideIndex.value = slides.length - 1
  syncTheme()
  touch()
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
function chooseSlide(index) {
  slideIndex.value = index
  selectedId.value = ''
  editingId.value = ''
  syncTheme()
}
function addSlide(layout = 'content') {
  if (props.readonly || props.modelValue.slides.length >= 30) return
  checkpoint()
  const made = slideLayout(layout, [newId(), newId()])
  props.modelValue.slides.splice(slideIndex.value + 1, 0, { id: newId(), ...made })
  slideIndex.value += 1
  selectedId.value = ''
  syncTheme()
  touch()
}
function duplicateSlide() {
  if (props.readonly || props.modelValue.slides.length >= 30 || !slide.value) return
  checkpoint()
  const copy = JSON.parse(JSON.stringify(slide.value))
  copy.id = newId()
  copy.elements.forEach((element) => { element.id = newId() })
  props.modelValue.slides.splice(slideIndex.value + 1, 0, copy)
  slideIndex.value += 1
  touch()
}
function removeSlide() {
  if (props.readonly || props.modelValue.slides.length <= 1) return
  checkpoint()
  props.modelValue.slides.splice(slideIndex.value, 1)
  slideIndex.value = Math.max(0, slideIndex.value - 1)
  selectedId.value = ''
  touch()
}
function moveSlide(delta) {
  if (props.readonly) return
  checkpoint()
  slideIndex.value = moveItem(props.modelValue.slides, slideIndex.value, delta)
  touch()
}
function addText() {
  if (props.readonly || slide.value.elements.length >= 40) return
  checkpoint()
  const element = textElement(newId(), { x: 120, y: 120, w: 320, h: 72, text: '文本框', font_size: 24 })
  slide.value.elements.push(element)
  selectedId.value = element.id
  editingId.value = element.id
  touch()
}
function addShape(shape) {
  if (props.readonly || slide.value.elements.length >= 40) return
  checkpoint()
  const element = shapeElement(newId(), shape)
  slide.value.elements.push(element)
  selectedId.value = element.id
  touch()
}
function removeElement() {
  if (!selected.value || props.readonly) return
  checkpoint()
  slide.value.elements = slide.value.elements.filter((item) => item.id !== selectedId.value)
  selectedId.value = ''
  editingId.value = ''
  touch()
}
function duplicateSelected() {
  if (!selected.value || props.readonly || slide.value.elements.length >= 40) return
  checkpoint()
  const copy = duplicateElement(selected.value, newId())
  slide.value.elements.push(copy)
  selectedId.value = copy.id
  touch()
}
function copySelected() {
  if (!selected.value) return
  clip = JSON.parse(JSON.stringify(selected.value))
}
function pasteClip() {
  if (!clip || props.readonly || slide.value.elements.length >= 40) return
  checkpoint()
  const copy = duplicateElement(clip, newId())
  slide.value.elements.push(copy)
  selectedId.value = copy.id
  clip = copy
  touch()
}
function order(delta) {
  if (!selected.value || props.readonly) return
  checkpoint()
  const index = slide.value.elements.findIndex((item) => item.id === selectedId.value)
  moveItem(slide.value.elements, index, delta)
  touch()
}
function alignElement(mode) {
  if (!selected.value || props.readonly) return
  checkpoint()
  const element = selected.value
  if (mode === 'left') element.x = 48
  if (mode === 'center') element.x = (CANVAS_W - element.w) / 2
  if (mode === 'right') element.x = CANVAS_W - element.w - 48
  if (mode === 'top') element.y = 36
  if (mode === 'middle') element.y = (CANVAS_H - element.h) / 2
  if (mode === 'bottom') element.y = CANVAS_H - element.h - 36
  touch()
}
function setFont(event) {
  if (!selected.value || props.readonly) return
  if (event.target.value) selected.value.font = event.target.value
  else delete selected.value.font
  touch()
}
function commitSize() {
  if (!selected.value) return
  selected.value.font_size = clamp(Number(selected.value.font_size) || 12, 12, 96)
  touch()
}
function flag(element, key, value) {
  if (!element || props.readonly) return
  if (value) element[key] = true
  else delete element[key]
  touch()
}
function applyLayout(layout) {
  if (!slide.value || props.readonly || !layout) return
  checkpoint()
  const made = slideLayout(layout, [newId(), newId()])
  slide.value.background = made.background
  slide.value.elements = made.elements
  selectedId.value = ''
  editingId.value = ''
  syncTheme()
  touch()
}
function applyTheme(theme) {
  if (!slide.value || props.readonly) return
  checkpoint()
  slide.value.background = theme.background
  slide.value.elements.forEach((element) => {
    if (element.kind !== 'text') return
    element.color = element.bold || element.font_size >= 32 ? theme.ink : theme.sub
  })
  activeTheme.value = theme.id
  touch()
}
function applyBackground(color) {
  if (!slide.value || props.readonly || !/^#[0-9a-fA-F]{6}$/.test(color)) return
  checkpoint()
  slide.value.background = color.toLowerCase()
  syncTheme()
  touch()
}
function syncTheme() {
  const found = THEMES.find((item) => item.background === (slide.value?.background || '').toLowerCase())
  activeTheme.value = found?.id || ''
}
function cycleTheme() {
  const index = Math.max(0, THEMES.findIndex((item) => item.id === activeTheme.value))
  applyTheme(THEMES[(index + 1) % THEMES.length])
}
function addPicture() {
  if (props.readonly || slide.value.elements.length >= 40) return
  checkpoint()
  const element = shapeElement(newId(), 'rect')
  element.x = 280
  element.y = 150
  element.w = 400
  element.h = 240
  element.fill = '#e8eaed'
  element.text = '图片'
  element.color = '#5f6368'
  slide.value.elements.push(element)
  selectedId.value = element.id
  touch()
}
function addLine() {
  if (props.readonly || slide.value.elements.length >= 40) return
  checkpoint()
  const element = shapeElement(newId(), 'rect')
  element.x = 160
  element.y = 258
  element.w = 640
  element.h = 24
  element.fill = '#202124'
  element.text = ''
  slide.value.elements.push(element)
  selectedId.value = element.id
  touch()
}
function togglePainter() {
  if (!selected.value) {
    painter.value = null
    return
  }
  painter.value = painter.value ? null : JSON.parse(JSON.stringify(selected.value))
}
function applyPainter(element) {
  const style = painter.value
  if (!style) return
  checkpoint()
  element.color = style.color
  element.font_size = style.font_size
  element.align = style.align || 'left'
  if (style.bold) element.bold = true
  else delete element.bold
  if (style.italic) element.italic = true
  else delete element.italic
  if (style.underline) element.underline = true
  else delete element.underline
  if (style.font) element.font = style.font
  else delete element.font
  if (style.fill) element.fill = style.fill
  else delete element.fill
  painter.value = null
  selectedId.value = element.id
  touch()
}
function isPlaceholder(element) {
  return element.kind === 'text' && PLACEHOLDERS.has(element.text)
}
function darkSlide(item) {
  const hex = String(item?.background || '').replace('#', '')
  if (!/^[0-9a-fA-F]{6}$/.test(hex)) return false
  const red = Number.parseInt(hex.slice(0, 2), 16)
  const green = Number.parseInt(hex.slice(2, 4), 16)
  const blue = Number.parseInt(hex.slice(4, 6), 16)
  return (red * 299 + green * 587 + blue * 114) / 1000 < 140
}
function toggleMenu(name) {
  menu.value = menu.value === name ? '' : name
}
function run(fn) {
  menu.value = ''
  fn()
}
function setZoom(value) {
  zoomChoice.value = value
  nextTick(measure)
}
function printDeck() {
  window.print()
}
function focusNotes() {
  document.querySelector('[data-testid="slide-notes"]')?.focus()
}
function startShowAt(index) {
  slideIndex.value = index
  startShow()
}
function commitNotes() {
  if (!slide.value || props.readonly) return
  const value = String(slide.value.notes || '').replace(/\r\n/g, '\n').slice(0, 4000)
  if (value) slide.value.notes = value
  else delete slide.value.notes
  touch()
}
function point(event) {
  const rect = frame.value.querySelector('.canvas').getBoundingClientRect()
  return { x: (event.clientX - rect.left) / viewScale.value, y: (event.clientY - rect.top) / viewScale.value }
}
function select(element, event) {
  if (event.button !== 0) return
  if (painter.value && !props.readonly && editingId.value !== element.id) {
    applyPainter(element)
    return
  }
  if (editingId.value === element.id) return
  selectedId.value = element.id
  editingId.value = ''
  if (props.readonly) return
  const start = point(event)
  const origin = { x: element.x, y: element.y }
  let moved = false
  const mark = past.value.length
  checkpoint()
  const added = past.value.length !== mark
  const move = (ev) => {
    const current = point(ev)
    const dx = current.x - start.x
    const dy = current.y - start.y
    if (!moved && Math.abs(dx) < 3 && Math.abs(dy) < 3) return
    moved = true
    element.x = clamp(origin.x + dx, -200, 1100)
    element.y = clamp(origin.y + dy, -200, 700)
  }
  const up = () => {
    window.removeEventListener('pointermove', move)
    window.removeEventListener('pointerup', up)
    if (!moved && added) past.value.pop()
    else if (moved) touch()
  }
  window.addEventListener('pointermove', move)
  window.addEventListener('pointerup', up)
}
function edit(element) {
  if (props.readonly) return
  selectedId.value = element.id
  editingId.value = element.id
  if (!isPlaceholder(element)) return
  nextTick(() => {
    const field = frame.value?.querySelector('[data-testid="ppt-text"]')
    field?.select()
  })
}
function resize(element, handle, event) {
  if (props.readonly) return
  checkpoint()
  const start = point(event)
  const origin = { x: element.x, y: element.y, w: element.w, h: element.h }
  const move = (ev) => {
    const current = point(ev)
    const next = resizeBox(origin, handle, current.x - start.x, current.y - start.y)
    element.x = next.x
    element.y = next.y
    element.w = next.w
    element.h = next.h
  }
  const up = () => {
    window.removeEventListener('pointermove', move)
    window.removeEventListener('pointerup', up)
    touch()
  }
  window.addEventListener('pointermove', move)
  window.addEventListener('pointerup', up)
}
function clearSelection(event) {
  if (!event.target.closest('.menu-anchor, .slides-search')) menu.value = ''
  if (event.target.closest('.ppt-el, .slides-chrome, .filmstrip, .theme-panel, .side-rail, .notes-bar, .menu-pop, textarea, input, select, button')) return
  selectedId.value = ''
  editingId.value = ''
}
function typing(event) {
  const tag = event.target.tagName
  return tag === 'TEXTAREA' || tag === 'INPUT' || tag === 'SELECT' || event.target.isContentEditable
}
function onKey(event) {
  if (props.readonly || showing.value || typing(event)) return
  const command = event.metaKey || event.ctrlKey
  const key = event.key.toLowerCase()
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
  if (command && key === 'd') {
    event.preventDefault()
    duplicateSelected()
    return
  }
  if (command && key === 'c') {
    copySelected()
    return
  }
  if (command && key === 'v') {
    event.preventDefault()
    pasteClip()
    return
  }
  if (command && key === 'x') {
    copySelected()
    removeElement()
    return
  }
  if (event.key === 'F5') {
    event.preventDefault()
    startShow()
    return
  }
  if (!selected.value) return
  if (event.key === 'Delete' || event.key === 'Backspace') {
    event.preventDefault()
    removeElement()
    return
  }
  if (event.key === 'Enter' || event.key === 'F2') {
    event.preventDefault()
    editingId.value = selected.value.id
    return
  }
  const step = event.shiftKey ? 10 : 1
  const deltas = { ArrowLeft: [-step, 0], ArrowRight: [step, 0], ArrowUp: [0, -step], ArrowDown: [0, step] }
  const delta = deltas[event.key]
  if (!delta) return
  event.preventDefault()
  checkpoint()
  selected.value.x = clamp(selected.value.x + delta[0], -200, 1100)
  selected.value.y = clamp(selected.value.y + delta[1], -200, 700)
  touch()
}
function fitShow() {
  showScale.value = Math.min((window.innerWidth - 48) / CANVAS_W, (window.innerHeight - 88) / CANVAS_H)
}
function startShow() {
  showing.value = true
  fitShow()
  nextTick(() => showEl.value?.focus())
}
function onShowKey(event) {
  if (event.key === 'Escape') {
    showing.value = false
    return
  }
  if (['ArrowRight', 'ArrowDown', 'PageDown', ' '].includes(event.key)) {
    event.preventDefault()
    slideIndex.value = Math.min(props.modelValue.slides.length - 1, slideIndex.value + 1)
  } else if (['ArrowLeft', 'ArrowUp', 'PageUp'].includes(event.key)) {
    event.preventDefault()
    slideIndex.value = Math.max(0, slideIndex.value - 1)
  } else if (event.key === 'Home') slideIndex.value = 0
  else if (event.key === 'End') slideIndex.value = props.modelValue.slides.length - 1
}
function measure() {
  if (!frame.value) return
  scale.value = Math.max(0.25, Math.min(1, (frame.value.clientWidth - 28) / CANVAS_W))
}
function elementStyle(element) {
  return {
    left: `${element.x}px`,
    top: `${element.y}px`,
    width: `${element.w}px`,
    height: `${element.h}px`,
    color: element.color,
    fontSize: `${element.font_size}px`,
    fontWeight: element.bold ? 700 : 400,
    fontStyle: element.italic ? 'italic' : 'normal',
    textDecoration: element.underline ? 'underline' : 'none',
    textAlign: element.align,
    background: element.fill || 'transparent',
    fontFamily: element.font === 'serif' ? 'Georgia, "Songti SC", serif' : element.font === 'mono' ? 'ui-monospace, monospace' : 'Arial, "PingFang SC", "Noto Sans SC", sans-serif',
  }
}
function previewStyle(element) {
  return {
    ...elementStyle(element),
    fontSize: `${Math.max(10, element.font_size)}px`,
  }
}

const commands = [
  { label: '撤销', run: undo },
  { label: '重做', run: redo },
  { label: '新建幻灯片', run: () => addSlide('title') },
  { label: '标题和内容', run: () => addSlide('content') },
  { label: '文本框', run: addText },
  { label: '矩形', run: () => addShape('rect') },
  { label: '椭圆', run: () => addShape('ellipse') },
  { label: '图片', run: addPicture },
  { label: '直线', run: addLine },
  { label: '放映', run: startShow },
  { label: '打印', run: printDeck },
  { label: '保存', run: () => emit('save') },
]
const hits = computed(() => {
  const keyword = query.value.trim()
  const list = keyword ? commands.filter((item) => item.label.includes(keyword)) : commands
  return list.slice(0, 8)
})

onMounted(() => {
  syncTheme()
  measure()
  observer = new ResizeObserver(measure)
  if (frame.value) observer.observe(frame.value)
  window.addEventListener('resize', fitShow)
})
onBeforeUnmount(() => {
  observer?.disconnect()
  window.removeEventListener('resize', fitShow)
})
</script>

<template>
  <div class="ppt-editor" data-testid="ppt-editor" tabindex="0" @keydown="onKey" @pointerdown="clearSelection">
    <header class="slides-top slides-chrome">
      <div class="slides-brand">
        <router-link class="slides-logo" to="/" aria-label="返回文库">
          <svg viewBox="0 0 24 24" aria-hidden="true"><rect x="3" y="5" width="18" height="12" rx="1.5" fill="none" stroke="#fff" stroke-width="1.8"/><path d="M8 20h8M12 17v3" stroke="#fff" stroke-width="1.8" stroke-linecap="round"/></svg>
        </router-link>
        <div class="slides-titlebox">
          <div class="slides-title-row">
            <input data-testid="doc-title" :value="title" :disabled="readonly || previewing" maxlength="80" :style="{ width: `${Math.min(480, Math.max(148, Array.from(title).length * 18 + 20))}px` }" @input="emit('update:title', $event.target.value)" />
            <button type="button" class="star" :class="{ on: starred }" title="加星标" @click="starred = !starred">
              <svg viewBox="0 0 24 24" aria-hidden="true"><path d="M12 3.2l2.6 5.3 5.8.8-4.2 4.1 1 5.8L12 16.4 6.8 19.2l1-5.8L3.6 9.3l5.8-.8z" fill="none" stroke="currentColor" stroke-width="1.6" stroke-linejoin="round"/></svg>
            </button>
            <span class="slides-status" :class="{ quiet: statusQuiet }" data-testid="save-status">{{ status }}</span>
          </div>
          <nav class="slides-menus">
            <div class="menu-anchor" v-for="name in ['文件', '编辑', '查看', '插入', '格式', '幻灯片', '排列', '工具', '扩展程序', '帮助']" :key="name">
              <button type="button" @click="toggleMenu(name)">{{ name }}</button>
              <div v-if="menu === name" class="menu-pop" @pointerdown.stop>
                <template v-if="name === '文件'">
                  <button type="button" :disabled="pending || previewing || readonly" @click="run(() => emit('save'))">保存到云端</button>
                  <button type="button" @click="run(() => emit('versions'))">版本历史</button>
                  <button type="button" @click="run(printDeck)">打印</button>
                </template>
                <template v-else-if="name === '编辑'">
                  <button type="button" @click="run(undo)">撤销</button>
                  <button type="button" @click="run(redo)">重做</button>
                  <button type="button" data-testid="duplicate-element" :disabled="!selected" @click="run(duplicateSelected)">复制对象</button>
                  <button type="button" :disabled="!selected" @click="run(copySelected)">复制</button>
                  <button type="button" :disabled="!selected" @click="run(() => { copySelected(); removeElement() })">剪切</button>
                  <button type="button" @click="run(pasteClip)">粘贴</button>
                  <button type="button" :disabled="!selected" @click="run(removeElement)">删除</button>
                </template>
                <template v-else-if="name === '查看'">
                  <button type="button" @click="run(() => setZoom('fit'))">适合大小</button>
                  <button type="button" @click="run(() => setZoom('1'))">100%</button>
                  <button type="button" @click="run(startShow)">放映</button>
                  <button type="button" @click="run(focusNotes)">演讲者备注</button>
                </template>
                <template v-else-if="name === '插入'">
                  <button type="button" @click="run(addText)">文本框</button>
                  <button type="button" @click="run(() => addShape('rect'))">矩形</button>
                  <button type="button" @click="run(() => addShape('ellipse'))">椭圆</button>
                  <button type="button" @click="run(() => addShape('round'))">圆角矩形</button>
                  <button type="button" @click="run(addPicture)">图片</button>
                  <button type="button" @click="run(addLine)">直线</button>
                  <button type="button" @click="run(() => addSlide('title'))">幻灯片</button>
                </template>
                <template v-else-if="name === '格式'">
                  <button type="button" :disabled="!selected" @click="run(() => flag(selected, 'bold', !selected.bold))">加粗</button>
                  <button type="button" :disabled="!selected" @click="run(() => flag(selected, 'italic', !selected.italic))">斜体</button>
                  <button type="button" :disabled="!selected" @click="run(() => flag(selected, 'underline', !selected.underline))">下划线</button>
                  <button type="button" :disabled="!selected" @click="run(() => { selected.align = 'left'; touch() })">左对齐</button>
                  <button type="button" :disabled="!selected" @click="run(() => { selected.align = 'center'; touch() })">居中</button>
                  <button type="button" :disabled="!selected" @click="run(() => { selected.align = 'right'; touch() })">右对齐</button>
                  <label>字号
                    <input :value="selected?.font_size || 24" :disabled="!selected || readonly" type="number" min="12" max="96" @change="selected.font_size = Number($event.target.value); commitSize()" />
                  </label>
                  <label>颜色
                    <input :value="selected?.color || '#202124'" :disabled="!selected || readonly" type="color" @input="selected.color = $event.target.value; touch()" />
                  </label>
                  <label>填充
                    <input :value="selected?.fill || '#ffffff'" :disabled="!selected || readonly" type="color" @input="selected.fill = $event.target.value; touch()" />
                  </label>
                  <label>字体
                    <select :value="selected?.font || ''" :disabled="!selected || readonly" @change="setFont">
                      <option value="">黑体</option>
                      <option value="serif">宋体</option>
                      <option value="mono">等宽</option>
                    </select>
                  </label>
                </template>
                <template v-else-if="name === '幻灯片'">
                  <button type="button" @click="run(() => addSlide('title'))">新建幻灯片</button>
                  <button type="button" @click="run(duplicateSlide)">复制页</button>
                  <button type="button" @click="run(() => moveSlide(-1))">上移</button>
                  <button type="button" @click="run(() => moveSlide(1))">下移</button>
                  <button type="button" @click="run(removeSlide)">删除页</button>
                  <button type="button" @click="run(() => startShowAt(0))">从头放映</button>
                  <button type="button" @click="run(startShow)">从当前页放映</button>
                </template>
                <template v-else-if="name === '排列'">
                  <button type="button" :disabled="!selected" @click="run(() => order(1))">上移一层</button>
                  <button type="button" :disabled="!selected" @click="run(() => order(-1))">下移一层</button>
                  <button type="button" :disabled="!selected" @click="run(() => alignElement('left'))">靠左</button>
                  <button type="button" :disabled="!selected" @click="run(() => alignElement('center'))">水平居中</button>
                  <button type="button" :disabled="!selected" @click="run(() => alignElement('middle'))">垂直居中</button>
                </template>
                <template v-else-if="name === '工具'">
                  <button type="button" @click="run(focusNotes)">演讲者备注</button>
                  <button type="button" @click="run(() => { panel = 'theme' })">主题</button>
                </template>
                <template v-else-if="name === '扩展程序'">
                  <p>没有已安装的扩展程序</p>
                </template>
                <template v-else>
                  <p>Ctrl+S 保存，F5 放映，方向键移动对象，Delete 删除。</p>
                </template>
              </div>
            </div>
          </nav>
        </div>
      </div>
      <div class="slides-actions">
        <button type="button" class="icon-button" title="版本历史" data-testid="version-button" @click="emit('versions')">
          <svg viewBox="0 0 24 24" aria-hidden="true"><circle cx="12" cy="12" r="8" fill="none" stroke="currentColor" stroke-width="1.7"/><path d="M12 8v5l3 2" fill="none" stroke="currentColor" stroke-width="1.7" stroke-linecap="round"/></svg>
        </button>
        <div class="menu-anchor show-group">
          <button type="button" class="slides-play" data-testid="start-show" @click="startShow">幻灯片</button>
          <button type="button" class="slides-play caret" title="放映选项" @click="toggleMenu('show')">▾</button>
          <div v-if="menu === 'show'" class="menu-pop">
            <button type="button" @click="run(() => startShowAt(0))">从头放映</button>
            <button type="button" @click="run(startShow)">从当前页放映</button>
          </div>
        </div>
        <div class="menu-anchor share-group">
          <button type="button" class="slides-share" data-testid="save-button" :disabled="pending || previewing || readonly" @click="emit('save')">共享</button>
          <button type="button" class="slides-share caret" title="保存选项" @click="toggleMenu('share')">▾</button>
          <div v-if="menu === 'share'" class="menu-pop">
            <button type="button" :disabled="pending || previewing || readonly" @click="run(() => emit('save'))">保存到云端</button>
            <button type="button" @click="run(() => emit('versions'))">版本历史</button>
          </div>
        </div>
        <span class="slides-avatar" :title="session.account?.display_name || ''">{{ initial }}</span>
      </div>
    </header>
    <div class="slides-ribbon slides-chrome" v-if="!readonly && ribbonOpen">
      <label class="slides-search">
        <svg viewBox="0 0 24 24" aria-hidden="true"><circle cx="11" cy="11" r="6" fill="none" stroke="#5f6368" stroke-width="1.8"/><path d="M16 16l4 4" stroke="#5f6368" stroke-width="1.8" stroke-linecap="round"/></svg>
        <input placeholder="菜单" v-model="query" @focus="menu = 'search'" />
        <div v-if="menu === 'search'" class="menu-pop">
          <button v-for="hit in hits" :key="hit.label" type="button" @click="run(hit.run)">{{ hit.label }}</button>
          <p v-if="!hits.length">没有匹配的菜单</p>
        </div>
      </label>
      <button type="button" title="撤销" @click="undo">
        <svg viewBox="0 0 24 24"><path d="M8 8H4v4" fill="none" stroke="currentColor" stroke-width="1.7" stroke-linecap="round"/><path d="M4 12a8 8 0 1 0 2-5" fill="none" stroke="currentColor" stroke-width="1.7" stroke-linecap="round"/></svg>
      </button>
      <button type="button" title="重做" @click="redo">
        <svg viewBox="0 0 24 24"><path d="M16 8h4v4" fill="none" stroke="currentColor" stroke-width="1.7" stroke-linecap="round"/><path d="M20 12a8 8 0 1 1-2-5" fill="none" stroke="currentColor" stroke-width="1.7" stroke-linecap="round"/></svg>
      </button>
      <button type="button" title="打印" @click="printDeck">
        <svg viewBox="0 0 24 24"><path d="M7 8V4h10v4M7 16H6a2 2 0 0 1-2-2v-4a2 2 0 0 1 2-2h12a2 2 0 0 1 2 2v4a2 2 0 0 1-2 2h-1" fill="none" stroke="currentColor" stroke-width="1.6"/><rect x="7" y="14" width="10" height="6" fill="none" stroke="currentColor" stroke-width="1.6"/></svg>
      </button>
      <button type="button" title="格式刷" :class="{ active: painter }" @click="togglePainter">
        <svg viewBox="0 0 24 24"><path d="M4 14l6-6 4 4-6 6H4v-4zM14 8l2-2 3 3-2 2" fill="none" stroke="currentColor" stroke-width="1.6" stroke-linejoin="round"/></svg>
      </button>
      <select aria-label="缩放" :value="zoomChoice" @change="setZoom($event.target.value)">
        <option value="fit">适合大小</option>
        <option value="0.5">50%</option>
        <option value="0.75">75%</option>
        <option value="1">100%</option>
        <option value="1.25">125%</option>
        <option value="1.5">150%</option>
        <option value="2">200%</option>
      </select>
      <i class="ribbon-sep"></i>
      <button type="button" title="选择" class="active">
        <svg viewBox="0 0 24 24"><path d="M6 4l10 8-4 1 3 5-2 1-3-5-4 3z" fill="currentColor"/></svg>
      </button>
      <button type="button" title="文本框" data-testid="add-text" @click="addText">
        <svg viewBox="0 0 24 24"><path d="M5 6h14M12 6v12M8 18h8" fill="none" stroke="currentColor" stroke-width="1.8" stroke-linecap="round"/></svg>
      </button>
      <button type="button" title="图片" @click="addPicture">
        <svg viewBox="0 0 24 24"><rect x="4" y="5" width="16" height="14" rx="1.5" fill="none" stroke="currentColor" stroke-width="1.6"/><circle cx="9" cy="10" r="1.4" fill="currentColor"/><path d="M5 16l4-3 3 2 3-4 4 5" fill="none" stroke="currentColor" stroke-width="1.6" stroke-linejoin="round"/></svg>
      </button>
      <div class="menu-anchor">
        <button type="button" title="形状" data-testid="add-shape" @click="addShape('rect')">
          <svg viewBox="0 0 24 24"><rect x="5" y="6" width="14" height="12" rx="1" fill="none" stroke="currentColor" stroke-width="1.6"/></svg>
        </button>
        <button type="button" class="caret-lite" title="更多形状" @click="toggleMenu('shape')">▾</button>
        <div v-if="menu === 'shape'" class="menu-pop">
          <button type="button" @click="run(() => addShape('rect'))">矩形</button>
          <button type="button" @click="run(() => addShape('ellipse'))">椭圆</button>
          <button type="button" @click="run(() => addShape('round'))">圆角矩形</button>
        </div>
      </div>
      <button type="button" title="直线" @click="addLine">
        <svg viewBox="0 0 24 24"><path d="M5 17L19 7" stroke="currentColor" stroke-width="1.8" stroke-linecap="round"/></svg>
      </button>
      <button type="button" title="备注" @click="focusNotes">
        <svg viewBox="0 0 24 24"><path d="M6 6h12v8H9l-3 3z" fill="none" stroke="currentColor" stroke-width="1.6" stroke-linejoin="round"/></svg>
      </button>
      <div class="menu-anchor">
        <button type="button" title="更多" @click="toggleMenu('more')">▾</button>
        <div v-if="menu === 'more'" class="menu-pop">
          <button type="button" :disabled="!selected" @click="run(duplicateSelected)">复制对象</button>
          <button type="button" :disabled="!selected" @click="run(() => order(1))">上移一层</button>
          <button type="button" :disabled="!selected" @click="run(() => order(-1))">下移一层</button>
          <button type="button" :disabled="!selected" @click="run(removeElement)">删除</button>
        </div>
      </div>
      <i class="ribbon-sep"></i>
      <div class="menu-anchor">
        <button type="button" @click="toggleMenu('spell')">拼 ▾</button>
        <div v-if="menu === 'spell'" class="menu-pop">
          <p>文字会随输入保存，这里没有单独的拼写词典。</p>
        </div>
      </div>
      <div class="menu-anchor">
        <button type="button" @click="toggleMenu('back')">背景 ▾</button>
        <div v-if="menu === 'back'" class="menu-pop">
          <button v-for="theme in THEMES" :key="theme.id" type="button" @click="run(() => applyTheme(theme))">{{ theme.name }}</button>
        </div>
      </div>
      <div class="menu-anchor">
        <button type="button" @click="toggleMenu('layout')">布局 ▾</button>
        <div v-if="menu === 'layout'" class="menu-pop">
          <select aria-label="版式" data-testid="slide-layout" @change="applyLayout($event.target.value); $event.target.value = ''">
            <option value="">应用到当前页</option>
            <option value="title">标题页</option>
            <option value="content">标题和内容</option>
            <option value="section">节标题</option>
            <option value="blank">空白</option>
          </select>
        </div>
      </div>
      <button type="button" :class="{ active: panel === 'theme' }" @click="panel = 'theme'">主题背景</button>
      <div class="menu-anchor">
        <button type="button" @click="toggleMenu('trans')">过渡 ▾</button>
        <div v-if="menu === 'trans'" class="menu-pop">
          <button type="button" @click="run(() => { transition = 'none' })">无</button>
          <button type="button" @click="run(() => { transition = 'fade' })">淡入</button>
        </div>
      </div>
    </div>
    <button v-if="!readonly" type="button" class="ribbon-collapse" :title="ribbonOpen ? '收起工具栏' : '展开工具栏'" @click="ribbonOpen = !ribbonOpen">{{ ribbonOpen ? '⌃' : '⌄' }}</button>
    <div class="slides-workspace">
      <aside class="filmstrip">
        <div
          v-for="(item, index) in modelValue.slides"
          :key="item.id"
          class="film-row"
          :class="{ active: index === slideIndex }"
        >
          <span>{{ index + 1 }}</span>
          <button type="button" class="slide-thumb" @click="chooseSlide(index)">
            <div class="mini" :style="{ background: item.background }">
              <span v-for="element in item.elements" :key="element.id" class="mini-el" :class="element.shape" :style="previewStyle(element)">{{ element.text }}</span>
            </div>
          </button>
        </div>
        <button v-if="!readonly" type="button" class="film-add" data-testid="add-slide" @click="addSlide('title')">+ 新建幻灯片</button>
      </aside>
      <div class="slides-center">
        <div class="slides-board" ref="frame">
          <div v-if="slide" class="slide-sheet" :style="{ width: `${sheetWidth}px`, height: `${sheetHeight}px` }">
            <div class="ruler-x" :style="{ width: `${CANVAS_W * viewScale}px` }">
              <span v-for="mark in rulerX" :key="mark">{{ mark }}</span>
            </div>
            <div class="ruler-y" :style="{ height: `${CANVAS_H * viewScale}px` }">
              <span v-for="mark in rulerY" :key="mark">{{ mark }}</span>
            </div>
            <div
              class="canvas"
              data-testid="ppt-canvas"
              :class="{ dark: darkSlide(slide) }"
              :style="{ background: slide.background, transform: `scale(${viewScale})` }"
            >
              <div
                v-for="element in slide.elements"
                :key="element.id"
                class="ppt-el"
                :class="[element.kind, element.shape, { selected: element.id === selectedId, placeholder: isPlaceholder(element) }]"
                :style="elementStyle(element)"
                @pointerdown.stop="select(element, $event)"
                @dblclick.stop="edit(element)"
              >
                <textarea
                  v-if="!readonly && element.id === editingId"
                  v-model="element.text"
                  data-testid="ppt-text"
                  @pointerdown.stop
                  @change="touch"
                />
                <div v-else class="ppt-text">{{ element.text }}</div>
                <button
                  v-for="handle in (!readonly && element.id === selectedId && element.id !== editingId ? HANDLES : [])"
                  :key="handle"
                  type="button"
                  class="grip"
                  :class="handle"
                  @pointerdown.stop.prevent="resize(element, handle, $event)"
                />
              </div>
            </div>
          </div>
        </div>
        <div class="notes-bar" v-if="slide">
          <div class="notes-handle"></div>
          <textarea data-testid="slide-notes" placeholder="点击以添加演讲者备注" :value="slide.notes || ''" :disabled="readonly" maxlength="4000" @focus="checkpoint" @input="slide.notes = $event.target.value" @change="commitNotes" />
        </div>
      </div>
      <aside class="theme-panel" v-if="panel">
        <div class="theme-head">
          <strong>{{ panel === 'media' ? '媒体' : '主题' }}</strong>
          <button type="button" title="关闭" @click="panel = ''">×</button>
        </div>
        <div class="theme-body" v-if="panel === 'theme'">
          <p class="theme-section">› 此演示文稿中的主题</p>
          <button
            v-for="theme in THEMES"
            :key="theme.id"
            type="button"
            class="theme-card"
            :class="{ active: activeTheme === theme.id }"
            :data-testid="`theme-${theme.id}`"
            @click="applyTheme(theme)"
          >
            <div class="theme-preview" :style="{ background: theme.background, color: theme.ink }">
              <strong>点击可添加标题</strong>
              <span :style="{ color: theme.sub }">点击可添加副标题</span>
              <i v-if="theme.id === 'simple'" class="theme-accent"></i>
            </div>
            <em>{{ theme.name }}</em>
          </button>
        </div>
        <div class="theme-body" v-else>
          <button type="button" @click="addPicture">图片占位</button>
          <button type="button" @click="addText">文本框</button>
          <button type="button" @click="addShape('rect')">矩形</button>
          <button type="button" @click="addShape('ellipse')">椭圆</button>
          <button type="button" @click="addShape('round')">圆角矩形</button>
          <button type="button" @click="addLine">直线</button>
        </div>
        <button type="button" class="theme-import" @click="colorInput?.click()">导入主题背景</button>
      </aside>
      <input ref="colorInput" class="sr-only" type="color" aria-label="主题背景色" @input="applyBackground($event.target.value)" />
      <nav class="side-rail">
        <button type="button" :class="{ active: panel === 'media' }" @click="panel = panel === 'media' ? 'theme' : 'media'">
          <svg viewBox="0 0 24 24"><rect x="4" y="5" width="16" height="14" rx="1.5" fill="none" stroke="currentColor" stroke-width="1.6"/><circle cx="9" cy="10" r="1.3" fill="currentColor"/><path d="M5 16l4-3 3 2 3-4 4 5" fill="none" stroke="currentColor" stroke-width="1.5"/></svg>
          媒体
        </button>
        <button type="button" @click="colorInput?.click()">
          <svg viewBox="0 0 24 24"><path d="M12 4v10M8 8l4-4 4 4M6 18h12" fill="none" stroke="currentColor" stroke-width="1.7" stroke-linecap="round"/></svg>
          上传
        </button>
        <button type="button" @click="cycleTheme">
          <svg viewBox="0 0 24 24"><path d="M6 8a6 6 0 0 1 10-2l2-2v5h-5" fill="none" stroke="currentColor" stroke-width="1.6" stroke-linecap="round" stroke-linejoin="round"/><path d="M18 16a6 6 0 0 1-10 2l-2 2v-5h5" fill="none" stroke="currentColor" stroke-width="1.6" stroke-linecap="round" stroke-linejoin="round"/></svg>
          转换
        </button>
      </nav>
    </div>
    <div v-if="showing" class="slideshow" ref="showEl" tabindex="0" data-testid="slideshow" @keydown.stop="onShowKey">
      <div class="show-fit" :style="{ transform: `scale(${showScale})` }">
        <div class="canvas show-canvas" :class="{ fade: transition === 'fade' }" :key="slide.id" :style="{ background: slide.background }">
          <div v-for="element in slide.elements" :key="element.id" class="ppt-el" :class="[element.kind, element.shape]" :style="elementStyle(element)">
            <div class="ppt-text">{{ element.text }}</div>
          </div>
        </div>
      </div>
      <p class="show-notes" v-if="slide.notes">{{ slide.notes }}</p>
      <button type="button" class="show-close" @click="showing = false">退出</button>
    </div>
  </div>
</template>
