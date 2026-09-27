import { describe, expect, it, vi } from 'vitest'
import { readCompleteCandidateDirectory, sameSiteSelection, scopeConfirmationMatches, validResourceVersion } from './dataPermissionRules'

describe('complete member scope candidate directory', () => {
  it('loads the 101st candidate before determining membership', async () => {
    const items = Array.from({ length: 101 }, (_, index) => ({ id: `site-${index}` }))
    const read = vi.fn(async (page: number) => ({ candidates: items.slice((page - 1) * 100, page * 100), total: 101 }))
    await expect(readCompleteCandidateDirectory(read)).resolves.toEqual(items)
    expect(read.mock.calls).toEqual([[1], [2]])
  })
  it('does not publish a partial list if the second page fails', async () => {
    const read = vi.fn().mockResolvedValueOnce({ candidates: [{ id: 'a' }], total: 2 }).mockRejectedValueOnce(new Error('network'))
    await expect(readCompleteCandidateDirectory(read)).rejects.toThrow('network')
  })
  it.each([
    [{ candidates: [{ id: 'a' }], total: 2 }, { candidates: [], total: 2 }],
    [{ candidates: [{ id: 'a' }], total: 2 }, { candidates: [{ id: 'a' }], total: 2 }],
    [{ candidates: [{ id: 'a' }], total: 2 }, { candidates: [{ id: 'b' }], total: 3 }],
  ])('fails closed on pagination gaps, duplicates or changed totals', async (first, second) => {
    const read = vi.fn().mockResolvedValueOnce(first).mockResolvedValueOnce(second)
    await expect(readCompleteCandidateDirectory(read)).rejects.toThrow()
    expect(read).toHaveBeenCalledTimes(2)
  })
  it('discards a response when the tenant context changed during a read', async () => {
    let changed = false
    const read = vi.fn(async () => { changed = true; return { candidates: [{ id: 'a' }], total: 1 } })
    await expect(readCompleteCandidateDirectory(read, () => { if (changed) throw new Error('CONTEXT_CHANGED') })).rejects.toThrow('CONTEXT_CHANGED')
  })
  it('accepts an explicitly empty complete directory', async () => {
    await expect(readCompleteCandidateDirectory(async () => ({ candidates: [], total: 0 }))).resolves.toEqual([])
  })
  it('bounds stalled and oversized directory reads instead of looping indefinitely', async () => {
    await expect(readCompleteCandidateDirectory(async () => ({ candidates: [], total: 10001 }))).rejects.toThrow()
    const read = vi.fn(async (page: number) => ({ candidates: [{ id: String(page) }], total: 101 }))
    await expect(readCompleteCandidateDirectory(read)).rejects.toThrow('INCOMPLETE_CANDIDATE_DIRECTORY')
    expect(read).toHaveBeenCalledTimes(100)
  })
})

describe('permission confirmation', () => {
  const expected = { userId: 'member-a', tenantId: 'tenant-a', version: '9007199254740993', siteIds: ['site-a'] }
  it('preserves exact uint64 versions rather than coercing them to JS numbers', () => {
    expect(validResourceVersion(expected.version)).toBe(true)
    expect(validResourceVersion(Number(expected.version))).toBe(false)
    expect(scopeConfirmationMatches(expected, expected)).toBe(true)
    expect(scopeConfirmationMatches({ ...expected, version: '9007199254740992' }, expected)).toBe(false)
  })
  it('rejects wrong member, tenant, selection and later conflicting version', () => {
    for (const delta of [{ userId: 'member-b' }, { tenantId: 'tenant-b' }, { siteIds: [] }, { version: '9007199254740994' }]) {
      expect(scopeConfirmationMatches({ ...expected, ...delta }, expected)).toBe(false)
    }
  })
  it('compares sets without changing the original selection', () => {
    const values = ['b', 'a', 'a']
    expect(sameSiteSelection(values, ['a', 'b'])).toBe(true)
    expect(values).toEqual(['b', 'a', 'a'])
  })
})
