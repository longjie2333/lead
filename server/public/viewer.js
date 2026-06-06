const statusEl = document.querySelector("#status");
const latencyEl = document.querySelector("#latency");
const routeModeEl = document.querySelector("#route-mode");
const routeDetailEl = document.querySelector("#route-detail");
const controlStatusEl = document.querySelector("#control-status");
const permissionButtons = Array.from(document.querySelectorAll(".permission-action"));
const videoEl = document.querySelector("#screen");
const touchLayerEl = document.querySelector("#touch-layer");
const qualityButtons = Array.from(document.querySelectorAll(".quality-option"));
const modeButtons = Array.from(document.querySelectorAll(".mode-option"));
const remoteButtons = Array.from(document.querySelectorAll(".remote-action"));
const statusPanelEl = document.querySelector("#status-panel");
const statusHandleEl = document.querySelector("#status-handle");
const controlFloatEl = document.querySelector("#control-float");
const controlBallEl = document.querySelector("#control-float-button");
const controlPanelEl = document.querySelector("#control-panel");
const loginPanelEl = document.querySelector("#login-panel");
const loginFormEl = document.querySelector("#login-form");
const loginUsernameEl = document.querySelector("#login-username");
const loginPasswordEl = document.querySelector("#login-password");
const loginErrorEl = document.querySelector("#login-error");
const authAlertEl = document.querySelector("#auth-alert");
const authAlertMessageEl = document.querySelector("#auth-alert-message");
const authParamFormEl = document.querySelector("#auth-param-form");
const authParamTokenEl = document.querySelector("#auth-param-token");
const authParamDeviceIdEl = document.querySelector("#auth-param-device-id");
const devicePanelEl = document.querySelector("#device-panel");
const deviceListEl = document.querySelector("#device-list");
const deviceSummaryEl = document.querySelector("#device-summary");
const refreshDevicesButtonEl = document.querySelector("#refresh-devices-button");
const logoutButtonEl = document.querySelector("#logout-button");
const contextSafeZoneEl = document.createElement("div");

const qualityLabels = {
  quality: "画质",
  balanced: "均衡",
  speed: "速度",
};

const isMobileClient = matchMedia("(pointer: coarse), (max-width: 920px)").matches;
document.body.classList.toggle("mobile-client", isMobileClient);

let socket;
let peerConnection;
let controlChannel;
let iceServers = [{ urls: "stun:stun.l.google.com:19302" }];
let relayMode = "turn";
let sfuUrl = "";
let statsTimer;
let usingRelayOnly = false;
let selectedQuality = localStorage.getItem("lead-quality-mode") || "balanced";
let inputMode = localStorage.getItem("lead-input-mode") || "direct";
const queryParams = new URLSearchParams(location.search);
let authToken = queryParams.get("token") || "";
let selectedDeviceId = queryParams.get("deviceId") || "";
const activePointers = new Map();
let gesturePointers = new Map();
let longPressTimer;
let longPressPointerId;
let longPressSent = false;
let contextMenuState;
let contextMenuCloseTimer;

contextSafeZoneEl.className = "context-safe-zone";
contextSafeZoneEl.hidden = true;
document.body.append(contextSafeZoneEl);

setQualityMode(selectedQuality, false);
setInputMode(inputMode);
updateControlPermission(false);
setupFloatingPanel({
  panel: statusPanelEl,
  handle: statusHandleEl,
  collapsedClass: "is-collapsed",
  storageKey: "lead-status-panel",
  defaultPosition: () => ({ left: 14, top: 14 }),
  onToggle: (collapsed) => statusHandleEl.setAttribute("aria-expanded", String(!collapsed)),
});
setupFloatingPanel({
  panel: controlFloatEl,
  handle: controlBallEl,
  collapsedClass: "is-collapsed",
  storageKey: "lead-control-float",
  defaultPosition: () => ({ left: 16, top: window.innerHeight - 74 }),
  onToggle: (collapsed) => {
    if (!collapsed) clearContextMenuMode();
    controlBallEl.setAttribute("aria-expanded", String(!collapsed));
  },
});
setupControlContextMenu();
initAuth();
renderControlChannel();

loginFormEl.addEventListener("submit", handleLogin);
authParamFormEl.addEventListener("submit", handleAuthParamSubmit);
refreshDevicesButtonEl.addEventListener("click", refreshDevices);
logoutButtonEl.addEventListener("click", handleLogout);

for (const button of qualityButtons) {
  button.addEventListener("click", () => {
    setQualityMode(button.dataset.quality, true);
  });
}

