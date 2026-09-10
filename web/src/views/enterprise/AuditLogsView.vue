<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import type { AuditRecord } from '@/types/enterprise'
import { useEnterpriseStore } from '@/stores/enterprise'
import { useUiStore } from '@/stores/ui'
import { downloadCsv } from '@/utils/format'
import PageHeading from '@/components/ui/PageHeading.vue'
import MetricCard from '@/components/ui/MetricCard.vue'
import SearchField from '@/components/ui/SearchField.vue'
import AppIcon from '@/components/ui/AppIcon.vue'
import StatusBadge from '@/components/ui/StatusBadge.vue'
import AvatarMark from '@/components/ui/AvatarMark.vue'
import UiDialog from '@/components/ui/UiDialog.vue'
import EmptyState from '@/components/ui/EmptyState.vue'
import AppPagination from '@/components/ui/AppPagination.vue'
const store = useEnterpriseStore(),
  ui = useUiStore(),
  query = ref(''),
  module = ref(''),
  risk = ref(''),
  page = ref(1),
  pageSize = ref(10),
  selected = ref<AuditRecord | null>(null)
const filtered = computed(() =>
  store.logs.filter(
    (l) =>
      (l.action + l.target + l.actor + l.requestId).includes(query.value) &&
      (!module.value || l.module === module.value) &&
      (!risk.value || l.risk === risk.value),
  ),
)
const paged = computed(() =>
  filtered.value.slice((page.value - 1) * pageSize.value, page.value * pageSize.value),
)
watch([query, module, risk, pageSize], () => {
  page.value = 1
})
const riskLabels = { low: '低风险', medium: '中风险', high: '高风险' }
function exportLogs() {
  const list = [...filtered.value]
  downloadCsv('操作日志-界面预览.csv', [
    ['时间', '操作人', '模块', '操作', '对象', '结果', '风险', '请求ID'],
    ...list.map((l) => [
      l.time,
      l.actor,
      l.module,
      l.action,
      l.target,
      l.result,
      riskLabels[l.risk],
      l.requestId,
    ]),
  ])
  store.audit('操作日志', '导出操作记录', `${list.length} 条预览记录`)
  ui.toast('日志已导出，导出行为已记录。')
}
function pretty(value: string) {
  try {
    return JSON.stringify(JSON.parse(value), null, 2)
  } catch {
    return value || '—'
  }
}
</script>
<template>
  <div class="page-stack">
    <PageHeading title="操作日志" description="记录成员、权限与配置变更，让每一次操作可检索、可追溯" />
    <div class="metric-grid">
      <MetricCard
        label="操作记录"
        :value="store.logs.length"
        icon="file"
        caption="当前企业预览记录"
      /><MetricCard
        label="高风险操作"
        :value="store.logs.filter((l) => l.risk === 'high').length"
        icon="shield"
        tone="orange"
        caption="身份与访问权限敏感变更"
      /><MetricCard
        label="权限变更"
        :value="store.logs.filter((l) => l.module === '角色权限').length"
        icon="key"
        tone="purple"
        caption="角色与数据范围变更"
      /><MetricCard
        label="导出记录"
        :value="store.logs.filter((l) => l.action.includes('导出')).length"
        icon="download"
        tone="green"
        caption="成员或日志文件导出"
      />
    </div>
    <section class="card data-panel">
      <div class="query-bar">
        <SearchField v-model="query" placeholder="搜索操作内容、对象名称、请求 ID…" /><select
          v-model="module"
          class="select"
          aria-label="筛选日志模块"
          @change="page = 1"
        >
          <option value="">全部模块</option>
          <option v-for="m in [...new Set(store.logs.map((l) => l.module))]" :key="m">{{ m }}</option></select
        ><select v-model="risk" class="select" aria-label="筛选风险等级" @change="page = 1">
          <option value="">全部风险</option>
          <option v-for="(label, key) in riskLabels" :key="key" :value="key">{{ label }}</option></select
        ><button
          class="btn"
          @click="
            () => {
              query = ''
              module = ''
              risk = ''
              page = 1
            }
          "
        >
          重置</button
        ><button class="btn btn-primary" @click="exportLogs">
          <AppIcon name="download" :size="16" />导出日志
        </button>
      </div>
      <div v-if="paged.length" class="table-scroll">
        <table class="data-table audit-table">
          <thead>
            <tr>
              <th>时间</th>
              <th>操作人</th>
              <th>模块</th>
              <th>操作内容</th>
              <th>结果</th>
              <th>风险等级</th>
              <th>详情</th>
            </tr>
          </thead>
          <tbody>
            <tr v-for="log in paged" :key="log.id">
              <td class="numeric muted">{{ log.time }}</td>
              <td>
                <div class="row"><AvatarMark :name="log.actor" :size="27" />{{ log.actor }}</div>
              </td>
              <td>{{ log.module }}</td>
              <td>
                <strong>{{ log.action }}</strong
                ><small>{{ log.target }}</small>
              </td>
              <td>
                <StatusBadge
                  :text="log.result === 'success' ? '成功' : '失败'"
                  :tone="log.result === 'success' ? 'success' : 'danger'"
                />
              </td>
              <td>
                <StatusBadge
                  :text="riskLabels[log.risk]"
                  :tone="log.risk === 'high' ? 'danger' : log.risk === 'medium' ? 'warning' : 'primary'"
                  :dot="false"
                />
              </td>
              <td>
                <button class="btn-link" :aria-label="'查看日志 ' + log.action" @click="selected = log">
                  查看
                </button>
              </td>
            </tr>
          </tbody>
        </table>
      </div>
      <EmptyState v-else /><AppPagination
        v-model:page="page"
        v-model:page-size="pageSize"
        :total="filtered.length"
      />
    </section>
    <div class="notice-box">
      <AppIcon
        name="shield"
      />日志页面只提供查询与导出，不提供修改或删除。当前记录存储于本地预览，不等同于生产审计留存或防篡改证明。
    </div>
    <UiDialog :open="Boolean(selected)" title="日志详情" width="580px" drawer @close="selected = null"
      ><div v-if="selected" class="page-stack">
        <div class="log-hero">
          <span><AppIcon name="file" :size="25" /></span>
          <div>
            <h2>{{ selected.action }}</h2>
            <p>{{ selected.time }}</p>
          </div>
          <StatusBadge
            :text="riskLabels[selected.risk]"
            :tone="selected.risk === 'high' ? 'danger' : 'warning'"
          />
        </div>
        <h3>请求摘要</h3>
        <dl class="detail-list">
          <dt>操作人</dt>
          <dd>{{ selected.actor }}</dd>
          <dt>所属模块</dt>
          <dd>{{ selected.module }}</dd>
          <dt>操作对象</dt>
          <dd>{{ selected.target }}</dd>
          <dt>执行结果</dt>
          <dd><StatusBadge :text="selected.result === 'success' ? '成功' : '失败'" /></dd>
          <dt>操作来源</dt>
          <dd>Web 界面预览 · 本地操作</dd>
          <dt>请求标识</dt>
          <dd class="mono">{{ selected.requestId }}</dd>
          <dt>操作原因</dt>
          <dd>{{ selected.reason || '未填写' }}</dd>
        </dl>
        <div class="divider" />
        <h3>变更内容</h3>
        <div class="diff-block">
          <h4>变更前</h4>
          <pre>{{ pretty(selected.before) }}</pre>
        </div>
        <div class="diff-block after">
          <h4>变更后</h4>
          <pre>{{ pretty(selected.after) }}</pre>
        </div>
      </div></UiDialog
    >
  </div>
</template>
<style scoped>
.audit-table {
  min-width: 960px;
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
}
.log-hero p {
  font-size: 12px;
  color: var(--color-text-muted);
  margin-top: 6px;
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
</style>
