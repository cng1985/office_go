<script setup>
import { computed, onBeforeUnmount, onMounted, ref, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { getDocument, getVersion, restoreVersion, saveDocument, searchVersions } from '../api/document'
import { ApiError } from '../api/http'
import { normalizeWorkbook } from '../editors/excel'
import { formatTime } from '../editors/kinds'
import { applySaveSnapshot } from '../editors/save'
import { mergeVersionPage } from '../editors/versions'
import ExcelEditor from '../components/ExcelEditor.vue'
import PptEditor from '../components/PptEditor.vue'
import WordEditor from '../components/WordEditor.vue'

const route = useRoute()
const router = useRouter()
const doc = ref(null)
const draft = ref(null)
const saved = ref('')
const note = ref('')
const error = ref('')
const status = ref('已保存')
const pending = ref(false)
const drawer = ref(false)
const versions = ref([])
const versionTotal = ref(0)
const versionPage = ref(1)
const versionsDone = ref(false)
const versionsLoading = ref(false)
const preview = ref(null)

const dirty = computed(() => doc.value && JSON.stringify({ title: doc.value.title, content: draft.value }) !== saved.value)
const excelPreview = computed(() => (preview.value && doc.value?.kind === 'excel' ? normalizeWorkbook(preview.value.content) : null))

function asDraft(kind, content) {
  return kind === 'excel' ? normalizeWorkbook(content) : content
}

async function load() {
  error.value = ''
  preview.value = null
  const result = await getDocument(route.params.id)
  doc.value = result.data
  draft.value = structuredClone(asDraft(doc.value.kind, result.data.content))
  saved.value = JSON.stringify({ title: doc.value.title, content: draft.value })
  status.value = '已保存到云端'
  drawer.value = false
  await loadVersions()
}

function applyVersionPage(existing, result, requestedPage) {
  const merged = mergeVersionPage(existing, result.data, result.total, result.page || requestedPage, result.page_size || 50)
  versions.value = merged.versions
  versionTotal.value = merged.total
  versionPage.value = merged.page
  versionsDone.value = merged.done
}

async function loadVersions() {
  versionsLoading.value = true
  try {
    const result = await searchVersions({ document_id: doc.value.id, page: 1, page_size: 50 })
    applyVersionPage([], result, 1)
  } finally {
    versionsLoading.value = false
  }
}

async function loadMoreVersions() {
  if (versionsLoading.value || versionsDone.value || versions.value.length >= versionTotal.value) return
  versionsLoading.value = true
  const page = versionPage.value + 1
  try {
    const result = await searchVersions({ document_id: doc.value.id, page, page_size: 50 })
    applyVersionPage(versions.value, result, page)
  } catch (err) {
    error.value = err.message
  } finally {
    versionsLoading.value = false
  }
}

function cloneContent(value) {
  return JSON.parse(JSON.stringify(value))
}

async function save() {
  if (!doc.value || pending.value) return
  pending.value = true
  error.value = ''
  const submitted = {
    title: doc.value.title,
    note: note.value,
    content: cloneContent(draft.value),
  }
  try {
    const result = await saveDocument({
      id: doc.value.id,
      revision: doc.value.revision,
      title: submitted.title,
      note: submitted.note,
      content: submitted.content,
    })
    applySaved(result.data.document, result.data.new_version, submitted)
    if (note.value === submitted.note) note.value = ''
  } catch (err) {
    handleConflict(err)
  } finally {
    pending.value = false
  }
}

function applySaved(document, created, submitted) {
  const current = { title: doc.value.title, content: draft.value }
  const next = submitted ? applySaveSnapshot(current, submitted, document) : null
  if (next?.pendingEdits) {
    doc.value.revision = document.revision
    doc.value.version_no = document.version_no
    doc.value.version_id = document.version_id
    doc.value.updated_at = document.updated_at
    saved.value = JSON.stringify({ title: next.savedTitle, content: next.savedContent })
    status.value = '有未保存的修改'
  } else {
    doc.value = document
    draft.value = structuredClone(asDraft(document.kind, document.content))
    saved.value = JSON.stringify({ title: doc.value.title, content: draft.value })
    status.value = created ? `已保存为版本 ${document.version_no}` : '内容没有变化'
  }
  preview.value = null
  loadVersions()
}

async function openVersion(item) {
  const result = await getVersion({ document_id: doc.value.id, version_id: item.id })
  preview.value = result.data
}

async function restore() {
  pending.value = true
  try {
    const result = await restoreVersion({
      id: doc.value.id,
      version_id: preview.value.id,
      revision: doc.value.revision,
      note: note.value,
    })
    applySaved(result.data.document, true)
  } catch (err) {
    handleConflict(err)
  } finally {
    pending.value = false
  }
}

function handleConflict(err) {
  error.value = err.message
  if (err instanceof ApiError && (err.code === 10004 || err.code === 409)) {
    status.value = '云端有更新'
  }
}

function onKeydown(event) {
  if ((event.metaKey || event.ctrlKey) && event.key.toLowerCase() === 's') {
    event.preventDefault()
    save()
  }
}

watch(() => route.params.id, load)
watch(dirty, (value) => {
  if (value) status.value = '有未保存的修改'
})
onMounted(() => {
  window.addEventListener('keydown', onKeydown)
  load().catch((err) => { error.value = err.message })
})
onBeforeUnmount(() => window.removeEventListener('keydown', onKeydown))
</script>

<template>
  <p class="error" v-if="!doc && error" style="margin: 24px">{{ error }}</p>
  <div class="editor-shell" :class="{ 'word-mode': doc.kind === 'word', 'excel-mode': doc.kind === 'excel', 'ppt-mode': doc.kind === 'ppt' }" v-if="doc && draft">
    <header class="editor-bar" v-if="doc.kind !== 'excel' && doc.kind !== 'ppt'">
      <router-link v-if="doc.kind === 'word'" class="word-back" to="/" aria-label="返回">‹</router-link>
      <router-link v-else class="brand" to="/"><span class="brand-mark">笺</span></router-link>
      <span v-if="doc.kind === 'word'" class="word-doc-icon" aria-hidden="true">W</span>
      <input class="title-input" data-testid="doc-title" v-model="doc.title" :disabled="!!preview" />
      <input v-if="doc.kind !== 'word'" class="note-input" data-testid="version-note" v-model="note" placeholder="版本说明" />
      <span class="status" data-testid="save-status">{{ status }}</span>
      <button class="primary" :class="{ 'word-save': doc.kind === 'word' }" type="button" data-testid="save-button" :disabled="pending || !!preview" @click="save">{{ doc.kind === 'word' ? '保存' : '保存到云端' }}</button>
      <button class="ghost" type="button" data-testid="version-button" @click="drawer = !drawer">{{ doc.kind === 'word' ? '历史' : '版本' }}</button>
    </header>
    <p class="error" v-if="error" style="margin: 12px 28px">{{ error }}</p>
    <div class="editor-body" :class="{ closed: !drawer }">
      <section class="stage">
        <div class="preview-banner" v-if="preview">
          <span>正在查看版本 {{ preview.version_no }} · {{ preview.note || '无说明' }} · {{ formatTime(preview.created_at) }}</span>
          <span>
            <button class="ghost" type="button" @click="preview = null">返回编辑</button>
            <button class="primary" type="button" data-testid="restore-button" @click="restore">恢复此版本</button>
          </span>
        </div>
        <WordEditor v-if="doc.kind === 'word' && !preview" v-model="draft" @save="save" @versions="drawer = !drawer" />
        <WordEditor v-else-if="doc.kind === 'word'" :model-value="preview.content" readonly />
        <ExcelEditor
          v-else-if="doc.kind === 'excel' && !preview"
          v-model="draft"
          :title="doc.title"
          :status="status"
          :pending="pending"
          @update:title="doc.title = $event"
          @save="save"
          @versions="drawer = !drawer"
        />
        <ExcelEditor
          v-else-if="doc.kind === 'excel'"
          :model-value="excelPreview"
          :title="doc.title"
          :status="status"
          readonly
          previewing
          @versions="drawer = !drawer"
        />
        <PptEditor
          v-else-if="!preview"
          v-model="draft"
          :title="doc.title"
          :status="status"
          :pending="pending"
          @update:title="doc.title = $event"
          @save="save"
          @versions="drawer = !drawer"
        />
        <PptEditor
          v-else
          :model-value="preview.content"
          :title="doc.title"
          :status="status"
          readonly
          previewing
          @versions="drawer = !drawer"
        />
      </section>
      <aside class="drawer" v-if="drawer" data-testid="version-drawer">
        <h3>版本记录</h3>
        <input v-if="doc.kind === 'word' || doc.kind === 'excel' || doc.kind === 'ppt'" class="note-input" data-testid="version-note" v-model="note" placeholder="版本说明" />
        <p class="muted">恢复会新增一个版本，不会抹掉后来的历史。</p>
        <button
          v-for="item in versions"
          :key="item.id"
          type="button"
          class="version-item"
          :class="{ active: preview?.id === item.id }"
          data-testid="version-item"
          @click="openVersion(item)"
        >
          <strong>版本 {{ item.version_no }}</strong>
          <div>{{ item.note || '无说明' }}</div>
          <div class="muted">{{ formatTime(item.created_at) }} · {{ item.byte_size }} 字节</div>
        </button>
        <button
          v-if="!versionsDone && versions.length < versionTotal"
          type="button"
          class="ghost"
          data-testid="load-more-versions"
          :disabled="versionsLoading"
          @click="loadMoreVersions"
        >
          更早的版本（{{ versions.length }}/{{ versionTotal }}）
        </button>
      </aside>
    </div>
  </div>
</template>
