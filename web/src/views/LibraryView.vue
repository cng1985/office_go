<script setup>
import { onMounted, ref, watch } from 'vue'
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

let timer
watch([kind, title], () => {
  clearTimeout(timer)
  timer = setTimeout(load, 250)
})
onMounted(load)
</script>

<template>
  <div class="desk">
    <header class="topbar">
      <router-link class="brand" to="/">
        <span class="brand-mark">笺</span>
        <strong>云笺</strong>
      </router-link>
      <input class="search" data-testid="library-search" v-model="title" placeholder="搜索标题" />
      <span class="spacer"></span>
      <span class="user-chip" data-testid="current-user">{{ session.account?.display_name }}</span>
      <button class="ghost" type="button" data-testid="logout" @click="signOut">退出</button>
    </header>
    <main class="library">
      <div class="library-head">
        <div>
          <h1>书案</h1>
          <p class="muted">文稿、表格和演示都保存在你的账号下。</p>
        </div>
        <div class="creates">
          <button class="primary" type="button" data-testid="create-word" :disabled="pending" @click="create('word')">新建文稿</button>
          <button class="ghost" type="button" data-testid="create-excel" :disabled="pending" @click="create('excel')">新建表格</button>
          <button class="ghost" type="button" data-testid="create-ppt" :disabled="pending" @click="create('ppt')">新建演示</button>
        </div>
      </div>
      <div class="filters">
        <button type="button" class="ghost" :aria-pressed="kind === ''" @click="kind = ''">全部</button>
        <button type="button" class="ghost" :aria-pressed="kind === 'word'" @click="kind = 'word'">文稿</button>
        <button type="button" class="ghost" :aria-pressed="kind === 'excel'" @click="kind = 'excel'">表格</button>
        <button type="button" class="ghost" :aria-pressed="kind === 'ppt'" @click="kind = 'ppt'">演示</button>
      </div>
      <p class="error" v-if="error">{{ error }}</p>
      <div class="grid" v-if="documents.length">
        <article v-for="item in documents" :key="item.id" class="card" :class="item.kind" data-testid="doc-card">
          <div class="kind">{{ KINDS[item.kind].label }} · 版本 {{ item.version_no }}</div>
          <h2>{{ item.title }}</h2>
          <div class="muted">{{ formatTime(item.updated_at) }}</div>
          <div class="card-actions">
            <button class="primary" type="button" @click="router.push(`/doc/${item.id}`)">编辑</button>
            <button class="ghost" type="button" @click="openRename(item)">重命名</button>
            <button class="danger" type="button" @click="deleting = item">删除</button>
          </div>
        </article>
      </div>
      <div class="empty" v-else data-testid="empty-library">
        <h2>书案还是空的</h2>
        <p class="muted">从一份文稿、一张表格或一页演示开始。</p>
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
