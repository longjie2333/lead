<template>
	<NavBar />
	<view class="remote-page min-h-screen bg-panel">
    <web-view
				:src="viewerUrl"
				v-if="viewerUrl"
				@message="handleMessage"
				@error="handleError"
		/>

    <view
				class="px-4 pt-4"
				v-else
		>
      <view class="rounded-ui border border-line bg-surface p-4 text-sm text-danger shadow-soft">
        无法打开远控页面，请重新登录后再选择设备。
      </view>
    </view>
  </view>
</template>

<script setup>
import NavBar from '../../components/NavBar.vue'
import { ref } from 'vue'
import { onLoad } from '@dcloudio/uni-app'
import { useUserStore } from '../../stores/user'
import { encodeQuery } from '../../utils'
import { SERVER_URL } from '../../utils/config'

const userStore = useUserStore()

const title = ref('')
const viewerUrl = ref('')

onLoad((query) => {
	const deviceId = query.deviceId || ''

	title.value = query.name ? decodeURIComponent(query.name) : '远程控制'

	if (!deviceId) return

	viewerUrl.value = toViewerUrl(userStore.token, deviceId)
})

function toViewerUrl(token, deviceId) {
	const params = encodeQuery({token, deviceId})
	return `${SERVER_URL}/viewer?${params}`
}

function handleMessage(event) {
	const payload = event?.detail?.data
	if (!payload) return
	console.log('viewer message', payload)
}

function handleError() {
	uni.showToast({
		title: 'Web viewer 加载失败',
		icon: 'none',
		mask: true,
		duration: 2000,
	})
}
</script>

<style scoped>

</style>
