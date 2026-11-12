<script setup>
import { computed, onMounted, ref, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import {
  createDocument,
  createFolder,
  deleteDocument,
  deleteFolder,
  exportDocument,
  getFolder,
  importDocument,
  moveDocument,
  searchDocuments,
  searchFolders,
  updateDocument,
  updateFolder,
} from '../api/document'
import KindMark from '../components/KindMark.vue'
import { KINDS, formatTime } from '../editors/kinds'
import { useSessionStore } from '../stores/session'

const session = useSessionStore()
const route = useRoute()
const router = useRouter()
const documents = ref([])
const folders = ref([])
const allFolders = ref([])
const currentFolder = ref(null)
const kind = ref('')
const title = ref('')
const error = ref('')
const pending = ref(false)
const renaming = ref(null)
const renameTitle = ref('')
const deleting = ref(null)
const deletingFolder = ref(null)
const makingFolder = ref(false)
const folderName = ref('')
const moving = ref(null)
const moveTarget = ref('0')
const view = ref('list')
const fileInput = ref(null)
const viewKey = 'yunjian-library-view'

const folderId = computed(() => {
  const raw = route.query.folder
  const value = Array.isArray(raw) ? raw[0] : raw
  return value && value !== '0' ? String(value) : ''
})
const searching = computed(() => title.value.trim() !== '')
const ancestors = computed(() => currentFolder.value?.ancestors || [])
const initial = computed(() => (session.account?.display_name || session.account?.username || '云').slice(0, 1))
const emptyTitle = computed(() => {
  if (searching.value) return '没有匹配的内容'
  if (currentFolder.value) return '这个文件夹是空的'
  return '还没有文档'
})
const emptyText = computed(() => {
  if (searching.value) return '换一个词，或清空搜索看看当前文件夹。'
  if (currentFolder.value) return '可以在这里新建文档，或把已有文档移动进来。'
  return '新建一份，或上传 docx、xlsx、pptx、txt、csv。'
})

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

function openFolder(id) {
  title.value = ''
  router.push(id ? { name: 'library', query: { folder: String(id) } } : { name: 'library' })
}

async function load() {
  error.value = ''
  const query = searching.value
  const current = folderId.value
  const stamp = `${query}|${current}|${kind.value}|${title.value}`
  try {
    const folderBody = query
      ? { name: title.value.trim(), page: 1, page_size: 200 }
      : { parent_id: current || '0', page: 1, page_size: 200 }
    const docBody = { kind: kind.value, title: title.value, page: 1, page_size: 60 }
    if (!query) docBody.folder_id = current || '0'
    const [folderPage, docPage] = await Promise.all([searchFolders(folderBody), searchDocuments(docBody)])
    if (`${searching.value}|${folderId.value}|${kind.value}|${title.value}` !== stamp) return
    folders.value = kind.value && !query ? [] : folderPage.data
    documents.value = docPage.data
    if (current && !query) {
      try {
        const detail = await getFolder(current)
        if (folderId.value !== current) return
        currentFolder.value = detail.data
      } catch (err) {
        error.value = err.message
        if (folderId.value === current) router.replace({ name: 'library' })
      }
      return
    }
    currentFolder.value = null
  } catch (err) {
    if (`${searching.value}|${folderId.value}|${kind.value}|${title.value}` !== stamp) return
    error.value = err.message
  }
}

async function create(nextKind) {
  pending.value = true
  try {
    const body = { kind: nextKind, title: KINDS[nextKind].untitled }
    if (folderId.value) body.folder_id = folderId.value
    const result = await createDocument(body)
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
    const body = { filename: file.name, content_base64: encodeBase64(bytes) }
    if (folderId.value) body.folder_id = folderId.value
    const result = await importDocument(body)
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
  renaming.value = { ...item, target: 'document' }
  renameTitle.value = item.title
}

function openRenameFolder(item) {
  renaming.value = { ...item, target: 'folder' }
  renameTitle.value = item.name
}

async function commitRename() {
  try {
    if (renaming.value.target === 'folder') {
      await updateFolder({ id: renaming.value.id, name: renameTitle.value })
    } else {
      await updateDocument({ id: renaming.value.id, revision: renaming.value.revision, title: renameTitle.value })
    }
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

async function commitDeleteFolder() {
  try {
    await deleteFolder(deletingFolder.value.id)
    deletingFolder.value = null
    await load()
  } catch (err) {
    error.value = err.message
  }
}

async function commitFolder() {
  try {
    await createFolder({ name: folderName.value, parent_id: folderId.value || '0' })
    makingFolder.value = false
    folderName.value = ''
    await load()
  } catch (err) {
    error.value = err.message
  }
}

async function openMove(item) {
  moving.value = item
  moveTarget.value = item.folder_id ? String(item.folder_id) : '0'
  error.value = ''
  try {
    const result = await searchFolders({ page: 1, page_size: 200 })
    allFolders.value = result.data
  } catch (err) {
    error.value = err.message
    allFolders.value = []
  }
}

function folderPath(folder) {
  const names = []
  let current = folder
  const seen = new Set()
  while (current && !seen.has(current.id)) {
    seen.add(current.id)
    names.unshift(current.name)
    current = allFolders.value.find((item) => item.id === current.parent_id)
  }
  return names.join(' / ')
}

const moveChoices = computed(() => [...allFolders.value].sort((a, b) => folderPath(a).localeCompare(folderPath(b), 'zh')))

async function commitMove() {
  try {
    await moveDocument({ id: moving.value.id, revision: moving.value.revision, folder_id: moveTarget.value || '0' })
    moving.value = null
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

let timer
watch([kind, title, folderId], () => {
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
        <button type="button" class="create-tile" data-testid="create-folder" :disabled="pending" @click="makingFolder = true; folderName = ''">
          <span class="mark folder-mark">
            <svg viewBox="0 0 24 24" aria-hidden="true"><path d="M3.5 7.5h6l1.5-2h9v11.5a1.5 1.5 0 0 1-1.5 1.5h-15a1.5 1.5 0 0 1-1.5-1.5v-8a1.5 1.5 0 0 1 1.5-1.5z" fill="none" stroke="currentColor" stroke-width="1.6" stroke-linejoin="round"/></svg>
          </span>
          文件夹
        </button>
        <button type="button" class="create-tile" data-testid="upload-button" :disabled="pending" title="上传 docx、xlsx、pptx、txt、csv 或 json" @click="pickFile">
          <span class="mark upload-mark">
            <svg viewBox="0 0 24 24" aria-hidden="true"><path d="M12 16V7M8.5 10.5 12 7l3.5 3.5M6 17.5h12" fill="none" stroke="currentColor" stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round"/></svg>
          </span>
          上传
        </button>
        <input ref="fileInput" class="file-input" data-testid="upload-input" type="file" accept=".docx,.xlsx,.pptx,.txt,.csv,.json,text/plain,text/csv,application/json" @change="onUpload" />
      </div>
      <nav class="crumbs" data-testid="folder-crumbs" aria-label="文件夹路径">
        <button type="button" @click="openFolder('')">我的文档</button>
        <template v-for="item in ancestors" :key="item.id">
          <span>/</span>
          <button type="button" @click="openFolder(item.id)">{{ item.name }}</button>
        </template>
        <template v-if="currentFolder">
          <span>/</span>
          <strong>{{ currentFolder.name }}</strong>
        </template>
      </nav>
      <p class="search-note" v-if="searching">正在搜索全部文档和文件夹</p>
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
      <div v-if="(folders.length || documents.length) && view === 'list'" class="doc-list" data-testid="library-view">
        <article v-for="item in folders" :key="`folder-${item.id}`" class="doc-row" data-testid="folder-card">
          <span class="mark folder-mark">
            <svg viewBox="0 0 24 24" aria-hidden="true"><path d="M3.5 7.5h6l1.5-2h9v11.5a1.5 1.5 0 0 1-1.5 1.5h-15a1.5 1.5 0 0 1-1.5-1.5v-8a1.5 1.5 0 0 1 1.5-1.5z" fill="none" stroke="currentColor" stroke-width="1.6" stroke-linejoin="round"/></svg>
          </span>
          <button type="button" class="doc-main" @click="openFolder(item.id)">
            <strong>{{ item.name }}</strong>
            <span>文件夹</span>
          </button>
          <div class="doc-actions">
            <button class="text-button" type="button" @click="openRenameFolder(item)">重命名</button>
            <button class="text-button danger" type="button" data-testid="delete-folder" @click="deletingFolder = item">删除</button>
          </div>
        </article>
        <article v-for="item in documents" :key="item.id" class="doc-row" data-testid="doc-card">
          <KindMark :kind="item.kind" />
          <button type="button" class="doc-main" @click="router.push(`/doc/${item.id}`)">
            <strong>{{ item.title }}</strong>
            <span>{{ meta(item) }}</span>
          </button>
          <div class="doc-actions">
            <button class="text-button" type="button" data-testid="download-doc" @click="download(item)">下载</button>
            <button class="text-button" type="button" data-testid="move-doc" @click="openMove(item)">移动</button>
            <button class="text-button" type="button" @click="openRename(item)">重命名</button>
            <button class="text-button danger" type="button" @click="deleting = item">删除</button>
          </div>
        </article>
      </div>
      <div v-else-if="folders.length || documents.length" class="doc-grid" data-testid="library-view">
        <article v-for="item in folders" :key="`folder-${item.id}`" class="doc-card" data-testid="folder-card">
          <button type="button" class="doc-main" @click="openFolder(item.id)">
            <span class="mark folder-mark">
              <svg viewBox="0 0 24 24" aria-hidden="true"><path d="M3.5 7.5h6l1.5-2h9v11.5a1.5 1.5 0 0 1-1.5 1.5h-15a1.5 1.5 0 0 1-1.5-1.5v-8a1.5 1.5 0 0 1 1.5-1.5z" fill="none" stroke="currentColor" stroke-width="1.6" stroke-linejoin="round"/></svg>
            </span>
            <strong>{{ item.name }}</strong>
          </button>
          <span class="doc-meta">文件夹</span>
          <div class="doc-actions">
            <button class="text-button" type="button" @click="openRenameFolder(item)">重命名</button>
            <button class="text-button danger" type="button" @click="deletingFolder = item">删除</button>
          </div>
        </article>
        <article v-for="item in documents" :key="item.id" class="doc-card" data-testid="doc-card">
          <button type="button" class="doc-main" @click="router.push(`/doc/${item.id}`)">
            <KindMark :kind="item.kind" />
            <strong>{{ item.title }}</strong>
          </button>
          <span class="doc-meta">{{ meta(item) }}</span>
          <div class="doc-actions">
            <button class="text-button" type="button" data-testid="download-doc" @click="download(item)">下载</button>
            <button class="text-button" type="button" data-testid="move-doc" @click="openMove(item)">移动</button>
            <button class="text-button" type="button" @click="openRename(item)">重命名</button>
            <button class="text-button danger" type="button" @click="deleting = item">删除</button>
          </div>
        </article>
      </div>
      <div class="empty" v-else data-testid="empty-library">
        <h2>{{ emptyTitle }}</h2>
        <p class="muted">{{ emptyText }}</p>
      </div>
    </main>
    <div class="modal-back" v-if="makingFolder">
      <div class="modal">
        <h3>新建文件夹</h3>
        <input data-testid="folder-name" v-model="folderName" placeholder="文件夹名称" @keyup.enter="commitFolder" />
        <div class="modal-actions">
          <button class="primary" type="button" data-testid="folder-save" @click="commitFolder">创建</button>
          <button class="ghost" type="button" @click="makingFolder = false">取消</button>
        </div>
      </div>
    </div>
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
    <div class="modal-back" v-if="deletingFolder">
      <div class="modal">
        <h3>删除文件夹「{{ deletingFolder.name }}」？</h3>
        <p class="muted">文件夹里还有文档或子文件夹时，需要先移走。</p>
        <div class="modal-actions">
          <button class="danger" type="button" data-testid="confirm-delete-folder" @click="commitDeleteFolder">删除</button>
          <button class="ghost" type="button" @click="deletingFolder = null">取消</button>
        </div>
      </div>
    </div>
    <div class="modal-back" v-if="moving">
      <div class="modal">
        <h3>移动「{{ moving.title }}」</h3>
        <div class="move-list" data-testid="move-list">
          <button type="button" data-testid="move-target" :aria-pressed="moveTarget === '0'" @click="moveTarget = '0'">我的文档</button>
          <button v-for="item in moveChoices" :key="item.id" type="button" data-testid="move-target" :aria-pressed="moveTarget === String(item.id)" @click="moveTarget = String(item.id)">
            {{ folderPath(item) }}
          </button>
        </div>
        <div class="modal-actions">
          <button class="primary" type="button" data-testid="confirm-move" @click="commitMove">移动</button>
          <button class="ghost" type="button" @click="moving = null">取消</button>
        </div>
      </div>
    </div>
  </div>
</template>
