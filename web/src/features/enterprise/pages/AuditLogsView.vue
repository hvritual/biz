<script setup lang="ts">
import { computed, onBeforeUnmount, ref, watch } from 'vue'
import { UiButton, UiInput, UiOption, UiSelect } from '@/ui/base'
import type { AuditRecord } from '@/types/enterprise'
import { useEnterpriseStore } from '@/stores/enterprise'
import { useUiStore } from '@/stores/ui'
import { downloadCsv } from '@/utils/format'
import PageHeading from '@/ui/common/PageHeading.vue'
import MetricCard from '@/ui/common/MetricCard.vue'
import SearchField from '@/ui/common/SearchField.vue'
import AppIcon from '@/ui/common/AppIcon.vue'
import StatusBadge from '@/ui/common/StatusBadge.vue'
import AvatarMark from '@/ui/common/AvatarMark.vue'
import UiDialog from '@/ui/common/UiDialog.vue'
import EmptyState from '@/ui/common/EmptyState.vue'
import AppPagination from '@/ui/common/AppPagination.vue'
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
</script>

<template>
  <div
    class="page-stack"
    data-enterprise-page="audit"
    data-ui-template="ListPage"
    :data-enterprise-audit-source="apiMode ? 'server' : 'demo'"
  >
    <PageHeading
      title="操作日志"
      :description="apiMode
        ? '服务端审计记录按可信租户隔离；查询、详情和导出均以 Access 审计读模型为准。'
        : '记录成员、权限与配置变更，让每一次操作可检索、可追溯'"
    />

    <template v-if="apiMode">
      <section class="card panel-pad audit-authority" aria-label="审计服务端身份上下文">
        <template v-if="serverSession.authenticated">
          <div class="authority-main">
            <strong>服务端身份</strong>
            <span>{{ serverSession.user_id || serverSession.platform_subject || '已认证账号' }}</span>
          </div>
          <label v-if="serverSession.tenants?.length" class="tenant-select">
            <span>当前租户</span>
            <UiSelect
              :value="serverSession.active_tenant_id"
              :disabled="serverBusy || exportBusy"
              @change="changeServerTenant"
            >
              <UiOption value="" disabled>请选择租户</UiOption>
              <UiOption v-for="tenant in serverSession.tenants" :key="tenant.id" :value="tenant.id">
                {{ tenant.name }}
              </UiOption>
            </UiSelect>
          </label>
          <UiButton class="btn" :disabled="serverBusy || exportBusy" @click="refreshServer">
            <AppIcon name="refresh" :size="15" />刷新
          </UiButton>
          <UiButton class="btn" :disabled="serverBusy || exportBusy" @click="logoutServer">退出登录</UiButton>
        </template>
        <template v-else>
          <div class="authority-main">
            <strong>尚未登录</strong>
            <span>API 模式不会回退到本地预览审计记录。</span>
          </div>
          <UiButton class="btn btn-primary" @click="loginServer">登录业务账号</UiButton>
        </template>
      </section>

      <p v-if="serverError" class="notice-box audit-error" role="alert">{{ serverError }}</p>
      <p v-if="serverNotice" class="notice-box" role="status">{{ serverNotice }}</p>

      <template v-if="canReadServer">
        <div class="metric-grid">
          <MetricCard label="审计记录" :value="serverTotal" icon="file" caption="当前筛选的服务端记录总数" />
          <MetricCard label="当前页高风险" :value="pageHighRisk" icon="shield" tone="orange" caption="仅统计当前分页" />
          <MetricCard label="当前页失败" :value="pageFailures" icon="file" tone="purple" caption="failure / panic" />
          <MetricCard label="当前页导出" :value="pageExports" icon="download" tone="green" caption="access.audit.export" />
        </div>

        <section class="card data-panel">
          <div class="query-bar audit-query-bar">
            <SearchField v-model="queryDraft" placeholder="搜索操作、对象、操作人或请求 ID…" />
            <UiInput v-model="operationDraft" class="input operation-filter" placeholder="操作 ID，例如 access.audit.export" />
            <UiSelect v-model="resultDraft" class="select" aria-label="筛选执行结果">
              <UiOption value="">全部结果</UiOption>
              <UiOption value="success">成功</UiOption>
              <UiOption value="failure">失败</UiOption>
              <UiOption value="panic">异常中断</UiOption>
              <UiOption value="pending">处理中</UiOption>
            </UiSelect>
            <UiSelect v-model="riskDraft" class="select" aria-label="筛选风险等级">
              <UiOption value="">全部风险</UiOption>
              <UiOption value="low">低风险</UiOption>
              <UiOption value="medium">中风险</UiOption>
              <UiOption value="high">高风险</UiOption>
            </UiSelect>
            <UiButton class="btn btn-primary" :disabled="serverBusy" @click="applyServerFilters">查询</UiButton>
            <UiButton class="btn" :disabled="serverBusy" @click="resetServerFilters">重置</UiButton>
            <UiButton class="btn" :disabled="serverBusy || exportBusy" @click="exportServerLogs">
              <AppIcon name="download" :size="16" />{{ exportBusy ? '导出中…' : exportRetry ? '重试导出' : '导出日志' }}
            </UiButton>
          </div>

          <div v-if="serverRecords.length" class="table-scroll">
            <table class="data-table audit-table server-table">
              <thead>
                <tr>
                  <th>时间</th><th>操作人</th><th>模块 / 操作</th><th>对象</th><th>结果</th><th>风险等级</th><th>详情</th>
                </tr>
              </thead>
              <tbody>
                <tr v-for="record in serverRecords" :key="record.auditId">
                  <td class="numeric muted">{{ formatTime(record.occurredAt) }}</td>
                  <td><div class="row"><AvatarMark :name="actorLabel(record)" :size="27" /><span>{{ actorLabel(record) }}</span></div></td>
                  <td><strong>{{ record.operationId }}</strong><small>{{ record.module || '—' }}</small></td>
                  <td class="mono target-cell">{{ record.target || '—' }}</td>
                  <td><StatusBadge :text="resultLabels[record.result]" :tone="resultTone(record.result)" /></td>
                  <td><StatusBadge :text="serverRiskLabels[record.risk]" :tone="riskTone(record.risk)" :dot="false" /></td>
                  <td>
                    <UiButton
                      class="btn-link"
                      :disabled="detailBusy"
                      :aria-label="`查看审计日志 ${record.operationId}`"
                      @click="openServerDetail(record)"
                    >查看</UiButton>
                  </td>
                </tr>
              </tbody>
            </table>
          </div>
          <EmptyState v-else-if="!serverBusy" />
          <div v-else class="audit-loading" role="status">正在读取服务端审计记录…</div>
          <AppPagination v-model:page="serverPage" v-model:page-size="serverPageSize" :total="serverTotal" />
        </section>

        <div class="notice-box">
          <AppIcon name="shield" />日志页只读取不可变审计读模型，不提供修改或删除。CSV 内容来自服务端导出接口，导出动作由服务端以独立高风险审计事件记录。
        </div>
      </template>
      <section v-else-if="serverSession.authenticated" class="card panel-pad">
        请选择可访问租户。操作日志不会展示 demo 记录作为替代。
      </section>
    </template>

    <template v-else>
      <div class="metric-grid">
        <MetricCard label="操作记录" :value="store.logs.length" icon="file" caption="当前企业预览记录" />
        <MetricCard
          label="高风险操作"
          :value="store.logs.filter((log) => log.risk === 'high').length"
          icon="shield"
          tone="orange"
          caption="身份与访问权限敏感变更"
        />
        <MetricCard
          label="权限变更"
          :value="store.logs.filter((log) => log.module === '角色权限').length"
          icon="key"
          tone="purple"
          caption="角色与数据范围变更"
        />
        <MetricCard
          label="导出记录"
          :value="store.logs.filter((log) => log.action.includes('导出')).length"
          icon="download"
          tone="green"
          caption="成员或日志文件导出"
        />
      </div>
      <section class="card data-panel">
        <div class="query-bar">
          <SearchField v-model="demoQuery" placeholder="搜索操作内容、对象名称、请求 ID…" />
          <UiSelect v-model="demoModule" class="select" aria-label="筛选日志模块" @change="demoPage = 1">
            <UiOption value="">全部模块</UiOption>
            <UiOption v-for="item in demoModules" :key="item" :value="item">{{ item }}</UiOption>
          </UiSelect>
          <UiSelect v-model="demoRisk" class="select" aria-label="筛选风险等级" @change="demoPage = 1">
            <UiOption value="">全部风险</UiOption>
            <UiOption v-for="(label, key) in demoRiskLabels" :key="key" :value="key">{{ label }}</UiOption>
          </UiSelect>
          <UiButton
            class="btn"
            @click="() => { demoQuery = ''; demoModule = ''; demoRisk = ''; demoPage = 1 }"
          >重置</UiButton>
          <UiButton class="btn btn-primary" @click="exportDemoLogs">
            <AppIcon name="download" :size="16" />导出日志
          </UiButton>
        </div>
        <div v-if="demoPaged.length" class="table-scroll">
          <table class="data-table audit-table">
            <thead>
              <tr><th>时间</th><th>操作人</th><th>模块</th><th>操作内容</th><th>结果</th><th>风险等级</th><th>详情</th></tr>
            </thead>
            <tbody>
              <tr v-for="log in demoPaged" :key="log.id">
                <td class="numeric muted">{{ log.time }}</td>
                <td><div class="row"><AvatarMark :name="log.actor" :size="27" />{{ log.actor }}</div></td>
                <td>{{ log.module }}</td>
                <td><strong>{{ log.action }}</strong><small>{{ log.target }}</small></td>
                <td><StatusBadge :text="log.result === 'success' ? '成功' : '失败'" :tone="log.result === 'success' ? 'success' : 'danger'" /></td>
                <td><StatusBadge :text="demoRiskLabels[log.risk]" :tone="log.risk === 'high' ? 'danger' : log.risk === 'medium' ? 'warning' : 'primary'" :dot="false" /></td>
                <td><UiButton class="btn-link" :aria-label="'查看日志 ' + log.action" @click="selectedDemo = log">查看</UiButton></td>
              </tr>
            </tbody>
          </table>
        </div>
        <EmptyState v-else />
        <AppPagination v-model:page="demoPage" v-model:page-size="demoPageSize" :total="demoFiltered.length" />
      </section>
      <div class="notice-box">
        <AppIcon name="shield" />日志页面只提供查询与导出，不提供修改或删除。当前记录存储于本地预览，不等同于生产审计留存或防篡改证明。
      </div>
    </template>

    <UiDialog
      v-if="apiMode"
      :open="Boolean(selectedServer)"
      title="审计日志详情"
      width="620px"
      drawer
      @close="selectedServer = null"
    >
      <div v-if="selectedServer" class="page-stack">
        <div class="log-hero">
          <span><AppIcon name="file" :size="25" /></span>
          <div><h2>{{ selectedServer.operationId }}</h2><p>{{ formatTime(selectedServer.occurredAt) }}</p></div>
          <StatusBadge :text="serverRiskLabels[selectedServer.risk]" :tone="riskTone(selectedServer.risk)" />
        </div>
        <dl class="detail-list">
          <dt>审计 ID</dt><dd class="mono">{{ selectedServer.auditId }}</dd>
          <dt>操作主体</dt><dd>{{ actorLabel(selectedServer) }}</dd>
          <dt>用户 ID</dt><dd class="mono">{{ selectedServer.actorUserId || '—' }}</dd>
          <dt>认证方式</dt><dd>{{ selectedServer.authMethod || '—' }} / {{ selectedServer.authChannel || '—' }}</dd>
          <dt>会话引用</dt><dd class="mono">{{ selectedServer.sessionRef || '—' }}</dd>
          <dt>请求 ID</dt><dd class="mono">{{ selectedServer.requestId || '—' }}</dd>
          <dt>幂等引用</dt><dd class="mono">{{ selectedServer.idempotencyRef || '—' }}</dd>
          <dt>所属模块</dt><dd>{{ selectedServer.module || '—' }}</dd>
          <dt>操作对象</dt><dd class="mono">{{ selectedServer.target || '—' }}</dd>
          <dt>执行结果</dt><dd><StatusBadge :text="resultLabels[selectedServer.result]" :tone="resultTone(selectedServer.result)" /></dd>
          <dt>风险等级</dt><dd>{{ serverRiskLabels[selectedServer.risk] }}</dd>
          <dt>回执引用</dt><dd class="mono">{{ selectedServer.receiptRef || '—' }}</dd>
          <dt>请求摘要</dt><dd class="mono digest">{{ selectedServer.requestDigest || '—' }}</dd>
          <dt>操作原因</dt><dd>{{ selectedServer.reason || '未填写' }}</dd>
        </dl>
        <div class="notice-box compact-notice">
          当前服务端审计契约保存请求摘要与回执引用；未提供 before/after 正文时，页面不会伪造变更前后内容。
        </div>
      </div>
    </UiDialog>

    <UiDialog
      v-else
      :open="Boolean(selectedDemo)"
      title="日志详情"
      width="580px"
      drawer
      @close="selectedDemo = null"
    >
      <div v-if="selectedDemo" class="page-stack">
        <div class="log-hero">
          <span><AppIcon name="file" :size="25" /></span>
          <div><h2>{{ selectedDemo.action }}</h2><p>{{ selectedDemo.time }}</p></div>
          <StatusBadge :text="demoRiskLabels[selectedDemo.risk]" :tone="selectedDemo.risk === 'high' ? 'danger' : 'warning'" />
        </div>
        <h3>请求摘要</h3>
        <dl class="detail-list">
          <dt>操作人</dt><dd>{{ selectedDemo.actor }}</dd>
          <dt>所属模块</dt><dd>{{ selectedDemo.module }}</dd>
          <dt>操作对象</dt><dd>{{ selectedDemo.target }}</dd>
          <dt>执行结果</dt><dd><StatusBadge :text="selectedDemo.result === 'success' ? '成功' : '失败'" /></dd>
          <dt>操作来源</dt><dd>Web 界面预览 · 本地操作</dd>
          <dt>请求标识</dt><dd class="mono">{{ selectedDemo.requestId }}</dd>
          <dt>操作原因</dt><dd>{{ selectedDemo.reason || '未填写' }}</dd>
        </dl>
        <div class="divider" />
        <h3>变更内容</h3>
        <div class="diff-block"><h4>变更前</h4><pre>{{ pretty(selectedDemo.before) }}</pre></div>
        <div class="diff-block after"><h4>变更后</h4><pre>{{ pretty(selectedDemo.after) }}</pre></div>
      </div>
    </UiDialog>
  </div>
