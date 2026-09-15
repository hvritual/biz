<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { useRoute } from 'vue-router'
import { UiButton, UiInput, UiOption, UiSelect } from '@/ui/base'
import AppIcon from '@/ui/common/AppIcon.vue'
import MetricCard from '@/ui/common/MetricCard.vue'
import PageHeading from '@/ui/common/PageHeading.vue'
import StatusBadge from '@/ui/common/StatusBadge.vue'
import { platformLifecyclePages, type LifecycleRow } from '@/features/platform/platformLifecycle'

const route = useRoute()
const query = ref('')
const status = ref('all')
const selectedId = ref('')
const showAction = ref(false)

const config = computed(() => {
  const key = String(route.meta.lifecycleKey ?? 'features')
  return platformLifecyclePages[key] ?? platformLifecyclePages.features
})

const statuses = computed(() => Array.from(new Set(config.value.rows.map((row) => row.status))))
const filteredRows = computed(() => {
  const keyword = query.value.trim().toLowerCase()
  return config.value.rows.filter((row) => {
    const matchesStatus = status.value === 'all' || row.status === status.value
    if (!matchesStatus) return false
    if (!keyword) return true
    return [row.name, row.status, row.summary, ...row.cells].join(' ').toLowerCase().includes(keyword)
  })
})
const selectedRow = computed<LifecycleRow | undefined>(() =>
  config.value.rows.find((row) => row.id === selectedId.value) ?? filteredRows.value[0] ?? config.value.rows[0],
)

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

function resetFilters() {
  query.value = ''
  status.value = 'all'
}
</script>

<template>
  <div class="page-stack lifecycle-page" data-ui-template="WorkbenchPage" :data-lifecycle-page="config.key">
    <div data-ui-region="page-heading">
      <PageHeading
        :title="config.title"
        breadcrumb="平台管理 / 租户功能生命周期"
        :description="config.description"
      >
        <UiButton class="desktop-primary" @click="showAction = !showAction">
          <AppIcon name="plus" :size="16" />{{ config.primaryAction }}
        </UiButton>
      </PageHeading>
    </div>

    <section class="authority-banner card" data-ui-region="preview" aria-label="服务端接入边界">
      <div class="authority-icon"><AppIcon name="shield" :size="22" /></div>
      <div class="authority-copy">
        <div class="authority-title">
          <strong>平台控制面 · 界面已实现</strong>
          <StatusBadge text="服务端契约待接入" tone="warning" />
        </div>
        <p>{{ config.authority }}</p>
      </div>
      <UiButton variant="outline" class="mobile-primary" @click="showAction = !showAction">
        <AppIcon name="plus" :size="15" />{{ config.primaryAction }}
      </UiButton>
    </section>

    <section v-if="showAction" class="action-preview card" aria-label="操作流程预览">
      <div>
        <span class="eyebrow">操作流程预览</span>
        <h2>{{ config.primaryAction }}</h2>
        <p>该操作入口已经纳入页面信息架构。正式提交动作将在服务端契约接入后启用，当前不会向后端写入任何虚构商业事实。</p>
      </div>
      <div class="action-steps">
        <span>填写业务信息</span><AppIcon name="right" :size="15" />
        <span>校验依赖与权限</span><AppIcon name="right" :size="15" />
        <span>服务端确认</span><AppIcon name="right" :size="15" />
        <span>读回结果</span>
      </div>
      <UiButton variant="secondary" @click="showAction = false">关闭预览</UiButton>
    </section>

    <section class="metric-grid" data-ui-region="metrics" aria-label="生命周期指标">
      <MetricCard
        v-for="metric in config.metrics"
        :key="metric.label"
        :label="metric.label"
        :value="metric.value"
        :caption="metric.caption"
        :icon="metric.icon"
        :tone="metric.tone"
      />
    </section>

    <section class="card lifecycle-flow" data-ui-region="lifecycle" aria-label="生命周期控制链">
      <div class="section-heading">
        <div>
          <span class="eyebrow">Lifecycle</span>
          <h2>{{ config.title }}控制链</h2>
        </div>
        <span class="section-note">每个阶段都有明确 authority，不用页面状态替代服务端事实</span>
      </div>
      <div class="flow-track">
        <template v-for="(stage, index) in config.stages" :key="stage">
          <div class="flow-node">
            <span>{{ String(index + 1).padStart(2, '0') }}</span>
            <strong>{{ stage }}</strong>
          </div>
          <AppIcon v-if="index < config.stages.length - 1" name="right" :size="16" class="flow-arrow" />
        </template>
      </div>
    </section>

    <section class="card query-panel" data-ui-region="query" aria-label="筛选条件">
      <div class="query-fields">
        <label class="field">
          <span>搜索</span>
          <UiInput v-model="query" :placeholder="`搜索${config.title}名称、状态或字段`" :aria-label="`搜索${config.title}`" />
        </label>
        <label class="field">
          <span>状态</span>
          <UiSelect v-model="status" :aria-label="`${config.title}状态`">
            <UiOption value="all">全部状态</UiOption>
            <UiOption v-for="item in statuses" :key="item" :value="item">{{ item }}</UiOption>
          </UiSelect>
        </label>
      </div>
      <div class="query-actions">
        <span>共 {{ filteredRows.length }} 条预览数据</span>
        <UiButton variant="ghost" @click="resetFilters">重置</UiButton>
      </div>
    </section>

    <section class="content-grid">
      <div class="card data-panel" data-ui-region="data">
        <div class="section-heading data-heading">
          <div>
            <span class="eyebrow">Management Records</span>
            <h2>{{ config.title }}记录</h2>
          </div>
          <StatusBadge text="Preview Data" tone="neutral" />
        </div>
        <div class="table-scroll">
          <table class="lifecycle-table">
            <thead>
              <tr>
                <th>对象</th>
                <th v-for="column in config.columns" :key="column">{{ column }}</th>
                <th>操作</th>
              </tr>
            </thead>
            <tbody>
              <tr v-for="row in filteredRows" :key="row.id" :class="{ selected: selectedRow?.id === row.id }">
                <td>
                  <strong>{{ row.name }}</strong>
                  <StatusBadge :text="row.status" :tone="row.tone" />
                </td>
                <td v-for="(cell, index) in row.cells" :key="`${row.id}-${index}`">{{ cell }}</td>
                <td><UiButton variant="link" @click="selectedId = row.id">查看</UiButton></td>
              </tr>
              <tr v-if="!filteredRows.length">
                <td class="empty-cell" :colspan="config.columns.length + 2">没有符合当前条件的记录</td>
              </tr>
            </tbody>
          </table>
        </div>
      </div>

      <aside class="card detail-panel" data-ui-region="detail" aria-label="记录详情">
        <template v-if="selectedRow">
          <div class="detail-heading">
            <div>
              <span class="eyebrow">Selected Record</span>
              <h2>{{ selectedRow.name }}</h2>
            </div>
            <StatusBadge :text="selectedRow.status" :tone="selectedRow.tone" />
          </div>
          <p class="detail-summary">{{ selectedRow.summary }}</p>
          <div class="detail-facts">
            <div v-for="(column, index) in config.columns" :key="column">
              <span>{{ column }}</span>
              <strong>{{ selectedRow.cells[index] }}</strong>
            </div>
          </div>
          <div class="evidence-block">
            <span class="eyebrow">Authority / Evidence</span>
            <ul>
              <li v-for="item in selectedRow.evidence" :key="item">
                <AppIcon name="check" :size="14" />{{ item }}
              </li>
            </ul>
          </div>
          <div class="detail-warning">
            <AppIcon name="warning" :size="16" />
            <span>当前详情用于验证信息架构与交互，不作为生产商业事实。</span>
          </div>
        </template>
      </aside>
    </section>
  </div>
