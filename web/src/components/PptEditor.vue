<script setup>
import { computed, onMounted, onBeforeUnmount, ref } from 'vue'
import { newId } from '../editors/text'

const props = defineProps({
  modelValue: { type: Object, required: true },
  readonly: { type: Boolean, default: false },
})
const emit = defineEmits(['update:modelValue'])

const slideIndex = ref(0)
const selectedId = ref('')
const frame = ref(null)
const scale = ref(1)
let observer

const slide = computed(() => props.modelValue.slides[slideIndex.value])
const selected = computed(() => slide.value?.elements.find((item) => item.id === selectedId.value))

function touch() {
  emit('update:modelValue', props.modelValue)
}

function chooseSlide(index) {
  slideIndex.value = index
  selectedId.value = ''
}

function addSlide() {
  if (props.readonly || props.modelValue.slides.length >= 30) return
  props.modelValue.slides.push({
    id: newId(),
    background: '#fffdf8',
    elements: [{
      id: newId(), kind: 'text', x: 72, y: 180, w: 760, h: 80,
      text: '新的一页', font_size: 36, color: '#1c1915', bold: true, align: 'left',
    }],
  })
  slideIndex.value = props.modelValue.slides.length - 1
  touch()
}

function removeSlide() {
  if (props.readonly || props.modelValue.slides.length <= 1) return
  props.modelValue.slides.splice(slideIndex.value, 1)
  slideIndex.value = Math.max(0, slideIndex.value - 1)
  touch()
}

function addText() {
  if (props.readonly || slide.value.elements.length >= 40) return
  const element = {
    id: newId(), kind: 'text', x: 80, y: 80, w: 280, h: 72,
    text: '文本', font_size: 24, color: '#1c1915', bold: false, align: 'left',
  }
  slide.value.elements.push(element)
  selectedId.value = element.id
  touch()
}

function removeElement() {
  if (!selected.value || props.readonly) return
  slide.value.elements = slide.value.elements.filter((item) => item.id !== selectedId.value)
  selectedId.value = ''
  touch()
}

function point(event) {
  const rect = frame.value.querySelector('.canvas').getBoundingClientRect()
  return {
    x: (event.clientX - rect.left) / scale.value,
    y: (event.clientY - rect.top) / scale.value,
  }
}

function drag(element, event) {
  if (props.readonly || event.target.dataset.editing === 'true') return
  selectedId.value = element.id
  const start = point(event)
  const origin = { x: element.x, y: element.y }
  const move = (ev) => {
    const current = point(ev)
    element.x = clamp(origin.x + current.x - start.x, -80, 900)
    element.y = clamp(origin.y + current.y - start.y, -80, 480)
  }
  const up = () => {
    window.removeEventListener('pointermove', move)
    window.removeEventListener('pointerup', up)
    touch()
  }
  window.addEventListener('pointermove', move)
  window.addEventListener('pointerup', up)
}

function resize(element, event) {
  if (props.readonly) return
  event.stopPropagation()
  const start = point(event)
  const origin = { w: element.w, h: element.h }
  const move = (ev) => {
    const current = point(ev)
    element.w = clamp(origin.w + current.x - start.x, 48, 960)
    element.h = clamp(origin.h + current.y - start.y, 32, 540)
  }
  const up = () => {
    window.removeEventListener('pointermove', move)
    window.removeEventListener('pointerup', up)
    touch()
  }
  window.addEventListener('pointermove', move)
  window.addEventListener('pointerup', up)
}

function clamp(value, min, max) {
  return Math.min(max, Math.max(min, value))
}

function measure() {
  if (!frame.value) return
  const width = frame.value.clientWidth - 36
  scale.value = Math.max(0.25, Math.min(1, width / 960))
}

onMounted(() => {
  measure()
  observer = new ResizeObserver(measure)
  if (frame.value) observer.observe(frame.value)
})
onBeforeUnmount(() => observer?.disconnect())
</script>

<template>
  <div class="ppt-layout" data-testid="ppt-editor">
    <div class="slide-list">
      <button v-if="!readonly" type="button" class="ghost" data-testid="add-slide" @click="addSlide">添加幻灯片</button>
      <button
        v-for="(item, index) in modelValue.slides"
        :key="item.id"
        type="button"
        class="slide-thumb"
        :class="{ active: index === slideIndex }"
        @click="chooseSlide(index)"
      >
        <strong>第 {{ index + 1 }} 页</strong>
        <div class="muted">{{ item.elements.length }} 个元素</div>
      </button>
      <button v-if="!readonly" type="button" class="danger" @click="removeSlide">删除当前页</button>
    </div>
    <div class="canvas-frame" ref="frame">
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
          :class="{ selected: element.id === selectedId }"
          :style="{
            left: element.x + 'px', top: element.y + 'px', width: element.w + 'px', height: element.h + 'px',
            color: element.color, fontSize: element.font_size + 'px', fontWeight: element.bold ? 700 : 400,
            textAlign: element.align,
          }"
          @pointerdown="drag(element, $event)"
        >
          <textarea
            v-if="!readonly && element.id === selectedId"
            v-model="element.text"
            data-testid="ppt-text"
            @pointerdown.stop
            @change="touch"
            :style="{ width: '100%', height: '100%', border: 0, resize: 'none', background: 'transparent', font: 'inherit', color: 'inherit', textAlign: 'inherit', fontWeight: 'inherit' }"
          />
          <div v-else>{{ element.text }}</div>
          <button v-if="!readonly && element.id === selectedId" type="button" class="handle drag" @pointerdown.stop.prevent="drag(element, $event)">拖动</button>
          <button v-if="!readonly && element.id === selectedId" type="button" class="handle resize" @pointerdown.stop.prevent="resize(element, $event)">缩放</button>
        </div>
      </div>
    </div>
    <div class="inspector" v-if="slide">
      <button v-if="!readonly" type="button" class="ghost" data-testid="add-text" @click="addText">添加文本</button>
      <label>背景
        <input v-model="slide.background" :disabled="readonly" maxlength="7" @change="touch" />
      </label>
      <template v-if="selected">
        <label>字号
          <input v-model.number="selected.font_size" :disabled="readonly" type="number" min="12" max="96" @change="touch" />
        </label>
        <label>颜色
          <input v-model="selected.color" :disabled="readonly" maxlength="7" @change="touch" />
        </label>
        <label>对齐
          <select v-model="selected.align" :disabled="readonly" @change="touch">
            <option value="left">左齐</option>
            <option value="center">居中</option>
            <option value="right">右齐</option>
          </select>
        </label>
        <button v-if="!readonly" type="button" class="ghost" @click="selected.bold = !selected.bold; touch()">{{ selected.bold ? '取消加粗' : '加粗' }}</button>
        <button v-if="!readonly" type="button" class="danger" @click="removeElement">删除元素</button>
      </template>
    </div>
  </div>
</template>
