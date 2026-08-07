import type { Department } from '@/types'

export function buildDepartmentTree(items: Department[], excluded = new Set<string>()) {
  const nodes = new Map<string, Department>()
  for (const item of items) {
    if (!excluded.has(item.id)) nodes.set(item.id, { ...item, children: [] })
  }
  const roots: Department[] = []
  for (const node of nodes.values()) {
    const parent = nodes.get(node.parentId)
    if (parent) parent.children?.push(node)
    else roots.push(node)
  }
  const sort = (values: Department[]) => {
    values.sort((left, right) => left.name.localeCompare(right.name, 'zh-CN'))
    values.forEach((value) => sort(value.children || []))
  }
  sort(roots)
  return roots
}

export function departmentDescendants(items: Department[], id: string) {
  const result = new Set<string>([id])
  let changed = true
  while (changed) {
    changed = false
    for (const item of items) {
      if (!result.has(item.id) && result.has(item.parentId)) {
        result.add(item.id)
        changed = true
      }
    }
  }
  return result
}
