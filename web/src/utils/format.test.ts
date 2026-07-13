import { describe, expect, it } from 'vitest'
import { usageStatus, formatBytes, formatUptime, percent } from './format'

describe('formatBytes', () => {
  it('formats scales', () => {
    expect(formatBytes(0)).toBe('0 B')
    expect(formatBytes(1024)).toBe('1.0 KiB')
    expect(formatBytes(8589934592)).toBe('8.0 GiB')
  })
})

describe('formatUptime', () => {
  it('formats days/hours/minutes', () => {
    expect(formatUptime(59)).toBe('59s')
    expect(formatUptime(3660)).toBe('1h 1m')
    expect(formatUptime(90061)).toBe('1d 1h')
  })
})

describe('percent', () => {
  it('handles zero denominator', () => {
    expect(percent(1, 0)).toBe(0)
    expect(percent(1, 4)).toBe(25)
  })
})

describe('usageStatus', () => {
  it('maps usage percentage to progress status', () => {
    expect(usageStatus(0)).toBe('success')
    expect(usageStatus(70)).toBe('success')
    expect(usageStatus(71)).toBe('warning')
    expect(usageStatus(90)).toBe('warning')
    expect(usageStatus(91)).toBe('error')
  })
})
