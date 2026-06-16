<template>
  <section class="flex h-full min-h-0 min-w-0 flex-1 select-none flex-col overflow-hidden overscroll-none bg-[#050608] text-[#f7f9fc]">
    <div
				ref="screenAreaEl"
				class="relative min-h-0 flex-1 overflow-hidden overscroll-none bg-[#050608] m-2"
		>
      <video
					ref="videoEl"
					class="block h-full w-full touch-none bg-[#050608] object-contain"
					autoplay
					playsinline
					muted
			></video>
      <div
					ref="touchLayerEl"
					class="absolute inset-0 cursor-crosshair touch-none select-none [-webkit-touch-callout:none] [-webkit-user-select:none]"
					:data-channel="controlChannelLabel"
					@contextmenu.prevent="preventNativeGesture"
					@dragstart.prevent="preventNativeGesture"
					@selectstart.prevent="preventNativeGesture"
					@touchstart.prevent="preventNativeGesture"
					@touchmove.prevent="preventNativeGesture"
					@pointerdown="handlePointerDown"
					@pointermove="handlePointerMove"
					@pointerup="handlePointerUp"
					@pointercancel="handlePointerCancel"
					@wheel="handleWheel"
			></div>

      <div
					v-if="!statusPanelCollapsed"
					data-floating-panel="status"
					class="absolute z-10"
					:style="panelStyle('status')"
			>
        <div class="flex flex-col gap-0.5 text-xs text-slate-300">
          <div class="max-w-max grid grid-cols-[auto_minmax(0,1fr)] items-baseline gap-2 px-2 py-1 rounded-lg bg-[#0e1116]/30 shadow-2xl backdrop-blur overflow-hidden whitespace-nowrap">
						<span>状态</span>
						<strong class="min-w-0 break-words font-semibold text-white overflow-ellipsis line-clamp-1">{{ status }}</strong>
					</div>
          <div class="max-w-max grid grid-cols-[auto_minmax(0,1fr)] items-baseline gap-2 px-2 py-1 rounded-lg bg-[#0e1116]/30 shadow-2xl backdrop-blur overflow-hidden whitespace-nowrap">
						<span>输入</span>
						<strong class="min-w-0 break-words font-semibold text-white overflow-ellipsis line-clamp-1">{{ controlStatus }}</strong>
					</div>
          <div class="w-max grid grid-cols-[auto_minmax(0,1fr)] items-baseline gap-2 px-2 py-1 rounded-lg bg-[#0e1116]/30 shadow-2xl backdrop-blur">
						<span>延时</span>
						<strong class="min-w-0 break-words font-semibold text-white">{{ latency }}</strong>
					</div>
          <div class="w-max grid grid-cols-[auto_minmax(0,1fr)] items-baseline gap-2 px-2 py-1 rounded-lg bg-[#0e1116]/30 shadow-2xl backdrop-blur">
						<span>线路</span>
						<strong class="min-w-0 break-words font-semibold text-white">{{ routeMode }}</strong>
					</div>
          <div class="w-max grid grid-cols-[auto_minmax(0,1fr)] items-baseline gap-2 px-2 py-1 rounded-lg bg-[#0e1116]/30 shadow-2xl backdrop-blur">
						<span>候选</span>
						<strong class="min-w-0 break-words font-semibold text-white">{{ routeDetail }}</strong>
					</div>
        </div>
      </div>

      <div
					v-if="!controlPanelCollapsed"
					ref="controlPanelEl"
					data-floating-panel="control"
					class="absolute z-10 w-max max-w-[calc(100%-24px)] overflow-auto rounded-2xl bg-[#0e1116]/75 shadow-2xl backdrop-blur"
					:style="controlPanelStyle()"
			>
        <div class="grid gap-2 p-2 text-xs text-slate-300">
          <button
							class="tool-button tool-button-panel tool-button-control-request"
							:class="{ 'is-enabled': controlEnabled }"
							type="button"
							@click="requestControlPermission"
					>
            <ShieldCheck class="w-4 h-4" />
            <span>{{ controlEnabled ? '控制权限已开启' : '申请控制' }}</span>
          </button>

          <div class="grid grid-cols-2 gap-1">
            <button
								class="tool-button tool-button-panel"
								:class="{ 'is-active': inputMode === 'direct' }"
								type="button"
								title="触控"
								@click="setInputMode('direct')"
						>
              <Hand class="w-4 h-4" />
            </button>
            <button
								class="tool-button tool-button-panel"
								:class="{ 'is-active': inputMode === 'trackpad' }"
								type="button"
								title="触控板"
								@click="setInputMode('trackpad')"
						>
              <Touchpad class="w-4 h-4" />
            </button>
          </div>

          <div class="grid grid-cols-3 gap-1">
            <button
								v-for="option in qualityOptions"
								:key="option.value"
								class="tool-button tool-button-panel tool-button-quality"
								:class="{ 'is-active': qualityMode === option.value }"
								type="button"
								@click="setQualityMode(option.value, true)"
						>
              {{ option.label }}
            </button>
          </div>
        </div>
      </div>
    </div>

    <nav
				class="w-max flex flex-none items-center justify-center gap-2 mx-auto mb-[max(10px,env(safe-area-inset-bottom))]"
				aria-label="系统导航"
		>
      <button
					class="tool-button tool-button-nav"
					type="button"
					title="连接监控"
					@click="togglePanel('status')"
			>
        <Activity class="w-3 h-3" />
      </button>
      <div class="flex">
				<button
						class="tool-button tool-button-nav tool-button-nav-left"
						type="button"
						title="返回"
						@click="sendControl({ action: 'back' })"
				>
					<ChevronLeft class="w-5 h-5" />
				</button>
				<button
						class="tool-button tool-button-nav tool-button-nav-middle"
						type="button"
						title="主页"
						@click="sendControl({ action: 'home' })"
				>
					<Home class="w-4 h-4" />
				</button>
				<button
						class="tool-button tool-button-nav tool-button-nav-right"
						type="button"
						title="后台"
						@click="sendControl({ action: 'recents' })"
				>
					<GalleryHorizontalEnd class="w-4 h-4" />
				</button>
			</div>
      <button
					ref="controlButtonEl"
					class="tool-button tool-button-nav"
					type="button"
					title="控制设置"
					@click="togglePanel('control')"
			>
        <SlidersHorizontal class="w-3 h-3" />
      </button>
    </nav>
  </section>
