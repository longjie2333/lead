import { defineStore } from 'pinia'

const NOTE_KEY = 'lead-device-notes'

export const useDeviceStore = defineStore('device', {
  state: () => ({
    currentId: '',
    devices: [],
    notes: {},
  }),
  getters: {
    current: (state) => state.devices.find((device) => device.id === state.currentId) || null,
    onlineCount: (state) => state.devices.filter((device) => device.online).length,
    noteById: (state) => (deviceId) => state.notes[deviceId] || '',
    displayName: (state) => (device) => state.notes[device?.id] || device?.name || device?.id || '',
  },
  actions: {
    saveDevices(payload) {
      const devices = Array.isArray(payload) ? payload : payload?.devices
      this.devices = Array.isArray(devices) ? devices : []
      if (this.currentId && !this.devices.some((device) => device.id === this.currentId)) {
        this.currentId = ''
      }
    },
    selectDevice(deviceId) {
      this.currentId = String(deviceId || '')
    },
    saveDeviceNote(deviceId, note) {
      const value = String(note || '').trim()
      if (!value) return this.removeDeviceNote(deviceId)
      this.notes = { ...this.notes, [deviceId]: value }
    },
    removeDeviceNote(deviceId) {
      const next = { ...this.notes }
      delete next[deviceId]
      this.notes = next
    },
    clear() {
      this.currentId = ''
      this.devices = []
    },
  },
  persist: {
    key: NOTE_KEY,
    paths: ['notes'],
    serializer: {
      deserialize(value) {
        const notes = JSON.parse(value || '{}')
        return { notes: notes.notes || notes }
      },
      serialize(value) {
        return JSON.stringify(value?.notes || {})
      },
    },
  },
})
