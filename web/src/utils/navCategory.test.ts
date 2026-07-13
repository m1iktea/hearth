import { describe, expect, it } from 'vitest'
import {
  DEFAULT_CATEGORY_NAME,
  resolveCategorySelection,
  suggestNameFromURL,
} from './navCategory'
import type { NavCategory } from '../types'

const cats: NavCategory[] = [
  { id: 1, name: '常用服务', sort_order: 1, items: [] },
  { id: 2, name: '基础设施', sort_order: 5, items: [] },
]

describe('resolveCategorySelection', () => {
  it('选中已有分类 id 时直接复用', () => {
    expect(resolveCategorySelection(cats, 2)).toEqual({ kind: 'existing', id: 2 })
  })

  it('输入的新分类名与已有分类同名时复用，不重复创建', () => {
    expect(resolveCategorySelection(cats, '基础设施')).toEqual({ kind: 'existing', id: 2 })
    expect(resolveCategorySelection(cats, '  基础设施  ')).toEqual({ kind: 'existing', id: 2 })
  })

  it('输入全新分类名时创建，排序排在最后', () => {
    expect(resolveCategorySelection(cats, '下载工具')).toEqual({
      kind: 'create',
      name: '下载工具',
      sortOrder: 6,
    })
  })

  it('未选分类时落入默认「未分类」', () => {
    expect(resolveCategorySelection(cats, null)).toEqual({
      kind: 'create',
      name: DEFAULT_CATEGORY_NAME,
      sortOrder: 6,
    })
    expect(resolveCategorySelection(cats, '   ')).toEqual({
      kind: 'create',
      name: DEFAULT_CATEGORY_NAME,
      sortOrder: 6,
    })
  })

  it('「未分类」已存在时复用', () => {
    const withDefault = [...cats, { id: 3, name: DEFAULT_CATEGORY_NAME, sort_order: 9, items: [] }]
    expect(resolveCategorySelection(withDefault, null)).toEqual({ kind: 'existing', id: 3 })
  })

  it('空分类列表时从 1 开始排序', () => {
    expect(resolveCategorySelection([], null)).toEqual({
      kind: 'create',
      name: DEFAULT_CATEGORY_NAME,
      sortOrder: 1,
    })
  })
})

describe('suggestNameFromURL', () => {
  it('合法 URL 返回 hostname', () => {
    expect(suggestNameFromURL('http://192.168.1.10:8096/web')).toBe('192.168.1.10')
    expect(suggestNameFromURL('https://jellyfin.local')).toBe('jellyfin.local')
  })

  it('非法 URL 返回空串', () => {
    expect(suggestNameFromURL('not a url')).toBe('')
    expect(suggestNameFromURL('')).toBe('')
  })
})
