<template>
  <section class="h-full overflow-y-auto px-4 pb-8 pt-4">
    <div class="mx-auto flex w-full max-w-3xl flex-col gap-4">
      <section class="rounded-2xl bg-white p-4 shadow-soft">
        <div class="mb-3 text-lg font-semibold text-ink">修改密码</div>
        <form
					class="flex flex-col gap-2 sm:flex-row"
					@submit.prevent="resetPassword"
				>
          <input
						v-model="newPassword"
						class="rounded-xl border border-[#f0f0f0] bg-[#f0f0f0] px-4 py-3 outline-none transition focus:border-primary focus:bg-white sm:min-w-0 sm:flex-1"
						type="password"
						autocomplete="new-password"
						placeholder="请输入新密码"
				/>
          <input
						v-model="confirmPassword"
						class="rounded-xl border border-[#f0f0f0] bg-[#f0f0f0] px-4 py-3 outline-none transition focus:border-primary focus:bg-white sm:min-w-0 sm:flex-1"
						type="password"
						autocomplete="new-password"
						placeholder="请输入刚才的新密码"
				/>
          <button
						class="mt-2 flex w-full items-center justify-center rounded-xl bg-primary px-4 py-3 font-semibold text-white disabled:opacity-70 sm:mt-0 sm:w-auto sm:px-6"
						:disabled="loading"
						type="submit"
				>
            {{ loading ? '修改中' : '确定' }}
          </button>
        </form>
      </section>

      <template v-if="userStore.user.isAdmin">
        <div class="flex items-center justify-between px-4">
          <div class="text-lg font-semibold text-ink">所有用户</div>
          <button
						class="flex items-center gap-1 rounded-xl bg-primary px-3 py-2 font-semibold text-white disabled:opacity-70"
						:disabled="loading"
						type="button"
						@click="doAddUser"
				>
            <Plus class="h-5 w-5" />
            <span>新增</span>
          </button>
        </div>

        <div
					v-if="users.length === 0"
					class="rounded-2xl bg-white p-5 text-center shadow-soft"
				>
          <div class="text-base font-semibold text-ink">暂无用户</div>
        </div>

        <div
					v-else
					class="flex flex-col gap-2"
				>
          <div
						v-for="user in users"
						:key="user.id"
						class="flex items-center gap-3 rounded-2xl bg-white p-4 shadow-soft"
				>
            <GripVertical class="h-5 w-5 shrink-0 text-gray-400" />
            <button
							class="min-w-0 flex-1 text-left"
							type="button"
							@click="doEditUser(user)"
					>
              <div class="truncate text-base font-semibold text-ink">
                {{ user.username }}{{ user.isAdmin ? `（${roleLabel(user)}）` : '' }}
              </div>
              <div class="truncate text-xs text-gray-600">{{ user.id }}</div>
            </button>
            <button
							class="icon-button shrink-0 rounded-xl p-2 text-danger disabled:opacity-50"
							:disabled="loading"
							type="button"
							title="删除"
							@click="handleDeleteUser(user)"
					>
              <Trash2 class="h-5 w-5" />
            </button>
          </div>
        </div>
      </template>
    </div>

    <div
			v-if="openDrawer"
			class="fixed inset-0 z-40 flex items-end bg-black/30 p-4 sm:items-center sm:justify-center"
			@click.self="closeDrawer"
		>
      <form
				class="w-full rounded-2xl bg-white p-4 shadow-soft sm:max-w-md"
				@submit.prevent="handleSubmit"
			>
        <div class="mb-3 flex items-center justify-between">
          <div class="text-lg font-semibold text-ink">{{ openDrawerMode === 'add' ? '新增用户' : '编辑用户' }}</div>
          <button
						class="icon-button rounded-xl p-2"
						type="button"
						title="关闭"
						@click="closeDrawer"
				>
            <X class="h-5 w-5" />
          </button>
        </div>
        <input
					v-model.trim="newUser.username"
					class="mb-2 w-full rounded-2xl border border-[#f0f0f0] bg-[#f0f0f0] p-4 outline-none transition focus:border-primary focus:bg-white"
					placeholder="用户名"
				/>
        <input
					v-model="newUser.password"
					class="mb-2 w-full rounded-2xl border border-[#f0f0f0] bg-[#f0f0f0] p-4 outline-none transition focus:border-primary focus:bg-white"
					type="password"
					autocomplete="new-password"
					:placeholder="openDrawerMode === 'add' ? '密码' : '留空则不重设密码'"
				/>
        <div class="mb-4 grid grid-cols-2 gap-1 rounded-2xl bg-[#f0f0f0] p-1">
          <button
						class="w-full rounded-xl px-3 py-2"
						:class="newUser.role === 'user' ? 'bg-white shadow-sm' : 'bg-inherit'"
						type="button"
						@click="newUser.role = 'user'"
				>
            普通用户
          </button>
          <button
						class="w-full rounded-xl px-3 py-2"
						:class="newUser.role === 'admin' ? 'bg-white shadow-sm' : 'bg-inherit'"
						type="button"
						@click="newUser.role = 'admin'"
				>
            管理员
          </button>
        </div>
        <button
					class="flex w-full items-center justify-center rounded-2xl bg-primary px-4 py-3 font-semibold text-white disabled:opacity-70"
					:disabled="loading || !canSubmit"
					type="submit"
				>
          {{ loading ? '保存中' : '保存' }}
        </button>
      </form>
    </div>
  </section>
