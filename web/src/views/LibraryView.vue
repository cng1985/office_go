<script setup>
import { computed, onMounted, ref, watch } from 'vue'
import { useRouter } from 'vue-router'
import { createDocument, deleteDocument, exportDocument, importDocument, searchDocuments, updateDocument } from '../api/document'
import KindMark from '../components/KindMark.vue'
import { KINDS, formatTime } from '../editors/kinds'
import { useSessionStore } from '../stores/session'

const session = useSessionStore()
const router = useRouter()
const documents = ref([])
const kind = ref('')
const title = ref('')
const error = ref('')
const pending = ref(false)
const renaming = ref(null)
const renameTitle = ref('')
const deleting = ref(null)
const view = ref('list')
const fileInput = ref(null)
const viewKey = 'yunjian-library-view'

function readView() {
  try {
    const saved = localStorage.getItem(viewKey)
    if (saved === 'grid' || saved === 'list') view.value = saved
  } catch {
    view.value = 'list'
  }
}

function setView(next) {
  view.value = next
  try {
    localStorage.setItem(viewKey, next)
  } catch {
    /* 浏览器不允许写入时只保留这次选择 */
  }
}

async function load() {
  error.value = ''
  try {
    const result = await searchDocuments({ kind: kind.value, title: title.value, page: 1, page_size: 60 })
    documents.value = result.data
  } catch (err) {
    error.value = err.message
  }
}

async function create(nextKind) {
  pending.value = true
  try {
    const result = await createDocument({ kind: nextKind, title: KINDS[nextKind].untitled })
    router.push(`/doc/${result.data.id}`)
  } catch (err) {
    error.value = err.message
  } finally {
    pending.value = false
  }
}

function pickFile() {
  fileInput.value?.click()
}

async function onUpload(event) {
  const file = event.target.files?.[0]
  event.target.value = ''
  if (!file) return
  await sendFile(file)
}

async function sendFile(file) {
  if (file.size > 1024 * 1024) {
    error.value = '文件超过 1MB 限制'
    return
  }
  pending.value = true
  error.value = ''
  try {
    const bytes = new Uint8Array(await file.arrayBuffer())
    const result = await importDocument({ filename: file.name, content_base64: encodeBase64(bytes) })
    title.value = ''
    kind.value = result.data.kind || ''
    await load()
  } catch (err) {
    error.value = err.message
  } finally {
    pending.value = false
  }
}

function encodeBase64(bytes) {
  let binary = ''
  const size = 0x8000
  for (let index = 0; index < bytes.length; index += size) {
    binary += String.fromCharCode(...bytes.subarray(index, index + size))
  }
  return btoa(binary)
}

async function download(item) {
  error.value = ''
  try {
    const result = await exportDocument(item.id)
    const file = result.data
    const binary = atob(file.content_base64 || '')
    const bytes = new Uint8Array(binary.length)
    for (let index = 0; index < binary.length; index += 1) bytes[index] = binary.charCodeAt(index)
    const blob = new Blob([bytes], { type: file.media_type || 'application/octet-stream' })
    const url = URL.createObjectURL(blob)
    const link = document.createElement('a')
    link.href = url
    link.download = file.filename || item.title
    document.body.appendChild(link)
    link.click()
    link.remove()
    URL.revokeObjectURL(url)
  } catch (err) {
    error.value = err.message
  }
}

function openRename(item) {
  renaming.value = item
  renameTitle.value = item.title
}

async function commitRename() {
  try {
    await updateDocument({ id: renaming.value.id, revision: renaming.value.revision, title: renameTitle.value })
    renaming.value = null
    await load()
  } catch (err) {
    error.value = err.message
  }
}

async function commitDelete() {
  try {
    await deleteDocument(deleting.value.id)
    deleting.value = null
    await load()
  } catch (err) {
    error.value = err.message
  }
}

async function signOut() {
  await session.logout()
  router.push('/login')
}

function meta(item) {
  return `${KINDS[item.kind].label} · 版本 ${item.version_no} · ${formatTime(item.updated_at)}`
}

const initial = computed(() => (session.account?.display_name || session.account?.username || '云').slice(0, 1))

let timer
watch([kind, title], () => {
  clearTimeout(timer)
  timer = setTimeout(load, 250)
})
onMounted(() => {
  readView()
  load()
})
</script>

