<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { useRoute } from 'vue-router'
import { UiButton, UiInput, UiOption, UiSelect } from '@/ui/base'
import AppIcon from '@/ui/common/AppIcon.vue'
import PageHeading from '@/ui/common/PageHeading.vue'
import StatusBadge from '@/ui/common/StatusBadge.vue'
import { platformLifecyclePages, type LifecyclePageConfig, type LifecycleRow } from '@/features/platform/platformLifecycle'

const route = useRoute()
const query = ref('')
const status = ref('all')
const selectedId = ref('')
const showAction = ref(false)
const fallbackConfig = platformLifecyclePages.features as LifecyclePageConfig
const config = computed<LifecyclePageConfig>(() => platformLifecyclePages[String(route.meta.lifecycleKey ?? 'features')] ?? fallbackConfig)
const statuses = computed(() => Array.from(new Set(config.value.rows.map((row) => row.status))))
const filteredRows = computed(() => {
  const keyword = query.value.trim().toLowerCase()
  return config.value.rows.filter((row) => {
    const matchesStatus = status.value === 'all' || row.status === status.value
    const haystack = [row.name, row.status, row.summary, ...row.cells].join(' ').toLowerCase()
    return matchesStatus && (!keyword || haystack.includes(keyword))
  })
})
const selectedRow = computed<LifecycleRow | undefined>(() =>
  config.value.rows.find((row) => row.id === selectedId.value) ?? filteredRows.value[0] ?? config.value.rows[0],
)

const valueLabels: Record<string, string> = {
  ACTIVE: '已生效',
  DRAFT: '草稿',
  DEPRECATED: '已停售',
  EXPIRING: '即将到期',
  GRACE_PERIOD: '宽限期',
  TRIAL: '试用',
  ALLOW: '允许',
  DENY: '拒绝',
  AVAILABLE: '可用',
  NEAR_LIMIT: '接近上限',
  EXHAUSTED: '已耗尽',
  PENDING: '待处理',
  SCHEDULED: '已排期',
  APPLIED: '已生效',
  REVOKED: '已撤销',
  EXPIRED: '已过期',
  SUCCESS: '成功',
  FAILED: '失败',
}

function displayCell(value: string) {
  return valueLabels[value] ?? value
}
function resetFilters() {
  query.value = ''
  status.value = 'all'
}

watch(
  () => config.value.key,
  () => {
    query.value = ''
    status.value = 'all'
    selectedId.value = config.value.rows[0]?.id ?? ''
    showAction.value = false
  },
  { immediate: true },
)
</script>

