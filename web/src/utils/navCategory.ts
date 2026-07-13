import type { NavCategory } from '../types'

export const DEFAULT_CATEGORY_NAME = '未分类'

/** 添加导航时的分类选择：number=已有分类 id；string=现场输入的新分类名；null=未选（落入默认分类） */
export type CategorySelection = number | string | null

export type CategoryResolution =
  | { kind: 'existing'; id: number }
  | { kind: 'create'; name: string; sortOrder: number }

/**
 * 把分类选择解析为动作：能复用已有分类就复用（含按名字匹配），否则创建。
 * 未选分类时落入「未分类」。
 */
export function resolveCategorySelection(
  categories: NavCategory[],
  sel: CategorySelection,
): CategoryResolution {
  if (typeof sel === 'number') return { kind: 'existing', id: sel }
  const name = typeof sel === 'string' && sel.trim() ? sel.trim() : DEFAULT_CATEGORY_NAME
  const existing = categories.find((c) => c.name === name)
  if (existing) return { kind: 'existing', id: existing.id }
  const maxOrder = categories.reduce((max, c) => Math.max(max, c.sort_order), 0)
  return { kind: 'create', name, sortOrder: maxOrder + 1 }
}

/** 从 URL 推断默认名称（取 hostname）；非法 URL 返回空串 */
export function suggestNameFromURL(url: string): string {
  try {
    return new URL(url).hostname
  } catch {
    return ''
  }
}