<template>
  <div class="home">
    <header class="home-bar">
      <router-link class="home-brand" to="/">
        <span class="mark word">W</span>
        <strong>云笺</strong>
      </router-link>
      <label class="home-search">
        <svg viewBox="0 0 24 24" aria-hidden="true"><circle cx="11" cy="11" r="6" fill="none" stroke="#5f6368" stroke-width="1.8"/><path d="M16 16l4 4" stroke="#5f6368" stroke-width="1.8" stroke-linecap="round"/></svg>
        <input data-testid="library-search" v-model="title" placeholder="搜索" />
      </label>
      <span class="home-user" data-testid="current-user">
        <i>{{ initial }}</i>
        {{ session.account?.display_name }}
      </span>
      <button class="text-button" type="button" data-testid="logout" @click="signOut">退出</button>
    </header>
    <main class="library">
      <h1>新建</h1>
      <div class="create-row">
        <button type="button" class="create-tile" data-testid="create-word" :disabled="pending" @click="create('word')">
          <KindMark kind="word" />
          文稿
        </button>
        <button type="button" class="create-tile" data-testid="create-excel" :disabled="pending" @click="create('excel')">
          <KindMark kind="excel" />
          表格
        </button>
        <button type="button" class="create-tile" data-testid="create-ppt" :disabled="pending" @click="create('ppt')">
          <KindMark kind="ppt" />
          演示
        </button>
        <button type="button" class="create-tile" data-testid="upload-button" :disabled="pending" title="上传 docx、xlsx、pptx、txt、csv 或 json" @click="pickFile">
          <span class="mark upload-mark">
            <svg viewBox="0 0 24 24" aria-hidden="true"><path d="M12 16V7M8.5 10.5 12 7l3.5 3.5M6 17.5h12" fill="none" stroke="currentColor" stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round"/></svg>
          </span>
          上传
        </button>
        <input ref="fileInput" class="file-input" data-testid="upload-input" type="file" accept=".docx,.xlsx,.pptx,.txt,.csv,.json,text/plain,text/csv,application/json" @change="onUpload" />
      </div>
      <div class="library-bar">
        <div class="filters">
          <button type="button" :aria-pressed="kind === ''" @click="kind = ''">全部</button>
          <button type="button" :aria-pressed="kind === 'word'" @click="kind = 'word'">文稿</button>
          <button type="button" :aria-pressed="kind === 'excel'" @click="kind = 'excel'">表格</button>
          <button type="button" :aria-pressed="kind === 'ppt'" @click="kind = 'ppt'">演示</button>
        </div>
        <div class="view-switch" role="group" aria-label="显示方式">
          <button type="button" data-testid="view-list" :aria-pressed="view === 'list'" @click="setView('list')">
            <svg viewBox="0 0 24 24" aria-hidden="true"><path d="M8 7h12M8 12h12M8 17h12M4 7h.01M4 12h.01M4 17h.01" fill="none" stroke="currentColor" stroke-width="1.8" stroke-linecap="round"/></svg>
            列表
          </button>
          <button type="button" data-testid="view-grid" :aria-pressed="view === 'grid'" @click="setView('grid')">
            <svg viewBox="0 0 24 24" aria-hidden="true"><rect x="4" y="4" width="6" height="6" rx="1" fill="none" stroke="currentColor" stroke-width="1.8"/><rect x="14" y="4" width="6" height="6" rx="1" fill="none" stroke="currentColor" stroke-width="1.8"/><rect x="4" y="14" width="6" height="6" rx="1" fill="none" stroke="currentColor" stroke-width="1.8"/><rect x="14" y="14" width="6" height="6" rx="1" fill="none" stroke="currentColor" stroke-width="1.8"/></svg>
            网格
          </button>
        </div>
      </div>
      <p class="error" v-if="error">{{ error }}</p>
      <div v-if="documents.length && view === 'list'" class="doc-list" data-testid="library-view">
        <article v-for="item in documents" :key="item.id" class="doc-row" data-testid="doc-card">
          <KindMark :kind="item.kind" />
          <button type="button" class="doc-main" @click="router.push(`/doc/${item.id}`)">
            <strong>{{ item.title }}</strong>
            <span>{{ meta(item) }}</span>
          </button>
          <div class="doc-actions">
            <button class="text-button" type="button" data-testid="download-doc" @click="download(item)">下载</button>
            <button class="text-button" type="button" @click="openRename(item)">重命名</button>
            <button class="text-button danger" type="button" @click="deleting = item">删除</button>
          </div>
        </article>
      </div>
      <div v-else-if="documents.length" class="doc-grid" data-testid="library-view">
        <article v-for="item in documents" :key="item.id" class="doc-card" data-testid="doc-card">
          <button type="button" class="doc-main" @click="router.push(`/doc/${item.id}`)">
            <KindMark :kind="item.kind" />
            <strong>{{ item.title }}</strong>
          </button>
          <span class="doc-meta">{{ meta(item) }}</span>
          <div class="doc-actions">
            <button class="text-button" type="button" data-testid="download-doc" @click="download(item)">下载</button>
            <button class="text-button" type="button" @click="openRename(item)">重命名</button>
            <button class="text-button danger" type="button" @click="deleting = item">删除</button>
          </div>
        </article>
      </div>
      <div class="empty" v-else data-testid="empty-library">
        <h2>还没有文档</h2>
        <p class="muted">新建一份，或上传 docx、xlsx、pptx、txt、csv。</p>
      </div>
    </main>
    <div class="modal-back" v-if="renaming">
      <div class="modal">
        <h3>重命名</h3>
        <input data-testid="rename-input" v-model="renameTitle" />
        <div class="modal-actions">
          <button class="primary" type="button" data-testid="rename-save" @click="commitRename">保存</button>
          <button class="ghost" type="button" @click="renaming = null">取消</button>
        </div>
      </div>
    </div>
    <div class="modal-back" v-if="deleting">
      <div class="modal">
        <h3>删除「{{ deleting.title }}」？</h3>
        <p class="muted">文档和全部历史版本会一起删除。</p>
        <div class="modal-actions">
          <button class="danger" type="button" data-testid="confirm-delete" @click="commitDelete">删除</button>
          <button class="ghost" type="button" @click="deleting = null">取消</button>
        </div>
      </div>
    </div>
  </div>
</template>
