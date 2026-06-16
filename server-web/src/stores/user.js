import { defineStore } from 'pinia'
import { useDeviceStore } from './device'

const AUTH_KEY = 'lead-web-auth'

const emptyUser = () => ({
  id: null,
  role: null,
  username: null,
  isAdmin: false,
  createdAt: null,
  updatedAt: null,
})

export const useUserStore = defineStore('user', {
  state: () => ({
    token: '',
    user: emptyUser(),
  }),
  getters: {
    isLoggedIn: (state) => Boolean(state.token),
  },
  actions: {
    saveAuth(auth) {
      this.token = auth?.token || ''
      this.user = auth?.user || emptyUser()
    },
    saveTokenLink(token) {
      this.token = token || ''
      this.user = { ...emptyUser(), username: '远控链接' }
    },
    clearAuth() {
      this.token = ''
      this.user = emptyUser()
      useDeviceStore().clear()
    },
  },
  persist: {
    key: AUTH_KEY,
    paths: ['token', 'user'],
  },
})