</template>

<script setup>
import { computed, nextTick, onBeforeUnmount, onMounted, ref, watch } from 'vue'
import {
	Activity,
	ChevronLeft,
	GalleryHorizontalEnd,
	Hand,
	Home,
	Touchpad,
	ShieldCheck,
	SlidersHorizontal,
} from 'lucide-vue-next'
import { fetchDevices } from '@/utils/api'

const props = defineProps({
	token: {
		type: String,
		required: true,
	},
	device: {
		type: Object,
		default: null,
	},
	title: {
		type: String,
		default: '远程控制',
	},
})

const emit = defineEmits(['device-error'])

const videoEl = ref(null)
const touchLayerEl = ref(null)
const screenAreaEl = ref(null)
const controlPanelEl = ref(null)
const controlButtonEl = ref(null)
const status = ref('等待选择设备')
const latency = ref('RTT -- ms')
const routeMode = ref('--')
const routeDetail = ref('--')
const controlStatus = ref('--')
const controlEnabled = ref(false)
const qualityMode = ref(localStorage.getItem('lead-quality-mode') || 'balanced')
const inputMode = ref(localStorage.getItem('lead-input-mode') || 'direct')
const controlChannelLabel = ref('WebSocket')
const statusPanelCollapsed = ref(true)
const controlPanelCollapsed = ref(true)
const panelPositions = ref({
	status: {x: 12, y: 12},
	control: {x: 12, y: 12},
})

const qualityOptions = [
	{value: 'quality', label: '画质'},
	{value: 'balanced', label: '均衡'},
	{value: 'speed', label: '速度'},
]

const deviceId = computed(() => props.device?.id || '')

