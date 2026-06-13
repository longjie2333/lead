import { useUserStore } from '../stores/user'
import { reLaunchOnce } from './index'

export const LOGIN_PAGE = '/pages/login/login'
export const AFTER_LOGIN_PAGE = '/pages/devices/devices'
export const AUTH_SKIP_PAGES = [
  LOGIN_PAGE,
]

const ROUTE_METHODS = ['navigateTo', 'redirectTo', 'reLaunch', 'switchTab']

let installed = false

export function installAuthGuard(app) {
  if (installed) return

  installed = true
  installRouteInterceptors()

  app.mixin({
    onLoad() {
      guardCurrentPage()
    },
    onShow() {
      guardCurrentPage()
    },
  })
}

export function guardCurrentPage() {
  const page = getCurrentPage()
  if (!page?.route) return true

  return guardPage(page.route)
}

export function guardPage(url) {
  const path = normalizePagePath(url)
  if (!path) return true

  const userStore = useUserStore()

  if (userStore.isLoggedIn) {
    if (path === normalizePagePath(LOGIN_PAGE)) {
      reLaunchOnce(AFTER_LOGIN_PAGE)
      return false
    }

    return true
  }

  if (isAuthSkipped(path)) return true

  reLaunchOnce(LOGIN_PAGE)
  return false
}

export function isAuthSkipped(url) {
  const path = normalizePagePath(url)

  return AUTH_SKIP_PAGES
    .map(normalizePagePath)
    .includes(path)
}

function installRouteInterceptors() {
  ROUTE_METHODS.forEach((method) => {
    uni.addInterceptor(method, {
      invoke(args = {}) {
        return guardPage(args.url)
      },
    })
  })
}

function getCurrentPage() {
  const pages = typeof getCurrentPages === 'function' ? getCurrentPages() : []
  return pages[pages.length - 1]
}

function normalizePagePath(url = '') {
  const path = String(url).split('?')[0].split('#')[0].replace(/^\/+/, '')
  return path ? `/${path}` : ''
}
