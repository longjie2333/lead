<template>
  <header class="sticky top-0 z-30 border-b border-line bg-[rgb(247_247_247_/_89%)] p-2 backdrop-blur">
    <div class="flex items-center justify-between gap-3">
      <div class="flex min-w-0 items-stretch gap-2">
        <button
						v-if="showBack"
						class="icon-button rounded-xl bg-white px-2"
						type="button"
						title="返回"
						@click="$emit('back')"
				>
          <ChevronLeft class="h-5 w-5" />
        </button>
        <div class="flex min-w-0 items-center gap-2 rounded-xl bg-white px-3 py-2">
          <UserCog
							v-if="user?.isAdmin"
							class="h-5 w-5 shrink-0"
					/>
          <User
							v-else
							class="h-5 w-5 shrink-0"
					/>
          <div class="min-w-0 truncate text-sm font-semibold text-ink">{{ user?.username || 'Lead' }}</div>
        </div>
      </div>
      <div class="flex items-center gap-2">
        <slot name="actions" />
        <button
						class="flex items-center rounded-xl bg-white p-2 text-sm font-semibold text-ink"
						type="button"
						:disabled="loading"
						@click="$emit('logout')"
				>
          <LogOut class="h-5 w-5" />
        </button>
      </div>
    </div>
  </header>
</template>

<script setup>
import { ChevronLeft, LogOut, User, UserCog } from 'lucide-vue-next'

defineProps({
	user: {
		type: Object,
		default: () => ({}),
	},
	showBack: {
		type: Boolean,
		default: false,
	},
	loading: {
		type: Boolean,
		default: false,
	},
})

defineEmits(['back', 'logout'])
</script>