let socket
let peerConnection
let controlChannel
let iceServers = [{urls: 'stun:stun.l.google.com:19302'}]
let relayMode = 'turn'
let sfuUrl = ''
let statsTimer
let reconnectTimer
let usingRelayOnly = false
let activePointers = new Map()
let gesturePointers = new Map()
let longPressTimer
let longPressPointerId
let longPressSent = false
let controlPanelDismissListenersActive = false
let controlPanelSafePoint

watch(
		() => [props.token, deviceId.value],
		async () => {
			await nextTick()
			restart()
		},
		{immediate: true},
)

watch(controlPanelCollapsed, async (collapsed) => {
	if (collapsed) {
		removeControlPanelDismissListeners()
		return
	}
	await nextTick()
	updateControlPanelPosition()
	addControlPanelDismissListeners()
})

onMounted(() => {
	window.addEventListener('keydown', handleKeydown)
	window.addEventListener('resize', resetPanelPositions)
	nextTick(resetPanelPositions)
})

onBeforeUnmount(() => {
	window.removeEventListener('keydown', handleKeydown)
	window.removeEventListener('resize', resetPanelPositions)
	removeControlPanelDismissListeners()
	shutdown()
})

function panelStyle(panel) {
	const position = panelPositions.value[panel]
	return {
		left: `${position.x}px`,
		right: `12px`,
		top: `${position.y}px`,
	}
}

function controlPanelStyle() {
	const position = panelPositions.value.control
	return {
		left: `${position.x}px`,
		top: `${position.y}px`,
	}
}

function togglePanel(panel) {
	if (panel === 'status') {
		statusPanelCollapsed.value = !statusPanelCollapsed.value
		nextTick(() => clampPanel(panel))
	} else {
		controlPanelCollapsed.value = !controlPanelCollapsed.value
	}
}

function resetPanelPositions() {
	const screenRect = screenAreaEl.value?.getBoundingClientRect()
	if (!screenRect?.width) return
	panelPositions.value = {
		status: {x: 12, y: 12},
		control: {x: 12, y: 12},
	}
	if (!controlPanelCollapsed.value) nextTick(updateControlPanelPosition)
}

function updateControlPanelPosition() {
	const screenRect = screenAreaEl.value?.getBoundingClientRect()
	const buttonRect = controlButtonEl.value?.getBoundingClientRect()
	const panelRect = controlPanelEl.value?.getBoundingClientRect()
	if (!screenRect || !buttonRect || !panelRect) return

	const inset = 12
	const gap = 8
	const panelWidth = Math.min(panelRect.width, Math.max(0, screenRect.width - inset * 2))
	const buttonCenterX = buttonRect.left + buttonRect.width / 2 - screenRect.left
	const maxX = Math.max(inset, screenRect.width - panelWidth - inset)
	const desiredX = buttonCenterX - panelWidth / 2
	const desiredY = buttonRect.top - screenRect.top - panelRect.height - gap
	const maxY = Math.max(inset, screenRect.height - panelRect.height - inset)

	panelPositions.value = {
		...panelPositions.value,
		control: {
			x: Math.round(Math.max(inset, Math.min(maxX, desiredX))),
			y: Math.round(Math.max(inset, Math.min(maxY, desiredY))),
		},
	}
	rememberControlPanelSafePoint()
}

function addControlPanelDismissListeners() {
	if (controlPanelDismissListenersActive) return
	document.addEventListener('pointerdown', handleControlPanelPointerDown, true)
	window.addEventListener('mousemove', handleControlPanelMouseMove)
	controlPanelDismissListenersActive = true
}

function removeControlPanelDismissListeners() {
	if (!controlPanelDismissListenersActive) return
	document.removeEventListener('pointerdown', handleControlPanelPointerDown, true)
	window.removeEventListener('mousemove', handleControlPanelMouseMove)
	controlPanelDismissListenersActive = false
	controlPanelSafePoint = undefined
}

function handleControlPanelPointerDown(event) {
	if (controlPanelCollapsed.value || event.pointerType === 'mouse') return
	if (isControlPanelTarget(event.target)) return
	controlPanelCollapsed.value = true
}

