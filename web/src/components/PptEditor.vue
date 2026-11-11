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

const props = defineProps({
  modelValue: { type: Object, required: true },
  readonly: { type: Boolean, default: false },
})
const emit = defineEmits(['update:modelValue'])

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
let observer
let clip = null

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
}
function addSlide(layout = 'content') {
  if (props.readonly || props.modelValue.slides.length >= 30) return
  checkpoint()
  const made = slideLayout(layout, [newId(), newId()])
  props.modelValue.slides.splice(slideIndex.value + 1, 0, { id: newId(), ...made })
  slideIndex.value += 1
  selectedId.value = ''
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
  if (value) element[key] = true
  else delete element[key]
  touch()
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
  return { x: (event.clientX - rect.left) / scale.value, y: (event.clientY - rect.top) / scale.value }
}
function select(element, event) {
  if (event.button !== 0) return
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
  if (event.target.closest('.ppt-el, .inspector, .ppt-ribbon, .slide-list, textarea, input, select, button')) return
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
    fontFamily: element.font === 'serif' ? 'var(--serif)' : element.font === 'mono' ? 'ui-monospace, monospace' : 'var(--sans)',
  }
}
function previewStyle(element) {
  return {
    ...elementStyle(element),
    fontSize: `${Math.max(10, element.font_size)}px`,
  }
}

onMounted(() => {
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
    <div class="ppt-ribbon" v-if="!readonly">
      <button type="button" data-testid="add-slide" @click="addSlide('content')">新建幻灯片</button>
      <select aria-label="版式" data-testid="slide-layout" @change="addSlide($event.target.value); $event.target.value = ''">
        <option value="">版式</option>
        <option value="title">标题页</option>
        <option value="content">标题和内容</option>
        <option value="section">节标题</option>
        <option value="blank">空白</option>
      </select>
      <button type="button" data-testid="add-text" @click="addText">文本框</button>
      <button type="button" data-testid="add-shape" @click="addShape('rect')">矩形</button>
      <button type="button" @click="addShape('ellipse')">椭圆</button>
      <button type="button" @click="addShape('round')">圆角</button>
      <i class="ribbon-sep"></i>
      <button type="button" title="撤销" @click="undo">撤销</button>
      <button type="button" title="重做" @click="redo">重做</button>
      <button type="button" data-testid="duplicate-element" @click="duplicateSelected">复制对象</button>
      <button type="button" @click="order(1)">上移一层</button>
      <button type="button" @click="order(-1)">下移一层</button>
      <button type="button" class="danger" @click="removeElement">删除</button>
      <button type="button" class="primary show-start" data-testid="start-show" @click="startShow">幻灯片放映</button>
    </div>
    <div class="ppt-layout">
      <div class="slide-list">
        <button
          v-for="(item, index) in modelValue.slides"
          :key="item.id"
          type="button"
          class="slide-thumb"
          :class="{ active: index === slideIndex }"
          @click="chooseSlide(index)"
        >
          <div class="mini" :style="{ background: item.background }">
            <span v-for="element in item.elements" :key="element.id" class="mini-el" :class="element.shape" :style="previewStyle(element)">{{ element.text }}</span>
          </div>
          <em>{{ index + 1 }}</em>
        </button>
        <div class="slide-order" v-if="!readonly">
          <button type="button" @click="moveSlide(-1)">上移</button>
          <button type="button" @click="moveSlide(1)">下移</button>
          <button type="button" @click="duplicateSlide">复制页</button>
          <button type="button" class="danger" @click="removeSlide">删除页</button>
        </div>
      </div>
      <div class="ppt-stage" ref="frame">
        <div
          v-if="slide"
          class="canvas"
          data-testid="ppt-canvas"
          :style="{ background: slide.background, transform: `scale(${scale})`, width: '960px', height: '540px', marginBottom: `${540 * (scale - 1)}px` }"
        >
          <div
            v-for="element in slide.elements"
            :key="element.id"
            class="ppt-el"
            :class="[element.kind, element.shape, { selected: element.id === selectedId }]"
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
        <label class="slide-notes" v-if="slide">备注
          <textarea data-testid="slide-notes" :value="slide.notes || ''" :disabled="readonly" maxlength="4000" @focus="checkpoint" @input="slide.notes = $event.target.value" @change="commitNotes" />
        </label>
      </div>
      <div class="inspector" v-if="slide">
        <label>背景
          <input v-model="slide.background" :disabled="readonly" maxlength="7" @change="touch" />
        </label>
        <template v-if="selected">
          <label>字号
            <input v-model.number="selected.font_size" :disabled="readonly" type="number" min="12" max="96" @change="commitSize" />
          </label>
          <label>颜色
            <input v-model="selected.color" :disabled="readonly" maxlength="7" @change="touch" />
          </label>
          <label>填充
            <input :value="selected.fill || ''" :disabled="readonly" maxlength="7" @change="selected.fill = $event.target.value; touch()" />
          </label>
          <label>字体
            <select :value="selected.font || ''" :disabled="readonly" @change="setFont">
              <option value="">黑体</option>
              <option value="serif">宋体</option>
              <option value="mono">等宽</option>
            </select>
          </label>
          <label>对齐
            <select v-model="selected.align" :disabled="readonly" @change="touch">
              <option value="left">左齐</option>
              <option value="center">居中</option>
              <option value="right">右齐</option>
            </select>
          </label>
          <div class="inspector-actions" v-if="!readonly">
            <button type="button" @click="flag(selected, 'bold', !selected.bold)">{{ selected.bold ? '取消加粗' : '加粗' }}</button>
            <button type="button" @click="flag(selected, 'italic', !selected.italic)">斜体</button>
            <button type="button" @click="flag(selected, 'underline', !selected.underline)">下划线</button>
            <button type="button" @click="alignElement('left')">靠左</button>
            <button type="button" @click="alignElement('center')">水平居中</button>
            <button type="button" @click="alignElement('middle')">垂直居中</button>
          </div>
        </template>
      </div>
    </div>
    <div v-if="showing" class="slideshow" ref="showEl" tabindex="0" data-testid="slideshow" @keydown.stop="onShowKey">
      <div class="show-fit" :style="{ transform: `scale(${showScale})` }">
        <div class="canvas show-canvas" :style="{ background: slide.background }">
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