for (const button of modeButtons) {
  button.addEventListener("click", () => setInputMode(button.dataset.mode));
}

for (const button of remoteButtons) {
  button.addEventListener("click", () => sendControl({ action: button.dataset.action }));
}

for (const button of permissionButtons) {
  button.addEventListener("click", requestControlPermission);
}

for (const eventName of ["pointerdown", "pointermove", "pointerup", "pointercancel", "click", "dblclick", "contextmenu", "wheel", "touchstart", "touchmove", "touchend"]) {
  authAlertEl.addEventListener(eventName, stopAuthAlertEvent, { passive: false });
}

touchLayerEl.addEventListener("pointerdown", handlePointerDown);
touchLayerEl.addEventListener("pointermove", handlePointerMove);
touchLayerEl.addEventListener("pointerup", handlePointerUp);
touchLayerEl.addEventListener("pointercancel", handlePointerCancel);
touchLayerEl.addEventListener("wheel", handleWheel, { passive: false });

window.addEventListener("keydown", (event) => {
  if (event.defaultPrevented || event.repeat) return;
  if (event.key === "Escape") sendControl({ action: "back" });
  if (event.ctrlKey && event.key.toLowerCase() === "h") sendControl({ action: "home" });
  if (event.ctrlKey && event.key.toLowerCase() === "r") sendControl({ action: "recents" });
});

async function initAuth() {
  if (!authToken || !selectedDeviceId) {
    showMissingParamsAlert();
    return;
  }
  try {
    const devices = await fetchDevices();
    const exists = devices.some((device) => device.id === selectedDeviceId);
    if (!exists) {
      showAuthAlert("未授权访问该设备，或设备不属于当前 token 对应的账号。");
      return;
    }
    hideAuthPanels();
    connectSignaling();
  } catch {
    showAuthAlert("未授权或 token 已失效，请重新生成带 token 和 deviceId 的访问链接。");
  }
}

async function handleLogin(event) {
  event.preventDefault();
  loginErrorEl.textContent = "";
  const payload = {
    username: loginUsernameEl.value.trim(),
    password: loginPasswordEl.value,
    source: "viewer",
  };
  try {
    const response = await fetch("/api/login", {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify(payload),
    });
    if (!response.ok) throw new Error("login failed");
    const data = await response.json();
    authToken = data.token;
    if (selectedDeviceId && (data.devices || []).some((device) => device.id === selectedDeviceId)) {
      hideAuthPanels();
      connectSignaling();
      return;
    }
    renderDevices(data.devices || []);
  } catch {
    showLogin("账号或密码错误");
  }
}

function showAuthAlert(message) {
  cleanupPeer();
  loginPanelEl.hidden = true;
  devicePanelEl.hidden = true;
  authParamFormEl.hidden = true;
  authAlertMessageEl.textContent = message;
  authAlertEl.hidden = false;
  statusEl.textContent = "未授权";
}

function showMissingParamsAlert() {
  cleanupPeer();
  loginPanelEl.hidden = true;
  devicePanelEl.hidden = true;
  authAlertMessageEl.textContent = "请提供以下必要的参数：";
  authParamTokenEl.value = authToken;
  authParamDeviceIdEl.value = selectedDeviceId;
  authParamFormEl.hidden = false;
  authAlertEl.hidden = false;
  statusEl.textContent = "缺少参数";
}

function handleAuthParamSubmit(event) {
  event.preventDefault();
  const token = authParamTokenEl.value.trim();
  const deviceId = authParamDeviceIdEl.value.trim();
  if (!token || !deviceId) {
    authAlertMessageEl.textContent = "请提供以下必要的参数：";
    authParamTokenEl.toggleAttribute("aria-invalid", !token);
    authParamDeviceIdEl.toggleAttribute("aria-invalid", !deviceId);
    if (!token) authParamTokenEl.focus();
    else authParamDeviceIdEl.focus();
    return;
  }
  const params = new URLSearchParams({ token, deviceId });
  location.assign(`/viewer?${params}`);
}

function stopAuthAlertEvent(event) {
  if (authAlertEl.hidden) return;
  if (!event.target.closest(".auth-alert-box")) event.preventDefault();
  event.stopPropagation();
}

async function refreshDevices() {
  if (!authToken) {
    showLogin();
    return;
  }
  refreshDevicesButtonEl.disabled = true;
  deviceSummaryEl.textContent = "正在刷新";
  try {
    renderDevices(await fetchDevices());
  } catch {
    showLogin("无法获取设备列表，请重新登录");
  } finally {
    refreshDevicesButtonEl.disabled = false;
  }
}