function handleControlPanelMouseMove(event) {
	if (controlPanelCollapsed.value || !hasFinePointer()) return
	if (isPointInControlPanelFocusArea(event.clientX, event.clientY)) return
	controlPanelCollapsed.value = true
}

function isControlPanelTarget(target) {
	return controlPanelEl.value?.contains(target) || controlButtonEl.value?.contains(target)
}

function isPointInControlPanelFocusArea(x, y) {
	const panelRect = controlPanelEl.value?.getBoundingClientRect()
	const buttonRect = controlButtonEl.value?.getBoundingClientRect()
	if (!panelRect || !buttonRect) return true
	if (rectContainsPoint(panelRect, x, y, 2)) return true
	if (rectContainsPoint(buttonRect, x, y, 2)) {
		controlPanelSafePoint = {x, y}
		return true
	}

	const anchor = controlPanelSafePoint || {
		x: buttonRect.left + buttonRect.width / 2,
		y: buttonRect.top,
	}
	return pointInTriangle(
			{x, y},
			anchor,
			{x: panelRect.left - 8, y: panelRect.bottom},
			{x: panelRect.right + 8, y: panelRect.bottom},
	)
}

function rememberControlPanelSafePoint() {
	const buttonRect = controlButtonEl.value?.getBoundingClientRect()
	if (!buttonRect) return
	controlPanelSafePoint = {
		x: buttonRect.left + buttonRect.width / 2,
		y: buttonRect.top,
	}
}

function rectContainsPoint(rect, x, y, padding = 0) {
	return x >= rect.left - padding && x <= rect.right + padding && y >= rect.top - padding && y <= rect.bottom + padding
}

function pointInTriangle(point, a, b, c) {
	const area = triangleArea(a, b, c)
	if (area === 0) return false
	const area1 = triangleArea(point, b, c)
	const area2 = triangleArea(a, point, c)
	const area3 = triangleArea(a, b, point)
	return Math.abs(area - (area1 + area2 + area3)) <= 0.5
}

function triangleArea(a, b, c) {
	return Math.abs((a.x * (b.y - c.y) + b.x * (c.y - a.y) + c.x * (a.y - b.y)) / 2)
}

function hasFinePointer() {
	return window.matchMedia?.('(hover: hover) and (pointer: fine)').matches ?? true
}

function setPanelPosition(panel, x, y) {
	const next = {...panelPositions.value}
	next[panel] = clampPanelPosition(panel, x, y)
	panelPositions.value = next
}

function clampPanel(panel) {
	const current = panelPositions.value[panel]
	setPanelPosition(panel, current.x, current.y)
}

function clampPanelPosition(panel, x, y) {
	const screenRect = screenAreaEl.value?.getBoundingClientRect()
	const panelEl = screenAreaEl.value?.querySelector(`[data-floating-panel="${panel}"]`)
	if (!screenRect || !panelEl) return {x, y}

	const panelRect = panelEl.getBoundingClientRect()
	const maxX = Math.max(0, screenRect.width - panelRect.width)
	const maxY = Math.max(0, screenRect.height - panelRect.height)

	return {
		x: Math.round(Math.max(0, Math.min(maxX, x))),
		y: Math.round(Math.max(0, Math.min(maxY, y))),
	}
}

function restart() {
	shutdown()
	if (!props.token || !deviceId.value) {
		status.value = '等待选择设备'
		return
	}
	connectSignaling()
}

function shutdown() {
	clearTimeout(reconnectTimer)
	cleanupPeer()
	if (socket) {
		socket.close(1000, 'switch device')
		socket = undefined
	}
}

function cleanupPeer() {
	clearInterval(statsTimer)
	clearTimeout(longPressTimer)
	statsTimer = undefined
	activePointers = new Map()
	gesturePointers = new Map()
	controlChannel = undefined
	controlChannelLabel.value = 'WebSocket'
	if (peerConnection) {
		peerConnection.close()
		peerConnection = undefined
	}
	if (videoEl.value) videoEl.value.srcObject = null
}

