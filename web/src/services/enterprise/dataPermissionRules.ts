/** A complete directory is required before an absent object can be called unavailable. */
export async function readCompleteCandidateDirectory<T extends { id: string }>(
  readPage: (page: number) => Promise<{ candidates: T[]; total: number }>,
  assertCurrent: () => void = () => undefined,
): Promise<T[]> {
  const result: T[] = []
  const seen = new Set<string>()
  let expectedTotal: number | undefined
  for (let page = 1; page <= 100; page += 1) {
    assertCurrent()
    const response = await readPage(page)
    assertCurrent()
    if (!Number.isSafeInteger(response.total) || response.total < 0 || response.total > 10000) {
      throw new Error('INCOMPLETE_CANDIDATE_DIRECTORY')
    }
    expectedTotal ??= response.total
    if (response.total !== expectedTotal) throw new Error('CANDIDATE_DIRECTORY_CHANGED')
    for (const item of response.candidates) {
      if (!item.id || seen.has(item.id)) throw new Error('CANDIDATE_DIRECTORY_CHANGED')
      seen.add(item.id)
      result.push(item)
    }
    if (result.length === expectedTotal) return result
    if (result.length > expectedTotal || response.candidates.length === 0) {
      throw new Error('INCOMPLETE_CANDIDATE_DIRECTORY')
    }
  }
  throw new Error('INCOMPLETE_CANDIDATE_DIRECTORY')
}

export function sameSiteSelection(left: readonly string[], right: readonly string[]) {
  const normalize = (values: readonly string[]) => [...new Set(values)].sort()
  return JSON.stringify(normalize(left)) === JSON.stringify(normalize(right))
}

export function validResourceVersion(value: string | number) {
  return typeof value === 'number'
    ? Number.isSafeInteger(value) && value > 0
    : /^[1-9]\d*$/.test(value)
}

/** Exact version and identity binding prevents a coincidentally equal later write confirming ours. */
export function scopeConfirmationMatches(
  value: { userId: string; tenantId: string; version: string | number; siteIds: string[] },
  expected: { userId: string; tenantId: string; version?: string | number; siteIds: string[] },
) {
  return value.userId === expected.userId && value.tenantId === expected.tenantId &&
    validResourceVersion(value.version) &&
    (expected.version === undefined || String(value.version) === String(expected.version)) &&
    sameSiteSelection(value.siteIds, expected.siteIds)
}
