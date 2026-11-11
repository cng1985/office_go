<script setup>
import { computed, ref, watch } from 'vue'
import { columnLabel, displayCell } from '../editors/formula'

const props = defineProps({
  modelValue: { type: Object, required: true },
  readonly: { type: Boolean, default: false },
})
const emit = defineEmits(['update:modelValue'])

const selected = ref({ row: 1, col: 1 })
const draft = ref(cell(1, 1)?.input || '')

const rows = computed(() => Array.from({ length: props.modelValue.row_count }, (_, index) => index + 1))
const cols = computed(() => Array.from({ length: props.modelValue.col_count }, (_, index) => index + 1))

function cell(row, col) {
  return props.modelValue.cells.find((item) => item.row === row && item.col === col)
}

function shown(row, col) {
  return displayCell(props.modelValue, row, col)
}

function select(row, col) {
  commit()
  selected.value = { row, col }
  draft.value = cell(row, col)?.input || ''
}

function commit(event) {
  if (props.readonly) return
  if (event?.target && typeof event.target.value === 'string') draft.value = event.target.value
  const { row, col } = selected.value
  const existing = cell(row, col)
  const input = draft.value
  if (!input && !existing?.bold) {
    props.modelValue.cells = props.modelValue.cells.filter((item) => !(item.row === row && item.col === col))
  } else if (existing) {
    existing.input = input
  } else if (input) {
    props.modelValue.cells.push({ row, col, input, bold: false })
  }
  emit('update:modelValue', props.modelValue)
}

function toggleBold() {
  if (props.readonly) return
  const { row, col } = selected.value
  const existing = cell(row, col)
  if (existing) existing.bold = !existing.bold
  else props.modelValue.cells.push({ row, col, input: draft.value, bold: true })
  emit('update:modelValue', props.modelValue)
}

function grow(kind) {
  if (props.readonly) return
  if (kind === 'row' && props.modelValue.row_count < 100) props.modelValue.row_count += 5
  if (kind === 'col' && props.modelValue.col_count < 26) props.modelValue.col_count += 2
  emit('update:modelValue', props.modelValue)
}

watch(() => props.modelValue, () => {
  draft.value = cell(selected.value.row, selected.value.col)?.input || ''
})
</script>

<template>
  <div data-testid="excel-editor">
    <div class="toolbar" v-if="!readonly">
      <button type="button" class="ghost" @click="toggleBold">加粗</button>
      <button type="button" class="ghost" @click="grow('row')">增加 5 行</button>
      <button type="button" class="ghost" @click="grow('col')">增加 2 列</button>
      <label>工作表
        <input v-model="modelValue.sheet_name" maxlength="31" @change="emit('update:modelValue', modelValue)" />
      </label>
    </div>
    <div class="formula-bar">
      <span>{{ columnLabel(selected.col) }}{{ selected.row }}</span>
      <input
        data-testid="formula-input"
        v-model="draft"
        :readonly="readonly"
        @input="commit"
        @keydown.enter.prevent="commit"
      />
    </div>
    <div class="sheet-wrap">
      <table class="sheet">
        <thead>
          <tr>
            <th></th>
            <th v-for="col in cols" :key="col">{{ columnLabel(col) }}</th>
          </tr>
        </thead>
        <tbody>
          <tr v-for="row in rows" :key="row">
            <th>{{ row }}</th>
            <td
              v-for="col in cols"
              :key="col"
              data-testid="excel-cell"
              :class="{ selected: selected.row === row && selected.col === col, bold: cell(row, col)?.bold, error: shown(row, col).startsWith('#') }"
              @click="select(row, col)"
            >{{ shown(row, col) }}</td>
          </tr>
        </tbody>
      </table>
    </div>
  </div>
</template>
