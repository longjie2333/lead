import { defineStore } from 'pinia'

export const useUserStore = defineStore(
  'user',
  {
    state: () => ({
      token: null,
      user: {
        id: null,
        role: null,
        username: null,
        isAdmin: false,
        createdAt: null,
        updatedAt: null,
      }
    }),
    getters: {
      isLoggedIn: (state) => Boolean(state?.token),
    },
    actions: {
      saveAuth(auth) {
        this.token = auth.token
        this.user = auth.user
      },
    },
    persist: {
      enabled: true
    },
  })