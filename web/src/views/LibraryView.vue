<script setup>
import { computed, onMounted, onUnmounted, ref, watch } from 'vue'
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
import { encodeBase64, formatBytes, isUploadCancel, uploadPercent } from '../editors/upload'
import { searchTeams } from '../api/team'
import { useSessionStore } from '../stores/session'

const session = useSessionStore()
const route = useRoute()
const router = useRouter()
const documents = ref([])
const folders = ref([])
const foldersBySpace = ref({})
const teams = ref([])
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
const upload = ref(null)
const openIds = ref({})
let uploadToken = 0
let uploadAbort = null
const viewKey = 'yunjian-library-view'
const treeKey = 'yunjian-folder-tree'

const folderId = computed(() => {
  const raw = route.query.folder
  const value = Array.isArray(raw) ? raw[0] : raw
  return value && value !== '0' ? String(value) : ''
})
const teamId = computed(() => {
  const raw = route.query.team
  const value = Array.isArray(raw) ? raw[0] : raw
  return value && value !== '0' ? String(value) : ''
})
const currentTeam = computed(() => teams.value.find((item) => String(item.id) === teamId.value))
const spaceName = computed(() => currentTeam.value?.name || (teamId.value ? '团队文档' : '我的文档'))
const allFolders = computed(() => foldersBySpace.value[teamId.value] || [])
const treeRoots = computed(() => [
  { id: '', key: 'personal', name: '我的文档', testId: 'space-personal' },
  ...teams.value.map((team) => ({
    id: String(team.id),
    key: `team-${team.id}`,
    name: team.name,
    testId: `space-team-${team.id}`,
  })),
])
const searching = computed(() => title.value.trim() !== '')
const ancestors = computed(() => currentFolder.value?.ancestors || [])
const initial = computed(() => (session.account?.display_name || session.account?.username || '云').slice(0, 1))
const emptyTitle = computed(() => {
  if (searching.value) return '没有匹配的内容'
  if (currentFolder.value) return '这个文件夹是空的'
  if (teamId.value) return '这个团队还没有文档'
  return '还没有文档'
})
const emptyText = computed(() => {
  if (searching.value) return '换一个词，或清空搜索看看当前文件夹。'
  if (currentFolder.value) return '可以在这里新建文档，或把已有文档移动进来。'
  return '新建一份，或上传 docx、xlsx、pptx、pdf、txt、csv。'
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

function withSpace(body) {
  if (teamId.value) body.team_id = teamId.value
  return body
}

function spaceStorageKey(id) {
  return id ? `space:${id}` : 'space:personal'
}

function rememberFolders(space, rows) {
  foldersBySpace.value = { ...foldersBySpace.value, [space || '']: rows || [] }
}

function openSpace(id) {
  title.value = ''
  kind.value = ''
  const key = spaceStorageKey(id)
  if (!openIds.value[key]) {
    openIds.value = { ...openIds.value, [key]: true }
    saveTree()
  }
  router.push({ name: 'library', query: id ? { team: String(id) } : {} })
}

function readTree() {
  try {
    const saved = JSON.parse(localStorage.getItem(treeKey) || '{}')
    if (saved && typeof saved === 'object') openIds.value = saved
  } catch {
    openIds.value = {}
  }
}

function saveTree() {
  try {
    localStorage.setItem(treeKey, JSON.stringify(openIds.value))
  } catch {
    /* 浏览器不允许写入时只保留这次展开状态 */
  }
}

function toggleFolder(id) {
  openIds.value = { ...openIds.value, [id]: !openIds.value[id] }
  saveTree()
}

function expandFolder(id) {
  if (!id || openIds.value[id]) return
  openIds.value = { ...openIds.value, [id]: true }
  saveTree()
}

function selectTree(space, id) {
  const key = spaceStorageKey(space)
  if (!openIds.value[key]) openIds.value = { ...openIds.value, [key]: true }
  expandFolder(id)
  title.value = ''
  const query = {}
  if (space) query.team = String(space)
  if (id) query.folder = String(id)
  router.push({ name: 'library', query })
}

function toggleSpace(id) {
  const key = spaceStorageKey(id)
  const opening = !openIds.value[key]
  openIds.value = { ...openIds.value, [key]: opening }
  saveTree()
  if (opening) loadSpaceFolders(id)
}

const spaceTokens = {}
async function loadSpaceFolders(space) {
  const key = space || ''
  const token = (spaceTokens[key] || 0) + 1
  spaceTokens[key] = token
  try {
    const body = { page: 1, page_size: 200 }
    if (space) body.team_id = space
    const page = await searchFolders(body)
    if (spaceTokens[key] !== token) return
    rememberFolders(key, page.data)
  } catch {
    /* 展开失败时保留已经看到的目录 */
  }
}

function expandAncestors() {
  const next = { ...openIds.value }
  let current = folderId.value
  const seen = new Set()
  if (current) next[current] = true
  while (current && !seen.has(current)) {
    seen.add(current)
    const folder = allFolders.value.find((item) => String(item.id) === String(current))
    if (!folder?.parent_id || folder.parent_id === '0') break
    next[String(folder.parent_id)] = true
    current = String(folder.parent_id)
  }
  openIds.value = next
  saveTree()
}

function rowsFor(space) {
  const children = new Map()
  for (const folder of foldersBySpace.value[space || ''] || []) {
    const parent = folder.parent_id && folder.parent_id !== '0' ? String(folder.parent_id) : '0'
    if (!children.has(parent)) children.set(parent, [])
    children.get(parent).push(folder)
  }
  for (const list of children.values()) list.sort((a, b) => a.name.localeCompare(b.name, 'zh'))
  const rows = []
  const walk = (parent, depth) => {
    for (const folder of children.get(parent) || []) {
      const kids = children.get(String(folder.id)) || []
      rows.push({ folder, depth, hasChildren: kids.length > 0, open: !!openIds.value[String(folder.id)] })
      if (openIds.value[String(folder.id)]) walk(String(folder.id), depth + 1)
    }
  }
  walk('0', 0)
  return rows
}

function openFolder(id) {
  title.value = ''
  const query = {}
  if (teamId.value) query.team = teamId.value
  if (id) query.folder = String(id)
  router.push({ name: 'library', query })
}

function restoredSpaces() {
  const spaces = []
  if (openIds.value['space:personal']) spaces.push('')
  for (const team of teams.value) {
    const id = String(team.id)
    if (openIds.value[spaceStorageKey(id)]) spaces.push(id)
  }
  return spaces
}

async function loadTeams() {
  try {
    const result = await searchTeams({ page: 1, page_size: 200 })
    teams.value = result.data
  } catch {
    teams.value = []
  }
  for (const space of restoredSpaces()) {
    if (space === teamId.value) continue
    loadSpaceFolders(space)
  }
}

async function load() {
  error.value = ''
  const query = searching.value
  const current = folderId.value
  const space = teamId.value
  const stamp = `${query}|${current}|${space}|${kind.value}|${title.value}`
  try {
    const folderBody = withSpace(query
      ? { name: title.value.trim(), page: 1, page_size: 200 }
      : { parent_id: current || '0', page: 1, page_size: 200 })
    const docBody = withSpace({ kind: kind.value, title: title.value, page: 1, page_size: 60 })
    if (!query) docBody.folder_id = current || '0'
    const folderPromise = searchFolders(folderBody).catch((err) => {
      if (!query) throw err
      return { data: [], failed: err }
    })
    const treePromise = searchFolders(withSpace({ page: 1, page_size: 200 })).catch(() => null)
    const [folderPage, docPage, treePage] = await Promise.all([folderPromise, searchDocuments(docBody), treePromise])
    if (`${searching.value}|${folderId.value}|${teamId.value}|${kind.value}|${title.value}` !== stamp) return
    if (treePage) {
      rememberFolders(space, treePage.data)
      if (folderId.value) {
        openIds.value = { ...openIds.value, [spaceStorageKey(space)]: true }
        saveTree()
      }
      expandAncestors()
    }
    if (folderPage.failed) error.value = folderPage.failed.message
    folders.value = kind.value && !query ? [] : folderPage.data
    documents.value = docPage.data
    if (current && !query) {
      try {
        const detail = await getFolder(current)
        if (folderId.value !== current) return
        currentFolder.value = detail.data
      } catch (err) {
        error.value = err.message
        if (folderId.value === current) router.replace({ name: 'library', query: teamId.value ? { team: teamId.value } : {} })
      }
      return
    }
    currentFolder.value = null
  } catch (err) {
    if (`${searching.value}|${folderId.value}|${teamId.value}|${kind.value}|${title.value}` !== stamp) return
    error.value = err.message
    if (teamId.value && err.message === '团队不存在') router.replace({ name: 'library' })
  }
}

async function create(nextKind) {
  pending.value = true
  try {
    const body = withSpace({ kind: nextKind, title: KINDS[nextKind].untitled })
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

function uploadLabel(item) {
  if (!item) return ''
  if (item.phase === 'error') return item.message || '上传失败'
  if (item.phase === 'done') return '上传完成'
  if (item.phase === 'import') return '正在导入'
  if (item.phase === 'upload') return '正在上传'
  if (item.phase === 'encode') return '正在准备'
  return '正在读取'
}

function cancelUpload() {
  uploadToken += 1
  uploadAbort?.abort()
  uploadAbort = null
  upload.value = null
  pending.value = false
}

function dismissUpload() {
  upload.value = null
}

async function sendFile(file) {
  const token = ++uploadToken
  uploadAbort?.abort()
  const controller = new AbortController()
  uploadAbort = controller
  const size = file.size || 0
  pending.value = true
  error.value = ''
  upload.value = { name: file.name, size, sent: 0, percent: 0, phase: 'read', message: '' }
  const patch = (next) => {
    if (token !== uploadToken) return
    upload.value = { ...upload.value, ...next }
  }
  try {
    const bytes = new Uint8Array(await file.arrayBuffer())
    if (token !== uploadToken) return
    const content = await encodeBase64(bytes, (done) => {
      patch({
        phase: 'encode',
        sent: done,
        percent: uploadPercent('encode', size ? done / size : 1),
      })
    }, controller.signal)
    if (token !== uploadToken) return
    patch({ phase: 'upload', sent: 0, percent: uploadPercent('upload', 0) })
    const body = withSpace({ filename: file.name, content_base64: content })
    if (folderId.value) body.folder_id = folderId.value
    const result = await importDocument(body, {
      signal: controller.signal,
      onProgress: ({ loaded, total }) => {
        const ratio = total ? loaded / total : 0
        const phase = ratio >= 1 ? 'import' : 'upload'
        patch({
          phase,
          sent: Math.min(size, Math.round(size * ratio)),
          percent: uploadPercent(phase, ratio),
        })
      },
    })
    if (token !== uploadToken) return
    patch({ phase: 'import', sent: size, percent: 99 })
    title.value = ''
    kind.value = result.data.kind || ''
    await load()
    if (token !== uploadToken) return
    patch({ phase: 'done', sent: size, percent: 100, message: '' })
    await new Promise((resolve) => setTimeout(resolve, 900))
    if (token === uploadToken && upload.value?.phase === 'done') upload.value = null
  } catch (err) {
    if (token !== uploadToken || isUploadCancel(err)) return
    patch({ phase: 'error', message: err.message || '上传失败' })
  } finally {
    if (token === uploadToken) pending.value = false
  }
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
    await createFolder(withSpace({ name: folderName.value, parent_id: folderId.value || '0' }))
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
    const result = await searchFolders(withSpace({ page: 1, page_size: 200 }))
    rememberFolders(teamId.value, result.data)
  } catch (err) {
    error.value = err.message
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
watch(teamId, (id) => {
  folders.value = []
  documents.value = []
  currentFolder.value = null
  const key = spaceStorageKey(id)
  if (!openIds.value[key]) {
    openIds.value = { ...openIds.value, [key]: true }
    saveTree()
  }
})
watch([kind, title, folderId, teamId], () => {
  clearTimeout(timer)
  timer = setTimeout(load, 250)
})
onUnmounted(() => {
  uploadToken += 1
  uploadAbort?.abort()
})
onMounted(() => {
  readView()
  readTree()
  const key = spaceStorageKey(teamId.value)
  if (!openIds.value[key]) {
    openIds.value = { ...openIds.value, [key]: true }
    saveTree()
  }
  loadTeams()
  load()
})
</script>

<template>
  <div class="home library-home">
    <header class="home-bar">
      <router-link class="home-brand" to="/">
        <span class="mark word">W</span>
        <strong>云笺</strong>
      </router-link>
      <label class="home-search">
        <svg viewBox="0 0 24 24" aria-hidden="true"><circle cx="11" cy="11" r="6" fill="none" stroke="#5f6368" stroke-width="1.8"/><path d="M16 16l4 4" stroke="#5f6368" stroke-width="1.8" stroke-linecap="round"/></svg>
        <input data-testid="library-search" v-model="title" placeholder="搜索" />
      </label>
      <router-link class="home-user" to="/account" data-testid="account-link" title="个人中心">
        <i>{{ initial }}</i>
        <span data-testid="current-user">{{ session.account?.display_name }}</span>
      </router-link>
      <button class="text-button" type="button" data-testid="logout" @click="signOut">退出</button>
    </header>
    <div class="library-frame">
      <aside class="library-side">
      <div class="folder-tree" data-testid="folder-tree" aria-label="文件夹">
        <div v-for="root in treeRoots" :key="root.key" class="tree-branch">
          <div class="tree-row" :data-testid="root.id ? null : 'tree-root'" :aria-current="teamId === root.id && !folderId ? 'page' : null">
            <button type="button" class="tree-chevron" :data-testid="`tree-toggle-${root.key}`" :aria-label="openIds[spaceStorageKey(root.id)] ? '折叠' : '展开'" @click="toggleSpace(root.id)">
              <svg viewBox="0 0 16 16" :class="{ open: openIds[spaceStorageKey(root.id)] }"><path d="M6 4l4 4-4 4" /></svg>
            </button>
            <button type="button" class="tree-label" :data-testid="root.testId" @click="openSpace(root.id)">
              <span class="tree-folder" :class="{ open: openIds[spaceStorageKey(root.id)] }"></span>
              <span class="tree-name">{{ root.name }}</span>
            </button>
          </div>
          <template v-if="openIds[spaceStorageKey(root.id)]">
            <div
              v-for="row in rowsFor(root.id)"
              :key="`${root.key}-${row.folder.id}`"
              class="tree-row"
              :style="{ paddingLeft: `${(row.depth + 1) * 16}px` }"
              :aria-current="teamId === root.id && folderId === String(row.folder.id) ? 'page' : null"
            >
              <button type="button" class="tree-chevron" :data-testid="`tree-toggle-${row.folder.id}`" :aria-label="row.open ? '折叠' : '展开'" @click="toggleFolder(row.folder.id)">
                <svg v-if="row.hasChildren" viewBox="0 0 16 16" :class="{ open: row.open }"><path d="M6 4l4 4-4 4" /></svg>
              </button>
              <button type="button" class="tree-label" :data-testid="`tree-folder-${row.folder.id}`" @click="selectTree(root.id, row.folder.id)">
                <span class="tree-folder" :class="{ open: row.open }"></span>
                <span class="tree-name">{{ row.folder.name }}</span>
              </button>
            </div>
          </template>
        </div>
      </div>
      <nav class="library-links" aria-label="更多">
        <router-link class="tree-manage" to="/calendar" data-testid="open-calendar">日历</router-link>
        <router-link class="tree-manage" to="/teams" data-testid="manage-teams">团队管理</router-link>
      </nav>
      </aside>
      <main class="library">
      <div class="library-scroll">
      <div class="library-intro">
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
        <button type="button" class="create-tile" data-testid="create-pdf" :disabled="pending" @click="create('pdf')">
          <KindMark kind="pdf" />
          PDF
        </button>
        <button type="button" class="create-tile" data-testid="create-folder" :disabled="pending" @click="makingFolder = true; folderName = ''">
          <span class="mark folder-mark">
            <svg viewBox="0 0 24 24" aria-hidden="true"><path d="M3.5 7.5h6l1.5-2h9v11.5a1.5 1.5 0 0 1-1.5 1.5h-15a1.5 1.5 0 0 1-1.5-1.5v-8a1.5 1.5 0 0 1 1.5-1.5z" fill="none" stroke="currentColor" stroke-width="1.6" stroke-linejoin="round"/></svg>
          </span>
          文件夹
        </button>
        <button type="button" class="create-tile" data-testid="upload-button" :disabled="pending" title="上传 docx、xlsx、pptx、pdf、txt、csv 或 json" @click="pickFile">
          <span class="mark upload-mark">
            <svg viewBox="0 0 24 24" aria-hidden="true"><path d="M12 16V7M8.5 10.5 12 7l3.5 3.5M6 17.5h12" fill="none" stroke="currentColor" stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round"/></svg>
          </span>
          上传
        </button>
        <input ref="fileInput" class="file-input" data-testid="upload-input" type="file" accept=".docx,.xlsx,.pptx,.pdf,.txt,.csv,.json,text/plain,text/csv,application/pdf,application/json" @change="onUpload" />
      </div>
      <nav class="crumbs" data-testid="folder-crumbs" aria-label="文件夹路径">
        <button type="button" @click="openFolder('')">{{ spaceName }}</button>
        <template v-for="item in ancestors" :key="item.id">
          <span>/</span>
          <button type="button" @click="openFolder(item.id)">{{ item.name }}</button>
        </template>
        <template v-if="currentFolder">
          <span>/</span>
          <strong>{{ currentFolder.name }}</strong>
        </template>
      </nav>
      <p class="search-note" v-if="searching">正在搜索当前文档库</p>
      </div>
      <div class="library-sticky">
      <div
        v-if="upload"
        class="upload-card"
        :class="{ 'is-done': upload.phase === 'done', 'is-error': upload.phase === 'error' }"
        data-testid="upload-progress"
      >
        <div class="upload-head">
          <span class="upload-badge" aria-hidden="true">
            <svg v-if="upload.phase === 'done'" viewBox="0 0 24 24"><path d="M6 12.5 10 16.5 18 8" fill="none" stroke="currentColor" stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round"/></svg>
            <svg v-else-if="upload.phase === 'error'" viewBox="0 0 24 24"><path d="M12 7v6M12 17h.01" fill="none" stroke="currentColor" stroke-width="1.8" stroke-linecap="round"/></svg>
            <svg v-else viewBox="0 0 24 24"><path d="M12 16V7M8.5 10.5 12 7l3.5 3.5" fill="none" stroke="currentColor" stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round"/></svg>
          </span>
          <span class="upload-name">{{ upload.name }}</span>
          <strong class="upload-percent" data-testid="upload-percent">{{ upload.percent }}%</strong>
        </div>
        <div
          class="upload-track"
          role="progressbar"
          :aria-valuenow="upload.percent"
          aria-valuemin="0"
          aria-valuemax="100"
          :aria-label="`${upload.name} ${uploadLabel(upload)}`"
        >
          <span class="upload-fill" :style="{ width: `${upload.percent}%` }"></span>
        </div>
        <div class="upload-foot">
          <span data-testid="upload-status">{{ uploadLabel(upload) }}</span>
          <span class="upload-size" v-if="upload.phase !== 'error'">{{ formatBytes(upload.sent) }} / {{ formatBytes(upload.size) }}</span>
          <button v-if="upload.phase === 'error'" class="text-button" type="button" data-testid="upload-dismiss" @click="dismissUpload">关闭</button>
          <button v-else-if="upload.phase !== 'done'" class="text-button" type="button" data-testid="upload-cancel" @click="cancelUpload">取消</button>
        </div>
      </div>
      <div class="library-bar">
        <div class="filters">
          <button type="button" :aria-pressed="kind === ''" @click="kind = ''">全部</button>
          <button type="button" :aria-pressed="kind === 'word'" @click="kind = 'word'">文稿</button>
          <button type="button" :aria-pressed="kind === 'excel'" @click="kind = 'excel'">表格</button>
          <button type="button" :aria-pressed="kind === 'ppt'" @click="kind = 'ppt'">演示</button>
          <button type="button" data-testid="filter-pdf" :aria-pressed="kind === 'pdf'" @click="kind = 'pdf'">PDF</button>
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
      </div>
      </main>
    </div>
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
          <button type="button" data-testid="move-target" :aria-pressed="moveTarget === '0'" @click="moveTarget = '0'">{{ spaceName }}</button>
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
