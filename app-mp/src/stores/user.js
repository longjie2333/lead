import { defineStore } from 'pinia'
import { useDeviceStore } from './device'
import { reLaunchOnce } from '../utils'

const LOGIN_PAGE = '/pages/login/login'
const createEmptyUser = () => ({
  id: null,
  role: null,
  username: null,
  isAdmin: false,
  createdAt: null,
  updatedAt: null,
})

export const useUserStore = defineStore(
  'user',
  {
    state: () => ({
      token: null,
      user: createEmptyUser(),
    }),
    getters: {
      isLoggedIn: (state) => Boolean(state?.token),
    },
    actions: {
      saveAuth(auth) {
        this.token = auth.token || null
        this.user = auth.user || createEmptyUser()
      },
      removeAuth(toastMessage) {
        this.$reset()
        useDeviceStore().$reset()
        uni.clearStorageSync()

        if (toastMessage) {
          uni.showToast({
            title: toastMessage,
            icon: 'none',
            duration: 2000,
          })
        }

        reLaunchOnce(LOGIN_PAGE)
      },
    },
    persist: {
      enabled: true
    },
  })