export function formatTime(value) {
  const time = Number(value)
  if (!time) return '--'

  const date = new Date(time)
  const pad = (item) => String(item).padStart(2, '0')
  return `${date.getFullYear()}-${pad(date.getMonth() + 1)}-${pad(date.getDate())} ${pad(date.getHours())}:${pad(date.getMinutes())}`
}

export function roleLabel(user) {
  return user?.isAdmin || user?.role === 'admin' ? '管理员' : '普通用户'
}
