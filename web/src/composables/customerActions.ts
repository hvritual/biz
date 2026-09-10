import { inject, type InjectionKey } from 'vue'
import type { ActionId } from '@/services/customer/commands'
import type { FormValues } from '@/types/customer'
export interface CustomerActions {
  open: (action: ActionId, id?: string, selected?: string[], defaults?: FormValues) => void
}
export const customerActionsKey: InjectionKey<CustomerActions> = Symbol('customer-actions')
export function useCustomerActions(): CustomerActions {
  const value = inject(customerActionsKey)
  if (!value) throw new Error('客户操作必须位于现有 AppShell 内的客户页面区域')
  return value
}
