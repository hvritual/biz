<script setup lang="ts">
import { computed, onBeforeUnmount, ref, watch } from 'vue'
import { UiButton, UiInput, UiOption, UiSelect } from '@/ui/base'
import AppIcon from '@/ui/common/AppIcon.vue'
import AppPagination from '@/ui/common/AppPagination.vue'
import AvatarMark from '@/ui/common/AvatarMark.vue'
import EmptyState from '@/ui/common/EmptyState.vue'
import MetricCard from '@/ui/common/MetricCard.vue'
import PageHeading from '@/ui/common/PageHeading.vue'
import SearchField from '@/ui/common/SearchField.vue'
import StatusBadge from '@/ui/common/StatusBadge.vue'
import UiDialog from '@/ui/common/UiDialog.vue'
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
import { downloadCsv } from '@/utils/format'

const session = ref<TrustedSession>({ authenticated: false })
const records = ref<EnterpriseAuditRecord[]>([])
const total = ref(0)
const page = ref(1)
const pageSize = ref(20)
const busy = ref(false)
const detailBusy = ref(false)
const exportBusy = ref(false)
const error = ref('')
const notice = ref('')
const selected = ref<EnterpriseAuditRecord | null>(null)
const queryDraft = ref('')
const operationDraft = ref('')
const resultDraft = ref<EnterpriseAuditResult | ''>('')
const riskDraft = ref<EnterpriseAuditRisk | ''>('')
const activeFilter = ref<Omit<EnterpriseAuditFilter, 'page' | 'pageSize'>>({})
const exportRetry = ref<{ fingerprint: string; key: string } | null>(null)
let epoch = 0

const canRead = computed(() => Boolean(session.value.authenticated && session.value.active_tenant_id))
const pageHighRisk = computed(() => records.value.filter((record) => record.risk === 'high').length)
const pageFailures = computed(() => records.value.filter((record) => record.result === 'failure' || record.result === 'panic').length)
const pageExports = computed(() => records.value.filter((record) => record.operationId === 'access.audit.export').length)

