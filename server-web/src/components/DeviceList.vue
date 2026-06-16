<template>
  <section class="flex min-h-0 w-full flex-1 flex-col pt-4">
    <div class="text-center p-4">
			<h1 class="text-lg font-semibold text-ink">设备列表</h1>
			<p class="mt-1 text-xs text-gray-600">{{ devices.length }} 台设备，{{ onlineCount }} 台在线</p>
    </div>

    <div
				v-if="devices.length === 0 && !loading"
				class="max-w-full mx-auto mt-4 rounded-2xl bg-white p-4 text-center shadow-soft"
		>
      <div class="text-base font-semibold text-ink">暂无设备</div>
      <div class="mt-2 text-sm text-gray-600">请先在安卓被控端登录同一账号</div>
    </div>

    <div class="grid grid-cols-1 xl:grid-cols-3 lg:grid-cols-3 md:grid-cols-2 sm:grid-cols-2 gap-4 p-4 mx-0 md:mx-auto">
      <button
					v-for="device in devices"
					:key="device.id"
					class="w-full rounded-2xl bg-white p-4 text-left shadow-soft transition hover:shadow-lg"
					:class="{ 'ring-2 ring-primary/35': selectedId === device.id }"
					type="button"
					@click="$emit('select', device)"
			>
        <div class="min-w-0">
          <div class="flex items-center justify-between gap-2">
            <div class="flex items-center gap-0.5 truncate text-base font-semibold text-ink">
							<span class="w-full overflow-hidden overflow-ellipsis">{{ displayName(device) }}</span>
							<span
									class="rounded-md p-1 text-xs opacity-40 text-ink hover:bg-gray-200 hover:opacity-90"
									@click.stop="$emit('note', device)"
							>
								<SquarePen class="w-4 h-4" />
							</span>
            </div>
            <div
								class="status-pill"
								:class="{ online: device.online }"
						>
              {{ device.online ? '在线' : '离线' }}
            </div>
          </div>
          <div class="mt-2 flex items-center justify-between gap-2 text-nowrap">
            <div class="truncate text-xs text-gray-600">{{ device.id }}</div>
            <div class="text-xs text-gray-600">{{ formatTime(device.updatedAt).slice(5) }}</div>
          </div>
        </div>
      </button>
    </div>
  </section>
</template>

<script setup>
import { computed } from 'vue'
import { useDeviceStore } from '@/stores/device'
import { formatTime } from '@/utils/format'
import { SquarePen } from 'lucide-vue-next'

const props = defineProps({
	devices: {
		type: Array,
		default: () => [],
	},
	selectedId: {
		type: String,
		default: '',
	},
	loading: {
		type: Boolean,
		default: false,
	},
})

defineEmits(['select', 'note'])

const deviceStore = useDeviceStore()
const onlineCount = computed(() => props.devices.filter((device) => device.online).length)
const displayName = (device) => deviceStore.displayName(device)
</script>
