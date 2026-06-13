<template>
	<NavBar main-class="pl-4">
		<button
				class="rounded-xl bg-white flex items-center gap-2"
				:disabled="loading"
				@click="userStore.removeAuth()"
		>
			<view class="text-base">登出</view>
			<view class="i-lucide-log-out text-xl"></view>
		</button>
	</NavBar>
	<view class="px-4 pb-8 pt-4">
		<view class="px-4 mb-3 flex items-center justify-between">
			<view class="text-lg font-semibold text-ink">修改密码</view>
		</view>
		<view class="p-4 flex flex-col gap-2 bg-white rounded-2xl">
			<input
					class="bg-[#f0f0f0] px-4 py-3 rounded-xl border border-[#f0f0f0]"
					v-model="newPassword"
					password
					placeholder="请输入新密码"
			/>
			<input
					class="bg-[#f0f0f0] px-4 py-3 rounded-xl border border-[#f0f0f0]"
					v-model="confirmPassword"
					password
					placeholder="请输入刚才的新密码"
			/>
			<button
					class="w-full flex items-center justify-center rounded-xl bg-primary text-white mt-2"
					:disabled="loading"
					@click="resetPassword"
			>
				{{ loading ? '修改中' : '确定' }}
			</button>
		</view>
		<template v-if="userStore.user.isAdmin">
			<view class="px-4 mt-4 mb-3 flex items-center justify-between">
				<view class="text-lg font-semibold text-ink">所有用户</view>
				<button
						class="w-max rounded-xl bg-primary text-white flex items-center gap-1 px-3 py-2 m-0"
						:disabled="loading"
						v-if="userStore.user.isAdmin"
						@click="doAddUser"
				>
					<view class="i-lucide-plus text-2xl" />
					<view class="leading-none">新增</view>
				</button>
			</view>
			<view
					class="rounded-2xl bg-white p-5 text-center"
					v-if="users.length === 0"
			>
				<view class="text-base font-semibold text-ink">暂无用户</view>
			</view>
			<view
					class="flex flex-col gap-2"
					v-else
			>
				<SwipeAction
						v-for="user in users"
						:key="user.id"
						:open="openSwipeId === user.id"
						:action-width-ratio="0.2"
						:actions="userSwipeActions"
						@action="handleUserSwipeAction(user, $event)"
						@update:open="setUserSwipeOpen(user, $event)"
						@tap="doEditUser(user)"
				>
					<view class="rounded-2xl bg-white p-4">
						<view class="flex items-center gap-4">
							<view class="i-lucide-grip-vertical text-2xl opacity-80" />
							<view class="flex-1">
								<view class="truncate text-base font-semibold text-ink">{{ user.username }}{{ user.isAdmin ? `（${roleLabel(user)}）` : '' }}</view>
								<view class="text-xs text-gray-600">{{ user.id }}</view>
							</view>
						</view>
					</view>
				</SwipeAction>
			</view>
		</template>
	</view>
	<BottomDrawer
			:title="openDrawerMode === 'add' ? '新增用户' : '编辑用户'"
			show-close
			show-submit
			:disable-submit="loading || !canSubmit"
			v-model:open="openDrawer"
			@submit="handleSubmit"
			@closed="handleDrawerClosed"
	>
		<view>
			<input
					class="bg-[#f0f0f0] p-4 rounded-2xl mb-2"
					v-model="newUser.username"
					placeholder="用户名"
			/>
			<input
					class="bg-[#f0f0f0] p-4 rounded-2xl mb-2"
					v-model="newUser.password"
					password
					:placeholder="openDrawerMode === 'add' ? '密码' : '留空则不重设密码'"
			/>
			<view class="grid grid-cols-2 gap-1 bg-[#f0f0f0] p-1 rounded-2xl">
				<button
						class="w-full rounded-xl"
						:class="newUser.role === 'user' ? 'bg-white' : 'bg-inherit'"
						@click="newUser.role = 'user'"
				>普通用户</button>
				<button
						class="w-full rounded-xl"
						:class="newUser.role === 'admin' ? 'bg-white' : 'bg-inherit'"
						@click="newUser.role = 'admin'"
				>管理员</button>
			</view>
		</view>
	</BottomDrawer>
</template>