async function handleLogout() {
  if (authToken) {
    await fetch("/api/logout", {
      method: "POST",
      headers: { Authorization: `Bearer ${authToken}` },
    }).catch(() => {});
  }
  cleanupPeer();
  socket?.close(1000, "logout");
  socket = undefined;
  authToken = "";
  selectedDeviceId = "";
  history.replaceState(null, "", "/viewer");
  showLogin();
}

async function fetchDevices() {
  const response = await fetch("/api/devices", {
    headers: { Authorization: `Bearer ${authToken}` },
  });
  if (!response.ok) throw new Error("fetch devices failed");
  const data = await response.json();
  return data.devices || [];
}

function renderDevices(devices) {
  loginPanelEl.hidden = true;
  devicePanelEl.hidden = false;
  deviceListEl.innerHTML = "";
  const onlineCount = devices.filter((device) => device.online).length;
  deviceSummaryEl.textContent = `${devices.length} 台设备，${onlineCount} 台在线`;
  if (devices.length === 0) {
    const empty = document.createElement("div");
    empty.className = "device-empty";
    empty.innerHTML = "<strong>暂无设备</strong><span>请先在安卓被控端启动应用并登录。</span>";
    deviceListEl.append(empty);
    return;
  }
  for (const device of devices) {
    const button = document.createElement("button");
    button.type = "button";
    button.className = "device-item";
    button.classList.toggle("online", Boolean(device.online));
    button.innerHTML = `
      <span>
        <strong>${escapeHTML(device.name || device.id)}</strong>
        <small>${escapeHTML(device.id)}</small>
      </span>
      <em>${device.online ? "在线" : "离线"}</em>
    `;
    button.addEventListener("click", () => {
      selectedDeviceId = device.id;
      const params = new URLSearchParams({ token: authToken, deviceId: selectedDeviceId });
      history.replaceState(null, "", `/viewer?${params}`);
      hideAuthPanels();
      connectSignaling();
    });
    deviceListEl.append(button);
  }
}

function showLogin(message = "") {
  loginPanelEl.hidden = false;
  devicePanelEl.hidden = true;
  loginErrorEl.textContent = message;
}

function hideAuthPanels() {
  loginPanelEl.hidden = true;
  devicePanelEl.hidden = true;
  authAlertEl.hidden = true;
}

function escapeHTML(value) {
  return String(value)
    .replaceAll("&", "&amp;")
    .replaceAll("<", "&lt;")
    .replaceAll(">", "&gt;")
    .replaceAll('"', "&quot;")
    .replaceAll("'", "&#039;");
}

function handlePointerDown(event) {
  if (event.button !== undefined && event.button !== 0) return;
  const point = videoPoint(event);
  if (!point) return;
  event.preventDefault();
  touchLayerEl.setPointerCapture(event.pointerId);
  const pointer = {
    id: event.pointerId,
    x: point.x,
    y: point.y,
    startX: point.x,
    startY: point.y,
    time: performance.now(),
    type: event.pointerType,
  };
  activePointers.set(event.pointerId, pointer);
  gesturePointers.set(event.pointerId, { ...pointer });
  sendPointerEvent("down", pointer);

  if (activePointers.size === 1) {
    longPressSent = false;
    longPressPointerId = event.pointerId;
    clearTimeout(longPressTimer);
    longPressTimer = setTimeout(() => {
      const current = activePointers.get(longPressPointerId);
      if (!current || activePointers.size !== 1) return;
      longPressSent = true;
      sendControl({ action: "longPress", x: current.x, y: current.y, durationMs: 650 });
    }, 650);
  } else {
    clearTimeout(longPressTimer);
  }
}

function handlePointerMove(event) {
  if (event.button !== undefined && event.buttons && (event.buttons & 1) !== 1) return;
  const pointer = activePointers.get(event.pointerId);
  if (!pointer) return;
  const point = videoPoint(event);
  if (!point) return;
  event.preventDefault();
  pointer.x = point.x;
  pointer.y = point.y;
  sendPointerEvent("move", pointer);
}

