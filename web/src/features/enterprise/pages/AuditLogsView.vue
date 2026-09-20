<script setup lang="ts">
import { UiButton, UiOption, UiSelect } from '@/ui/base'
import { computed } from 'vue'
import { currentAuthorizationAllows } from '@/services/runtime/authorization'
import PageHeading from '@/ui/common/PageHeading.vue'
import MetricCard from '@/ui/common/MetricCard.vue'
import SearchField from '@/ui/common/SearchField.vue'
import AppIcon from '@/ui/common/AppIcon.vue'
import StatusBadge from '@/ui/common/StatusBadge.vue'
import AvatarMark from '@/ui/common/AvatarMark.vue'
import UiDialog from '@/ui/common/UiDialog.vue'
import EmptyState from '@/ui/common/EmptyState.vue'
import AppPagination from '@/ui/common/AppPagination.vue'
import { useAuditLogs } from '../composables/useAuditLogs'

const {
  store, apiMode,
  demoQuery, demoModule, demoRisk, demoPage, demoPageSize, selectedDemo,
  demoFiltered, demoPaged, demoModules, demoRiskLabels, exportDemoLogs, pretty,
  serverSession, serverRecords, serverTotal, serverPage, serverPageSize,
  serverBusy, detailBusy, exportBusy, serverError, serverNotice, selectedServer,
  queryDraft, resultDraft, riskDraft, exportRetry,
  canReadServer, pageHighRisk, pageFailures, pageExports, serverRiskLabels, resultLabels,
  actorLabel, formatTime, resultTone, riskTone,
  refreshServer, applyServerFilters, resetServerFilters,
  changeServerTenant, logoutServer, loginServer, openServerDetail, exportServerLogs,
} = useAuditLogs()
const canExportServer = computed(() => !apiMode || currentAuthorizationAllows('access.audit.export'))
</script>

<template>
  <div
    class="page-stack"
    data-enterprise-page="audit"
    data-ui-template="ListPage"
    :data-enterprise-audit-source="apiMode ? 'server' : 'demo'"
  >
    <PageHeading
      data-ui-region="page-heading"
      title="操作日志"
      description="记录成员、权限与配置变更，让每一次操作可检索、可追溯"
    />

    <template v-if="apiMode">
      <section class="card panel-pad audit-authority" aria-label="当前账号与企业">
        <template v-if="serverSession.authenticated">
          <div class="authority-main">
            <strong>当前账号</strong>
            <span>{{ serverSession.user_id || serverSession.platform_subject || '已认证账号' }}</span>
          </div>
          <label v-if="serverSession.tenants?.length" class="tenant-select">
            <span>当前企业</span>
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
            <span>登录后可查看当前企业的操作日志。</span>
          </div>
          <UiButton class="btn btn-primary" @click="loginServer">登录业务账号</UiButton>
        </template>
      </section>

      <p v-if="serverError" class="notice-box audit-error" role="alert">{{ serverError }}</p>
      <p v-if="serverNotice" class="notice-box" role="status">{{ serverNotice }}</p>

      <template v-if="canReadServer">
        <div class="metric-grid">
          <MetricCard label="审计记录" :value="serverTotal" icon="file" caption="当前筛选的记录总数" />
          <MetricCard label="当前页高风险" :value="pageHighRisk" icon="shield" tone="orange" caption="仅统计当前分页" />
          <MetricCard label="当前页失败" :value="pageFailures" icon="file" tone="purple" caption="当前分页中的失败记录" />
          <MetricCard label="当前页导出" :value="pageExports" icon="download" tone="green" caption="当前分页中的导出操作" />
        </div>

        <section class="card data-panel" data-ui-region="data">
          <div class="query-bar audit-query-bar" data-ui-region="query">
            <SearchField v-model="queryDraft" placeholder="搜索操作、对象或操作人…" />
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
            <UiButton v-if="canExportServer" class="btn" :disabled="serverBusy || exportBusy" @click="exportServerLogs">
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
                  <td><strong>{{ record.module || '业务操作' }}</strong><small>{{ record.reason || '操作记录' }}</small></td>
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
          <div v-else class="audit-loading" role="status">正在读取操作日志…</div>
          <AppPagination v-model:page="serverPage" v-model:page-size="serverPageSize" :total="serverTotal" />
        </section>

        <div class="notice-box">
          <AppIcon name="shield" />操作日志仅支持查看和导出，不支持修改或删除；导出操作也会记录在日志中。
        </div>
      </template>
      <section v-else-if="serverSession.authenticated" class="card panel-pad">
        请选择可访问企业。
      </section>
    </template>

    <template v-else>
      <div class="metric-grid">
        <MetricCard label="操作记录" :value="store.logs.length" icon="file" caption="当前企业操作记录" />
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
      <section class="card data-panel" data-ui-region="data">
        <div class="query-bar" data-ui-region="query">
          <SearchField v-model="demoQuery" placeholder="搜索操作内容或对象名称…" />
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
        <AppIcon name="shield" />操作日志仅支持查询和导出，不支持修改或删除。
      </div>
    </template>

    <UiDialog
      v-if="apiMode"
      :open="Boolean(selectedServer)"
      title="日志详情"
      width="620px"
      drawer
      @close="selectedServer = null"
    >
      <div v-if="selectedServer" class="page-stack">
        <div class="log-hero">
          <span><AppIcon name="file" :size="25" /></span>
          <div><h2>{{ selectedServer.module || '业务操作' }}</h2><p>{{ formatTime(selectedServer.occurredAt) }}</p></div>
          <StatusBadge :text="serverRiskLabels[selectedServer.risk]" :tone="riskTone(selectedServer.risk)" />
        </div>
        <dl class="detail-list">
          <dt>操作编号</dt><dd class="mono">{{ selectedServer.auditId }}</dd>
          <dt>操作人</dt><dd>{{ actorLabel(selectedServer) }}</dd>
          <dt>所属模块</dt><dd>{{ selectedServer.module || '—' }}</dd>
          <dt>操作对象</dt><dd>{{ selectedServer.target || '—' }}</dd>
          <dt>执行结果</dt><dd><StatusBadge :text="resultLabels[selectedServer.result]" :tone="resultTone(selectedServer.result)" /></dd>
          <dt>风险等级</dt><dd>{{ serverRiskLabels[selectedServer.risk] }}</dd>
          <dt>操作原因</dt><dd>{{ selectedServer.reason || '未填写' }}</dd>
        </dl>
        <div class="notice-box compact-notice">
          此处仅展示已记录的操作信息，没有记录的变更详情不会补充展示。
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