function connectSignaling() {
	if (!props.token || !deviceId.value) return

	const params = new URLSearchParams({token: props.token, deviceId: deviceId.value})
	const scheme = location.protocol === 'https:' ? 'wss' : 'ws'
	const nextSocket = new WebSocket(`${scheme}://${location.host}/ws?${params}`)
	socket = nextSocket
	status.value = '正在连接信令服务器'

	nextSocket.addEventListener('open', () => {
		if (nextSocket !== socket) return
		status.value = '已连接信令服务器'
		nextSocket.send(JSON.stringify({
			type: 'hello',
			role: 'viewer',
			deviceId: deviceId.value,
			qualityMode: qualityMode.value,
		}))
		sendQualityMode()
	})

	nextSocket.addEventListener('message', async (event) => {
		if (nextSocket !== socket) return
		await handleSignal(JSON.parse(event.data))
	})

	nextSocket.addEventListener('close', () => {
		if (nextSocket !== socket) return
		cleanupPeer()
		if (!props.token || !deviceId.value) return
		status.value = '信令已断开，正在重连'
		latency.value = 'RTT -- ms'
		routeMode.value = '--'
		routeDetail.value = '--'
		controlStatus.value = '--'
		updateControlPermission(false)
		reconnectTimer = setTimeout(connectSignaling, 1000)
	})
}

async function handleSignal(data) {
	if (data.iceServers) iceServers = data.iceServers
	if (data.relayMode) relayMode = data.relayMode
	if (data.sfuUrl) sfuUrl = data.sfuUrl

	if (data.type === 'error') {
		cleanupPeer()
		status.value = data.message || '连接失败'
		emit('device-error')
		await fetchDevices().catch(() => [])
		return
	}

	if (data.type === 'waiting') {
		status.value = '等待安卓端连接'
		updateControlPermission(false)
		return
	}

	if (data.type === 'android-disconnected') {
		cleanupPeer()
		status.value = '安卓端已断开'
		return
	}

	if (data.type === 'stream-info') {
		if (data.qualityMode) setQualityMode(data.qualityMode, false)
		if (typeof data.controlEnabled === 'boolean') updateControlPermission(data.controlEnabled)
		status.value = videoEl.value?.readyState >= 2 && videoEl.value?.videoWidth > 0
				? '正在接收画面'
				: `等待 WebRTC 媒体 ${data.width || ''}x${data.height || ''}`
		return
	}

	if (data.type === 'offer') {
		usingRelayOnly = false
		await acceptOffer(data.sdp, false)
		return
	}

	if (data.type === 'candidate' && peerConnection) {
		await peerConnection.addIceCandidate({
			candidate: data.candidate,
			sdpMid: data.sdpMid,
			sdpMLineIndex: data.sdpMLineIndex,
		})
		return
	}

	if (data.type === 'route') {
		renderRoute(data.localType, data.remoteType)
		return
	}

	if (data.type === 'control-permission-status') {
		updateControlPermission(data.enabled)
		controlStatus.value = data.message || (data.enabled ? '控制权限已开启' : '等待安卓端授权')
		return
	}

	if (data.type === 'control-ack') renderControlAck(data)
}

async function acceptOffer(sdp, relayOnly) {
	cleanupPeer()
	peerConnection = new RTCPeerConnection({
		iceServers,
		iceTransportPolicy: relayOnly ? 'relay' : 'all',
	})
	peerConnection.addTransceiver('video', {direction: 'recvonly'})

	peerConnection.addEventListener('datachannel', (event) => {
		if (event.channel.label !== 'control') return
		controlChannel = event.channel
		controlChannel.addEventListener('open', renderControlChannel)
		controlChannel.addEventListener('close', renderControlChannel)
		controlChannel.addEventListener('error', renderControlChannel)
		controlChannel.addEventListener('message', (message) => {
			renderControlAck(JSON.parse(message.data))
		})
		renderControlChannel()
	})

	peerConnection.addEventListener('track', (event) => {
		videoEl.value.srcObject = event.streams[0]
		status.value = '正在接收画面'
	})

	peerConnection.addEventListener('icecandidate', (event) => {
		if (!event.candidate || socket?.readyState !== WebSocket.OPEN) return
		socket.send(JSON.stringify({
			type: 'candidate',
			candidate: event.candidate.candidate,
			sdpMid: event.candidate.sdpMid,
			sdpMLineIndex: event.candidate.sdpMLineIndex,
		}))
	})

	peerConnection.addEventListener('connectionstatechange', () => {
		const state = peerConnection?.connectionState
		if (state === 'connected') {
			status.value = '正在接收画面'
			return
		}
		if (state === 'failed') {
			handleConnectionFailed()
			return
		}
		status.value = relayOnly ? `中继 ${state}` : `WebRTC ${state}`
	})

	await peerConnection.setRemoteDescription({type: 'offer', sdp})
	const answer = await peerConnection.createAnswer()
	await peerConnection.setLocalDescription(answer)
	socket.send(JSON.stringify({type: 'answer', sdp: answer.sdp}))
	statsTimer = setInterval(updateStats, 1000)
}