<script setup>
import NavBar from '../../components/NavBar.vue'
import SwipeAction from '../../components/SwipeAction.vue'
import BottomDrawer from '../../components/BottomDrawer.vue'
import { computed, ref } from 'vue'
import { useUserStore } from '../../stores/user'
import { createUser, deleteUser, fetchUsers, updateUser } from '../../utils/api'
import { roleLabel } from '../../utils'

const userStore = useUserStore()

const loading = ref(false)
const openDrawer = ref(false)
const openDrawerMode = ref('add')
const newPassword = ref('')
const confirmPassword = ref('')

const users = ref([])
const newUser = ref({
	username: '',
	password: '',
	role: 'user',
})
const openSwipeId = ref('')
const userSwipeActions = [
	{
		key: 'delete',
		text: '删除',
		className: 'bg-red-500 text-white',
	}
]

const canSubmit = computed(() => {
	if (openDrawerMode.value === 'add') {
		return newUser.value.username && newUser.value.password
	}

	return newUser.value.username
})

if (userStore.user.isAdmin) {
	loadUsers()
}

async function resetPassword() {
	if (loading.value) return

	loading.value = true

	if (newPassword.value === '') {
		loading.value = false
		uni.showToast({
			title: '请输入新密码',
			icon: 'none',
			duration: 2000,
		})
		return
	}

	if (confirmPassword.value === '') {
		loading.value = false
		uni.showToast({
			title: '请输入确认密码',
			icon: 'none',
			duration: 2000,
		})
		return
	}

	if (confirmPassword.value !== newPassword.value) {
		loading.value = false
		uni.showToast({
			title: '两次密码不一致',
			icon: 'none',
			duration: 2000,
		})
		return
	}

	try {
		await updateUser({
			...userStore.user,
			password: newPassword.value,
		})
		uni.showToast({
			title: '设置成功',
			icon: 'none',
			duration: 2000,
		})

		newPassword.value = ''
		confirmPassword.value = ''
	} catch (err) {
		uni.showToast({
			title: '设置失败',
			icon: 'none',
			duration: 2000,
		})
	} finally {
		loading.value = false
	}
}

function doAddUser() {
	openDrawer.value = true
	openDrawerMode.value = 'add'
}

function doEditUser(user) {
	openDrawer.value = true
	openDrawerMode.value = 'edit'

	newUser.value = JSON.parse(JSON.stringify(user))
}

async function loadUsers() {
	try {
		users.value = await fetchUsers()
	} catch (err) {
		uni.showToast({
			title: '用户列表加载失败',
			icon: 'none',
			duration: 2000,
		})
	}
}

function setUserSwipeOpen(user, open) {
	const id = user.id

	if (open) {
		openSwipeId.value = id
	} else if (openSwipeId.value === id) {
		openSwipeId.value = ''
	}
}

async function handleUserSwipeAction(user, action) {
	switch (action.key) {
		case 'delete':
			if (loading.value) return

			if (userStore.user.id === user.id) {
				uni.showToast({
					title: '无法删除自己',
					icon: 'none',
					duration: 2000,
				})
				return
			}

			if ('admin' === user.id) {
				uni.showToast({
					title: '无法删除 admin 用户',
					icon: 'none',
					duration: 2000,
				})
				return
			}

			loading.value = true
			try {
				await deleteUser(user.id)
				await loadUsers()
			} catch (err) {
				uni.showToast({
					title: '删除失败',
					icon: 'none',
					duration: 2000,
				})
			} finally {
				loading.value = false
				openSwipeId.value = ''
			}
			break
	}
}

async function handleSubmit() {
	if (loading.value) return

	loading.value = true

	try {
		if (openDrawerMode.value === 'add') {
			await createUser(newUser.value)
		} else {
			await updateUser(newUser.value)
		}

		await loadUsers()

		openDrawer.value = false
	} catch (err) {
		uni.showToast({
			title: openDrawerMode.value === 'add' ? '新增失败' : '更新失败',
			icon: 'none',
			duration: 2000,
		})
	} finally {
		loading.value = false
	}
}

function handleDrawerClosed() {
	newUser.value = {
		username: '',
		password: '',
		role: 'user',
	}
}
</script>

<style scoped>

</style>
