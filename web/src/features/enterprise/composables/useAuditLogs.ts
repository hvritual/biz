import { computed, onBeforeUnmount, ref, watch } from 'vue'
import type { AuditRecord } from '@/types/enterprise'
import { useEnterpriseStore } from '@/stores/enterprise'
import { useUiStore } from '@/stores/ui'
import { downloadCsv } from '@/utils/format'
import { loginUrl, logoutSession, type TrustedSession } from '@/services/runtime/api'
import {
  auditRequestId,
  auditRuntimeError,
  exportEnterpriseAuditRecords,
  getEnterpriseAuditRecord,
  listEnterpriseAuditRecords,
  readEnterpriseAuditSession,
  sameAuditSession,
  switchEnterpriseAuditTenant,
  type EnterpriseAuditFilter,
  type EnterpriseAuditRecord,
  type EnterpriseAuditResult,
  type EnterpriseAuditRisk,
} from '@/services/enterprise/auditRuntime'

// Own page-local audit state and workflows; transport and authority remain in services.
export function useAuditLogs() {
  const store = useEnterpriseStore()
  const ui = useUiStore()
  const apiMode = computed(() => store.sourceKind === 'api')

  const demoQuery = ref('')
  const demoModule = ref('')
  const demoRisk = ref('')
  const demoPage = ref(1)
  const demoPageSize = ref(10)
  const selectedDemo = ref<AuditRecord | null>(null)
  const demoFiltered = computed(() =>
    store.logs.filter(
      (log) =>
        (log.action + log.target + log.actor + log.requestId).includes(demoQuery.value) &&
        (!demoModule.value || log.module === demoModule.value) &&
        (!demoRisk.value || log.risk === demoRisk.value),
    ),
  )
  const demoPaged = computed(() =>
    demoFiltered.value.slice((demoPage.value - 1) * demoPageSize.value, demoPage.value * demoPageSize.value),
  )
  const demoModules = computed(() => [...new Set(store.logs.map((log) => log.module))])
  const demoRiskLabels: Record<AuditRecord['risk'], string> = {
    low: '低风险',
    medium: '中风险',
    high: '高风险',
  }

  watch([demoQuery, demoModule, demoRisk, demoPageSize], () => {
    demoPage.value = 1
  })

  function exportDemoLogs() {
    const list = [...demoFiltered.value]
    downloadCsv('操作日志-界面预览.csv', [
      ['时间', '操作人', '模块', '操作', '对象', '结果', '风险', '请求ID'],
      ...list.map((log) => [
        log.time,
        log.actor,
        log.module,
        log.action,
        log.target,
        log.result,
        demoRiskLabels[log.risk],
        log.requestId,
      ]),
    ])
    store.audit('操作日志', '导出操作记录', `${list.length} 条预览记录`)
    ui.toast('本地预览日志已导出。')
  }

  function pretty(value: string) {
    try {
      return JSON.stringify(JSON.parse(value), null, 2)
    } catch {
      return value || '—'
    }
  }

  const serverSession = ref<TrustedSession>({ authenticated: false })
  const serverRecords = ref<EnterpriseAuditRecord[]>([])
  const serverTotal = ref(0)
  const serverPage = ref(1)
  const serverPageSize = ref(20)
  const serverBusy = ref(false)
  const detailBusy = ref(false)
  const exportBusy = ref(false)
  const serverError = ref('')
  const serverNotice = ref('')
  const selectedServer = ref<EnterpriseAuditRecord | null>(null)
  const queryDraft = ref('')
  const operationDraft = ref('')
  const resultDraft = ref<EnterpriseAuditResult | ''>('')
  const riskDraft = ref<EnterpriseAuditRisk | ''>('')
  const activeFilter = ref<Omit<EnterpriseAuditFilter, 'page' | 'pageSize'>>({})
  const exportRetry = ref<{ fingerprint: string; key: string } | null>(null)
  let serverEpoch = 0

  const canReadServer = computed(() => Boolean(serverSession.value.authenticated && serverSession.value.active_tenant_id))
  const pageHighRisk = computed(() => serverRecords.value.filter((record) => record.risk === 'high').length)
  const pageFailures = computed(() => serverRecords.value.filter((record) => record.result === 'failure' || record.result === 'panic').length)
  const pageExports = computed(() => serverRecords.value.filter((record) => record.operationId === 'access.audit.export').length)
  const serverRiskLabels: Record<EnterpriseAuditRisk, string> = {
    low: '低风险',
    medium: '中风险',
    high: '高风险',
  }
  const resultLabels: Record<EnterpriseAuditResult, string> = {
    pending: '处理中',
    success: '成功',
    failure: '失败',
    panic: '异常中断',
  }

  function actorLabel(record: EnterpriseAuditRecord) {
    return record.actorSubject || record.actorUserId || '未知主体'
  }

  function formatTime(value: string) {
    if (!value) return '—'
    const timestamp = Date.parse(value)
    if (!Number.isFinite(timestamp)) return value
    return new Intl.DateTimeFormat('zh-CN', {
      year: 'numeric',
      month: '2-digit',
      day: '2-digit',
      hour: '2-digit',
      minute: '2-digit',
      second: '2-digit',
      hour12: false,
    }).format(new Date(timestamp))
  }

  function resultTone(result: EnterpriseAuditResult) {
    if (result === 'success') return 'success'
    if (result === 'pending') return 'warning'
    return 'danger'
  }

  function riskTone(risk: EnterpriseAuditRisk) {
    if (risk === 'high') return 'danger'
    if (risk === 'medium') return 'warning'
    return 'primary'
  }

  function clearServerRows() {
    serverRecords.value = []
    serverTotal.value = 0
    selectedServer.value = null
  }

  function currentServerFilter(): EnterpriseAuditFilter {
    return {
      ...activeFilter.value,
      page: serverPage.value,
      pageSize: serverPageSize.value,
    }
  }

  async function refreshServer() {
    if (!apiMode.value) return
    const token = ++serverEpoch
    serverBusy.value = true
    serverError.value = ''
    serverNotice.value = ''
    clearServerRows()
    try {
      const current = await readEnterpriseAuditSession()
      if (token !== serverEpoch) return
      serverSession.value = current
      if (!current.authenticated || !current.active_tenant_id) return
      const result = await listEnterpriseAuditRecords(current, currentServerFilter())
      if (token !== serverEpoch) return
      serverRecords.value = result.records
      serverTotal.value = result.total
      serverPage.value = result.page
      serverPageSize.value = result.pageSize
    } catch (error) {
      if (token === serverEpoch) serverError.value = auditRuntimeError(error)
    } finally {
      if (token === serverEpoch) serverBusy.value = false
    }
  }

  async function loadServerPage() {
    if (!apiMode.value || !canReadServer.value) return
    const token = ++serverEpoch
    const expectedSession = serverSession.value
    serverBusy.value = true
    serverError.value = ''
    selectedServer.value = null
    try {
      const current = await readEnterpriseAuditSession()
      if (token !== serverEpoch) return
      if (!sameAuditSession(expectedSession, current)) {
        void refreshServer()
        return
      }
      const result = await listEnterpriseAuditRecords(current, currentServerFilter())
      if (token !== serverEpoch) return
      serverRecords.value = result.records
      serverTotal.value = result.total
    } catch (error) {
      if (token === serverEpoch) serverError.value = auditRuntimeError(error)
    } finally {
      if (token === serverEpoch) serverBusy.value = false
    }
  }

  async function applyServerFilters() {
    activeFilter.value = {
      query: queryDraft.value.trim(),
      operationId: operationDraft.value.trim(),
      result: resultDraft.value,
      risk: riskDraft.value,
    }
    exportRetry.value = null
    serverNotice.value = ''
    if (serverPage.value !== 1) {
      serverPage.value = 1
      return
    }
    await loadServerPage()
  }

  async function resetServerFilters() {
    queryDraft.value = ''
    operationDraft.value = ''
    resultDraft.value = ''
    riskDraft.value = ''
    activeFilter.value = {}
    exportRetry.value = null
    serverNotice.value = ''
    if (serverPage.value !== 1) {
      serverPage.value = 1
      return
    }
    await loadServerPage()
  }

  async function changeServerTenant(event: Event) {
    const tenantId = (event.target as HTMLSelectElement).value
    ++serverEpoch
    serverBusy.value = true
    serverError.value = ''
    serverNotice.value = ''
    clearServerRows()
    exportRetry.value = null
    try {
      await switchEnterpriseAuditTenant(tenantId)
      serverPage.value = 1
      await refreshServer()
    } catch (error) {
      serverError.value = auditRuntimeError(error)
      serverBusy.value = false
    }
  }

  async function logoutServer() {
    ++serverEpoch
    serverBusy.value = true
    serverError.value = ''
    serverNotice.value = ''
    clearServerRows()
    exportRetry.value = null
    try {
      await logoutSession()
      serverSession.value = { authenticated: false }
    } catch (error) {
      serverError.value = auditRuntimeError(error)
    } finally {
      serverBusy.value = false
    }
  }

  function loginServer() {
    window.location.assign(loginUrl())
  }

  async function openServerDetail(record: EnterpriseAuditRecord) {
    if (!canReadServer.value || detailBusy.value) return
    const token = serverEpoch
    const expectedSession = serverSession.value
    detailBusy.value = true
    serverError.value = ''
    try {
      const current = await readEnterpriseAuditSession()
      if (token !== serverEpoch) return
      if (!sameAuditSession(expectedSession, current)) {
        detailBusy.value = false
        void refreshServer()
        return
      }
      const detail = await getEnterpriseAuditRecord(current, record.auditId)
      if (token === serverEpoch) selectedServer.value = detail
    } catch (error) {
      if (token === serverEpoch) serverError.value = auditRuntimeError(error)
    } finally {
      if (token === serverEpoch) detailBusy.value = false
    }
  }

  async function exportServerLogs() {
    if (!canReadServer.value || exportBusy.value) return
    const token = serverEpoch
    const expectedSession = serverSession.value
    const filter = { ...activeFilter.value }
    const fingerprint = JSON.stringify(filter)
    const key = exportRetry.value?.fingerprint === fingerprint ? exportRetry.value.key : auditRequestId()
    exportRetry.value = { fingerprint, key }
    exportBusy.value = true
    serverError.value = ''
    serverNotice.value = ''
    try {
      const current = await readEnterpriseAuditSession()
      if (token !== serverEpoch) return
      if (!sameAuditSession(expectedSession, current)) {
        exportBusy.value = false
        void refreshServer()
        return
      }
      const exported = await exportEnterpriseAuditRecords(current, filter, key)
      if (token !== serverEpoch) return
      downloadCsv(`操作日志-${exported.exportId || 'server-export'}.csv`, [
        ['时间', '操作人', '用户ID', '操作', '对象', '结果', '风险', '请求ID', '会话引用', '回执引用', '原因'],
        ...exported.records.map((record) => [
          record.occurredAt,
          record.actorSubject,
          record.actorUserId,
          record.operationId,
          record.target,
          record.result,
          record.risk,
          record.requestId,
          record.sessionRef,
          record.receiptRef,
          record.reason,
        ]),
      ])
      exportRetry.value = null
      serverNotice.value = `服务端导出已完成：${exported.records.length} 条；导出操作本身已进入审计链，可刷新列表查看。`
    } catch (error) {
      if (token === serverEpoch) serverError.value = auditRuntimeError(error)
    } finally {
      if (token === serverEpoch) exportBusy.value = false
    }
  }

  watch(
    apiMode,
    (enabled) => {
      if (enabled) void refreshServer()
      else ++serverEpoch
    },
    { immediate: true },
  )
  watch([serverPage, serverPageSize], () => {
    if (apiMode.value && canReadServer.value && !serverBusy.value) void loadServerPage()
  })
  onBeforeUnmount(() => {
    serverEpoch++
  })

  return {
    store, apiMode,
    demoQuery, demoModule, demoRisk, demoPage, demoPageSize, selectedDemo,
    demoFiltered, demoPaged, demoModules, demoRiskLabels, exportDemoLogs, pretty,
    serverSession, serverRecords, serverTotal, serverPage, serverPageSize,
    serverBusy, detailBusy, exportBusy, serverError, serverNotice, selectedServer,
    queryDraft, operationDraft, resultDraft, riskDraft, exportRetry,
    canReadServer, pageHighRisk, pageFailures, pageExports, serverRiskLabels, resultLabels,
    actorLabel, formatTime, resultTone, riskTone,
    refreshServer, applyServerFilters, resetServerFilters,
    changeServerTenant, logoutServer, loginServer, openServerDetail, exportServerLogs,
  }
}