</template>

<style scoped>
.lifecycle-page { gap: 18px; }
.desktop-primary { align-self: flex-end; margin-bottom: 5px; }
.authority-banner {
  display: flex;
  align-items: center;
  gap: 14px;
  padding: 14px 16px;
  border-color: var(--color-border-strong);
  background: linear-gradient(110deg, var(--color-primary-soft), var(--color-surface));
}
.authority-icon {
  display: grid;
  place-items: center;
  flex: 0 0 42px;
  height: 42px;
  border-radius: 12px;
  background: var(--color-surface);
  color: var(--color-primary);
  box-shadow: var(--shadow-panel);
}
.authority-copy { flex: 1; min-width: 0; }
.authority-title { display: flex; align-items: center; gap: 10px; flex-wrap: wrap; }
.authority-copy p { margin-top: 4px; color: var(--color-text-secondary); font-size: var(--text-xs); line-height: 1.6; }
.mobile-primary { display: none; }
.action-preview { display: grid; grid-template-columns: minmax(0, 1fr) auto auto; gap: 18px; align-items: center; padding: 18px; }
.action-preview h2 { margin-top: 3px; font-size: var(--text-lg); }
.action-preview p { margin-top: 6px; max-width: 720px; color: var(--color-text-secondary); font-size: var(--text-xs); line-height: 1.65; }
.action-steps { display: flex; align-items: center; gap: 7px; color: var(--color-text-secondary); font-size: 11px; white-space: nowrap; }
.action-steps span { padding: 6px 8px; border-radius: 999px; background: var(--color-surface-soft); }
.metric-grid { display: grid; grid-template-columns: repeat(4, minmax(0, 1fr)); gap: 14px; }
.lifecycle-flow { padding: 18px; }
.section-heading { display: flex; align-items: flex-start; justify-content: space-between; gap: 18px; }
.section-heading h2 { margin-top: 3px; font-size: var(--text-lg); }
.eyebrow { color: var(--color-primary); font-size: 10px; font-weight: 700; letter-spacing: .06em; text-transform: uppercase; }
.section-note { max-width: 430px; color: var(--color-text-muted); font-size: 11px; text-align: right; }
.flow-track { display: flex; align-items: center; gap: 7px; margin-top: 16px; overflow-x: auto; padding-bottom: 2px; }
.flow-node { display: flex; align-items: center; gap: 8px; min-width: max-content; padding: 9px 11px; border: 1px solid var(--color-border); border-radius: var(--radius-md); background: var(--color-surface-soft); }
.flow-node span { display: grid; place-items: center; width: 24px; height: 24px; border-radius: 50%; background: var(--color-primary-soft); color: var(--color-primary); font-size: 10px; font-weight: 700; }
.flow-node strong { font-size: 12px; }
.flow-arrow { flex: 0 0 auto; color: var(--color-text-muted); }
.query-panel { display: flex; align-items: end; justify-content: space-between; gap: 18px; padding: 16px 18px; }
.query-fields { display: grid; grid-template-columns: minmax(260px, 1.6fr) minmax(180px, .8fr); gap: 12px; width: min(720px, 100%); }
.field { display: grid; gap: 6px; color: var(--color-text-secondary); font-size: 11px; }
.query-actions { display: flex; align-items: center; gap: 8px; color: var(--color-text-muted); font-size: 11px; }
.content-grid { display: grid; grid-template-columns: minmax(0, 1fr) 340px; gap: 16px; align-items: start; }
.data-panel, .detail-panel { min-width: 0; padding: 18px; }
.data-heading { align-items: center; margin-bottom: 14px; }
.table-scroll { overflow-x: auto; }
.lifecycle-table { width: 100%; border-collapse: collapse; min-width: 820px; }
th { padding: 9px 10px; border-bottom: 1px solid var(--color-border); color: var(--color-text-muted); font-size: 11px; font-weight: 600; text-align: left; background: var(--color-surface-soft); }
td { padding: 12px 10px; border-bottom: 1px solid var(--color-border); color: var(--color-text-secondary); font-size: 12px; vertical-align: middle; }
td:first-child { min-width: 190px; }
td:first-child strong { display: block; margin-bottom: 6px; color: var(--color-text); font-size: 12px; }
tr.selected td { background: var(--color-primary-soft); }
.empty-cell { height: 120px; text-align: center; color: var(--color-text-muted); }
.detail-panel { position: sticky; top: calc(var(--header-height) + 18px); }
.detail-heading { display: flex; align-items: flex-start; justify-content: space-between; gap: 10px; }
.detail-heading h2 { margin-top: 4px; font-size: 16px; line-height: 1.45; }
.detail-summary { margin-top: 14px; color: var(--color-text-secondary); font-size: 12px; line-height: 1.75; }
.detail-facts { display: grid; gap: 10px; margin-top: 16px; }
.detail-facts div { display: grid; gap: 3px; padding: 10px 11px; border-radius: var(--radius-sm); background: var(--color-surface-soft); }
.detail-facts span { color: var(--color-text-muted); font-size: 10px; }
.detail-facts strong { font-size: 12px; font-weight: 600; }
.evidence-block { margin-top: 18px; padding-top: 16px; border-top: 1px solid var(--color-border); }
.evidence-block ul { display: grid; gap: 9px; margin: 10px 0 0; padding: 0; list-style: none; }
.evidence-block li { display: flex; align-items: flex-start; gap: 7px; color: var(--color-text-secondary); font-size: 11px; line-height: 1.5; }
.evidence-block li :deep(.icon) { margin-top: 1px; color: var(--color-success); flex: 0 0 auto; }
.detail-warning { display: flex; align-items: flex-start; gap: 8px; margin-top: 18px; padding: 10px; border-radius: var(--radius-sm); background: var(--color-warning-soft); color: var(--color-warning); font-size: 11px; line-height: 1.5; }
@media (max-width: 1200px) {
  .metric-grid { grid-template-columns: repeat(2, minmax(0, 1fr)); }
  .content-grid { grid-template-columns: minmax(0, 1fr) 300px; }
  .action-preview { grid-template-columns: 1fr auto; }
  .action-steps { grid-column: 1 / -1; flex-wrap: wrap; white-space: normal; }
}
@media (max-width: 900px) {
  .content-grid { grid-template-columns: 1fr; }
  .detail-panel { position: static; }
  .query-panel { align-items: stretch; flex-direction: column; }
  .query-actions { justify-content: space-between; }
  .section-note { display: none; }
}
@media (max-width: 600px) {
  .desktop-primary { display: none; }
  .mobile-primary { display: inline-flex; width: 100%; }
  .authority-banner { align-items: flex-start; flex-wrap: wrap; }
  .authority-copy { min-width: calc(100% - 58px); }
  .metric-grid { grid-template-columns: 1fr 1fr; gap: 10px; }
  .query-fields { grid-template-columns: 1fr; }
  .query-actions { flex-wrap: wrap; }
  .action-preview { grid-template-columns: 1fr; }
  .flow-track { margin-right: -18px; padding-right: 18px; }
  .data-panel, .detail-panel, .lifecycle-flow { padding: 14px; }
}
</style>
