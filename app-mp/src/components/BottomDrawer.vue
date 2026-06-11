<template>
	<view
			v-if="rendered"
			class="bottom-drawer fixed top-0 bottom-0 left-0 right-0"
			:style="rootStyle"
	>
		<view
				class="bottom-drawer__mask absolute top-0 bottom-0 left-0 right-0 bg-black"
				:style="maskStyle"
				@click="handleMaskTap"
				@touchmove.stop="lockTouchMove"
		/>
		<view
				class="bottom-drawer__panel min-h-[90px] absolute left-0 right-0 bottom-0 flex flex-col overflow-hidden bg-white rounded-t-2xl"
				:class="[panelClass, { 'is-dragging': isDragging }]"
				:style="panelStyle"
				@click.stop
				@touchstart="startDrag($event, 'panel')"
				@touchmove="moveDrag"
				@touchend="endDrag"
				@touchcancel="cancelDrag"
		>
			<view
					v-if="showHandle"
					class="bottom-drawer__handle-wrap h-6 flex items-center justify-center shrink-0"
					@touchstart.stop="startDrag($event, 'handle')"
					@touchmove.stop="moveDrag"
					@touchend.stop="endDrag"
					@touchcancel.stop="cancelDrag"
			>
				<view class="bottom-drawer__handle w-9 h-1 rounded-full bg-[#d6d6d6]" />
			</view>

			<view
					v-if="hasHeader"
					class="bottom-drawer__header min-h-11 flex items-center justify-stretch pt-1.5 pb-4"
			>
				<slot name="header">
					<button
							v-if="showClose"
							class="bottom-drawer__close flex items-center justify-center gap-1 bg-transparent px-4 py-2"
							@tap.stop="requestClose('close')"
					>
						<view class="i-lucide-x text-2xl" />
						<view class="leading-none font-semibold opacity-0">取消</view>
					</button>
					<view class="bottom-drawer__title flex-1 text-center font-semibold">{{ title }}</view>
					<button
							v-if="showSubmit"
							class="bottom-drawer__submit flex items-center justify-center gap-1 bg-transparent disabled:bg-transparent px-4 py-2"
							:class="disableSubmit ? 'text-gray-400' : 'text-primary'"
							@tap.stop="requestSubmit('submit')"
					>
						<view class="leading-none font-semibold">确定</view>
						<view class="i-lucide-check text-2xl" />
					</button>
				</slot>
			</view>

			<view
					class="bottom-drawer__body min-h-0 flex-1 overflow-auto px-4 pb-4"
					:class="bodyClass"
			>
				<slot></slot>
			</view>

			<view
					v-if="slots.footer"
					class="bottom-drawer__footer px-4 pb-4"
			>
				<slot name="footer"></slot>
			</view>
		</view>
	</view>
</template>

<script setup>
import { computed, nextTick, onBeforeUnmount, ref, useSlots, watch } from 'vue'

const props = defineProps({
	open: {
		type: Boolean,
		default: false,
	},
	title: {
		type: String,
		default: '',
	},
	closeOnMask: {
		type: Boolean,
		default: true,
	},
	closeOnDrag: {
		type: Boolean,
		default: true,
	},
	showHandle: {
		type: Boolean,
		default: true,
	},
	showClose: {
		type: Boolean,
		default: false,
	},
	showSubmit: {
		type: Boolean,
		default: false,
	},
	disableSubmit: {
		type: Boolean,
		default: false,
	},
	dragHandleOnly: {
		type: Boolean,
		default: true,
	},
	height: {
		type: [String, Number],
		default: '',
	},
	maxHeight: {
		type: [String, Number],
		default: '82vh',
	},
	zIndex: {
		type: Number,
		default: 1000,
	},
	duration: {
		type: Number,
		default: 220,
	},
	maskOpacity: {
		type: Number,
		default: 0.45,
	},
	dragThreshold: {
		type: Number,
		default: 72,
	},
	velocityThreshold: {
		type: Number,
		default: 0.45,
	},
	panelClass: {
		type: [String, Array, Object],
		default: '',
	},
	bodyClass: {
		type: [String, Array, Object],
		default: '',
	},
})

const emit = defineEmits([
	'update:open',
	'open',
	'opened',
	'close',
	'closed',
	'mask-tap',
	'drag-close',
	'submit',
])

const slots = useSlots()
const rendered = ref(false)
const visible = ref(false)
const isDragging = ref(false)
const dragOffset = ref(0)

let dragStartX = 0
let dragStartY = 0
let dragStartTime = 0
let dragDirection = ''
let closeTimer = null
let openTimer = null

const hasHeader = computed(() => {
	return Boolean(slots.header || props.title || props.showClose || props.showSubmit)
})