function handleConnectionFailed() {
	if (!usingRelayOnly && hasTurnServer()) {
		usingRelayOnly = true
		status.value = '直连失败，正在切换中继'
		routeMode.value = '中继重试'
		routeDetail.value = '--'
		socket?.send(JSON.stringify({type: 'renegotiate'}))
		return
	}

	if (!hasTurnServer()) {
		status.value = '直连失败，未配置中继'
		routeMode.value = '需要中继'
		routeDetail.value = '--'
		return
	}

	status.value = relayMode === 'sfu' && sfuUrl ? '中继失败，请接入 SFU' : '中继连接失败'
}

async function updateStats() {
	if (!peerConnection) return
	const stats = await peerConnection.getStats()
	let selectedPair
	let inbound

	for (const report of stats.values()) {
		if (report.type === 'candidate-pair' && report.nominated && report.state === 'succeeded') {
			selectedPair = report
		}
		if (report.type === 'inbound-rtp' && report.kind === 'video') inbound = report
	}

	if (selectedPair?.currentRoundTripTime !== undefined) {
		latency.value = `RTT ${Math.round(selectedPair.currentRoundTripTime * 1000)} ms`
	} else if (inbound?.jitter !== undefined) {
		latency.value = `Jitter ${Math.round(inbound.jitter * 1000)} ms`
	}

	if (selectedPair) {
		const local = stats.get(selectedPair.localCandidateId)
		const remote = stats.get(selectedPair.remoteCandidateId)
		renderRoute(local?.candidateType, remote?.candidateType)
	}
}

function setQualityMode(mode, notify) {
	qualityMode.value = qualityOptions.some((option) => option.value === mode) ? mode : 'balanced'
	localStorage.setItem('lead-quality-mode', qualityMode.value)
	if (notify) sendQualityMode()
}

function setInputMode(mode) {
	inputMode.value = mode === 'trackpad' ? 'trackpad' : 'direct'
	localStorage.setItem('lead-input-mode', inputMode.value)
}

function sendQualityMode() {
	if (socket?.readyState === WebSocket.OPEN) {
		socket.send(JSON.stringify({type: 'quality-mode', mode: qualityMode.value}))
	}
}

function requestControlPermission() {
	if (socket?.readyState !== WebSocket.OPEN) {
		controlStatus.value = '信令未连接，无法申请'
		return
	}
	socket.send(JSON.stringify({type: 'control-permission-request'}))
	controlStatus.value = '已向安卓端申请控制权限'
}

function handleKeydown(event) {
	if (event.defaultPrevented || event.repeat) return
	if (event.key === 'Escape') sendControl({action: 'back'})
	if (event.ctrlKey && event.key.toLowerCase() === 'h') sendControl({action: 'home'})
	if (event.ctrlKey && event.key.toLowerCase() === 'r') sendControl({action: 'recents'})
}

function updateControlPermission(enabled) {
	controlEnabled.value = Boolean(enabled)
}

function preventNativeGesture(event) {
	event.preventDefault()
}

