import { defineStore } from 'pinia'
import { computed, ref, watch } from 'vue'
import { useEnterpriseStore } from '@/stores/enterprise'
import {
  enterprisePlanReadIssue,
  enterprisePlanRuntimeError,
  loadEnterprisePlanReadModel,
  type EnterprisePlanReadModel,
  type PlanReadIssue,
} from '@/services/enterprise/planRuntime'
import { createPlanLoadCoordinator } from '@/services/enterprise/planLoadCoordinator'
import { knownQuotaNumber } from '@/services/enterprise/planAccess'
import { sessionContext } from '@/services/runtime/api'
import { backendStateTone, backendTermLabel } from '@/i18n/backend-terms'
import { t } from '@/i18n'

export type EnterprisePlanFeature = {
  key: string
  label: string
  description: string
  enabled: boolean
  icon: string
}
export type EnterprisePlanQuota = {
  key: string
  label: string
  used: number | null
  total: number | null
  unit: string
  unlimited: boolean
  status: string
  icon: string
}

function demoFeatures(): EnterprisePlanFeature[] {
  return [
    { key: 'members', label: '成员与权限', description: '成员生命周期、角色与数据范围', icon: 'shield', enabled: true },
    { key: 'organization', label: '组织管理', description: '多层级部门与组织协同', icon: 'organization', enabled: true },
    { key: 'device', label: '设备管理', description: '接入、分组与设备状态', icon: 'device', enabled: true },
    { key: 'operations', label: '远程运维', description: '参数下发与固件升级', icon: 'operations', enabled: true },
    { key: 'analytics', label: '数据分析', description: '设备与出杯数据分析', icon: 'chart', enabled: true },
    { key: 'ticket', label: '故障工单', description: '故障受理与服务跟踪', icon: 'ticket', enabled: true },
    { key: 'api', label: '开放接口', description: '面向系统集成的开放能力', icon: 'link', enabled: false },
    { key: 'report', label: '定制化报表', description: '按需配置报表与分析', icon: 'file', enabled: false },
  ]
}

