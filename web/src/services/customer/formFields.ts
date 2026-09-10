import type { FormField } from '@/types/customer'
import { operators } from './seed'
export const field = (
  key: string,
  label: string,
  type: FormField['type'] = 'text',
  required = true,
  options?: string[],
  help?: string,
): FormField => ({
  key,
  label,
  type,
  required,
  options,
  help,
  full: type === 'textarea' || type === 'checkbox',
})
export const ownerField = field('owner', '负责人', 'select', true, operators)
export const reasonField = field('reason', '说明与依据', 'textarea')
export const actionFields = [
  field('nextAction', '下一步行动', 'textarea'),
  field('nextAt', '下次行动时间', 'datetime-local'),
  field('deadline', '截止日期', 'date'),
]
export const followupFields = [
  field('followup', '创建独立后续事项', 'checkbox', false),
  field('title', '后续事项标题', 'text', false),
  ownerField,
  ...actionFields,
]
export const confirmField = field('confirm', '已核对范围、责任与执行结果', 'checkbox')
export const customerOptions = [
  'CUS-0186',
  'CUS-0185',
  'CUS-0183',
  'CUS-0181',
  'CUS-0179',
  'CUS-0176',
  'CUS-0173',
  'CUS-0171',
]
export const customerField = field('customerId', '关联客户', 'select', true, customerOptions)