function handlePointerUp(event) {
  if (event.button !== undefined && event.button !== 0) return;
  const pointer = activePointers.get(event.pointerId);
  if (!pointer) return;
  const point = videoPoint(event) || { x: pointer.x, y: pointer.y };
  event.preventDefault();
  pointer.x = point.x;
  pointer.y = point.y;
  sendPointerEvent("up", pointer);
  activePointers.delete(event.pointerId);
  clearTimeout(longPressTimer);

  const record = gesturePointers.get(event.pointerId);
  if (record) {
    record.endX = pointer.x;
    record.endY = pointer.y;
    record.durationMs = performance.now() - record.time;
    gesturePointers.set(event.pointerId, record);
  }

  if (activePointers.size === 0) finishGesture();
}

function handlePointerCancel(event) {
  const pointer = activePointers.get(event.pointerId);
  if (pointer) sendPointerEvent("cancel", pointer);
  activePointers.delete(event.pointerId);
  clearTimeout(longPressTimer);
  if (activePointers.size === 0) gesturePointers.clear();
}

function setupControlContextMenu() {
  window.addEventListener("contextmenu", (event) => {
    if (event.target.closest("#control-panel")) {
      event.preventDefault();
      return;
    }
    event.preventDefault();
    event.stopPropagation();
    openControlPanelAt(event.clientX, event.clientY);
  });

  window.addEventListener("pointerdown", (event) => {
    if (!contextMenuState || event.button === 2) return;
    if (event.target.closest("#control-float")) return;
    closeContextControlPanel();
  }, true);

  window.addEventListener("pointermove", (event) => {
    if (!contextMenuState) return;
    if (event.target.closest("#control-float")) {
      clearTimeout(contextMenuCloseTimer);
      return;
    }
    if (isPointInContextSafeArea(event.clientX, event.clientY)) {
      clearTimeout(contextMenuCloseTimer);
      return;
    }
    clearTimeout(contextMenuCloseTimer);
    contextMenuCloseTimer = setTimeout(closeContextControlPanel, 260);
  });

  window.addEventListener("keydown", (event) => {
    if (event.key === "Escape" && contextMenuState) {
      closeContextControlPanel();
      event.preventDefault();
    }
  }, true);

  window.addEventListener("resize", () => {
    if (!contextMenuState) return;
    openControlPanelAt(contextMenuState.anchor.x, contextMenuState.anchor.y);
  });
}

function openControlPanelAt(clientX, clientY) {
  clearTimeout(contextMenuCloseTimer);
  const floatState = contextMenuState?.floatState || captureFloatingControlState();
  controlFloatEl.classList.add("is-context-menu");
  controlFloatEl.classList.remove("is-collapsed");
  controlBallEl.setAttribute("aria-expanded", "true");

  const margin = 8;
  const gap = 12;
  const panelRect = controlPanelEl.getBoundingClientRect();
  const width = panelRect.width;
  const height = panelRect.height;
  const opensRight = clientX + gap + width <= window.innerWidth - margin;
  const opensDown = clientY + height <= window.innerHeight - margin;
  const left = opensRight
    ? clientX + gap
    : Math.max(margin, clientX - gap - width);
  const top = opensDown
    ? clientY
    : Math.max(margin, clientY - height);

  setPanelPosition(controlFloatEl, left, top);
  const rect = controlPanelEl.getBoundingClientRect();
  contextMenuState = {
    anchor: { x: clientX, y: clientY },
    floatState,
    rect: {
      left: rect.left,
      top: rect.top,
      right: rect.right,
      bottom: rect.bottom,
    },
    side: rect.left >= clientX ? "right" : "left",
  };
  contextSafeZoneEl.hidden = false;
  contextSafeZoneEl.dataset.side = contextMenuState.side;
}

function closeContextControlPanel() {
  if (!contextMenuState) return;
  const { floatState } = contextMenuState;
  clearTimeout(contextMenuCloseTimer);
  contextMenuState = undefined;
  contextSafeZoneEl.hidden = true;
  controlFloatEl.classList.remove("is-context-menu");
  controlFloatEl.classList.toggle("is-collapsed", floatState.collapsed);
  controlBallEl.setAttribute("aria-expanded", String(!floatState.collapsed));
  setPanelPosition(controlFloatEl, floatState.left, floatState.top);
}

function clearContextMenuMode() {
  if (!controlFloatEl.classList.contains("is-context-menu")) return;
  const floatState = contextMenuState?.floatState;
  clearTimeout(contextMenuCloseTimer);
  contextMenuState = undefined;
  contextSafeZoneEl.hidden = true;
  controlFloatEl.classList.remove("is-context-menu");
  if (floatState) {
    controlFloatEl.classList.toggle("is-collapsed", floatState.collapsed);
    controlBallEl.setAttribute("aria-expanded", String(!floatState.collapsed));
    setPanelPosition(controlFloatEl, floatState.left, floatState.top);
  }
}

