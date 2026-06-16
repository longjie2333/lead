import { API_BASE } from './config'
import { useDeviceStore } from '@/stores/device'
import { useUserStore } from '@/stores/user'

export class UnauthorizedError extends Error {
  constructor(message = 'Unauthorized') {
    super(message)
    this.statusCode = 401
  }
}

export class RequestError extends Error {
  constructor(message, response) {
    super(message)
    this.statusCode = response?.status
  }
}

export async function login(payload) {
  const data = await request('/api/login', {
    method: 'POST',
    body: {
      ...payload,
      source: 'viewer',
    },
  })
  useUserStore().saveAuth(data)
  useDeviceStore().saveDevices(data)
  return data
}

export async function logout() {
  const userStore = useUserStore()
  if (userStore.token) {
    await request('/api/logout', {
      method: 'POST',
      token: userStore.token,
      allowEmpty: true,
    }).catch(() => {})
  }
  userStore.clearAuth()
}

export async function fetchDevices() {
  const userStore = useUserStore()
  if (!userStore.token) throw new UnauthorizedError()

  const data = await request('/api/devices', {
    token: userStore.token,
  })
  useDeviceStore().saveDevices(data)
  return data.devices || []
}

export async function fetchUsers() {
  const userStore = requireUserStore()
  const data = await request('/api/users', {
    token: userStore.token,
  })
  return data.users || []
}

export async function createUser(payload) {
  const userStore = requireUserStore()
  const data = await request('/api/users', {
    method: 'POST',
    token: userStore.token,
    body: buildUserPayload(payload),
  })
  return data.user
}

export async function updateUser(user) {
  const userStore = requireUserStore()
  const id = requireUserId(user?.id)
  const data = await request(`/api/users/${encodeURIComponent(id)}`, {
    method: 'PUT',
    token: userStore.token,
    body: buildUserPayload(user),
  })

  if (data.user && userStore.user?.id === data.user.id) {
    userStore.user = data.user
  }

  return data.user
}

export async function deleteUser(id) {
  const userStore = requireUserStore()
  await request(`/api/users/${encodeURIComponent(requireUserId(id))}`, {
    method: 'DELETE',
    token: userStore.token,
    allowEmpty: true,
  })
}

async function request(path, { method = 'GET', body, token, allowEmpty = false } = {}) {
  const headers = {
    'Content-Type': 'application/json',
  }
  if (token) headers.Authorization = `Bearer ${token}`

  const response = await fetch(API_BASE + path, {
    method,
    headers,
    body: body ? JSON.stringify(body) : undefined,
  })

  if (response.status === 401) {
    useUserStore().clearAuth()
    throw new UnauthorizedError('登录已过期，请重新登录')
  }

  if (!response.ok) {
    throw new RequestError(await response.text().catch(() => '请求失败'), response)
  }

  if (allowEmpty || response.status === 204) return null
  return response.json()
}

function requireUserStore() {
  const userStore = useUserStore()
  if (!userStore.token) throw new UnauthorizedError()
  return userStore
}

function requireUserId(id) {
  const value = String(id || '').trim()
  if (!value) throw new Error('user id is required')
  return value
}

function buildUserPayload(payload = {}) {
  const data = {}

  if (payload.username !== undefined) data.username = payload.username
  if (payload.password !== undefined && payload.password !== '') data.password = payload.password
  if (payload.role !== undefined) data.role = payload.role
  if (payload.isAdmin !== undefined) data.isAdmin = payload.isAdmin

  return data
}
