<template>
	<view
			class="swipe-action relative rounded-2xl overflow-hidden"
			@touchstart="startSwipe"
			@touchmove="moveSwipe"
			@touchend="endSwipe"
			@touchcancel="cancelSwipe"
	>
		<view
				class="swipe-action__actions absolute top-0 left-0 right-0 bottom-0 flex justify-end"
				:style="actionsStyle"
		>
			<button
					v-for="action in normalizedActions"
					:key="action.key"
					class="swipe-action__button"
					:class="action.className"
					:style="action.style"
					:disabled="action.disabled"
					@click.stop="handleActionTap(action)"
			>
				{{ action.text }}
			</button>
		</view>
		<view
				class="swipe-action__content relative z-10 bg-white"
				:class="{ 'is-swiping': isSwiping }"
				:style="contentStyle"
				@click="handleTap"
		>
			<slot></slot>
		</view>
	</view>
</template>

<script setup>
import { computed, getCurrentInstance, nextTick, onMounted, ref } from 'vue'

const props = defineProps({
	open: {
		type: Boolean,
		default: false,
	},
	actionWidthRatio: {
		type: Number,
		default: 0.2,
	},
	actionOverlap: {
		type: Number,
		default: 0,
	},
	actions: {
		type: Array,
		default: () => [],
	},
	thresholdRatio: {
		type: Number,
		default: 1 / 3,
	},
	disabled: {
		type: Boolean,
		default: false,
	},
	lockScrollOnSwipe: {
		type: Boolean,
		default: true,
	},
})

const emit = defineEmits(['update:open', 'tap', 'action'])

const instance = getCurrentInstance()
const rowWidth = ref(0)
const swipeOffset = ref(0)
const isSwiping = ref(false)
const suppressNextTap = ref(false)

let swipeStartX = 0
let swipeStartY = 0
let swipeStartOffset = 0
let swipeDirection = ''

const normalizedActionWidthRatio = computed(() => {
	return Math.min(Math.max(Number(props.actionWidthRatio) || 0, 0), 1)
})

const normalizedThresholdRatio = computed(() => {
	return Math.min(Math.max(Number(props.thresholdRatio) || 0, 0), 1)
})

const normalizedActions = computed(() => {
	return props.actions.map((action, index) => {
		if (typeof action === 'string') {
			return {
				key: action,
				text: action,
				className: '',
				style: {},
				closeOnTap: true,
			}
		}

		return {
			key: action.key || action.value || action.text || action.label || index,
			text: action.text || action.label || '',
			className: action.className || '',
			style: action.style || {},
			background: action.background || action.backgroundColor,
			color: action.color,
			flex: action.flex,
			disabled: Boolean(action.disabled),
			closeOnTap: action.closeOnTap !== false,
		}
	})
})

const actionsStyle = computed(() => {
	return {
		left: 'auto',
		width: `${normalizedActionWidthRatio.value * 100}%`,
	}
})

const contentStyle = computed(() => {
	const offset = isSwiping.value
			? swipeOffset.value
			: props.open
					? getActionWidth()
					: 0

	return {
		transform: `translateX(-${offset}px)`,
	}
})

onMounted(measureWidth)

function measureWidth() {
	nextTick(() => {
		const query = uni.createSelectorQuery().in(instance.proxy)
		query
				.select('.swipe-action')
				.boundingClientRect((rect) => {
					if (rect && rect.width) {
						rowWidth.value = rect.width
					}
				})
				.exec()
	})
}

function getActionWidth() {
	return Math.max(rowWidth.value * normalizedActionWidthRatio.value - props.actionOverlap, 0)
}

function startSwipe(event) {
	if (props.disabled) return

	const touch = event.touches && event.touches[0]
	if (!touch) return

	measureWidth()

	isSwiping.value = true
	swipeStartX = touch.clientX
	swipeStartY = touch.clientY
	swipeStartOffset = props.open ? getActionWidth() : 0
	swipeOffset.value = swipeStartOffset
	swipeDirection = ''
}

function moveSwipe(event) {
	if (!isSwiping.value) return

	const touch = event.touches && event.touches[0]
	if (!touch) return

	const deltaX = touch.clientX - swipeStartX
	const deltaY = touch.clientY - swipeStartY

	if (!swipeDirection) {
		if (Math.abs(deltaX) < 6 && Math.abs(deltaY) < 6) return
		swipeDirection = Math.abs(deltaX) > Math.abs(deltaY) ? 'horizontal' : 'vertical'
	}

	if (swipeDirection !== 'horizontal') return

	lockPageScroll(event)

	const nextOffset = swipeStartOffset - deltaX
	swipeOffset.value = Math.min(Math.max(nextOffset, 0), getActionWidth())
}

function endSwipe() {
	if (!isSwiping.value) return

	if (swipeDirection === 'horizontal') {
		const shouldOpen = swipeOffset.value >= getActionWidth() * normalizedThresholdRatio.value
		emit('update:open', shouldOpen)
	}

	isSwiping.value = false
	swipeOffset.value = 0

	if (swipeDirection) {
		suppressNextTap.value = true
		setTimeout(() => {
			suppressNextTap.value = false
		}, 220)
	}

	swipeDirection = ''
}

function cancelSwipe() {
	isSwiping.value = false
	swipeOffset.value = 0
	swipeDirection = ''
}

function close() {
	emit('update:open', false)
}

function lockPageScroll(event) {
	if (!props.lockScrollOnSwipe) return

	if (typeof event.preventDefault === 'function') {
		event.preventDefault()
	}
	if (typeof event.stopPropagation === 'function') {
		event.stopPropagation()
	}
}

function handleActionTap(action) {
	if (action.disabled) return

	if (action.closeOnTap) {
		close()
	}

	emit('action', action)
}

function handleTap(event) {
	if (suppressNextTap.value) return

	if (props.open) {
		close()
		return
	}

	emit('tap', event)
}

defineExpose({
	close,
	measureWidth,
})
</script>

<style scoped>
.swipe-action__content {
	transition: transform 180ms ease;
}

.swipe-action__content.is-swiping {
	transition: none;
}

.swipe-action__button {
	flex: 1;
	height: 100%;
	margin: 0;
	padding: 0;
	display: flex;
	align-items: center;
	justify-content: center;
	border-radius: 0;
	font-size: 28rpx;
	font-weight: 600;
	line-height: 1;
}

.swipe-action__button:last-child {
	border-radius: 0 1rem 1rem 0;
}

.swipe-action__button::after {
	border: none;
}
</style>
