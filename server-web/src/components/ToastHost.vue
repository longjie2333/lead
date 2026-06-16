<template>
  <div class="pointer-events-none fixed inset-x-0 top-0 z-50 flex justify-center px-4 pt-[max(12px,env(safe-area-inset-top))]">
    <Transition name="toast-drop">
      <div
        v-if="toastStore.current"
        :key="toastStore.current.id"
        class="pointer-events-auto flex max-w-[min(92vw,420px)] items-center gap-2 rounded-2xl px-4 py-3 text-sm font-semibold text-white shadow-soft"
        :class="toastStore.current.type === 'success' ? 'bg-emerald-600' : 'bg-danger'"
        role="status"
      >
        <CircleCheck
					v-if="toastStore.current.type === 'success'"
					class="h-5 w-5 shrink-0"
				/>
        <CircleAlert
					v-else
					class="h-5 w-5 shrink-0"
				/>
        <span class="min-w-0 break-words">{{ toastStore.current.message }}</span>
      </div>
    </Transition>
  </div>
</template>

<script setup>
import { CircleAlert, CircleCheck } from 'lucide-vue-next'
import { useToastStore } from '@/stores/toast'

const toastStore = useToastStore()
</script>

<style scoped>
.toast-drop-enter-active {
  @apply transition duration-200 ease-out;
}

.toast-drop-leave-active {
  @apply transition duration-200 ease-in;
}

.toast-drop-enter-from {
  @apply -translate-y-full opacity-0;
}

.toast-drop-leave-from {
  @apply translate-y-0 opacity-100;
}

.toast-drop-enter-to {
  @apply translate-y-0 opacity-100;
}

.toast-drop-leave-to {
  @apply -translate-y-full opacity-0;
}
</style>
