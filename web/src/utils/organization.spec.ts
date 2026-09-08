import { describe, it, expect } from 'vitest'
import { createSeed } from '@/services/demo/seed'
import { descendantIds, departmentMoveAllowed, flattenDepartments } from './organization'
const ds = createSeed('shanghai').departments
describe('department hierarchy', () => {
  it('finds own and descendant departments', () =>
    expect(descendantIds(ds, 'operations')).toEqual(['operations', 'success', 'east', 'south']))
  it('rejects self and descendant moves', () => {
    expect(departmentMoveAllowed(ds, 'operations', 'operations')).toBe(false)
    expect(departmentMoveAllowed(ds, 'operations', 'east')).toBe(false)
  })
  it('permits another root and rejects unknown parent', () => {
    expect(departmentMoveAllowed(ds, 'east', 'product')).toBe(true)
    expect(departmentMoveAllowed(ds, 'east', 'missing')).toBe(false)
  })
  it('flattens the tree exactly once per department', () => {
    const flat = flattenDepartments(ds)
    expect(flat).toHaveLength(ds.length)
    expect(new Set(flat.map((d) => d.id)).size).toBe(ds.length)
  })
})
