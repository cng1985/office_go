<script setup>
import { ref } from 'vue'
import { useRouter } from 'vue-router'
import { useSessionStore } from '../stores/session'

const session = useSessionStore()
const router = useRouter()
const username = ref('')
const displayName = ref('')
const password = ref('')
const error = ref('')
const pending = ref(false)

async function submit() {
  error.value = ''
  pending.value = true
  try {
    await session.register({
      username: username.value,
      display_name: displayName.value,
      password: password.value,
    })
    router.push('/')
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
      <h1>注册</h1>
      <p class="muted">用户名会按小写保存，密码至少 8 个字符。注册后进入文档。</p>
      <label>用户名
        <input data-testid="register-username" v-model="username" autocomplete="username" />
      </label>
      <label>显示名称
        <input data-testid="register-display" v-model="displayName" />
      </label>
      <label>密码
        <input data-testid="register-password" v-model="password" type="password" autocomplete="new-password" />
      </label>
      <p class="error" v-if="error">{{ error }}</p>
      <button class="primary" data-testid="register-submit" :disabled="pending" type="submit">创建账号</button>
      <p class="auth-switch">已有账号？ <router-link to="/login">登录</router-link></p>
    </form>
  </main>
</template>
