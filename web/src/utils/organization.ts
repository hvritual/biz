import type { Department } from '@/types/enterprise'
export function descendantIds(departments: Department[], id: string): string[] {
  const seen = new Set<string>(),
    queue = [id]
  while (queue.length) {
    const current = queue.shift()!
    if (seen.has(current)) continue
    seen.add(current)
    queue.push(...departments.filter((d) => d.parentId === current).map((d) => d.id))
  }
  return [...seen]
}
export function departmentMoveAllowed(departments: Department[], id: string, parentId: string | null) {
  return (
    parentId === null ||
    (!descendantIds(departments, id).includes(parentId) && departments.some((d) => d.id === parentId))
  )
}
export function flattenDepartments(
  departments: Department[],
  parentId: string | null = null,
  depth = 0,
  seen = new Set<string>(),
): Array<Department & { depth: number }> {
  return departments
    .filter((d) => d.parentId === parentId && !seen.has(d.id))
    .flatMap((d) => {
      seen.add(d.id)
      return [{ ...d, depth }, ...flattenDepartments(departments, d.id, depth + 1, seen)]
    })
}
