export type ThemeMode = 'light' | 'dark'

/** 解析初始主题：localStorage 有合法值则用之，否则跟随系统偏好 */
export function resolveInitialMode(stored: string | null, systemDark: boolean): ThemeMode {
  if (stored === 'light' || stored === 'dark') return stored
  return systemDark ? 'dark' : 'light'
}
