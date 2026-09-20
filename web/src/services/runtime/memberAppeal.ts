import { mutate, request } from '@/services/commercial/platformCommercial'

export type MemberAppealEligibility = {
  tenant_id: string
  tenant_name: string
  status: 'suspended' | 'removed' | string
  appeal_id?: string
  appeal_state?: string
  last_submitted?: string
}

export type MemberAppealReceipt = {
  accepted: boolean
  appeal_id: string
  tenant_id: string
  membership_status: string
  state: string
  submitted_at: string
  notification_event_ids: string[]
}

export async function listMemberAppeals() {
  const result = await request<{ eligible?: MemberAppealEligibility[] }>('/auth/member-appeals')
  return Array.isArray(result.eligible) ? result.eligible : []
}

export function submitMemberAppeal(tenantId: string, reason: string) {
  return mutate<MemberAppealReceipt>('/auth/member-appeals', 'POST', {
    tenant_id: tenantId.trim(),
    reason: reason.trim(),
  })
}
