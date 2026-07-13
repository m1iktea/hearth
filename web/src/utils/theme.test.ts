import { describe, expect, it } from 'vitest'
import { resolveInitialMode } from './theme'

describe('resolveInitialMode', () => {
  it('localStorage 有合法值时优先', () => {
    expect(resolveInitialMode('light', true)).toBe('light')
    expect(resolveInitialMode('dark', false)).toBe('dark')
  })

  it('无存储或值非法时跟随系统偏好', () => {
    expect(resolveInitialMode(null, true)).toBe('dark')
    expect(resolveInitialMode(null, false)).toBe('light')
    expect(resolveInitialMode('garbage', true)).toBe('dark')
  })
})
