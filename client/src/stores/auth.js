import { defineStore } from 'pinia'
import { authApi } from '../api/services'

export const useAuthStore = defineStore('auth', {
  state: () => ({
    token: localStorage.getItem('auth_token') || '',
    user: null,
    loading: false,
    initialized: false
  }),
  getters: {
    isAuthenticated: (state) => Boolean(state.token)
  },
  actions: {
    async login(payload) {
      this.loading = true
      try {
        const { data } = await authApi.login(payload)
        this.token = data.token
        localStorage.setItem('auth_token', data.token)
        await this.fetchMe()
      } finally {
        this.loading = false
      }
    },
    async register(payload) {
      this.loading = true
      try {
        const { data } = await authApi.register(payload)
        if (data.token) {
          this.token = data.token
          localStorage.setItem('auth_token', data.token)
          await this.fetchMe()
        }
      } finally {
        this.loading = false
      }
    },
    async fetchMe() {
      if (!this.token) {
        this.user = null
        this.initialized = true
        return
      }
      try {
        const { data } = await authApi.me()
        this.user = data
      } catch (error) {
        this.logout()
        throw error
      } finally {
        this.initialized = true
      }
    },
    async updateProfile(payload) {
      const { data } = await authApi.updateProfile(payload)
      this.user = data
    },
    logout() {
      this.token = ''
      this.user = null
      localStorage.removeItem('auth_token')
      this.initialized = true
    }
  }
})