</template>

<script setup>
import { computed, onMounted, ref } from 'vue'
import { GripVertical, Plus, Trash2, X } from 'lucide-vue-next'
import { useToastStore } from '@/stores/toast'
import { useUserStore } from '@/stores/user'
import { createUser, deleteUser, fetchUsers, updateUser } from '@/utils/api'
import { roleLabel } from '@/utils/format'

const userStore = useUserStore()
const toastStore = useToastStore()

const loading = ref(false)
const openDrawer = ref(false)
const openDrawerMode = ref('add')
const newPassword = ref('')
const confirmPassword = ref('')
const users = ref([])
const newUser = ref(createEmptyUser())

const canSubmit = computed(() => {
	if (openDrawerMode.value === 'add') {
		return newUser.value.username && newUser.value.password
	}

	return newUser.value.username
})

onMounted(() => {
	if (userStore.user.isAdmin) {
		loadUsers()
	}
})

async function resetPassword() {
	if (loading.value) return

	if (newPassword.value === '') {
		toastStore.error('请输入新密码')
		return
	}

	if (confirmPassword.value === '') {
		toastStore.error('请输入确认密码')
		return
	}

	if (confirmPassword.value !== newPassword.value) {
		toastStore.error('两次密码不一致')
		return
	}

	loading.value = true
	try {
		await updateUser({
			...userStore.user,
			password: newPassword.value,
		})
		toastStore.success('设置成功')
		newPassword.value = ''
		confirmPassword.value = ''
	} catch {
		toastStore.error('设置失败')
	} finally {
		loading.value = false
	}
}

async function loadUsers() {
	try {
		users.value = await fetchUsers()
	} catch {
		toastStore.error('用户列表加载失败')
	}
}

function doAddUser() {
	openDrawerMode.value = 'add'
	newUser.value = createEmptyUser()
	openDrawer.value = true
}

function doEditUser(user) {
	openDrawerMode.value = 'edit'
	newUser.value = {
		...user,
		password: '',
		role: user.role || (user.isAdmin ? 'admin' : 'user'),
	}
	openDrawer.value = true
}

function closeDrawer() {
	openDrawer.value = false
	newUser.value = createEmptyUser()
}

async function handleSubmit() {
	if (loading.value || !canSubmit.value) return

	loading.value = true
	try {
		if (openDrawerMode.value === 'add') {
			await createUser(newUser.value)
			toastStore.success('新增成功')
		} else {
			await updateUser(newUser.value)
			toastStore.success('更新成功')
		}

		await loadUsers()
		closeDrawer()
	} catch {
		toastStore.error(openDrawerMode.value === 'add' ? '新增失败' : '更新失败')
	} finally {
		loading.value = false
	}
}

async function handleDeleteUser(user) {
	if (loading.value) return

	if (userStore.user.id === user.id) {
		toastStore.error('无法删除自己')
		return
	}

	if (user.id === 'admin') {
		toastStore.error('无法删除 admin 用户')
		return
	}

	if (!window.confirm(`确定删除用户 ${user.username} 吗？`)) return

	loading.value = true
	try {
		await deleteUser(user.id)
		await loadUsers()
		toastStore.success('删除成功')
	} catch {
		toastStore.error('删除失败')
	} finally {
		loading.value = false
	}
}

function createEmptyUser() {
	return {
		username: '',
		password: '',
		role: 'user',
	}
}
</script>