</template>

<style scoped>
.audit-authority {
  display: flex;
  align-items: center;
  gap: 14px;
  flex-wrap: wrap;
}
.authority-main {
  display: grid;
  gap: 4px;
  margin-right: auto;
}
.authority-main span,
.tenant-select span {
  color: var(--color-text-muted);
  font-size: 12px;
}
.tenant-select {
  display: flex;
  align-items: center;
  gap: 8px;
}
.audit-error {
  border-color: var(--color-danger);
}
.audit-query-bar {
  align-items: center;
}
.operation-filter {
  min-width: 230px;
}
.audit-table {
  min-width: 960px;
}
.server-table {
  min-width: 1100px;
}
.audit-table td {
  height: 65px;
}
.audit-table td strong {
  font-weight: 500;
  font-size: 12px;
}
.audit-table td small {
  display: block;
  color: var(--color-text-muted);
  font-size: 10px;
  margin-top: 4px;
}
.target-cell {
  max-width: 260px;
  overflow-wrap: anywhere;
}
.audit-loading {
  padding: 44px 20px;
  text-align: center;
  color: var(--color-text-muted);
}
.log-hero {
  display: flex;
  gap: 13px;
  align-items: center;
  padding-bottom: 20px;
  border-bottom: 1px solid var(--color-border);
}
.log-hero > span {
  display: grid;
  place-items: center;
  width: 48px;
  height: 48px;
  border-radius: 50%;
  background: var(--color-primary-soft);
  color: var(--color-primary);
}
.log-hero > div {
  flex: 1;
  min-width: 0;
}
.log-hero h2 {
  overflow-wrap: anywhere;
}
.log-hero p {
  font-size: 12px;
  color: var(--color-text-muted);
  margin-top: 6px;
}
.digest {
  overflow-wrap: anywhere;
}
.compact-notice {
  margin-top: 4px;
}
.diff-block {
  background: var(--color-surface-soft);
  border: 1px solid var(--color-border);
  border-radius: 8px;
  padding: 15px;
}
.diff-block h4 {
  color: var(--color-text-muted);
  font-size: 12px;
  font-weight: 500;
  margin-bottom: 12px;
}
.diff-block pre {
  font-family: var(--font-mono);
  font-size: 11px;
  line-height: 1.8;
  white-space: pre-wrap;
  overflow-wrap: anywhere;
  margin: 0;
}
.diff-block.after {
  border-color: var(--color-success-soft);
  background: var(--color-success-soft);
}
.diff-block.after h4 {
  color: var(--color-success);
}
@media (max-width: 900px) {
  .audit-query-bar {
    align-items: stretch;
  }
  .operation-filter {
    min-width: 0;
    width: 100%;
  }
}
@media (max-width: 480px) {
  .audit-authority {
    align-items: stretch;
  }
  .tenant-select {
    width: 100%;
    align-items: stretch;
    flex-direction: column;
  }
}
</style>
