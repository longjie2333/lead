import { defineStore } from 'pinia'

let toastId = 0
let dismissTimer
let nextTimer
let isClosing = false

const LEAVE_DURATION = 200

export const useToastStore = defineStore('toast', {
  state: () => ({
    current: null,
    queue: [],
  }),
  actions: {
    show(message, options = {}) {
      const text = String(message || '').trim()
      if (!text) return

      this.queue.push({
        id: ++toastId,
        message: text,
        type: options.type || 'error',
        duration: options.duration || 1600,
      })

      if (this.current) {
        this.closeCurrent()
        return
      }

      if (!isClosing) {
        this.showNext()
      }
    },
    error(message) {
      this.show(message, { type: 'error' })
    },
    success(message) {
      this.show(message, { type: 'success' })
    },
    showNext() {
      if (this.current || this.queue.length === 0) return

      window.clearTimeout(nextTimer)
      window.clearTimeout(dismissTimer)
      isClosing = false
      this.current = this.queue.shift()
      dismissTimer = window.setTimeout(() => this.closeCurrent(), this.current.duration)
    },
    closeCurrent() {
      window.clearTimeout(dismissTimer)
      window.clearTimeout(nextTimer)
      if (!this.current) return

      isClosing = true
      this.current = null
      nextTimer = window.setTimeout(() => {
        isClosing = false
        this.showNext()
      }, LEAVE_DURATION)
    },
    clear() {
      window.clearTimeout(dismissTimer)
      window.clearTimeout(nextTimer)
      isClosing = false
      this.current = null
      this.queue = []
    },
  },
})
