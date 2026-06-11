import { defineStore } from 'pinia'

export const useDeviceStore = defineStore(
  'device',
  {
    state: () => ({
      current: null,
      devices: [],
      notes: {},
    }),
    getters: {
      onlineCount: (state) => state.devices.filter((device) => device.online).length,
      noteById: (state) => (deviceId) => state.notes[deviceId] || '',
    },
    actions: {
      saveDevices(devices) {
        devices = devices.devices
        this.devices = Array.isArray(devices) ? devices : []
      },
      setCurrent(device) {
        this.current = device || null
      },
      saveDeviceNote(deviceId, note) {
        const value = String(note || '').trim()

        if (value) {
          this.notes[deviceId] = value
        }
      },
      removeDeviceNote(deviceId) {
        delete this.notes[deviceId]
      }
    },
    persist: {
      enabled: true,
    }
  })
