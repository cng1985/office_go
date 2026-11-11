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
    <section class="auth-story">
      <div>
        <div class="seal">笺</div>
        <h1>开一卷</h1>
        <p>用户名会按小写保存。密码至少 8 个字符。注册完成后直接进入你的文档库。</p>
      </div>
    </section>
    <section class="auth-panel">
      <div class="auth-card">
        <h2>注册</h2>
        <form @submit.prevent="submit">
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
        </form>
        <p>已有账号？ <router-link to="/login">登录</router-link></p>
      </div>
    </section>
  </main>
</template>
