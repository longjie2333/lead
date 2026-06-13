export function roleLabel(user) {
  return user?.isAdmin || user?.role === 'admin' ? '管理员' : '普通用户'
}

export function formatTime(value) {
  if (!value) return "--";
  const date = new Date(value);
  const pad = (number) => String(number).padStart(2, "0");
  return `${date.getFullYear()}-${pad(date.getMonth() + 1)}-${pad(date.getDate())} ${pad(date.getHours())}:${pad(date.getMinutes())}`;
}

export function encodeQuery(params) {
  return Object.entries(params)
    .filter(([, value]) => value !== undefined && value !== null && value !== "")
    .map(([key, value]) => `${encodeURIComponent(key)}=${encodeURIComponent(value)}`)
    .join("&");
}

let redirectingPath = ''

export function reLaunchOnce(url) {
  if (redirectingPath === url) return

  redirectingPath = url

  uni.reLaunch({
    url,
    complete: () => {
      redirectingPath = ''
    },
  })
}