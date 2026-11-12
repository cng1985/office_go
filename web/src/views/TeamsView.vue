<script setup>
import { computed, ref, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { addTeamMember, createTeam, deleteTeam, getTeam, leaveTeam, removeTeamMember, searchTeams, updateTeam } from '../api/team'
import { formatTime } from '../editors/kinds'
import { useSessionStore } from '../stores/session'

const session = useSessionStore()
const route = useRoute()
const router = useRouter()
const teams = ref([])
const team = ref(null)
const teamName = ref('')
const name = ref('')
const username = ref('')
const keyword = ref('')
const error = ref('')
const notice = ref('')
const pending = ref(false)
const confirmingDelete = ref(false)
const confirmingLeave = ref(false)
const removing = ref(null)
const initial = computed(() => (session.account?.display_name || session.account?.username || '云').slice(0, 1))
const currentId = computed(() => {
  const raw = route.params.id
  return raw ? String(raw) : ''
})
const isOwner = computed(() => team.value?.role === 'owner')
const visibleTeams = computed(() => {
  const word = keyword.value.trim().toLowerCase()
  if (!word) return teams.value
  return teams.value.filter((item) => item.name.toLowerCase().includes(word))
})

function roleLabel(role) {
  return role === 'owner' ? '所有者' : '成员'
}

async function loadTeams() {
  const result = await searchTeams({ page: 1, page_size: 200 })
  teams.value = result.data
}

async function loadTeam() {
  const id = currentId.value
  confirmingDelete.value = false
  confirmingLeave.value = false
  removing.value = null
  notice.value = ''
  if (!id) {
    team.value = null
    return
  }
  try {
    const result = await getTeam(id)
    if (currentId.value !== id) return
    team.value = result.data
    name.value = result.data.name
    error.value = ''
  } catch (err) {
    if (currentId.value !== id) return
    team.value = null
    error.value = err.message
  }
}

async function create() {
  pending.value = true
  error.value = ''
  notice.value = ''
  try {
    const result = await createTeam({ name: teamName.value })
    teamName.value = ''
    await loadTeams()
    router.push(`/teams/${result.data.id}`)
  } catch (err) {
    error.value = err.message
  } finally {
    pending.value = false
  }
}

async function rename() {
  pending.value = true
  error.value = ''
  notice.value = ''
  try {
    const result = await updateTeam({ id: team.value.id, name: name.value })
    team.value = result.data
    name.value = result.data.name
    await loadTeams()
    notice.value = '团队名称已保存'
  } catch (err) {
    error.value = err.message
  } finally {
    pending.value = false
  }
}

async function invite() {
  pending.value = true
  error.value = ''
  notice.value = ''
  try {
    const result = await addTeamMember({ team_id: team.value.id, username: username.value })
    team.value = result.data
    username.value = ''
    notice.value = '已邀请成员'
  } catch (err) {
    error.value = err.message
  } finally {
    pending.value = false
  }
}

async function commitRemove() {
  pending.value = true
  error.value = ''
  notice.value = ''
  try {
    const result = await removeTeamMember({ team_id: team.value.id, user_id: removing.value.user_id })
    team.value = result.data
    removing.value = null
    notice.value = '已移出成员'
  } catch (err) {
    error.value = err.message
  } finally {
    pending.value = false
  }
}

async function commitLeave() {
  pending.value = true
  error.value = ''
  try {
    await leaveTeam(team.value.id)
    await loadTeams()
    router.push('/teams')
  } catch (err) {
    error.value = err.message
    confirmingLeave.value = false
  } finally {
    pending.value = false
  }
}

async function commitDelete() {
  pending.value = true
  error.value = ''
  try {
    await deleteTeam(team.value.id)
    await loadTeams()
    router.push('/teams')
  } catch (err) {
    error.value = err.message
    confirmingDelete.value = false
  } finally {
    pending.value = false
  }
}

async function signOut() {
  await session.logout()
  router.push('/login')
}

watch(currentId, loadTeam, { immediate: true })
loadTeams().catch((err) => {
  error.value = err.message
})
</script>

<template>
  <div class="home">
    <header class="home-bar">
      <router-link class="home-brand" to="/">
        <span class="mark word">W</span>
        <strong>云笺</strong>
      </router-link>
      <span class="spacer"></span>
      <router-link class="home-user" to="/account" data-testid="account-link" title="个人中心">
        <i>{{ initial }}</i>
        <span data-testid="current-user">{{ session.account?.display_name }}</span>
      </router-link>
      <button class="text-button" type="button" data-testid="logout" @click="signOut">退出</button>
    </header>
    <main class="account-page teams-page" data-testid="teams-page">
      <div class="teams-head">
        <div>
          <p class="crumbs"><router-link to="/">文档库</router-link></p>
          <h1>团队管理</h1>
        </div>
        <form class="team-create" @submit.prevent="create">
          <input data-testid="team-name" v-model="teamName" placeholder="新团队名称" maxlength="40" />
          <button class="primary" type="submit" data-testid="team-create" :disabled="pending">创建团队</button>
        </form>
      </div>
      <p class="error" v-if="error">{{ error }}</p>
      <p class="notice" v-if="notice" data-testid="team-notice">{{ notice }}</p>
      <div class="teams-layout">
        <section class="account-card team-index">
          <input data-testid="team-filter" v-model="keyword" placeholder="搜索团队" />
          <div v-if="visibleTeams.length" class="team-picks">
            <router-link
              v-for="item in visibleTeams"
              :key="item.id"
              class="team-pick"
              :to="`/teams/${item.id}`"
              :aria-current="currentId === String(item.id) ? 'page' : null"
              data-testid="team-link"
            >
              <strong>{{ item.name }}</strong>
              <span>{{ roleLabel(item.role) }} · {{ formatTime(item.created_at) }}</span>
            </router-link>
          </div>
          <p class="muted" v-else-if="teams.length">没有匹配的团队。</p>
          <p class="muted" v-else>还没有团队。先在右上角创建一个。</p>
        </section>
        <div class="team-detail" data-testid="team-page">
          <section class="account-card" v-if="team">
            <h2>{{ team.name }}</h2>
            <p class="muted">{{ roleLabel(team.role) }} · {{ team.members.length }} 人 · 创建于 {{ formatTime(team.created_at) }}</p>
            <p class="muted" v-if="isOwner">所有者可以重命名、邀请和移出成员。团队里还有文档或文件夹时不能删除，所有者也不能退出。</p>
            <p class="muted" v-else>成员可以一起编辑这个团队的文档，也可以退出团队。</p>
            <form v-if="isOwner" class="team-create" @submit.prevent="rename">
              <input data-testid="team-rename" v-model="name" maxlength="40" />
              <button class="primary" type="submit" data-testid="team-rename-save" :disabled="pending">保存名称</button>
            </form>
            <div class="team-actions">
              <router-link class="primary team-open" :to="{ path: '/', query: { team: String(team.id) } }" data-testid="open-team-library">打开文档库</router-link>
              <button v-if="!isOwner" class="ghost" type="button" data-testid="team-leave" @click="confirmingLeave = true">退出团队</button>
              <button v-if="isOwner" class="text-button danger" type="button" data-testid="team-delete" @click="confirmingDelete = true">删除团队</button>
            </div>
          </section>
          <section class="account-card" v-if="team">
            <h2>成员</h2>
            <form v-if="isOwner" class="team-create" @submit.prevent="invite">
              <input data-testid="team-invite" v-model="username" placeholder="输入用户名邀请" maxlength="32" />
              <button class="primary" type="submit" data-testid="team-invite-save" :disabled="pending">邀请</button>
            </form>
            <div class="team-list">
              <div v-for="item in team.members" :key="item.user_id" class="account-row" data-testid="team-member">
                <div class="member-name">
                  <strong>{{ item.display_name }}</strong>
                  <span class="muted">{{ item.username }} · 加入于 {{ formatTime(item.created_at) }}</span>
                </div>
                <div class="member-side">
                  <span>{{ roleLabel(item.role) }}</span>
                  <button v-if="isOwner && item.role !== 'owner'" class="text-button danger" type="button" data-testid="team-remove" @click="removing = item">移出</button>
                </div>
              </div>
            </div>
          </section>
          <section class="account-card" v-else-if="!currentId">
            <h2>选择一个团队</h2>
            <p class="muted">创建团队后，在这里邀请同事。成员看到的是同一套团队文档，个人文档仍然只在「我的文档」里。</p>
          </section>
        </div>
      </div>
    </main>
    <div class="modal-back" v-if="removing">
      <div class="modal">
        <h3>移出 {{ removing.display_name }}？</h3>
        <p class="muted">移出后不能再查看这个团队的文档。</p>
        <div class="modal-actions">
          <button class="danger" type="button" data-testid="team-remove-confirm" :disabled="pending" @click="commitRemove">移出</button>
          <button class="ghost" type="button" @click="removing = null">取消</button>
        </div>
      </div>
    </div>
    <div class="modal-back" v-if="confirmingLeave">
      <div class="modal">
        <h3>退出 {{ team?.name }}？</h3>
        <p class="muted">退出后不能再查看这个团队的文档。</p>
        <div class="modal-actions">
          <button class="danger" type="button" data-testid="team-leave-confirm" :disabled="pending" @click="commitLeave">退出</button>
          <button class="ghost" type="button" @click="confirmingLeave = false">取消</button>
        </div>
      </div>
    </div>
    <div class="modal-back" v-if="confirmingDelete">
      <div class="modal">
        <h3>删除 {{ team?.name }}？</h3>
        <p class="muted">团队里还有文档或文件夹时不能删除。删除后成员都不能再打开这个团队。</p>
        <div class="modal-actions">
          <button class="danger" type="button" data-testid="team-delete-confirm" :disabled="pending" @click="commitDelete">删除</button>
          <button class="ghost" type="button" @click="confirmingDelete = false">取消</button>
        </div>
      </div>
    </div>
  </div>
</template>
