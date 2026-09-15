import { defineStore } from 'pinia'
import { computed, ref, watch } from 'vue'
import { useEnterpriseStore } from '@/stores/enterprise'
import {
  enterprisePlanRuntimeError,
  loadEnterprisePlanReadModel,
  type EnterprisePlanReadModel,
} from '@/services/enterprise/planRuntime'
import type { EntitlementDecisionDTO } from '@/services/commercial/platformCommercial'

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

function parseNumber(value: unknown) {
  const number = Number(value)
  return Number.isFinite(number) && number >= 0 ? number : null
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

function decisionLabel(decision: EntitlementDecisionDTO) {
  return decision.key || decision.moduleCode || '未命名权益'
}

export const useEnterprisePlanStore = defineStore('enterprise-plan', () => {
  const enterprise = useEnterpriseStore()
  const loading = ref(false)
  const error = ref('')
  const model = ref<EnterprisePlanReadModel | null>(null)
  const demoRequestCount = ref(0)

  const isServerBacked = computed(() => enterprise.sourceKind === 'api')
  const currentPlan = computed(() => model.value?.subscription.planCode || '标准版')
  const periodStart = computed(() => model.value?.subscription.periodStart || model.value?.subscription.createdAt || '2026-09-08')
  const periodEnd = computed(() => model.value?.subscription.periodEnd || '2027-09-07')
  const cycle = computed(() => model.value ? '按服务端订阅周期' : '按年')
  const subscriptionState = computed(() => model.value?.subscription.state || '使用中')
  const serverChangeContext = computed(() => model.value ? { session: model.value.session, subscription: model.value.subscription } : null)

  const features = computed<EnterprisePlanFeature[]>(() => {
    if (!model.value) return demoFeatures()
    const rows = model.value.entitlements.decisions.filter((decision) => decision.kind === 'module')
    return rows.map((decision) => ({
      key: `${decision.moduleCode}:${decision.key}`,
      label: decision.moduleCode || decisionLabel(decision),
      description: decision.reason || decisionLabel(decision),
      icon: 'shield',
      enabled: Boolean(decision.allowed),
    }))
  })

  const quotas = computed<EnterprisePlanQuota[]>(() => {
    if (!model.value) {
      const memberUsed = enterprise.members.filter((member) => member.status !== 'removed').length
      return [
        { key: 'members', label: '成员账号', used: memberUsed, total: 500, unit: '人', unlimited: false, status: '正常', icon: 'users' },
        { key: 'sites', label: '点位数量', used: 86, total: 150, unit: '个', unlimited: false, status: '正常', icon: 'site' },
        { key: 'devices', label: '设备数量', used: 320, total: 500, unit: '台', unlimited: false, status: '正常', icon: 'device' },
        { key: 'storage', label: '数据存储', used: 128, total: 500, unit: 'GB', unlimited: false, status: '正常', icon: 'database' },
      ]
    }

    const usage = new Map<string, number | null>(
      model.value.usage.usages
        .filter((item) => item.known)
        .map((item): [string, number | null] => [`${item.moduleCode}:${item.key}`, parseNumber(item.used)]),
    )
    return model.value.entitlements.decisions
      .filter((decision) => decision.kind === 'quota')
      .map((decision) => {
        const key = `${decision.moduleCode}:${decision.key}`
        const unlimited = Boolean(decision.limit?.unlimited)
        const total = unlimited ? null : parseNumber(decision.limit?.value)
        const used = usage.get(key) ?? null
        return {
          key,
          label: decisionLabel(decision),
          used,
          total,
          unit: '',
          unlimited,
          status: !decision.allowed
            ? '未开放'
            : unlimited
              ? '无限额度'
              : used == null
                ? '用量未知'
                : total != null && used >= total
                  ? '额度已用尽'
                  : '正常',
          icon: 'database',
        }
      })
  })

  async function load() {
    if (!isServerBacked.value) {
      model.value = null
      error.value = ''
      return
    }
    loading.value = true
    error.value = ''
    try {
      model.value = await loadEnterprisePlanReadModel()
      if (model.value.usageError) error.value = model.value.usageError
    } catch (cause) {
      model.value = null
      error.value = enterprisePlanRuntimeError(cause)
      throw cause
    } finally {
      loading.value = false
    }
  }

  async function refreshAfterChange() {
    await load()
  }

  function recordDemoChange(targetPlan: string, note: string) {
    if (isServerBacked.value) throw new Error('真实套餐变更必须使用服务端 preview / confirm / receipt 生命周期。')
    demoRequestCount.value += 1
    enterprise.audit(
      '套餐信息',
      '创建套餐调整申请（预览）',
      targetPlan,
      currentPlan.value,
      '待商务确认',
      note,
      'medium',
    )
  }

  watch(
    () => enterprise.tenantId,
    () => void load().catch(() => undefined),
    { immediate: true },
  )

  return {
    loading,
    error,
    model,
    isServerBacked,
    currentPlan,
    periodStart,
    periodEnd,
    cycle,
    subscriptionState,
    serverChangeContext,
    features,
    quotas,
    demoRequestCount,
    load,
    refreshAfterChange,
    recordDemoChange,
  }
})
