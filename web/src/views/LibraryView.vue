<script setup>
import { computed, onMounted, ref, watch } from 'vue'
import { useRouter } from 'vue-router'
import { createDocument, deleteDocument, searchDocuments, updateDocument } from '../api/document'
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

const initial = computed(() => (session.account?.display_name || session.account?.username || '云').slice(0, 1))

let timer
watch([kind, title], () => {
  clearTimeout(timer)
  timer = setTimeout(load, 250)
})
onMounted(load)
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
      <div class="creates">
        <button type="button" class="create-tile" data-testid="create-word" :disabled="pending" @click="create('word')">
          <span class="mark word">W</span>
          文稿
        </button>
        <button type="button" class="create-tile" data-testid="create-excel" :disabled="pending" @click="create('excel')">
          <span class="mark excel">
            <svg viewBox="0 0 24 24"><path d="M5 5h14v14H5z" fill="none" stroke="#fff" stroke-width="1.6"/><path d="M5 10h14M5 15h14M10 5v14M15 5v14" stroke="#fff" stroke-width="1.2"/></svg>
          </span>
          表格
        </button>
        <button type="button" class="create-tile" data-testid="create-ppt" :disabled="pending" @click="create('ppt')">
          <span class="mark ppt">
            <svg viewBox="0 0 24 24"><rect x="4" y="6" width="16" height="10" rx="1.2" fill="none" stroke="#fff" stroke-width="1.6"/><path d="M9 19h6M12 16v3" stroke="#fff" stroke-width="1.6" stroke-linecap="round"/></svg>
          </span>
          演示
        </button>
      </div>
      <div class="filters">
        <button type="button" :aria-pressed="kind === ''" @click="kind = ''">全部</button>
        <button type="button" :aria-pressed="kind === 'word'" @click="kind = 'word'">文稿</button>
        <button type="button" :aria-pressed="kind === 'excel'" @click="kind = 'excel'">表格</button>
        <button type="button" :aria-pressed="kind === 'ppt'" @click="kind = 'ppt'">演示</button>
      </div>
      <p class="error" v-if="error">{{ error }}</p>
      <div class="doc-list" v-if="documents.length">
        <article v-for="item in documents" :key="item.id" class="doc-row" :class="item.kind" data-testid="doc-card">
          <span v-if="item.kind === 'word'" class="mark word">W</span>
          <span v-else-if="item.kind === 'excel'" class="mark excel">
            <svg viewBox="0 0 24 24"><path d="M5 5h14v14H5z" fill="none" stroke="#fff" stroke-width="1.6"/><path d="M5 10h14M5 15h14M10 5v14M15 5v14" stroke="#fff" stroke-width="1.2"/></svg>
          </span>
          <span v-else class="mark ppt">
            <svg viewBox="0 0 24 24"><rect x="4" y="6" width="16" height="10" rx="1.2" fill="none" stroke="#fff" stroke-width="1.6"/><path d="M9 19h6M12 16v3" stroke="#fff" stroke-width="1.6" stroke-linecap="round"/></svg>
          </span>
          <button type="button" class="doc-main" @click="router.push(`/doc/${item.id}`)">
            <strong>{{ item.title }}</strong>
            <span>{{ KINDS[item.kind].label }} · 版本 {{ item.version_no }} · {{ formatTime(item.updated_at) }}</span>
          </button>
          <div class="doc-actions">
            <button class="text-button" type="button" @click="openRename(item)">重命名</button>
            <button class="text-button danger" type="button" @click="deleting = item">删除</button>
          </div>
        </article>
      </div>
      <div class="empty" v-else data-testid="empty-library">
        <h2>还没有文档</h2>
        <p class="muted">从一份文稿、一张表格或一份演示开始。</p>
      </div>
    </main>
    <div class="modal-back" v-if="renaming">
      <div class="modal">
        <h3>重命名</h3>
        <input data-testid="rename-input" v-model="renameTitle" />
        <div class="creates" style="margin-top: 12px">
          <button class="primary" type="button" data-testid="rename-save" @click="commitRename">保存</button>
          <button class="ghost" type="button" @click="renaming = null">取消</button>
        </div>
      </div>
    </div>
    <div class="modal-back" v-if="deleting">
      <div class="modal">
        <h3>删除「{{ deleting.title }}」？</h3>
        <p class="muted">文档和全部历史版本会一起删除。</p>
        <div class="creates">
          <button class="danger" type="button" data-testid="confirm-delete" @click="commitDelete">删除</button>
          <button class="ghost" type="button" @click="deleting = null">取消</button>
        </div>
      </div>
    </div>
  </div>
</template>
