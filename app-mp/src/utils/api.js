import { SERVER_URL } from './config'
import { useUserStore } from '../stores/user'
import { useDeviceStore } from '../stores/device'

export class RequestError extends Error {
  constructor(message, res, path) {
    super(message)

    if (res.statusCode) this.statusCode = res.statusCode
    if (res.data) this.resData = res.data
    if (path) this.path = path

    if (Error.captureStackTrace) {
      Error.captureStackTrace(this, RequestError);
    }
  }
}

export class UnauthorizedError extends Error {
  constructor(message) {
    super(message || 'Unauthorized')
    this.statusCode = 401

    if (Error.captureStackTrace) {
      Error.captureStackTrace(this, UnauthorizedError);
    }
  }
}

export async function login({username, password}) {
  const userStore = useUserStore()
  const deviceStore = useDeviceStore()
  const data = await request({
    baseUrl: SERVER_URL,
    path: '/api/login',
    method: 'POST',
    data: {
      username,
      password,
      source: 'viewer',
    },
  })

  userStore.saveAuth(data)
  deviceStore.saveDevices(data)

  return data
}

export async function logout() {
  const userStore = useUserStore()

  await request({
    baseUrl: SERVER_URL,
    path: '/api/logout',
    method: 'POST',
    token: userStore.token,
    allowEmpty: true,
  }).catch(() => {})

  userStore.$reset()
}

export async function fetchDevices() {
  const auth = requireAuth()
  const data = await request({
    baseUrl: SERVER_URL,
    path: '/api/devices',
    token: auth.token,
  })

  useDeviceStore().saveDevices(data)

  return data
}

export async function fetchUsers() {
  const auth = requireAuth()
  const data = await request({
    baseUrl: SERVER_URL,
    path: '/api/users',
    token: auth.token,
  })

  return data.users || []
}

export async function fetchUser(id) {
  const auth = requireAuth()
  const data = await request({
    baseUrl: SERVER_URL,
    path: `/api/users/${encodeURIComponent(requireUserId(id))}`,
    token: auth.token,
  })

  return data.user
}

export async function createUser(payload) {
  const auth = requireAuth()
  const data = await request({
    baseUrl: SERVER_URL,
    path: '/api/users',
    method: 'POST',
    token: auth.token,
    data: buildUserPayload(payload),
  })

  return data.user
}

export async function updateUser(user) {
  const id = requireUserId(user?.id)
  const data = await saveUser(id, user, 'PUT')
  syncCurrentUser(data.user)

  return data.user
}

export async function patchUser(idOrUser, payload = {}) {
  const isUserObject = idOrUser && typeof idOrUser === 'object'
  const id = requireUserId(isUserObject ? idOrUser.id : idOrUser)
  const data = await saveUser(id, isUserObject ? idOrUser : payload, 'PATCH')
  syncCurrentUser(data.user)

  return data.user
}

export async function deleteUser(id) {
  const auth = requireAuth()

  await request({
    baseUrl: SERVER_URL,
    path: `/api/users/${encodeURIComponent(requireUserId(id))}`,
    method: 'DELETE',
    token: auth.token,
    allowEmpty: true,
  })
}

function requireAuth() {
  const userStore = useUserStore()

  if (!userStore.isLoggedIn) throw new UnauthorizedError()
  return userStore
}

function saveUser(id, user, method) {
  const auth = requireAuth()

  return request({
    baseUrl: SERVER_URL,
    path: `/api/users/${encodeURIComponent(id)}`,
    method,
    token: auth.token,
    data: buildUserPayload(user),
  })
}

function buildUserPayload(payload = {}) {
  payload = payload || {}

  const data = {}

  if (payload.username !== undefined) data.username = payload.username
  if (payload.password !== undefined) data.password = payload.password
  if (payload.role !== undefined) data.role = payload.role
  if (payload.isAdmin !== undefined) data.isAdmin = payload.isAdmin

  return data
}

function requireUserId(id) {
  const value = String(id || '').trim()
  if (!value) throw new Error('user id is required')
  return value
}

function syncCurrentUser(user) {
  const userStore = useUserStore()

  if (user && userStore.user?.id === user.id) {
    userStore.user = user
  }
}

function request({baseUrl, path, method = 'GET', data, token, allowEmpty = false}) {
  return new Promise((resolve, reject) => {
    uni.request({
      url: baseUrl + path,
      method,
      data,
      header: {
        'Content-Type': 'application/json',
        ...(token ? {Authorization: `Bearer ${token}`} : {}),
      },
      success: (res) => {
        if (res.statusCode >= 200 && res.statusCode < 300) {
          resolve(allowEmpty ? null : res.data || {})
          return
        }

        if (res.statusCode === 401 && token) {
          useUserStore().removeAuth('登录已过期，请重新登录')
          reject(new UnauthorizedError('登录已过期，请重新登录'))
          return
        }

        reject(RequestError('请求失败', res, path))
      },
      fail: (error) => reject(error),
    })
  })
}
