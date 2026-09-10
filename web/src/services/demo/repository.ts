import type { TenantSnapshot } from '@/types/enterprise'
import { createSeed } from './seed'
const PREFIX = 'coffeelink:preview:v1:'
export function loadSnapshot(tenant: string): TenantSnapshot {
  try {
    const value = localStorage.getItem(PREFIX + tenant)
    if (value) {
      const parsed: unknown = JSON.parse(value)
      if (
        parsed &&
        typeof parsed === 'object' &&
        'members' in parsed &&
        'roles' in parsed &&
        'company' in parsed &&
        'departments' in parsed &&
        'logs' in parsed &&
        'settings' in parsed
      ) {
        const p = parsed as TenantSnapshot
        if ([p.members, p.roles, p.departments, p.logs].every(Array.isArray)) return p
      }
    }
  } catch {
    /* Corrupt/unavailable preview storage is never treated as an API response. */
  }
  return createSeed(tenant)
}
export function saveSnapshot(tenant: string, snapshot: TenantSnapshot): void {
  localStorage.setItem(PREFIX + tenant, JSON.stringify(snapshot))
}