<template>
  <div class="page-stack lifecycle-page" data-ui-template="WorkbenchPage" :data-lifecycle-page="config.key">
    <div data-ui-region="page-heading">
      <PageHeading
        :title="config.title"
        breadcrumb="平台管理 / 租户功能生命周期"
        :description="config.description"
      />
    </div>

    <section class="integration-note" data-ui-region="preview" aria-label="服务端接入边界">
      <span class="integration-icon"><AppIcon name="shield" :size="19" /></span>
      <div class="integration-copy">
        <div><strong>平台控制面</strong><StatusBadge text="服务端契约待接入" tone="warning" /></div>
        <p>{{ config.authority }}</p>
      </div>
      <span class="preview-label">只读设计预览</span>
    </section>

    <section class="card overview-panel" aria-label="生命周期概览">
      <div class="metrics-strip" data-ui-region="metrics">
        <article v-for="metric in config.metrics" :key="metric.label" class="metric-item">
          <span :class="['metric-icon', metric.tone || 'blue']"><AppIcon :name="metric.icon" :size="21" /></span>
          <div>
            <span>{{ metric.label }}</span>
            <strong class="numeric">{{ metric.value }}</strong>
            <small>{{ metric.caption }}</small>
          </div>
        </article>
      </div>

      <div class="lifecycle-rail" data-ui-region="lifecycle">
        <div class="lifecycle-copy">
          <div><span>生命周期</span><strong>{{ config.title }}控制链</strong></div>
          <small>每个阶段由服务端事实驱动，页面不以展示状态替代真实结果。</small>
        </div>
        <div class="flow-track">
          <template v-for="(stage, index) in config.stages" :key="stage">
            <div class="flow-node"><span>{{ index + 1 }}</span><strong>{{ stage }}</strong></div>
            <AppIcon v-if="index < config.stages.length - 1" name="right" :size="15" class="flow-arrow" />
          </template>
        </div>
      </div>
    </section>

    <section v-if="showAction" class="card action-preview" aria-label="操作流程预览">
      <div>
        <span class="section-kicker">操作流程</span>
        <h2>{{ config.primaryAction }}</h2>
        <p>正式提交动作将在服务端契约接入后启用；当前页面只展示流程，不写入商业事实。</p>
      </div>
      <div class="action-steps">
        <span>填写业务信息</span><AppIcon name="right" :size="14" />
        <span>校验权限与依赖</span><AppIcon name="right" :size="14" />
        <span>服务端确认</span><AppIcon name="right" :size="14" />
        <span>结果读回</span>
      </div>
      <UiButton variant="secondary" @click="showAction = false">关闭预览</UiButton>
    </section>

    <section class="card query-panel" data-ui-region="query" aria-label="筛选条件">
      <label class="field search-field">
        <span>搜索</span>
        <UiInput v-model="query" :placeholder="`搜索${config.title}名称、状态或字段`" :aria-label="`搜索${config.title}`" />
      </label>
      <label class="field status-field">
        <span>状态</span>
        <UiSelect v-model="status" :aria-label="`${config.title}状态`">
          <UiOption value="all">全部状态</UiOption>
          <UiOption v-for="item in statuses" :key="item" :value="item">{{ item }}</UiOption>
        </UiSelect>
      </label>
      <div class="query-actions">
        <span>{{ filteredRows.length }} 条结果</span>
        <UiButton variant="ghost" @click="resetFilters">重置</UiButton>
      </div>
    </section>

    <section class="card workspace-panel">
      <div class="data-pane" data-ui-region="data">
        <div class="data-toolbar">
          <div>
            <span class="section-kicker">管理记录</span>
            <h2>{{ config.title }}记录</h2>
            <p>按当前筛选条件展示管理对象；预览数据不会被解释为生产事实。</p>
          </div>
          <div class="toolbar-actions">
            <StatusBadge text="预览数据" tone="neutral" />
            <UiButton @click="showAction = !showAction"><AppIcon name="plus" :size="15" />{{ config.primaryAction }}</UiButton>
          </div>
        </div>

        <div class="table-scroll">
          <table class="lifecycle-table">
            <thead>
              <tr><th>对象</th><th v-for="column in config.columns" :key="column">{{ column }}</th><th>操作</th></tr>
            </thead>
            <tbody>
              <tr v-for="row in filteredRows" :key="row.id" :class="{ selected: selectedRow?.id === row.id }">
                <td><strong>{{ row.name }}</strong><StatusBadge :text="row.status" :tone="row.tone" /></td>
                <td v-for="(cell, index) in row.cells" :key="`${row.id}-${index}`">{{ displayCell(cell) }}</td>
                <td><UiButton variant="link" @click="selectedId = row.id">查看</UiButton></td>
              </tr>
              <tr v-if="!filteredRows.length"><td class="empty-cell" :colspan="config.columns.length + 2">没有符合当前条件的记录</td></tr>
            </tbody>
          </table>
        </div>
      </div>

      <aside class="detail-pane" data-ui-region="detail" aria-label="记录详情">
        <template v-if="selectedRow">
          <div class="detail-heading">
            <div><span class="section-kicker">当前记录</span><h2>{{ selectedRow.name }}</h2></div>
            <StatusBadge :text="selectedRow.status" :tone="selectedRow.tone" />
          </div>
          <p class="detail-summary">{{ selectedRow.summary }}</p>
          <dl class="detail-facts">
            <div v-for="(column, index) in config.columns" :key="column"><dt>{{ column }}</dt><dd>{{ displayCell(selectedRow.cells[index] || '—') }}</dd></div>
          </dl>
          <div class="evidence-block">
            <span class="section-kicker">依据与边界</span>
            <ul><li v-for="item in selectedRow.evidence" :key="item"><AppIcon name="check" :size="14" />{{ item }}</li></ul>
          </div>
          <div class="detail-warning"><AppIcon name="warning" :size="16" /><span>当前详情用于验证信息架构与交互，不作为生产商业事实。</span></div>
        </template>
      </aside>
    </section>
  </div>
</template>

