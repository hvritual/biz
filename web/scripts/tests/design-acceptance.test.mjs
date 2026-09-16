import assert from 'node:assert/strict'
import test from 'node:test'
import { checkDesignAcceptance } from '../lib/design-acceptance.mjs'

const valid = {
  navigation: { closed: { x: 208, width: 1232 }, open: { x: 208, width: 1232 } },
  requiredRegions: [{ id: 'heading', visible: true }, { id: 'data', visible: true }],
  overlay: { closed: true, focusReturned: true, scrimVisible: true, clickThrough: false },
  member: { viewedId: 'm-2', selectedIds: [], detailClosed: true, queryPreserved: true },
  layout: { viewportWidth: 1440, scrollWidth: 1440, longTextOverflow: false },
}

test('legal acceptance snapshot is not rejected', () => {
  assert.deepEqual(checkDesignAcceptance(valid), [])
})

for (const fixture of [
  [{ ...valid, navigation: { closed: { x: 208, width: 1232 }, open: { x: 688, width: 752 } } }, 'navigation-reflows-main-content'],
  [{ ...valid, requiredRegions: [{ id: 'data', visible: false }] }, 'required-region-hidden:data'],
  [{ ...valid, overlay: { closed: true, focusReturned: false, scrimVisible: true, clickThrough: false } }, 'overlay-focus-not-returned'],
  [{ ...valid, overlay: { closed: false, focusReturned: true, scrimVisible: true, clickThrough: true } }, 'overlay-click-through'],
  [{ ...valid, member: { viewedId: 'm-2', selectedIds: ['m-2'], detailClosed: false, queryPreserved: true } }, 'member-view-selection-coupled'],
  [{ ...valid, member: { viewedId: 'm-2', selectedIds: [], detailClosed: true, queryPreserved: false } }, 'member-query-not-preserved'],
  [{ ...valid, layout: { viewportWidth: 390, scrollWidth: 460, longTextOverflow: false } }, 'horizontal-overflow'],
  [{ ...valid, layout: { viewportWidth: 390, scrollWidth: 390, longTextOverflow: true } }, 'long-text-overflow'],
]) {
  const [snapshot, expected] = fixture
  test(`negative fixture is rejected: ${expected}`, () => {
    const errors = checkDesignAcceptance(snapshot)
    assert.ok(errors.includes(expected), `${expected} missing from ${JSON.stringify(errors)}`)
  })
}