function handlePointerDown(event) {
	if (event.button !== undefined && event.button !== 0) return
	const point = videoPoint(event)
	if (!point) return
	event.preventDefault()
	touchLayerEl.value?.setPointerCapture(event.pointerId)
	const pointer = {
		id: event.pointerId,
		x: point.x,
		y: point.y,
		startX: point.x,
		startY: point.y,
		time: performance.now(),
		type: event.pointerType,
	}
	activePointers.set(event.pointerId, pointer)
	gesturePointers.set(event.pointerId, {...pointer})
	sendPointerEvent('down', pointer)

	if (activePointers.size === 1) {
		longPressSent = false
		longPressPointerId = event.pointerId
		clearTimeout(longPressTimer)
		longPressTimer = setTimeout(() => {
			const current = activePointers.get(longPressPointerId)
			if (!current || activePointers.size !== 1) return
			longPressSent = true
			sendControl({action: 'longPress', x: current.x, y: current.y, durationMs: 650})
		}, 650)
	} else {
		clearTimeout(longPressTimer)
	}
}

function handlePointerMove(event) {
	if (event.button !== undefined && event.buttons && (event.buttons & 1) !== 1) return
	const pointer = activePointers.get(event.pointerId)
	if (!pointer) return
	const point = videoPoint(event)
	if (!point) return
	event.preventDefault()
	pointer.x = point.x
	pointer.y = point.y
	sendPointerEvent('move', pointer)
}

function handlePointerUp(event) {
	if (event.button !== undefined && event.button !== 0) return
	const pointer = activePointers.get(event.pointerId)
	if (!pointer) return
	const point = videoPoint(event) || {x: pointer.x, y: pointer.y}
	event.preventDefault()
	pointer.x = point.x
	pointer.y = point.y
	sendPointerEvent('up', pointer)
	activePointers.delete(event.pointerId)
	clearTimeout(longPressTimer)

	const record = gesturePointers.get(event.pointerId)
	if (record) {
		record.endX = pointer.x
		record.endY = pointer.y
		record.durationMs = performance.now() - record.time
		gesturePointers.set(event.pointerId, record)
	}

	if (activePointers.size === 0) finishGesture()
}

function handlePointerCancel(event) {
	const pointer = activePointers.get(event.pointerId)
	if (pointer) sendPointerEvent('cancel', pointer)
	activePointers.delete(event.pointerId)
	clearTimeout(longPressTimer)
	if (activePointers.size === 0) gesturePointers.clear()
}

function handleWheel(event) {
	const point = videoPoint(event)
	if (!point) return
	event.preventDefault()
	const delta = Math.sign(event.deltaY) * 0.18
	sendControl({
		action: 'swipe',
		startX: point.x,
		startY: clamp(point.y + delta),
		endX: point.x,
		endY: clamp(point.y - delta),
		durationMs: 160,
	})
}

function finishGesture() {
	const pointers = Array.from(gesturePointers.values())
	gesturePointers = new Map()
	if (pointers.length === 0 || longPressSent) return

	if (pointers.length > 1) {
		const durationMs = Math.max(80, Math.min(900, Math.max(...pointers.map((pointer) => pointer.durationMs || 80))))
		sendControl({
			action: 'multiSwipe',
			durationMs,
			pointers: pointers.map((pointer) => ({
				startX: pointer.startX,
				startY: pointer.startY,
				endX: pointer.endX ?? pointer.x,
				endY: pointer.endY ?? pointer.y,
			})),
		})
		return
	}

	const pointer = pointers[0]
	const endX = pointer.endX ?? pointer.x
	const endY = pointer.endY ?? pointer.y
	const dx = endX - pointer.startX
	const dy = endY - pointer.startY
	const distance = Math.hypot(dx, dy)
	const durationMs = Math.max(60, Math.min(900, pointer.durationMs || 80))
	if (distance < 0.015 && durationMs < 550) {
		sendControl({action: 'tap', x: endX, y: endY, durationMs: 70})
		return
	}
	sendControl({
		action: 'swipe',
		startX: pointer.startX,
		startY: pointer.startY,
		endX,
		endY,
		durationMs,
	})
}

function sendPointerEvent(phase, pointer) {
	sendControl({
		action: 'pointer',
		phase,
		pointerId: pointer.id,
		pointerType: pointer.type,
		x: pointer.x,
		y: pointer.y,
		mode: inputMode.value,
	}, {silent: true})
}