function captureFloatingControlState() {
  const rect = controlFloatEl.getBoundingClientRect();
  return {
    left: rect.left,
    top: rect.top,
    collapsed: controlFloatEl.classList.contains("is-collapsed"),
  };
}

function isPointInContextSafeArea(x, y) {
  if (!contextMenuState) return false;
  const { anchor, rect, side } = contextMenuState;
  if (x >= rect.left && x <= rect.right && y >= rect.top && y <= rect.bottom) return true;
  const edgeX = side === "right" ? rect.left : rect.right;
  const topY = rect.top;
  const bottomY = rect.bottom;
  return pointInTriangle(
    { x, y },
    anchor,
    { x: edgeX, y: topY },
    { x: edgeX, y: bottomY },
  );
}

function pointInTriangle(point, a, b, c) {
  const area = triangleArea(a, b, c);
  const area1 = triangleArea(point, b, c);
  const area2 = triangleArea(a, point, c);
  const area3 = triangleArea(a, b, point);
  return Math.abs(area - (area1 + area2 + area3)) < 0.5;
}

function triangleArea(a, b, c) {
  return Math.abs((a.x * (b.y - c.y) + b.x * (c.y - a.y) + c.x * (a.y - b.y)) / 2);
}

function handleWheel(event) {
  const point = videoPoint(event);
  if (!point) return;
  event.preventDefault();
  const delta = Math.sign(event.deltaY) * 0.18;
  sendControl({
    action: "swipe",
    startX: point.x,
    startY: clamp(point.y + delta),
    endX: point.x,
    endY: clamp(point.y - delta),
    durationMs: 160,
  });
}

function finishGesture() {
  const pointers = Array.from(gesturePointers.values());
  gesturePointers = new Map();
  if (pointers.length === 0 || longPressSent) return;

  if (pointers.length > 1) {
    const durationMs = Math.max(80, Math.min(900, Math.max(...pointers.map((pointer) => pointer.durationMs || 80))));
    sendControl({
      action: "multiSwipe",
      durationMs,
      pointers: pointers.map((pointer) => ({
        startX: pointer.startX,
        startY: pointer.startY,
        endX: pointer.endX ?? pointer.x,
        endY: pointer.endY ?? pointer.y,
      })),
    });
    return;
  }

  const pointer = pointers[0];
  const endX = pointer.endX ?? pointer.x;
  const endY = pointer.endY ?? pointer.y;
  const dx = endX - pointer.startX;
  const dy = endY - pointer.startY;
  const distance = Math.hypot(dx, dy);
  const durationMs = Math.max(60, Math.min(900, pointer.durationMs || 80));
  if (distance < 0.015 && durationMs < 550) {
    sendControl({ action: "tap", x: endX, y: endY, durationMs: 70 });
    return;
  }
  sendControl({
    action: "swipe",
    startX: pointer.startX,
    startY: pointer.startY,
    endX,
    endY,
    durationMs,
  });
}

function connectSignaling() {
  if (!authToken || !selectedDeviceId) {
    initAuth();
    return;
  }
  const params = new URLSearchParams({ token: authToken, deviceId: selectedDeviceId });
  socket = new WebSocket(`${location.protocol === "https:" ? "wss" : "ws"}://${location.host}/ws?${params}`);

  socket.addEventListener("open", () => {
    statusEl.textContent = "已连接信令服务器";
    socket.send(JSON.stringify({ type: "hello", role: "viewer", deviceId: selectedDeviceId, qualityMode: selectedQuality }));
    sendQualityMode();
  });

  socket.addEventListener("message", async (event) => {
    const data = JSON.parse(event.data);
    await handleSignal(data);
  });

  socket.addEventListener("close", () => {
    cleanupPeer();
    if (!authToken || !selectedDeviceId) return;
    statusEl.textContent = "信令已断开，正在重连";
    latencyEl.textContent = "RTT -- ms";
    routeModeEl.textContent = "--";
    routeDetailEl.textContent = "--";
    controlStatusEl.textContent = "--";
    updateControlPermission(false);
    setTimeout(connectSignaling, 1000);
  });
}

