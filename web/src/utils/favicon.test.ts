import { describe, expect, it } from 'vitest'
import { faviconURL } from './favicon'

describe('faviconURL', () => {
  it('取 origin 下的 favicon.ico，忽略路径和查询', () => {
    expect(faviconURL('http://192.168.1.10:8096/web/index.html')).toBe(
      'http://192.168.1.10:8096/favicon.ico',
    )
    expect(faviconURL('https://jellyfin.local/?a=1')).toBe('https://jellyfin.local/favicon.ico')
  })

  it('非法 URL 返回空串', () => {
    expect(faviconURL('not a url')).toBe('')
    expect(faviconURL('')).toBe('')
  })
})
