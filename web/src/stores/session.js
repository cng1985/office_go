import { defineStore } from 'pinia'
import { login, logout, me, register } from '../api/account'

export const useSessionStore = defineStore('session', {
  state: () => ({
    account: null,
    loaded: false,
  }),
  actions: {
    async refresh() {
      try {
        const result = await me()
        this.account = result.data
      } catch {
        this.account = null
      } finally {
        this.loaded = true
      }
    },
    async login(body) {
      const result = await login(body)
      this.account = result.data.account
      this.loaded = true
    },
    async register(body) {
      const result = await register(body)
      this.account = result.data.account
      this.loaded = true
    },
    async logout() {
      try {
        await logout()
      } finally {
        this.account = null
      }
    },
  },
})