async function handleSignal(data) {
  if (data.iceServers) iceServers = data.iceServers;
  if (data.relayMode) relayMode = data.relayMode;
  if (data.sfuUrl) sfuUrl = data.sfuUrl;

  if (data.type === "error") {
    cleanupPeer();
    statusEl.textContent = data.message || "连接失败";
    selectedDeviceId = "";
    history.replaceState(null, "", "/viewer");
    renderDevices(await fetchDevices().catch(() => []));
    return;
  }

  if (data.type === "waiting") {
    statusEl.textContent = "等待安卓端连接";
    updateControlPermission(false);
    return;
  }

  if (data.type === "android-disconnected") {
    cleanupPeer();
    statusEl.textContent = "安卓端已断开";
    return;
  }

  if (data.type === "stream-info") {
    if (data.qualityMode) setQualityMode(data.qualityMode, false);
    if (typeof data.controlEnabled === "boolean") updateControlPermission(data.controlEnabled);
    statusEl.textContent = videoEl.readyState >= 2 && videoEl.videoWidth > 0
      ? "正在接收画面"
      : `等待 WebRTC 媒体 ${data.width || ""}x${data.height || ""}`;
    return;
  }

  if (data.type === "offer") {
    usingRelayOnly = false;
    await acceptOffer(data.sdp, false);
    return;
  }

  if (data.type === "candidate" && peerConnection) {
    await peerConnection.addIceCandidate({
      candidate: data.candidate,
      sdpMid: data.sdpMid,
      sdpMLineIndex: data.sdpMLineIndex,
    });
    return;
  }

  if (data.type === "route") {
    renderRoute(data.localType, data.remoteType);
    return;
  }

  if (data.type === "control-permission-status") {
    updateControlPermission(data.enabled);
    controlStatusEl.textContent = data.message || (data.enabled ? "控制权限已开启" : "等待安卓端授权");
    return;
  }

  if (data.type === "control-ack") renderControlAck(data);
}

async function acceptOffer(sdp, relayOnly) {
  cleanupPeer();
  const rtcConfig = {
    iceServers,
    iceTransportPolicy: relayOnly ? "relay" : "all",
  };
  peerConnection = new RTCPeerConnection(rtcConfig);
  peerConnection.addTransceiver("video", { direction: "recvonly" });

  peerConnection.addEventListener("datachannel", (event) => {
    if (event.channel.label !== "control") return;
    controlChannel = event.channel;
    controlChannel.addEventListener("open", renderControlChannel);
    controlChannel.addEventListener("close", renderControlChannel);
    controlChannel.addEventListener("error", renderControlChannel);
    controlChannel.addEventListener("message", (message) => {
      renderControlAck(JSON.parse(message.data));
    });
    renderControlChannel();
  });

  peerConnection.addEventListener("track", (event) => {
    videoEl.srcObject = event.streams[0];
    statusEl.textContent = "正在接收画面";
  });

  peerConnection.addEventListener("icecandidate", (event) => {
    if (event.candidate) {
      socket.send(JSON.stringify({
        type: "candidate",
        candidate: event.candidate.candidate,
        sdpMid: event.candidate.sdpMid,
        sdpMLineIndex: event.candidate.sdpMLineIndex,
      }));
    }
  });

  peerConnection.addEventListener("connectionstatechange", () => {
    const state = peerConnection.connectionState;
    if (state === "connected") {
      statusEl.textContent = "正在接收画面";
      return;
    }
    if (state === "failed") {
      handleConnectionFailed();
      return;
    }
    statusEl.textContent = relayOnly ? `中继 ${state}` : `WebRTC ${state}`;
  });

  await peerConnection.setRemoteDescription({ type: "offer", sdp });
  const answer = await peerConnection.createAnswer();
  await peerConnection.setLocalDescription(answer);
  socket.send(JSON.stringify({ type: "answer", sdp: answer.sdp }));

  statsTimer = setInterval(updateStats, 1000);
}

async function handleConnectionFailed() {
  if (!usingRelayOnly && hasTurnServer()) {
    usingRelayOnly = true;
    statusEl.textContent = "直连失败，正在切换中继";
    routeModeEl.textContent = "中继重试";
    routeDetailEl.textContent = "--";
    socket.send(JSON.stringify({ type: "renegotiate" }));
    return;
  }

  if (!hasTurnServer()) {
    statusEl.textContent = "直连失败，未配置中继";
    routeModeEl.textContent = "需要中继";
    routeDetailEl.textContent = "--";
    return;
  }

  statusEl.textContent = relayMode === "sfu" && sfuUrl ? "中继失败，请接入 SFU" : "中继连接失败";
}

