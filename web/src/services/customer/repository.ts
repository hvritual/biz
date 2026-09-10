import type { CustomerSnapshot } from '@/types/customer'
import { createCustomerSeed } from './seed'
import { assertRule } from './policy'
export const customerStorageKey = (tenant: string) => `coffeelink:customer-preview:v1:${tenant}`
export function loadCustomerSnapshot(tenant: string, storage: Pick<Storage, 'getItem'>): CustomerSnapshot {
  const raw = storage.getItem(customerStorageKey(tenant))
  if (!raw) return createCustomerSeed(tenant)
  try {
    const data: CustomerSnapshot = JSON.parse(raw)
    assertRule(
      data.schema === 1 &&
        data.tenant === tenant &&
        Array.isArray(data.work) &&
        Array.isArray(data.customers) &&
        Array.isArray(data.receipts),
      'BAD_SNAPSHOT',
      '本地预览数据格式异常；请显式重置预览数据',
    )
    return data
  } catch {
    throw new Error('无法读取本地预览数据。原始数据未覆盖；请重置此租户的预览数据后重试。')
  }
}
export function persistCustomerSnapshot(snapshot: CustomerSnapshot, storage: Pick<Storage, 'setItem'>) {
  try {
    storage.setItem(customerStorageKey(snapshot.tenant), JSON.stringify(snapshot))
  } catch {
    throw new Error('本地存储失败，未保存任何变更。请检查浏览器存储空间后重试。')
  }
}