function sendControl(command, options = {}) {
	const message = JSON.stringify({type: 'control', source: 'datachannel', ...command})
	if (controlChannel?.readyState === 'open') {
		controlChannel.send(message)
		if (!options.silent) controlStatus.value = controlActionLabel(command.action)
		return
	}

	if (socket?.readyState === WebSocket.OPEN) {
		socket.send(JSON.stringify({type: 'control', source: 'websocket-fallback', ...command}))
		if (!options.silent) controlStatus.value = controlActionLabel(command.action)
		return
	}

	controlStatus.value = '未连接'
}

function renderControlAck(data) {
	controlStatus.value = data.ok ? controlActionLabel(data.action) : data.message
}

function renderControlChannel() {
	const ready = controlChannel?.readyState === 'open'
	controlChannelLabel.value = ready ? 'DataChannel' : 'WebSocket'
	if (ready) controlStatus.value = 'DataChannel'
}

function controlActionLabel(action) {
	const labels = {
		back: '返回',
		home: '主页',
		recents: '最近任务',
		tap: '点击',
		longPress: '长按',
		swipe: '滑动',
		multiSwipe: '多指滑动',
		pointer: '触控',
	}
	return labels[action] || action || '控制'
}

function videoPoint(event) {
	const video = videoEl.value
	if (!video) return null
	const rect = video.getBoundingClientRect()
	if (rect.width <= 0 || rect.height <= 0) return null

	let content = rect
	if (video.videoWidth > 0 && video.videoHeight > 0) {
		const videoRatio = video.videoWidth / video.videoHeight
		const boxRatio = rect.width / rect.height
		let width = rect.width
		let height = rect.height
		let left = rect.left
		let top = rect.top
		if (boxRatio > videoRatio) {
			width = rect.height * videoRatio
			left = rect.left + (rect.width - width) / 2
		} else {
			height = rect.width / videoRatio
			top = rect.top + (rect.height - height) / 2
		}
		content = {left, top, width, height}
	}

	const x = (event.clientX - content.left) / content.width
	const y = (event.clientY - content.top) / content.height
	if (x < 0 || x > 1 || y < 0 || y > 1) return null
	return {x: clamp(x), y: clamp(y)}
}

function renderRoute(localType, remoteType) {
	const local = localType || '--'
	const remote = remoteType || '--'
	routeDetail.value = `${local} -> ${remote}`
	if (local === 'relay' || remote === 'relay') {
		routeMode.value = 'TURN 中继'
		return
	}
	if (local === 'srflx' || remote === 'srflx') {
		routeMode.value = '公网直连'
		return
	}
	if (local === 'host' || remote === 'host') {
		routeMode.value = '局域网直连'
		return
	}
	routeMode.value = '--'
}

function hasTurnServer() {
	return iceServers.some((server) => {
		const urls = Array.isArray(server.urls) ? server.urls : [server.urls]
		return urls.some((url) => String(url).startsWith('turn:') || String(url).startsWith('turns:'))
	})
}

function clamp(value) {
	return Math.max(0, Math.min(1, value))
}
</script>

<style scoped>
.tool-button {
	@apply inline-flex min-h-9 cursor-pointer items-center justify-center rounded-xl border border-white/15 bg-white/[0.07] font-semibold leading-none text-slate-200 hover:border-white/25 hover:text-white;
}

.tool-button.is-active {
	@apply border-white/60 bg-[#eef3f7] text-[#11151b];
}

.tool-button.is-enabled {
	@apply border-emerald-300/60 bg-emerald-900/80 text-emerald-100;
}

.tool-button-panel {
	@apply min-h-9;
}

.tool-button-control-request {
	@apply w-full gap-2 px-3;
}

.tool-button-quality {
	@apply px-3 text-xs;
}

.tool-button-nav {
	@apply px-3;
}

.tool-button-nav-left {
	@apply rounded-none rounded-bl-xl rounded-tl-xl;
}

.tool-button-nav-middle {
	@apply rounded-none border-x-0;
}

.tool-button-nav-right {
	@apply rounded-none rounded-br-xl rounded-tr-xl;
}
</style>
