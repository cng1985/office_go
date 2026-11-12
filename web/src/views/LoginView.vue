<script setup>
import { ref } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { useSessionStore } from '../stores/session'

const session = useSessionStore()
const router = useRouter()
const route = useRoute()
const username = ref('')
const password = ref('')
const error = ref('')
const pending = ref(false)

async function submit() {
  error.value = ''
  pending.value = true
  try {
    await session.login({ username: username.value, password: password.value })
    const next = route.query.next
    const target = typeof next === 'string' && next.startsWith('/') && !next.startsWith('//') ? next : '/'
    router.push(target)
  } catch (err) {
    error.value = err.message
  } finally {
    pending.value = false
  }
}
</script>

<template>
  <main class="auth-screen">
    <form class="auth-card" @submit.prevent="submit">
      <div class="product-marks" aria-hidden="true">
        <span class="mark word">W</span>
        <span class="mark excel">
          <svg viewBox="0 0 24 24"><path d="M5 5h14v14H5z" fill="none" stroke="#fff" stroke-width="1.6"/><path d="M5 10h14M5 15h14M10 5v14M15 5v14" stroke="#fff" stroke-width="1.2"/></svg>
        </span>
        <span class="mark ppt">
          <svg viewBox="0 0 24 24"><rect x="4" y="6" width="16" height="10" rx="1.2" fill="none" stroke="#fff" stroke-width="1.6"/><path d="M9 19h6M12 16v3" stroke="#fff" stroke-width="1.6" stroke-linecap="round"/></svg>
        </span>
        <span class="mark pdf">P</span>
      </div>
      <h1>登录</h1>
      <p class="muted">使用云笺账号继续编辑文稿、表格、演示和 PDF。</p>
      <label>用户名
        <input data-testid="login-username" v-model="username" autocomplete="username" />
      </label>
      <label>密码
        <input data-testid="login-password" v-model="password" type="password" autocomplete="current-password" />
      </label>
      <p class="error" v-if="error">{{ error }}</p>
      <button class="primary" data-testid="login-submit" :disabled="pending" type="submit">登录</button>
      <p class="auth-switch">还没有账号？ <router-link to="/register" data-testid="register-link">注册</router-link></p>
    </form>
  </main>
</template>