<style scoped>
.lifecycle-page { gap: 14px; }
.integration-note { display: flex; align-items: center; gap: 12px; min-height: 58px; padding: 10px 14px; border: 1px solid var(--color-border); border-left: 3px solid var(--color-primary); border-radius: var(--radius-md); background: var(--color-surface); }
.integration-icon { display: grid; place-items: center; flex: 0 0 34px; height: 34px; border-radius: var(--radius-sm); background: var(--color-primary-soft); color: var(--color-primary); }
.integration-copy { flex: 1; min-width: 0; }
.integration-copy > div { display: flex; align-items: center; gap: 9px; flex-wrap: wrap; }
.integration-copy p { margin-top: 3px; color: var(--color-text-secondary); font-size: var(--text-xs); line-height: 1.55; }
.preview-label { color: var(--color-text-muted); font-size: var(--text-xs); white-space: nowrap; }
.overview-panel { overflow: hidden; }
.metrics-strip { display: grid; grid-template-columns: repeat(4, minmax(0, 1fr)); }
.metric-item { display: flex; align-items: flex-start; gap: 12px; min-width: 0; padding: 17px 18px; border-right: 1px solid var(--color-border); }
.metric-item:last-child { border-right: 0; }
.metric-icon { display: grid; place-items: center; flex: 0 0 38px; height: 38px; border-radius: 10px; background: var(--color-primary-soft); color: var(--color-primary); }
.metric-icon.green { background: var(--color-success-soft); color: var(--color-success); }
.metric-icon.orange { background: var(--color-warning-soft); color: var(--color-warning); }
.metric-icon.purple { background: var(--color-violet-soft); color: var(--color-violet); }
.metric-item div { min-width: 0; }
.metric-item div > span { display: block; color: var(--color-text-secondary); font-size: var(--text-xs); }
.metric-item strong { display: block; margin-top: 1px; font-size: 30px; line-height: 1.25; letter-spacing: -0.4px; }
.metric-item small { display: block; margin-top: 2px; color: var(--color-text-muted); line-height: 1.45; }
.lifecycle-rail { display: grid; grid-template-columns: minmax(210px, .7fr) minmax(0, 1.7fr); gap: 20px; align-items: center; padding: 14px 18px; border-top: 1px solid var(--color-border); background: var(--color-surface-soft); }
.lifecycle-copy > div { display: flex; align-items: baseline; gap: 8px; }
.lifecycle-copy span, .section-kicker { color: var(--color-primary); font-size: 10px; font-weight: 700; letter-spacing: .05em; }
.lifecycle-copy strong { font-size: var(--text-sm); }
.lifecycle-copy small { display: block; margin-top: 4px; color: var(--color-text-muted); line-height: 1.45; }
.flow-track { display: flex; align-items: center; gap: 6px; min-width: 0; overflow-x: auto; padding-bottom: 2px; }
.flow-node { display: inline-flex; align-items: center; gap: 7px; min-width: max-content; padding: 7px 9px; border: 1px solid var(--color-border); border-radius: var(--radius-sm); background: var(--color-surface); }
.flow-node span { display: grid; place-items: center; width: 20px; height: 20px; border-radius: 50%; background: var(--color-primary-soft); color: var(--color-primary); font-size: 10px; font-weight: 700; }
.flow-node strong { font-size: var(--text-xs); }
.flow-arrow { flex: 0 0 auto; color: var(--color-text-muted); }
.action-preview { display: grid; grid-template-columns: minmax(0, 1fr) auto auto; gap: 16px; align-items: center; padding: 16px 18px; }
.action-preview h2 { margin-top: 3px; }
.action-preview p { margin-top: 5px; color: var(--color-text-secondary); font-size: var(--text-xs); line-height: 1.55; }
.action-steps { display: flex; align-items: center; gap: 6px; color: var(--color-text-secondary); font-size: 11px; white-space: nowrap; }
.action-steps span { padding: 5px 7px; border-radius: 999px; background: var(--color-surface-soft); }
.query-panel { display: grid; grid-template-columns: minmax(260px, 1fr) 190px auto; gap: 12px; align-items: end; padding: 14px 16px; }
.field { gap: 6px; }
.field > span { font-size: var(--text-xs); }
.query-actions { display: flex; align-items: center; justify-content: flex-end; gap: 8px; min-height: var(--control-height); color: var(--color-text-muted); font-size: var(--text-xs); }
.workspace-panel { display: grid; grid-template-columns: minmax(0, 1fr) 320px; min-width: 0; overflow: hidden; box-shadow: var(--shadow-panel) !important; }
.data-pane { min-width: 0; padding: 18px; }
.data-toolbar { display: flex; align-items: flex-start; justify-content: space-between; gap: 18px; margin-bottom: 14px; }
.data-toolbar h2, .detail-heading h2 { margin-top: 3px; font-size: var(--text-lg); }
.data-toolbar p { margin-top: 4px; color: var(--color-text-muted); font-size: var(--text-xs); }
.toolbar-actions { display: flex; align-items: center; gap: 9px; flex: 0 0 auto; }
.table-scroll { width: 100%; min-width: 0; max-width: 100%; overflow-x: auto; }
.lifecycle-table { width: 100%; min-width: 790px; border-collapse: collapse; }
th { height: 40px; padding: 0 10px; border-bottom: 1px solid var(--color-border); background: var(--color-surface-soft); color: var(--color-text-secondary); font-size: var(--text-xs); font-weight: 500; text-align: left; }
td { padding: 11px 10px; border-bottom: 1px solid var(--color-border); color: var(--color-text-secondary); font-size: var(--text-xs); vertical-align: middle; }
td:first-child { min-width: 188px; }
td:first-child strong { display: block; margin-bottom: 5px; color: var(--color-text); font-size: var(--text-sm); }
tr.selected td { background: var(--color-primary-soft); }
.empty-cell { height: 128px; text-align: center; color: var(--color-text-muted); }
.detail-pane { min-width: 0; padding: 18px; border-left: 1px solid var(--color-border); background: var(--color-surface-soft); }
.detail-heading { display: flex; align-items: flex-start; justify-content: space-between; gap: 10px; }
.detail-summary { margin-top: 12px; color: var(--color-text-secondary); font-size: var(--text-xs); line-height: 1.7; }
.detail-facts { display: grid; gap: 0; margin: 14px 0 0; }
.detail-facts div { display: grid; grid-template-columns: 88px minmax(0, 1fr); gap: 8px; padding: 9px 0; border-bottom: 1px solid var(--color-border); }
.detail-facts dt { color: var(--color-text-muted); font-size: 11px; }
.detail-facts dd { margin: 0; color: var(--color-text); font-size: var(--text-xs); font-weight: 600; }
.evidence-block { margin-top: 16px; }
.evidence-block ul { display: grid; gap: 7px; margin: 9px 0 0; padding: 0; list-style: none; }
.evidence-block li { display: flex; align-items: flex-start; gap: 7px; color: var(--color-text-secondary); font-size: 11px; line-height: 1.45; }
.detail-warning { display: flex; align-items: flex-start; gap: 7px; margin-top: 16px; padding: 9px 10px; border-radius: var(--radius-sm); background: var(--color-warning-soft); color: var(--color-warning); font-size: 11px; line-height: 1.45; }
@media (max-width: 1200px) { .metrics-strip { grid-template-columns: repeat(2, minmax(0, 1fr)); } .metric-item:nth-child(2) { border-right: 0; } .metric-item:nth-child(-n + 2) { border-bottom: 1px solid var(--color-border); } .lifecycle-rail { grid-template-columns: 1fr; gap: 10px; } .workspace-panel { grid-template-columns: minmax(0, 1fr) 290px; } .action-preview { grid-template-columns: 1fr auto; } .action-steps { grid-column: 1 / -1; flex-wrap: wrap; white-space: normal; } }
@media (max-width: 900px) { .workspace-panel { grid-template-columns: minmax(0, 1fr); } .detail-pane { border-left: 0; border-top: 1px solid var(--color-border); } .query-panel { grid-template-columns: minmax(0, 1fr) 180px; } .query-actions { grid-column: 1 / -1; justify-content: space-between; } }
@media (max-width: 600px) { .integration-note { align-items: flex-start; flex-wrap: wrap; } .integration-copy { min-width: calc(100% - 48px); } .preview-label { width: 100%; padding-left: 46px; } .metrics-strip { grid-template-columns: 1fr 1fr; } .metric-item { padding: 14px 12px; } .metric-icon { display: none; } .metric-item strong { font-size: 27px; } .query-panel { grid-template-columns: 1fr; } .query-actions { grid-column: auto; } .data-toolbar { align-items: stretch; flex-direction: column; } .toolbar-actions { justify-content: space-between; } .toolbar-actions > :last-child { flex: 1; } .action-preview { grid-template-columns: 1fr; } .data-pane, .detail-pane { padding: 14px; } .lifecycle-rail { padding: 13px 14px; } }
</style>
