<template>
	<view
			class="min-h-screen flex items-center justify-center px-4"
	>
		<view
				class="w-3/4 h-max flex flex-col gap-3"
		>
			<input
					class="bg-[#f0f0f0] px-4 py-3 rounded-2xl border border-[#f0f0f0]"
					v-model="username"
					placeholder="账号"
			/>
			<input
					class="bg-[#f0f0f0] px-4 py-3 rounded-2xl border border-[#f0f0f0]"
					v-model="password"
					password
					placeholder="密码"
			/>
			<button
					class="w-full flex items-center justify-center rounded-2xl bg-primary text-white"
					:disabled="loading"
					@click="submit"
			>
				{{ loading ? '登录中' : '登录' }}
			</button>
		</view>
	</view>
</template>

<script setup>
import { ref } from 'vue'
import { login } from '../../utils/api'

const username = ref('')
const password = ref('')
const loading = ref(false)

async function submit() {
	if (loading.value) return

	if (!username.value.trim() || !password.value) {
		uni.showToast({
			title: '请填写账号和密码',
			icon: 'none',
			duration: 2000,
		})
		return
	}

	loading.value = true

	try {
		await login({
			username: username.value.trim(),
			password: password.value,
		})

		password.value = ''
		uni.reLaunch({
			url: '/pages/devices/devices'
		})
	} catch (err) {
		uni.showToast({
			title: '登录失败，请检查账号密码',
			icon: 'none',
			duration: 2000,
		})
	} finally {
		loading.value = false
	}
}
</script>