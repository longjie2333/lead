<template>
  <main class="flex h-screen items-center justify-center bg-panel px-4">
    <form
				class="flex h-max w-3/4 max-w-sm flex-col gap-3"
				@submit.prevent="submit"
		>
      <input
					v-model.trim="username"
					class="rounded-2xl border border-[#f0f0f0] bg-[#f0f0f0] px-4 py-3 outline-none transition focus:border-primary focus:bg-white"
					autocomplete="username"
					placeholder="账号"
			/>
      <input
					v-model="password"
					class="rounded-2xl border border-[#f0f0f0] bg-[#f0f0f0] px-4 py-3 outline-none transition focus:border-primary focus:bg-white"
					type="password"
					autocomplete="current-password"
					placeholder="密码"
			/>
      <button
					class="flex w-full items-center justify-center rounded-2xl bg-primary px-4 py-3 font-semibold text-white disabled:opacity-70"
					:disabled="loading"
					type="submit"
			>
        {{ loading ? '登录中' : '登录' }}
      </button>
    </form>
  </main>
</template>

<script setup>
import { ref } from 'vue'
import { useToastStore } from '@/stores/toast'
import { login } from '@/utils/api'

const emit = defineEmits(['logged-in'])
const toastStore = useToastStore()

const username = ref('')
const password = ref('')
const loading = ref(false)

async function submit() {
	if (loading.value) return

	if (!username.value || !password.value) {
		toastStore.error('请填写账号和密码')
		return
	}

	loading.value = true
	try {
		await login({
			username: username.value,
			password: password.value,
		})
		password.value = ''
		emit('logged-in')
	} catch {
		toastStore.error('登录失败，请检查账号密码')
	} finally {
		loading.value = false
	}
}
</script>