const riskLabels: Record<EnterpriseAuditRisk, string> = {
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

function clearRows() {
  records.value = []
  total.value = 0
  selected.value = null
}

function currentFilter(): EnterpriseAuditFilter {
  return {
    ...activeFilter.value,
    page: page.value,
    pageSize: pageSize.value,
  }
}

async function refresh() {
  const token = ++epoch
  busy.value = true
  error.value = ''
  notice.value = ''
  clearRows()
  try {
    const current = await readEnterpriseAuditSession()
    if (token !== epoch) return
    session.value = current
    if (!current.authenticated || !current.active_tenant_id) return
    const result = await listEnterpriseAuditRecords(current, currentFilter())
    if (token !== epoch) return
    records.value = result.records
    total.value = result.total
    page.value = result.page
    pageSize.value = result.pageSize
  } catch (e) {
    if (token === epoch) error.value = auditRuntimeError(e)
  } finally {
    if (token === epoch) busy.value = false
  }
}

async function loadPage() {
  if (!canRead.value) return
  const token = ++epoch
  const expectedSession = session.value
  busy.value = true
  error.value = ''
  selected.value = null
  try {
    const current = await readEnterpriseAuditSession()
    if (token !== epoch) return
    if (!sameAuditSession(expectedSession, current)) {
      void refresh()
      return
    }
    const result = await listEnterpriseAuditRecords(current, currentFilter())
    if (token !== epoch) return
    records.value = result.records
    total.value = result.total
  } catch (e) {
    if (token === epoch) error.value = auditRuntimeError(e)
  } finally {
    if (token === epoch) busy.value = false
  }
}

async function applyFilters() {
  activeFilter.value = {
    query: queryDraft.value.trim(),
    operationId: operationDraft.value.trim(),
    result: resultDraft.value,
    risk: riskDraft.value,
  }
  exportRetry.value = null
  notice.value = ''
  if (page.value !== 1) {
    page.value = 1
    return
  }
  await loadPage()
}

async function resetFilters() {
  queryDraft.value = ''
  operationDraft.value = ''
  resultDraft.value = ''
  riskDraft.value = ''
  activeFilter.value = {}
  exportRetry.value = null
  notice.value = ''
  if (page.value !== 1) {
    page.value = 1
    return
  }
  await loadPage()
}

async function changeTenant(event: Event) {
  const tenantId = (event.target as HTMLSelectElement).value
  ++epoch
  busy.value = true
  error.value = ''
  notice.value = ''
  clearRows()
  exportRetry.value = null
  try {
    await switchEnterpriseAuditTenant(tenantId)
    page.value = 1
    await refresh()
  } catch (e) {
    error.value = auditRuntimeError(e)
    busy.value = false
  }
}

async function logout() {
  ++epoch
  busy.value = true
  error.value = ''
  notice.value = ''
  clearRows()
  exportRetry.value = null
  try {
    await logoutSession()
    session.value = { authenticated: false }
  } catch (e) {
    error.value = auditRuntimeError(e)
  } finally {
    busy.value = false
  }
}

async function openDetail(record: EnterpriseAuditRecord) {
  if (!canRead.value || detailBusy.value) return
  const token = epoch
  const expectedSession = session.value
  detailBusy.value = true
  error.value = ''
  try {
    const current = await readEnterpriseAuditSession()
    if (token !== epoch) return
    if (!sameAuditSession(expectedSession, current)) {
      detailBusy.value = false
      void refresh()
      return
    }
    const detail = await getEnterpriseAuditRecord(current, record.auditId)
    if (token === epoch) selected.value = detail
  } catch (e) {
    if (token === epoch) error.value = auditRuntimeError(e)
  } finally {
    if (token === epoch) detailBusy.value = false
  }
}

async function exportLogs() {
  if (!canRead.value || exportBusy.value) return
  const token = epoch
  const expectedSession = session.value
  const filter = { ...activeFilter.value }
  const fingerprint = JSON.stringify(filter)
  const key = exportRetry.value?.fingerprint === fingerprint
    ? exportRetry.value.key
    : auditRequestId()
  exportRetry.value = { fingerprint, key }
  exportBusy.value = true
  error.value = ''
  notice.value = ''
  try {
    const current = await readEnterpriseAuditSession()
    if (token !== epoch) return
    if (!sameAuditSession(expectedSession, current)) {
      exportBusy.value = false
      void refresh()
      return
    }
    const exported = await exportEnterpriseAuditRecords(current, filter, key)
    if (token !== epoch) return
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
    notice.value = `服务端导出已完成：${exported.records.length} 条；导出操作本身已进入审计链，可刷新列表查看。`
  } catch (e) {
    if (token === epoch) error.value = auditRuntimeError(e)
  } finally {
    if (token === epoch) exportBusy.value = false
  }
}

void refresh()
watch([page, pageSize], () => {
  if (canRead.value && !busy.value) void loadPage()
})
onBeforeUnmount(() => {
  epoch++
})
</script>

<template>
  <div class="page-stack real-audit" data-enterprise-audit-source="server">
    <PageHeading
      title="操作日志"
      breadcrumb="企业中心"
      description="服务端审计记录按可信租户隔离；查询、详情和导出均以 Access 审计读模型为准。"
    />

    <section class="card panel-pad audit-authority" aria-label="审计服务端身份上下文">
      <template v-if="session.authenticated">
        <div class="authority-main">
          <strong>服务端身份</strong>
          <span>{{ session.user_id || session.platform_subject || '已认证账号' }}</span>
        </div>
        <label v-if="session.tenants?.length" class="tenant-select">
          <span>当前租户</span>
          <UiSelect :value="session.active_tenant_id" :disabled="busy || exportBusy" @change="changeTenant">
            <UiOption value="" disabled>请选择租户</UiOption>
            <UiOption v-for="tenant in session.tenants" :key="tenant.id" :value="tenant.id">{{ tenant.name }}</UiOption>
          </UiSelect>
        </label>
        <UiButton class="btn" :disabled="busy || exportBusy" @click="refresh"><AppIcon name="refresh" :size="15" />刷新</UiButton>
        <UiButton class="btn" :disabled="busy || exportBusy" @click="logout">退出登录</UiButton>
      </template>
      <template v-else>
        <div class="authority-main">
          <strong>尚未登录</strong>
          <span>API 模式不会回退到本地预览审计记录。</span>
        </div>
        <a class="btn btn-primary" :href="loginUrl()">登录业务账号</a>
      </template>
    </section>

    <p v-if="error" class="notice-box audit-error" role="alert">{{ error }}</p>
    <p v-if="notice" class="notice-box" role="status">{{ notice }}</p>

    <template v-if="canRead">
      <div class="metric-grid">
        <MetricCard label="审计记录" :value="total" icon="file" caption="当前筛选的服务端记录总数" />
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
          <UiButton class="btn btn-primary" :disabled="busy" @click="applyFilters">查询</UiButton>
          <UiButton class="btn" :disabled="busy" @click="resetFilters">重置</UiButton>
          <UiButton class="btn" :disabled="busy || exportBusy" @click="exportLogs">
            <AppIcon name="download" :size="16" />{{ exportBusy ? '导出中…' : exportRetry ? '重试导出' : '导出日志' }}
          </UiButton>
        </div>

        <div v-if="records.length" class="table-scroll">
          <table class="data-table audit-table">
            <thead>
              <tr><th>时间</th><th>操作人</th><th>模块 / 操作</th><th>对象</th><th>结果</th><th>风险等级</th><th>详情</th></tr>
            </thead>
            <tbody>
              <tr v-for="record in records" :key="record.auditId">
                <td class="numeric muted">{{ formatTime(record.occurredAt) }}</td>
                <td><div class="row"><AvatarMark :name="actorLabel(record)" :size="27" /><span>{{ actorLabel(record) }}</span></div></td>
                <td><strong>{{ record.operationId }}</strong><small>{{ record.module || '—' }}</small></td>
                <td class="mono target-cell">{{ record.target || '—' }}</td>
                <td><StatusBadge :text="resultLabels[record.result]" :tone="resultTone(record.result)" /></td>
                <td><StatusBadge :text="riskLabels[record.risk]" :tone="riskTone(record.risk)" :dot="false" /></td>
                <td><UiButton class="btn-link" :disabled="detailBusy" :aria-label="`查看审计日志 ${record.operationId}`" @click="openDetail(record)">查看</UiButton></td>
              </tr>
            </tbody>
          </table>
        </div>
        <EmptyState v-else-if="!busy" />
        <div v-else class="audit-loading" role="status">正在读取服务端审计记录…</div>
        <AppPagination v-model:page="page" v-model:page-size="pageSize" :total="total" />
      </section>

      <div class="notice-box">
        <AppIcon name="shield" />日志页只读取不可变审计读模型，不提供修改或删除。CSV 内容来自服务端导出接口，导出动作由服务端以独立高风险审计事件记录。
      </div>
    </template>
    <section v-else-if="session.authenticated" class="card panel-pad">请选择可访问租户。操作日志不会展示 demo 记录作为替代。</section>

    <UiDialog :open="Boolean(selected)" title="审计日志详情" width="620px" drawer @close="selected = null">
      <div v-if="selected" class="page-stack">
        <div class="log-hero">
          <span><AppIcon name="file" :size="25" /></span>
          <div><h2>{{ selected.operationId }}</h2><p>{{ formatTime(selected.occurredAt) }}</p></div>
          <StatusBadge :text="riskLabels[selected.risk]" :tone="riskTone(selected.risk)" />
        </div>
        <dl class="detail-list">
          <dt>审计 ID</dt><dd class="mono">{{ selected.auditId }}</dd>
          <dt>操作主体</dt><dd>{{ actorLabel(selected) }}</dd>
          <dt>用户 ID</dt><dd class="mono">{{ selected.actorUserId || '—' }}</dd>
          <dt>认证方式</dt><dd>{{ selected.authMethod || '—' }} / {{ selected.authChannel || '—' }}</dd>
          <dt>会话引用</dt><dd class="mono">{{ selected.sessionRef || '—' }}</dd>
          <dt>请求 ID</dt><dd class="mono">{{ selected.requestId || '—' }}</dd>
          <dt>幂等引用</dt><dd class="mono">{{ selected.idempotencyRef || '—' }}</dd>
          <dt>所属模块</dt><dd>{{ selected.module || '—' }}</dd>
          <dt>操作对象</dt><dd class="mono">{{ selected.target || '—' }}</dd>
          <dt>执行结果</dt><dd><StatusBadge :text="resultLabels[selected.result]" :tone="resultTone(selected.result)" /></dd>
          <dt>风险等级</dt><dd>{{ riskLabels[selected.risk] }}</dd>
          <dt>回执引用</dt><dd class="mono">{{ selected.receiptRef || '—' }}</dd>
          <dt>请求摘要</dt><dd class="mono digest">{{ selected.requestDigest || '—' }}</dd>
          <dt>操作原因</dt><dd>{{ selected.reason || '未填写' }}</dd>
        </dl>
        <div class="notice-box compact-notice">当前服务端审计契约保存请求摘要与回执引用；未提供 before/after 正文时，页面不会伪造变更前后内容。</div>
      </div>
    </UiDialog>
  </div>
</template>

<style scoped>
.audit-authority { display: flex; align-items: center; gap: 14px; flex-wrap: wrap; }
.authority-main { display: grid; gap: 4px; margin-right: auto; }
.authority-main span, .tenant-select span { color: var(--color-text-muted); font-size: 12px; }
.tenant-select { display: flex; align-items: center; gap: 8px; }
.audit-error { border-color: var(--color-danger); }
.audit-query-bar { align-items: center; }
.operation-filter { min-width: 230px; }
.audit-table { min-width: 1100px; }
.audit-table td { height: 65px; }
.audit-table td strong { font-weight: 500; font-size: 12px; }
.audit-table td small { display: block; color: var(--color-text-muted); font-size: 10px; margin-top: 4px; }
.target-cell { max-width: 260px; overflow-wrap: anywhere; }
.audit-loading { padding: 44px 20px; text-align: center; color: var(--color-text-muted); }
.log-hero { display: flex; gap: 13px; align-items: center; padding-bottom: 20px; border-bottom: 1px solid var(--color-border); }
.log-hero > span { display: grid; place-items: center; width: 48px; height: 48px; border-radius: 50%; background: var(--color-primary-soft); color: var(--color-primary); }
.log-hero > div { flex: 1; min-width: 0; }
.log-hero h2 { overflow-wrap: anywhere; }
.log-hero p { font-size: 12px; color: var(--color-text-muted); margin-top: 6px; }
.digest { overflow-wrap: anywhere; }
.compact-notice { margin-top: 4px; }
@media (max-width: 900px) {
  .audit-query-bar { align-items: stretch; }
  .operation-filter { min-width: 0; width: 100%; }
}
@media (max-width: 480px) {
  .audit-authority { align-items: stretch; }
  .tenant-select { width: 100%; align-items: stretch; flex-direction: column; }
}
</style>