export const useEnterprisePlanStore = defineStore('enterprise-plan', () => {
  const enterprise = useEnterpriseStore()
  const loading = ref(false)
  const error = ref('')
  const readIssue = ref<PlanReadIssue | null>(null)
  const model = ref<EnterprisePlanReadModel | null>(null)
  const lastReadAt = ref('')
  const demoRequestCount = ref(0)
  const reads = createPlanLoadCoordinator<EnterprisePlanReadModel>()
  const isServerBacked = computed(() => enterprise.sourceKind === 'api')
  const scopeKey = computed(() => JSON.stringify([
    enterprise.sourceKind, enterprise.tenantId,
    enterprise.session ? sessionContext(enterprise.session) : '',
    Boolean(enterprise.session?.authenticated),
  ]))
  const stale = computed(() => Boolean(model.value && readIssue.value && readIssue.value !== 'usage'))
  const canUseCurrentFacts = computed(() => !isServerBacked.value || Boolean(model.value && !loading.value && !readIssue.value))
  const currentPlan = computed(() => model.value ? backendTermLabel('plan', model.value.subscription.planCode) : isServerBacked.value ? '—' : '标准版')
  const periodStart = computed(() => model.value?.subscription.periodStart || model.value?.subscription.createdAt || (isServerBacked.value ? '' : '2026-09-08'))
  const periodEnd = computed(() => model.value?.subscription.periodEnd || (isServerBacked.value ? '' : '2027-09-07'))
  const cycle = computed(() => model.value ? '按订阅有效期' : isServerBacked.value ? '—' : '按年')
  const subscriptionState = computed(() => model.value ? backendTermLabel('subscriptionState', model.value.subscription.state) : isServerBacked.value ? t('planFeedback.unknown') : '使用中')
  const subscriptionTone = computed(() => model.value ? backendStateTone('subscriptionState', model.value.subscription.state) : isServerBacked.value ? 'neutral' : 'success')
  // Preserve a visible receipt during a same-context reload. The consumer must
  // also use canUseCurrentFacts to block new writes while data is stale/loading.
  const serverChangeContext = computed(() => model.value ? { session: model.value.session, subscription: model.value.subscription } : null)

  const features = computed<EnterprisePlanFeature[]>(() => {
    if (!model.value) return isServerBacked.value ? [] : demoFeatures()
    return model.value.entitlements.decisions.filter((decision) => decision.kind === 'module').map((decision) => ({
      key: `${decision.moduleCode}:${decision.key}`,
      label: backendTermLabel('module', decision.moduleCode),
      description: decision.allowed ? '当前套餐已包含该能力。' : t('planFeedback.featureUnavailable'),
      icon: 'shield',
      enabled: Boolean(decision.allowed),
    }))
  })

  const quotas = computed<EnterprisePlanQuota[]>(() => {
    if (!model.value) {
      if (isServerBacked.value) return []
      const memberUsed = enterprise.members.filter((member) => member.status !== 'removed').length
      return [
        { key: 'members', label: '成员账号', used: memberUsed, total: 500, unit: '人', unlimited: false, status: '正常', icon: 'users' },
        { key: 'sites', label: '点位数量', used: 86, total: 150, unit: '个', unlimited: false, status: '正常', icon: 'site' },
        { key: 'devices', label: '设备数量', used: 320, total: 500, unit: '台', unlimited: false, status: '正常', icon: 'device' },
        { key: 'storage', label: '数据存储', used: 128, total: 500, unit: 'GB', unlimited: false, status: '正常', icon: 'database' },
      ]
    }
    const usage = new Map<string, number | null>(model.value.usage.usages
      .filter((item) => item.known)
      .map((item): [string, number | null] => [`${item.moduleCode}:${item.key}`, knownQuotaNumber(item.used)]))
    return model.value.entitlements.decisions.filter((decision) => decision.kind === 'quota').map((decision) => {
      const key = `${decision.moduleCode}:${decision.key}`
      const unlimited = Boolean(decision.limit?.unlimited)
      const total = unlimited ? null : knownQuotaNumber(decision.limit?.value)
      const used = usage.get(key) ?? null
      return {
        key, label: backendTermLabel('entitlementKey', decision.key || decision.moduleCode),
        used, total, unit: '', unlimited,
        status: !decision.allowed ? '未开放' : unlimited ? '无限额度' : used == null ? '用量未知' : total != null && used >= total ? '额度已用尽' : '正常',
        icon: 'database',
      }
    })
  })

  function resetReadState() {
    reads.invalidate()
    model.value = null
    loading.value = false
    error.value = ''
    readIssue.value = null
    lastReadAt.value = ''
  }

  function load(): Promise<void> {
    if (!isServerBacked.value) { resetReadState(); return Promise.resolve() }
    const session = enterprise.session
    if (!session?.authenticated || !enterprise.tenantId || session.active_tenant_id !== enterprise.tenantId) {
      resetReadState()
      return Promise.resolve()
    }
    return reads.run(scopeKey.value, () => loadEnterprisePlanReadModel(session), () => scopeKey.value, {
      start: () => { loading.value = true; error.value = '' },
      success: (value) => {
        model.value = value
        lastReadAt.value = new Date().toISOString()
        readIssue.value = value.usageError ? 'usage' : null
        error.value = value.usageError ? t('planFeedback.usageBody') : ''
      },
      failure: (cause) => {
        readIssue.value = enterprisePlanReadIssue(cause)
        error.value = enterprisePlanRuntimeError(cause)
        if (readIssue.value !== 'unavailable') {
          model.value = null
          lastReadAt.value = ''
        }
      },
      finish: () => { loading.value = false },
    })
  }

  async function refreshAfterChange() { await load() }

  function recordDemoChange(targetPlan: string, note: string) {
    if (isServerBacked.value) throw new Error('套餐变更请通过正式的套餐变更流程完成。')
    demoRequestCount.value += 1
    enterprise.audit('套餐信息', '创建套餐调整申请（演示）', targetPlan, currentPlan.value, '待商务确认', note, 'medium')
  }

  watch(scopeKey, () => { resetReadState(); void load() }, { immediate: true, flush: 'sync' })

  return {
    loading, error, readIssue, model, lastReadAt, stale, canUseCurrentFacts, scopeKey,
    isServerBacked, currentPlan, periodStart, periodEnd, cycle, subscriptionState,
    subscriptionTone, serverChangeContext, features, quotas, demoRequestCount,
    load, refreshAfterChange, recordDemoChange,
  }
})
