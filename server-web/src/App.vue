<template>
  <ToastHost />

  <LoginView
			v-if="!userStore.isLoggedIn"
			@logged-in="afterLogin"
	/>

  <div
			v-else
			class="flex h-dvh flex-col overflow-hidden bg-panel"
	>
    <AppHeader
				ref="headerRef"
				:user="userStore.user"
				:show-back="Boolean(deviceStore.currentId) || currentPage === 'users'"
				:loading="loading"
				@back="handleBack"
				@logout="handleLogout"
		>
      <template
					v-if="!deviceStore.currentId && currentPage === 'devices'"
					#actions
			>
        <button
						class="icon-button p-2 rounded-xl bg-white disabled:opacity-60"
						type="button"
						title="刷新"
						:disabled="loading"
						@click="refresh"
				>
          <RefreshCw
							class="h-5 w-5"
							:class="{ 'animate-spin': loading }"
					/>
        </button>
        <button
						class="icon-button rounded-xl bg-white p-2 disabled:opacity-60"
						type="button"
						title="用户管理"
						:disabled="loading"
						@click="openUserManage"
				>
          <Users class="h-5 w-5" />
        </button>
      </template>
    </AppHeader>

    <main
				class="min-h-0 w-full flex-1 overflow-hidden"
				:style="remoteMainStyle"
		>
      <DeviceList
					v-if="currentPage === 'devices' && !deviceStore.current"
					:devices="deviceStore.devices"
					:selected-id="deviceStore.currentId"
					:loading="loading"
					@select="selectDevice"
					@note="editNote"
			/>
      <UserManage
					v-else-if="currentPage === 'users'"
			/>
      <RemoteViewer
					v-else
					:key="deviceStore.current.id"
					:token="userStore.token"
					:device="deviceStore.current"
					:title="deviceStore.displayName(deviceStore.current)"
					@device-error="clearSelection"
			/>
    </main>
  </div>
</template>

<script setup>
import { computed, nextTick, onBeforeUnmount, onMounted, ref, watch } from 'vue'
import AppHeader from '@/components/AppHeader.vue'
import DeviceList from '@/components/DeviceList.vue'
import LoginView from '@/components/LoginView.vue'
import RemoteViewer from '@/components/RemoteViewer.vue'
import ToastHost from '@/components/ToastHost.vue'
import UserManage from '@/components/UserManage.vue'
import { RefreshCw, Users } from 'lucide-vue-next'
import { useDeviceStore } from '@/stores/device'
import { useToastStore } from '@/stores/toast'
import { useUserStore } from '@/stores/user'
import { fetchDevices, logout } from '@/utils/api'

const userStore = useUserStore()
const deviceStore = useDeviceStore()
const toastStore = useToastStore()

const loading = ref(false)
const headerRef = ref(null)
const headerHeight = ref(0)
const currentPage = ref('devices')
let refreshTimer
let headerResizeObserver

const remoteMainStyle = computed(() => (
	deviceStore.current ? { height: `calc(100dvh - ${headerHeight.value}px)` } : null
))

onMounted(async () => {
	await bootstrapFromURL()
	if (userStore.isLoggedIn) {
		await refresh()
		startAutoRefresh()
		await nextTick()
		setupHeaderObserver()
	}
})

onBeforeUnmount(() => {
	cleanupHeaderObserver()
	stopAutoRefresh()
})

watch(
	() => userStore.isLoggedIn,
	async (isLoggedIn) => {
		if (!isLoggedIn) {
			cleanupHeaderObserver()
			return
		}
		await nextTick()
		setupHeaderObserver()
	},
)

async function bootstrapFromURL() {
	const params = new URLSearchParams(location.search)
	const token = params.get('token')
	const deviceId = params.get('deviceId')

	if (token && !userStore.isLoggedIn) {
		userStore.saveTokenLink(token)
	}

	if (deviceId) {
		deviceStore.selectDevice(deviceId)
		params.delete('token')
		history.replaceState(null, '', params.size ? `/?${params}` : '/')
	}
}

async function afterLogin() {
	await refresh()
	const params = new URLSearchParams(location.search)
	const deviceId = params.get('deviceId')
	if (deviceId && deviceStore.devices.some((device) => device.id === deviceId)) {
		deviceStore.selectDevice(deviceId)
	}
	startAutoRefresh()
}

async function refresh() {
	if (loading.value) return
	loading.value = true

	try {
		await fetchDevices()
	} catch (err) {
		toastStore.error(err?.message || '设备列表刷新失败')
		stopAutoRefresh()
	} finally {
		loading.value = false
	}
}

function startAutoRefresh() {
	stopAutoRefresh()
	refreshTimer = setInterval(refresh, 10000)
}

function stopAutoRefresh() {
	if (!refreshTimer) return
	clearInterval(refreshTimer)
	refreshTimer = undefined
}

function selectDevice(device) {
	if (!device.online) {
		toastStore.error('请先启动该设备的安卓被控端')
		return
	}
	currentPage.value = 'devices'
	deviceStore.selectDevice(device.id)
	history.replaceState(null, '', `/?deviceId=${encodeURIComponent(device.id)}`)
}

function openUserManage() {
	currentPage.value = 'users'
}

function handleBack() {
	if (currentPage.value === 'users') {
		currentPage.value = 'devices'
		return
	}

	closeMobileRemote()
}

function closeMobileRemote() {
	deviceStore.selectDevice('')
	history.replaceState(null, '', '/')
}

function clearSelection() {
	deviceStore.selectDevice('')
	history.replaceState(null, '', '/')
}

function editNote(device) {
	const current = deviceStore.noteById(device.id) || device.name || ''
	const note = window.prompt('设置备注', current)
	if (note === null) return
	deviceStore.saveDeviceNote(device.id, note)
}

async function handleLogout() {
	loading.value = true
	try {
		await logout()
		currentPage.value = 'devices'
		history.replaceState(null, '', '/')
		stopAutoRefresh()
	} finally {
		loading.value = false
	}
}

function setupHeaderObserver() {
	cleanupHeaderObserver()
	syncHeaderHeight()

	const headerEl = headerRef.value?.$el
	if (!headerEl || typeof ResizeObserver === 'undefined') return

	headerResizeObserver = new ResizeObserver(syncHeaderHeight)
	headerResizeObserver.observe(headerEl)
}

function cleanupHeaderObserver() {
	if (!headerResizeObserver) return
	headerResizeObserver.disconnect()
	headerResizeObserver = undefined
}

function syncHeaderHeight() {
	const headerEl = headerRef.value?.$el
	headerHeight.value = headerEl?.getBoundingClientRect().height || 0
}
</script>
