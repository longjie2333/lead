<template>
	<NavBar main-class="pl-4" />
	<view class="px-4 pb-8 pt-4">
		<view class="px-4 mb-3 flex items-center justify-between">
			<view>
				<view class="text-lg font-semibold text-ink">设备列表</view>
				<view class="mt-1 text-xs text-gray-600">{{ deviceStore.devices.length }} 台设备，{{ deviceStore.onlineCount }} 台在线</view>
			</view>
		</view>

		<view
				class="rounded-2xl bg-white p-5 text-center"
				v-if="deviceStore.devices.length === 0 && !loading"
		>
			<view class="text-base font-semibold text-ink">暂无设备</view>
			<view class="mt-2 text-sm text-gray-600">请先在安卓被控端登录同一账号</view>
		</view>

		<view class="grid gap-3">
			<SwipeAction
					v-for="device in deviceStore.devices"
					:key="device.id"
					:open="openSwipeId === getDeviceId(device)"
					:action-width-ratio="0.2"
					:actions="deviceSwipeActions"
					@action="handleDeviceSwipeAction(device, $event)"
					@update:open="setDeviceSwipeOpen(device, $event)"
					@tap="openRemote(device)"
			>
				<view class="rounded-2xl bg-white p-4">
					<view class="min-w-0">
						<view class="flex items-center justify-between gap-2">
							<view class="truncate text-base font-semibold text-ink">
								{{ deviceStore.noteById(device.id) || device.name || device.id }}
							</view>
							<view
									class="status-pill"
									:style="{ color: device.online ? '#1d7c45' : '' }"
							>
								{{ device.online ? '在线' : '离线' }}
							</view>
						</view>
						<view class="flex items-center justify-between gap-2 mt-2">
							<view class="truncate text-xs text-gray-600">{{ device.id }}</view>
							<view class="text-xs text-gray-600">{{ formatTime(device.updatedAt).slice(5) }}</view>
						</view>
					</view>
				</view>
			</SwipeAction>
		</view>
	</view>
</template>

<script setup>
import NavBar from '../../components/NavBar.vue'
import SwipeAction from '../../components/SwipeAction.vue'
import { ref } from 'vue'
import { useUserStore } from '../../stores/user'
import { useDeviceStore } from '../../stores/device'
import { formatTime } from '../../utils'
import { onHide, onShow, onUnload, onPullDownRefresh } from '@dcloudio/uni-app'
import { fetchDevices } from '../../utils/api'

const userStore = useUserStore()
const deviceStore = useDeviceStore()

const loading = ref(false)
const openSwipeId = ref('')
const deviceSwipeActions = [
	{
		key: 'note',
		text: '备注',
		className: 'bg-primary text-white',
	}
]
let refreshTimer = null

if (!userStore.isLoggedIn) {
	uni.reLaunch({
		url: '/pages/login/login',
	})
}

onShow(startAutoRefresh)
onHide(stopAutoRefresh)
onUnload(stopAutoRefresh)

onPullDownRefresh(async () => {
	await fetchDevices()
	uni.stopPullDownRefresh()
})

function startAutoRefresh() {
	stopAutoRefresh()
	refreshTimer = setInterval(() => {
		fetchDevices()
	}, 10000)
}

function stopAutoRefresh() {
	if (!refreshTimer) return
	clearInterval(refreshTimer)
	refreshTimer = null
}

function getDeviceId(device) {
	return String(device.id)
}

function setDeviceSwipeOpen(device, open) {
	const id = getDeviceId(device)

	if (open) {
		openSwipeId.value = id
	} else if (openSwipeId.value === id) {
		openSwipeId.value = ''
	}
}

function handleDeviceSwipeAction(device, action) {
	if (action.key === 'note') {
		openNoteEditor(device)
	}
}

function openNoteEditor(device) {
	const deviceId = getDeviceId(device)
	openSwipeId.value = ''

	uni.showModal({
		title: '设置备注',
		editable: true,
		placeholderText: '请输入设备备注',
		content: deviceStore.noteById(deviceId) || device.name || '',
		success: (res) => {
			if (!res.confirm) return

			const note = String(res.content || '').trim()
			if (note) {
				deviceStore.saveDeviceNote(deviceId, note)
			} else {
				deviceStore.removeDeviceNote(deviceId)
			}
		},
	})
}

function openRemote(device) {
	const id = encodeURIComponent(device.id)
	const name = encodeURIComponent(deviceStore.noteById(device.id) || device.name || device.id)

	if (!device.online) {
		return uni.showToast({
			title: '请启动该设备的应用',
			icon: 'none',
			mask: true,
			duration: 2000,
		})
	}

	uni.navigateTo({
		url: `/pages/remote/remote?deviceId=${id}&name=${name}`,
	})
}
</script>