async function updateStats() {
  if (!peerConnection) return;
  const stats = await peerConnection.getStats();
  let selectedPair;
  let inbound;

  for (const report of stats.values()) {
    if (report.type === "candidate-pair" && report.nominated && report.state === "succeeded") {
      selectedPair = report;
    }
    if (report.type === "inbound-rtp" && report.kind === "video") inbound = report;
  }

  if (selectedPair?.currentRoundTripTime !== undefined) {
    latencyEl.textContent = `RTT ${Math.round(selectedPair.currentRoundTripTime * 1000)} ms`;
  } else if (inbound?.jitter !== undefined) {
    latencyEl.textContent = `Jitter ${Math.round(inbound.jitter * 1000)} ms`;
  }

  if (selectedPair) {
    const local = stats.get(selectedPair.localCandidateId);
    const remote = stats.get(selectedPair.remoteCandidateId);
    renderRoute(local?.candidateType, remote?.candidateType);
  }
}

function setQualityMode(mode, notify) {
  selectedQuality = qualityLabels[mode] ? mode : "balanced";
  localStorage.setItem("lead-quality-mode", selectedQuality);
  for (const button of qualityButtons) {
    button.classList.toggle("active", button.dataset.quality === selectedQuality);
  }
  if (notify) sendQualityMode();
}

function setInputMode(mode) {
  inputMode = mode === "trackpad" ? "trackpad" : "direct";
  localStorage.setItem("lead-input-mode", inputMode);
  document.body.dataset.inputMode = inputMode;
  for (const button of modeButtons) {
    button.classList.toggle("active", button.dataset.mode === inputMode);
  }
}

function sendQualityMode() {
  if (socket?.readyState === WebSocket.OPEN) {
    socket.send(JSON.stringify({ type: "quality-mode", mode: selectedQuality }));
  }
}

function requestControlPermission() {
  if (socket?.readyState !== WebSocket.OPEN) {
    controlStatusEl.textContent = "信令未连接，无法申请";
    return;
  }
  socket.send(JSON.stringify({ type: "control-permission-request" }));
  controlStatusEl.textContent = "已向安卓端申请控制权限";
}

function updateControlPermission(enabled) {
  for (const button of permissionButtons) {
    button.classList.toggle("enabled", enabled);
    button.setAttribute("aria-label", enabled ? "控制权限已开启" : "申请控制权限");
    button.title = enabled ? "控制权限已开启" : "申请控制权限";
    button.disabled = false;
  }
}

function sendPointerEvent(phase, pointer) {
  sendControl({
    action: "pointer",
    phase,
    pointerId: pointer.id,
    pointerType: pointer.type,
    x: pointer.x,
    y: pointer.y,
    mode: inputMode,
  }, { silent: true });
}

function sendControl(command, options = {}) {
  const message = JSON.stringify({ type: "control", source: "datachannel", ...command });
  if (controlChannel?.readyState === "open") {
    controlChannel.send(message);
    if (!options.silent) controlStatusEl.textContent = controlActionLabel(command.action);
    return;
  }
  if (socket?.readyState === WebSocket.OPEN) {
    socket.send(JSON.stringify({ type: "control", source: "websocket-fallback", ...command }));
    if (!options.silent) controlStatusEl.textContent = controlActionLabel(command.action);
    return;
  }
  controlStatusEl.textContent = "未连接";
}

function renderControlAck(data) {
  controlStatusEl.textContent = data.ok ? controlActionLabel(data.action) : data.message;
}

function renderControlChannel() {
  const ready = controlChannel?.readyState === "open";
  touchLayerEl.dataset.channel = ready ? "DataChannel" : "WebSocket";
  if (ready) controlStatusEl.textContent = "DataChannel";
}

function controlActionLabel(action) {
  const labels = {
    tap: "点击",
    swipe: "滑动",
    multiSwipe: "多指滑动",
    longPress: "长按",
    pointer: "触控",
    back: "返回",
    home: "主页",
    recents: "后台",
  };
  return labels[action] || action || "--";
}

function videoPoint(event) {
  const rect = videoEl.getBoundingClientRect();
  const videoRatio = videoEl.videoWidth && videoEl.videoHeight
    ? videoEl.videoWidth / videoEl.videoHeight
    : 9 / 16;
  const boxRatio = rect.width / rect.height;
  let width = rect.width;
  let height = rect.height;
  let left = rect.left;
  let top = rect.top;

  if (boxRatio > videoRatio) {
    width = rect.height * videoRatio;
    left = rect.left + (rect.width - width) / 2;
  } else {
    height = rect.width / videoRatio;
    top = rect.top + (rect.height - height) / 2;
  }

  const x = (event.clientX - left) / width;
  const y = (event.clientY - top) / height;
  if (x < 0 || x > 1 || y < 0 || y > 1) return null;
  return { x: clamp(x), y: clamp(y) };
}

