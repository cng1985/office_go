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
    <section class="auth-story">
      <div>
        <div class="seal">笺</div>
        <h1>云笺</h1>
        <p>把文稿、表格和演示放在同一张书案上。写完即存到云端，每一次保存都留下可以回看的版本。</p>
      </div>
      <div class="story-points">
        <span>文稿</span>
        <span>表格</span>
        <span>演示</span>
        <span>版本</span>
      </div>
    </section>
    <section class="auth-panel">
      <div class="auth-card">
        <h2>登录</h2>
        <p class="muted">使用你的云笺账号继续编辑。</p>
        <form @submit.prevent="submit">
          <label>用户名
            <input data-testid="login-username" v-model="username" autocomplete="username" />
          </label>
          <label>密码
            <input data-testid="login-password" v-model="password" type="password" autocomplete="current-password" />
          </label>
          <p class="error" v-if="error">{{ error }}</p>
          <button class="primary" data-testid="login-submit" :disabled="pending" type="submit">进入书案</button>
        </form>
        <p>还没有账号？ <router-link to="/register" data-testid="register-link">注册</router-link></p>
      </div>
    </section>
  </main>
</template>
