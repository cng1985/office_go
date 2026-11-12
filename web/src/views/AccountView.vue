<script setup>
import { computed, onMounted, ref } from 'vue'
import { useRouter } from 'vue-router'
import { changePassword, createTeam, searchTeams, updateProfile } from '../api/team'
import { useSessionStore } from '../stores/session'

const session = useSessionStore()
const router = useRouter()
const displayName = ref(session.account?.display_name || '')
const currentPassword = ref('')
const nextPassword = ref('')
const teams = ref([])
const teamName = ref('')
const error = ref('')
const notice = ref('')
const pending = ref(false)
const initial = computed(() => (session.account?.display_name || session.account?.username || '云').slice(0, 1))

function roleLabel(role) {
  return role === 'owner' ? '所有者' : '成员'
}

async function loadTeams() {
  const result = await searchTeams({ page: 1, page_size: 200 })
  teams.value = result.data
}

async function saveProfile() {
  pending.value = true
  error.value = ''
  notice.value = ''
  try {
    const result = await updateProfile({ display_name: displayName.value })
    session.account = result.data
    displayName.value = result.data.display_name
    notice.value = '资料已保存'
  } catch (err) {
    error.value = err.message
  } finally {
    pending.value = false
  }
}

async function savePassword() {
  pending.value = true
  error.value = ''
  notice.value = ''
  try {
    await changePassword({ current_password: currentPassword.value, new_password: nextPassword.value })
    currentPassword.value = ''
    nextPassword.value = ''
    notice.value = '密码已更新，下次登录请使用新密码'
  } catch (err) {
    error.value = err.message
  } finally {
    pending.value = false
  }
}

async function create() {
  pending.value = true
  error.value = ''
  notice.value = ''
  try {
    const result = await createTeam({ name: teamName.value })
    teamName.value = ''
    router.push(`/team/${result.data.id}`)
  } catch (err) {
    error.value = err.message
  } finally {
    pending.value = false
  }
}

async function signOut() {
  await session.logout()
  router.push('/login')
}

onMounted(async () => {
  try {
    await loadTeams()
  } catch (err) {
    error.value = err.message
  }
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
    <main class="account-page" data-testid="account-page">
      <h1>个人中心</h1>
      <p class="error" v-if="error">{{ error }}</p>
      <p class="notice" v-if="notice" data-testid="account-notice">{{ notice }}</p>
      <div class="account-grid">
      <div class="account-stack">
      <section class="account-card">
        <h2>资料</h2>
        <label>
          用户名
          <input data-testid="profile-username" :value="session.account?.username" disabled />
        </label>
        <label>
          显示名称
          <input data-testid="profile-name" v-model="displayName" maxlength="64" @keyup.enter="saveProfile" />
        </label>
        <div>
          <button class="primary" type="button" data-testid="profile-save" :disabled="pending" @click="saveProfile">保存资料</button>
        </div>
      </section>
      <section class="account-card">
        <h2>密码</h2>
        <label>
          当前密码
          <input data-testid="password-current" v-model="currentPassword" type="password" autocomplete="current-password" />
        </label>
        <label>
          新密码
          <input data-testid="password-new" v-model="nextPassword" type="password" autocomplete="new-password" @keyup.enter="savePassword" />
        </label>
        <div>
          <button class="primary" type="button" data-testid="password-save" :disabled="pending" @click="savePassword">更新密码</button>
        </div>
      </section>
      </div>
      <section class="account-card">
        <h2>团队</h2>
        <p class="muted">团队成员可以一起新建、编辑和整理文档。个人文档仍然只有自己能看到。</p>
        <form class="team-create" @submit.prevent="create">
          <input data-testid="team-name" v-model="teamName" placeholder="团队名称" maxlength="40" />
          <button class="primary" type="submit" data-testid="team-create" :disabled="pending">创建团队</button>
        </form>
        <div v-if="teams.length" class="team-list">
          <router-link v-for="team in teams" :key="team.id" class="account-row" :to="`/team/${team.id}`" data-testid="team-link">
            <strong>{{ team.name }}</strong>
            <span>{{ roleLabel(team.role) }}</span>
          </router-link>
        </div>
        <p class="muted" v-else>还没有团队。创建一个，再邀请同事一起编辑。</p>
      </section>
      </div>
    </main>
  </div>
</template>
