<script setup lang="ts">
import { UiButton, UiOption, UiSelect, UiTextarea } from '@/ui/base'
import { computed, ref, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { useEnterpriseStore } from '@/stores/enterprise'
import { useEnterprisePlanStore } from '@/stores/enterprisePlan'
import { useUiStore } from '@/stores/ui'
import PageHeading from '@/ui/common/PageHeading.vue'
import AppIcon from '@/ui/common/AppIcon.vue'
import StatusBadge from '@/ui/common/StatusBadge.vue'
import UiDialog from '@/ui/common/UiDialog.vue'
import EnterpriseSourceBanner from '@/features/enterprise/components/EnterpriseSourceBanner.vue'
import PlanChangeLifecycle from '@/features/enterprise/components/PlanChangeLifecycle.vue'

const store = useEnterpriseStore()
const plan = useEnterprisePlanStore()
const ui = useUiStore()
const route = useRoute()
const router = useRouter()

const requestOpen = ref(false)
const lifecycleOpen = ref(false)
const targetPlan = ref('企业版')
const note = ref('')
const tab = ref('套餐概览')

const enabledCount = computed(() => plan.features.filter((feature) => feature.enabled).length)
const quotaCards = computed(() => plan.quotas.slice(0, 4))

watch(
  () => route.query.action,
  (action) => {
    if (action === 'upgrade') {
      if (plan.isServerBacked) lifecycleOpen.value = true
      else requestOpen.value = true
      void router.replace({ path: route.path, query: {} })
    }
  },
  { immediate: true },
)

function formatDate(value: string) {
  if (!value) return '—'
  const date = new Date(value)
  if (Number.isNaN(date.getTime())) return value
  return new Intl.DateTimeFormat('zh-CN', { year: 'numeric', month: '2-digit', day: '2-digit' }).format(date)
}

function quotaUsedLabel(used: number | null, unit: string) {
  return used == null ? '未知' : `${used}${unit ? ` ${unit}` : ''}`
}

function quotaTotalLabel(total: number | null, unlimited: boolean, unit: string) {
  if (unlimited) return '无限'
  return total == null ? '未声明' : `${total}${unit ? ` ${unit}` : ''}`
}

function quotaPercent(used: number | null, total: number | null) {
  if (used == null || total == null || total <= 0) return 0
  return Math.min(100, (used / total) * 100)
}

function openChange() {
  if (plan.isServerBacked) lifecycleOpen.value = true
  else requestOpen.value = true
}

function submitDemoChange() {
  try {
    plan.recordDemoChange(targetPlan.value, note.value)
    requestOpen.value = false
    ui.toast('已记录预览申请；未变更真实套餐，也不会发起扣费。', 'info')
  } catch (error) {
    ui.toast(error instanceof Error ? error.message : '套餐调整申请失败。', 'error')
  }
}
</script>

<template>
  <div class="page-stack" data-enterprise-page="plan" data-ui-template="WorkbenchPage">
    <PageHeading title="套餐信息" description="查看当前订阅、可用功能与使用额度，让业务增长有据可依" />
    <EnterpriseSourceBanner />

    <div v-if="plan.loading && !plan.model" class="card state-card" role="status">正在读取当前租户套餐、权益与用量…</div>
    <div v-else-if="plan.error && plan.isServerBacked && !plan.model" class="card state-card error-state" role="alert">
      <div>
        <strong>无法读取套餐额度</strong>
        <p>{{ plan.error }}</p>
      </div>
      <UiButton class="btn" @click="plan.load">重新读取</UiButton>
    </div>

    <template v-else>
      <div class="plan-top">
        <section class="card current-plan" data-ui-region="subscription">
          <div class="row">
            <span class="plan-crown"><AppIcon name="crown" :size="30" /></span>
            <div>
              <small class="muted">当前套餐</small>
              <h2>{{ plan.currentPlan }} <StatusBadge :text="plan.subscriptionState" /></h2>
            </div>
          </div>
          <p>{{ plan.isServerBacked ? '当前订阅、功能权益与额度来自服务端权威读模型。' : '适用于多点位运营团队，统一管理设备、成员与服务。' }}</p>
          <div class="plan-dates">
            <div><span>生效日期</span><strong>{{ formatDate(plan.periodStart) }}</strong></div>
            <div><span>到期日期</span><strong>{{ formatDate(plan.periodEnd) }}</strong></div>
            <div><span>订阅周期</span><strong>{{ plan.cycle }}</strong></div>
          </div>
          <div class="row">
            <UiButton class="btn btn-primary" @click="openChange">
              <AppIcon name="crown" :size="16" />{{ plan.serverChangeContext ? '管理套餐变更' : '申请升级套餐' }}
            </UiButton>
            <UiButton class="btn" @click="openChange">{{ plan.serverChangeContext ? '查看变更生命周期' : '申请调整额度' }}</UiButton>
          </div>
          <small class="preview-plan">
            {{ plan.isServerBacked ? '真实变更必须经过 preview / confirm / receipt 权威链路，不由页面自行认定付款或生效。' : '当前为界面演示数据，不代表真实订阅。' }}
          </small>
        </section>

        <section class="card quota-overview" data-ui-region="quota-summary">
          <div class="row-between">
            <h2>额度使用概览</h2>
            <span class="muted">当前企业</span>
          </div>
          <div class="quota-cards">
            <div v-for="quota in quotaCards" :key="quota.key" class="quota-item">
              <div class="row-between">
                <span class="row"><AppIcon :name="quota.icon" :size="17" />{{ quota.label }}</span>
                <span class="muted">{{ quota.status }}</span>
              </div>
              <strong class="numeric">
                {{ quotaUsedLabel(quota.used, quota.unit) }}
                <small>/ {{ quotaTotalLabel(quota.total, quota.unlimited, quota.unit) }}</small>
              </strong>
              <div class="progress-track">
                <div class="progress-fill" :style="{ width: quotaPercent(quota.used, quota.total) + '%' }" />
              </div>
            </div>
            <div v-if="quotaCards.length === 0" class="muted">当前套餐未返回可展示的额度项。</div>
          </div>
        </section>
      </div>

      <PlanChangeLifecycle
        v-if="lifecycleOpen && plan.serverChangeContext"
        :session="plan.serverChangeContext.session"
        :subscription="plan.serverChangeContext.subscription"
        @changed="plan.refreshAfterChange"
      />

      <section class="card panel-pad" data-ui-region="workspace">
        <div class="tabs">
          <UiButton v-for="item in ['套餐概览', '功能权益', '使用额度', '变更记录']" :key="item" :class="['tab', { active: tab === item }]" @click="tab = item">
            {{ item }}
          </UiButton>
        </div>

        <template v-if="tab === '套餐概览' || tab === '功能权益'">
          <div class="row-between plan-section-heading">
            <h2>套餐权益</h2>
            <span class="muted">已开通 {{ enabledCount }} / {{ plan.features.length }} 项</span>
          </div>
          <div class="feature-grid">
            <div v-for="feature in plan.features" :key="feature.key" class="feature-row">
              <span class="feature-icon"><AppIcon :name="feature.icon" :size="22" /></span>
              <div class="flex-1">
                <h3>{{ feature.label }}</h3>
                <p>{{ feature.description }}</p>
              </div>
              <StatusBadge :text="feature.enabled ? '已开通' : '未开通'" :tone="feature.enabled ? 'success' : 'warning'" :dot="false" />
            </div>
            <div v-if="plan.features.length === 0" class="muted">当前服务端未返回模块级权益。</div>
          </div>
        </template>

        <template v-else-if="tab === '使用额度'">
          <div v-if="plan.error && plan.model" class="notice-box quota-warning">
            <AppIcon name="help" />{{ plan.error }}；未取得权威 meter 的额度保持“未知”，不会显示为 0。
          </div>
          <div class="table-scroll quota-table">
            <table class="data-table">
              <thead>
                <tr><th>资源类型</th><th>已使用</th><th>总额度</th><th>使用率</th><th>状态</th></tr>
              </thead>
              <tbody>
                <tr v-for="quota in plan.quotas" :key="quota.key">
                  <td>{{ quota.label }}</td>
                  <td>{{ quotaUsedLabel(quota.used, quota.unit) }}</td>
                  <td>{{ quotaTotalLabel(quota.total, quota.unlimited, quota.unit) }}</td>
                  <td>{{ quota.used != null && quota.total != null ? quotaPercent(quota.used, quota.total).toFixed(1) + '%' : '—' }}</td>
                  <td><StatusBadge :text="quota.status" :tone="quota.status === '正常' || quota.status === '无限额度' ? 'success' : 'warning'" /></td>
                </tr>
                <tr v-if="plan.quotas.length === 0"><td colspan="5" class="muted">当前套餐未返回额度项。</td></tr>
              </tbody>
            </table>
          </div>
        </template>

        <template v-else>
          <div class="timeline plan-timeline">
            <template v-if="plan.isServerBacked && plan.model">
              <div class="timeline-item">
                <strong>当前订阅修订 r{{ plan.model.subscription.revision }}</strong>
                <p>{{ plan.model.subscription.planCode }} · {{ plan.model.subscription.state }}</p>
                <small>{{ plan.model.subscription.updatedAt || plan.model.subscription.createdAt || '服务端记录' }}</small>
              </div>
              <div v-if="plan.model.subscription.pendingChangeId" class="timeline-item">
                <strong>存在待处理套餐变更</strong>
                <p>{{ plan.model.subscription.pendingChangeId }}</p>
              </div>
            </template>
            <template v-else>
              <div v-for="log in store.logs.filter((item) => item.module === '套餐信息')" :key="log.id" class="timeline-item">
                <strong>{{ log.action }}</strong>
                <p>{{ log.target }} · {{ log.after }}</p>
                <small>{{ log.time }}</small>
              </div>
              <div class="timeline-item"><strong>标准版预览套餐初始化</strong><small>2026-09-08 · 示例数据</small></div>
            </template>
          </div>
        </template>
      </section>
    </template>

    <UiDialog :open="requestOpen" title="申请套餐调整" @close="requestOpen = false">
      <div class="page-stack">
        <div class="notice-box"><AppIcon name="help" />此操作仅用于 demo 演示，不会购买服务、变更实际订阅或扣费。</div>
        <label class="field"><span>意向套餐</span><UiSelect v-model="targetPlan" class="select"><UiOption>企业版</UiOption><UiOption>标准版扩容</UiOption><UiOption>联系商务定制</UiOption></UiSelect></label>
        <label class="field"><span>需求说明</span><UiTextarea v-model="note" class="textarea" maxlength="500" placeholder="描述所需成员、点位、设备或功能额度" /></label>
      </div>
      <template #footer><UiButton class="btn" @click="requestOpen = false">取消</UiButton><UiButton class="btn btn-primary" @click="submitDemoChange">记录申请</UiButton></template>
    </UiDialog>
  </div>
</template>

<style scoped>
.state-card { min-height: 120px; padding: 24px; display: flex; align-items: center; justify-content: space-between; gap: 16px; }
.error-state strong { color: var(--color-danger); }
.error-state p { margin-top: 6px; color: var(--color-text-secondary); }
.plan-top { display: grid; grid-template-columns: 1fr 1.05fr; gap: 16px; }
.current-plan { padding: 28px; background: linear-gradient(125deg, var(--color-primary-soft), var(--color-surface) 68%); }
.plan-crown { width: 62px; height: 62px; display: grid; place-items: center; background: linear-gradient(130deg, var(--color-gradient-end), var(--color-primary)); color: var(--color-on-primary); border-radius: 50%; }
.current-plan h2 { font-size: 24px; margin-top: 4px; display: flex; align-items: center; gap: 12px; overflow-wrap: anywhere; }
.current-plan > p { font-size: 13px; color: var(--color-text-secondary); margin-top: 18px; }
.plan-dates { display: grid; grid-template-columns: repeat(3, 1fr); gap: 12px; margin: 25px 0; }
.plan-dates span { display: block; color: var(--color-text-muted); font-size: 12px; margin-bottom: 7px; }
.plan-dates strong { font-size: 14px; font-weight: 500; }
.preview-plan { display: block; color: var(--color-text-muted); font-size: 10px; margin-top: 18px; }
.quota-overview { padding: 28px; }
.quota-overview > .row-between > span { font-size: 12px; }
.quota-cards { display: grid; grid-template-columns: 1fr 1fr; gap: 27px 24px; margin-top: 26px; }
.quota-item .row-between { font-size: 12px; }
.quota-item .icon { color: var(--color-primary); }
.quota-item strong { display: block; font-size: 22px; margin: 11px 0 12px; }
.quota-item strong small { font-size: 12px; color: var(--color-text-muted); font-weight: 400; }
.plan-section-heading { margin: 25px 0 20px; }
.plan-section-heading > span { font-size: 12px; }
.feature-grid { display: grid; grid-template-columns: 1fr 1fr; gap: 13px 20px; }
.feature-row { display: flex; align-items: center; gap: 14px; border: 1px solid var(--color-border); padding: 17px; border-radius: 9px; }
.feature-row h3 { font-size: 14px; }
.feature-row p { font-size: 12px; color: var(--color-text-muted); margin-top: 5px; }
.feature-icon { height: 40px; width: 40px; border-radius: 12px; display: grid; place-items: center; flex-shrink: 0; background: var(--color-primary-soft); color: var(--color-primary); }
.quota-table { margin-top: 24px; }
.quota-warning { margin-top: 20px; }
.plan-timeline { margin-top: 28px; }
@media (max-width: 1100px) { .plan-top { grid-template-columns: 1fr; } }
@media (max-width: 767px) {
  .feature-grid { grid-template-columns: 1fr; }
  .current-plan, .quota-overview { padding: 20px; }
  .current-plan > .row { flex-wrap: wrap; }
  .plan-dates { grid-template-columns: 1fr; }
  .feature-row p { font-size: 11px; }
  .quota-cards { grid-template-columns: 1fr; }
}
</style>