const rootStyle = computed(() => {
	return {
		zIndex: props.zIndex,
	}
})

const maskStyle = computed(() => {
	const opacity = visible.value ? props.maskOpacity : 0

	return {
		transition: `opacity ${props.duration}ms ease`,
		opacity,
	}
})

const panelStyle = computed(() => {
	const y = isDragging.value
			? `${dragOffset.value}px`
			: visible.value
					? '0'
					: '100%'
	const style = {
		transform: `translate3d(0, ${y}, 0)`,
		transition: isDragging.value ? 'none' : `transform ${props.duration}ms ease`,
		maxHeight: normalizeSize(props.maxHeight),
	}

	const height = normalizeSize(props.height)
	if (height) {
		style.height = height
	}

	return style
})

watch(
		() => props.open,
		(open) => {
			if (open) {
				show()
			} else if (rendered.value || visible.value) {
				hide()
			}
		},
		{immediate: true}
)

onBeforeUnmount(() => {
	clearOpenTimer()
	clearCloseTimer()
})

function show() {
	clearCloseTimer()
	rendered.value = true
	dragOffset.value = 0
	isDragging.value = false

	nextTick(() => {
		if (!props.open || !rendered.value) return

		visible.value = true
		emit('open')

		clearOpenTimer()
		openTimer = setTimeout(() => {
			emit('opened')
		}, props.duration)
	})
}

function hide() {
	clearOpenTimer()
	visible.value = false
	isDragging.value = false
	dragOffset.value = 0
	dragDirection = ''

	clearCloseTimer()
	closeTimer = setTimeout(() => {
		if (!props.open) {
			rendered.value = false
			emit('closed')
		}
	}, props.duration)
}

function requestClose(source = 'program') {
	emit('close', source)
	emit('update:open', false)
}

function requestSubmit(source = 'program') {
	if (props.disableSubmit) return

	emit('submit', source)
}

function handleMaskTap() {
	emit('mask-tap')

	if (props.closeOnMask) {
		requestClose('mask')
	}
}

function startDrag(event, area) {
	if (!props.closeOnDrag || !props.open) return
	if (props.dragHandleOnly && area !== 'handle' && area !== 'header') return

	const touch = event.touches && event.touches[0]
	if (!touch) return

	dragStartX = touch.clientX
	dragStartY = touch.clientY
	dragStartTime = Date.now()
	dragDirection = ''
	dragOffset.value = 0
	isDragging.value = true
}

function moveDrag(event) {
	if (!isDragging.value) return

	const touch = event.touches && event.touches[0]
	if (!touch) return

	const deltaX = touch.clientX - dragStartX
	const deltaY = touch.clientY - dragStartY

	if (!dragDirection) {
		if (Math.abs(deltaX) < 6 && Math.abs(deltaY) < 6) return
		dragDirection = Math.abs(deltaY) >= Math.abs(deltaX) ? 'vertical' : 'horizontal'
	}

	if (dragDirection !== 'vertical') return

	if (typeof event.preventDefault === 'function') {
		event.preventDefault()
	}
	if (typeof event.stopPropagation === 'function') {
		event.stopPropagation()
	}

	dragOffset.value = Math.max(deltaY, 0)
}

function endDrag() {
	if (!isDragging.value) return

	const elapsed = Math.max(Date.now() - dragStartTime, 1)
	const velocity = dragOffset.value / elapsed
	const shouldClose = dragOffset.value >= props.dragThreshold || velocity >= props.velocityThreshold

	isDragging.value = false
	dragDirection = ''

	if (shouldClose) {
		emit('drag-close')
		requestClose('drag')
		return
	}

	dragOffset.value = 0
}

function cancelDrag() {
	isDragging.value = false
	dragOffset.value = 0
	dragDirection = ''
}

function lockTouchMove(event) {
	if (typeof event.preventDefault === 'function') {
		event.preventDefault()
	}
	if (typeof event.stopPropagation === 'function') {
		event.stopPropagation()
	}
}

function normalizeSize(value) {
	if (value === '' || value === null || value === undefined) return ''
	if (typeof value === 'number') return `${value}px`
	return value
}

function clearCloseTimer() {
	if (!closeTimer) return
	clearTimeout(closeTimer)
	closeTimer = null
}

function clearOpenTimer() {
	if (!openTimer) return
	clearTimeout(openTimer)
	openTimer = null
}

defineExpose({
	close: requestClose,
})
</script>

<style scoped>
.bottom-drawer__panel {
	padding-bottom: env(safe-area-inset-bottom);
	box-sizing: border-box;
}

.bottom-drawer__panel.is-dragging {
	will-change: transform;
}
</style>
