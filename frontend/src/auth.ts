import { computed, reactive } from 'vue'
import { peopleApi, isUnauthorized } from '@/api'
import type { Employee } from '@/types'

const state = reactive<{ user: Employee | null; initialized: boolean }>({ user: null, initialized: false })

export const auth = {
  state,
  authenticated: computed(() => Boolean(state.user)),
  can(code: string) {
    return state.user?.permissions?.includes(code) || false
  },
  async hydrate() {
    if (state.initialized) return
    try {
      state.user = await peopleApi.me()
    } catch (error) {
      if (!isUnauthorized(error)) throw error
      state.user = null
    } finally {
      state.initialized = true
    }
  },
  async login(username: string, password: string) {
    state.user = await peopleApi.login(username, password)
    state.initialized = true
  },
  async logout() {
    try {
      await peopleApi.logout()
    } finally {
      state.user = null
    }
  },
  async changePassword(currentPassword: string, newPassword: string) {
    state.user = await peopleApi.changePassword(currentPassword, newPassword)
  },
}
