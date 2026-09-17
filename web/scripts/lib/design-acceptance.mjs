export function checkDesignAcceptance(snapshot) {
  const errors = []
  const closed = snapshot.navigation?.closed
  const open = snapshot.navigation?.open
  if (closed && open) {
    if (Math.abs((open.x ?? 0) - (closed.x ?? 0)) > 1 || Math.abs((open.width ?? 0) - (closed.width ?? 0)) > 1) {
      errors.push('navigation-reflows-main-content')
    }
  }

  for (const region of snapshot.requiredRegions ?? []) {
    if (!region.visible) errors.push(`required-region-hidden:${region.id}`)
  }

  if (snapshot.overlay?.closed && snapshot.overlay?.focusReturned === false) {
    errors.push('overlay-focus-not-returned')
  }
  if (snapshot.overlay?.scrimVisible && snapshot.overlay?.clickThrough) {
    errors.push('overlay-click-through')
  }

  if (snapshot.member?.viewedId && (snapshot.member?.selectedIds ?? []).includes(snapshot.member.viewedId)) {
    errors.push('member-view-selection-coupled')
  }
  if (snapshot.member?.detailClosed && snapshot.member?.queryPreserved === false) {
    errors.push('member-query-not-preserved')
  }

  if ((snapshot.layout?.scrollWidth ?? 0) > (snapshot.layout?.viewportWidth ?? Number.POSITIVE_INFINITY)) {
    errors.push('horizontal-overflow')
  }
  if (snapshot.layout?.longTextOverflow) errors.push('long-text-overflow')

  return errors
}
