export const DEFAULT_ICON = '🌐'

/** 目标服务的 favicon 地址（origin/favicon.ico）；非法 URL 返回空串 */
export function faviconURL(url: string): string {
  try {
    return new URL(url).origin + '/favicon.ico'
  } catch {
    return ''
  }
}