function renderRoute(localType = "--", remoteType = "--") {
  const viaTurn = localType === "relay" || remoteType === "relay";
  routeModeEl.textContent = viaTurn ? "中继" : "直连";
  routeDetailEl.textContent = `${localType || "--"} -> ${remoteType || "--"}`;
}

function hasTurnServer() {
  return iceServers.some((server) => {
    const urls = Array.isArray(server.urls) ? server.urls : [server.urls];
    return urls.some((url) => typeof url === "string" && url.startsWith("turn"));
  });
}

function clamp(value) {
  return Math.min(1, Math.max(0, value));
}

function cleanupPeer() {
  clearInterval(statsTimer);
  if (controlChannel) {
    controlChannel.close();
    controlChannel = undefined;
  }
  if (peerConnection) {
    peerConnection.close();
    peerConnection = undefined;
  }
  activePointers.clear();
  gesturePointers.clear();
  updateControlPermission(false);
  videoEl.srcObject = null;
  renderControlChannel();
}

function setupFloatingPanel({ panel, handle, collapsedClass, storageKey, defaultPosition, onToggle }) {
  const saved = readPanelState(storageKey);
  if (saved?.collapsed !== undefined) {
    panel.classList.toggle(collapsedClass, saved.collapsed);
    onToggle(saved.collapsed);
  } else {
    onToggle(panel.classList.contains(collapsedClass));
  }

  requestAnimationFrame(() => {
    const position = saved?.position || defaultPosition();
    setPanelPosition(panel, position.left, position.top);
  });

  let drag;
  handle.addEventListener("pointerdown", (event) => {
    if (event.button !== undefined && event.button !== 0) return;
    event.preventDefault();
    handle.setPointerCapture(event.pointerId);
    const rect = panel.getBoundingClientRect();
    drag = {
      pointerId: event.pointerId,
      startX: event.clientX,
      startY: event.clientY,
      left: rect.left,
      top: rect.top,
      moved: false,
    };
  });

  handle.addEventListener("pointermove", (event) => {
    if (!drag || drag.pointerId !== event.pointerId) return;
    event.preventDefault();
    const dx = event.clientX - drag.startX;
    const dy = event.clientY - drag.startY;
    if (Math.hypot(dx, dy) > 4) drag.moved = true;
    setPanelPosition(panel, drag.left + dx, drag.top + dy);
  });

  const finish = (event) => {
    if (!drag || drag.pointerId !== event.pointerId) return;
    event.preventDefault();
    const shouldToggle = !drag.moved;
    drag = undefined;
    if (shouldToggle) {
      panel.classList.toggle(collapsedClass);
      onToggle(panel.classList.contains(collapsedClass));
      requestAnimationFrame(() => {
        const rect = panel.getBoundingClientRect();
        setPanelPosition(panel, rect.left, rect.top);
        persistPanelState(panel, storageKey, collapsedClass);
      });
    }
    persistPanelState(panel, storageKey, collapsedClass);
  };

  handle.addEventListener("pointerup", finish);
  handle.addEventListener("pointercancel", finish);

  window.addEventListener("resize", () => {
    const rect = panel.getBoundingClientRect();
    setPanelPosition(panel, rect.left, rect.top);
    persistPanelState(panel, storageKey, collapsedClass);
  });
}

function setPanelPosition(panel, left, top) {
  const rect = panel.getBoundingClientRect();
  const maxLeft = Math.max(8, window.innerWidth - rect.width - 8);
  const maxTop = Math.max(8, window.innerHeight - rect.height - 8);
  const nextLeft = Math.min(Math.max(8, left), maxLeft);
  const nextTop = Math.min(Math.max(8, top), maxTop);
  panel.style.left = `${nextLeft}px`;
  panel.style.top = `${nextTop}px`;
  panel.style.right = "auto";
  panel.style.bottom = "auto";
}

function readPanelState(storageKey) {
  try {
    return JSON.parse(localStorage.getItem(storageKey) || "null");
  } catch {
    return null;
  }
}

function persistPanelState(panel, storageKey, collapsedClass) {
  const rect = panel.getBoundingClientRect();
  localStorage.setItem(storageKey, JSON.stringify({
    collapsed: panel.classList.contains(collapsedClass),
    position: { left: Math.round(rect.left), top: Math.round(rect.top) },
  }));
}


