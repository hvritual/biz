<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { useEnterpriseStore } from '@/stores/enterprise'
import { useUiStore } from '@/stores/ui'
import PageHeading from '@/components/ui/PageHeading.vue'
import AppIcon from '@/components/ui/AppIcon.vue'
import StatusBadge from '@/components/ui/StatusBadge.vue'
import UiDialog from '@/components/ui/UiDialog.vue'
const store = useEnterpriseStore(),
  ui = useUiStore(),
  route = useRoute(),
  router = useRouter()
const requestOpen = ref(false),
  targetPlan = ref('企业版'),
  note = ref(''),
  tab = ref('套餐概览')
const used = computed(() => store.members.filter((m) => m.status !== 'removed').length)
const quotas = computed(() => [
  { label: '成员账号', used: used.value, total: 500, unit: '人', icon: 'users' },
  { label: '点位数量', used: 86, total: 150, unit: '个', icon: 'site' },
  { label: '设备数量', used: 320, total: 500, unit: '台', icon: 'device' },
  { label: '数据存储', used: 128, total: 500, unit: 'GB', icon: 'database' },
])
const features = [
  { label: '成员与权限', description: '成员生命周期、角色与数据范围', icon: 'shield', enabled: true },
  { label: '组织管理', description: '多层级部门与组织协同', icon: 'organization', enabled: true },
  { label: '设备管理', description: '接入、分组与设备状态', icon: 'device', enabled: true },
  { label: '远程运维', description: '参数下发与固件升级', icon: 'operations', enabled: true },
  { label: '数据分析', description: '设备与出杯数据分析', icon: 'chart', enabled: true },
  { label: '故障工单', description: '故障受理与服务跟踪', icon: 'ticket', enabled: true },
  { label: '开放接口', description: '面向系统集成的开放能力', icon: 'link', enabled: false },
  { label: '定制化报表', description: '按需配置报表与分析', icon: 'file', enabled: false },
]
watch(
  () => route.query.action,
  (a) => {
    if (a === 'upgrade') {
      requestOpen.value = true
      void router.replace({ path: route.path, query: {} })
    }
  },
  { immediate: true },
)
function submit() {
  store.audit(
    '套餐信息',
    '创建套餐调整申请（预览）',
    targetPlan.value,
    '标准版',
    '待商务确认',
    note.value,
    'medium',
  )
  requestOpen.value = false
  ui.toast('已记录预览申请；未变更真实套餐，也不会发起扣费。', 'info')
}
</script>
<template>
  <div class="page-stack">
    <PageHeading title="套餐信息" description="查看当前订阅、可用功能与使用额度，让业务增长有据可依" />
    <div class="plan-top">
      <section class="card current-plan">
        <div class="row">
          <span class="plan-crown"><AppIcon name="crown" :size="30" /></span>
          <div>
            <small class="muted">当前套餐</small>
            <h2>标准版 <StatusBadge text="使用中" /></h2>
          </div>
        </div>
        <p>适用于多点位运营团队，统一管理设备、成员与服务。</p>
        <div class="plan-dates">
          <div><span>生效日期</span><strong>2026-09-08</strong></div>
          <div><span>到期日期</span><strong>2027-09-07</strong></div>
          <div><span>订阅周期</span><strong>按年</strong></div>
        </div>
        <div class="row">
          <button class="btn btn-primary" @click="requestOpen = true">
            <AppIcon name="crown" :size="16" />申请升级套餐</button
          ><button class="btn" @click="requestOpen = true">申请调整额度</button>
        </div>
        <small class="preview-plan">套餐、日期与额度均为界面示例，不代表真实订阅。</small>
      </section>
      <section class="card quota-overview">
        <div class="row-between">
          <h2>额度使用概览</h2>
          <span class="muted">当前企业</span>
        </div>
        <div class="quota-cards">
          <div v-for="q in quotas" :key="q.label" class="quota-item">
            <div class="row-between">
              <span class="row"><AppIcon :name="q.icon" :size="17" />{{ q.label }}</span
              ><span class="muted">{{ Math.round((q.used / q.total) * 100) }}%</span>
            </div>
            <strong class="numeric"
              >{{ q.used }} <small>/ {{ q.total }} {{ q.unit }}</small></strong
            >
            <div class="progress-track">
              <div class="progress-fill" :style="{ width: Math.min(100, (q.used / q.total) * 100) + '%' }" />
            </div>
          </div>
        </div>
      </section>
    </div>
    <section class="card panel-pad">
      <div class="tabs">
        <button
          v-for="t in ['套餐概览', '功能权益', '使用额度', '变更记录']"
          :key="t"
          :class="['tab', { active: tab === t }]"
          @click="tab = t"
        >
          {{ t }}
        </button>
      </div>
      <template v-if="tab === '套餐概览' || tab === '功能权益'"
        ><div class="row-between plan-section-heading">
          <h2>套餐权益</h2>
          <span class="muted">已开通 6 / 8 项</span>
        </div>
        <div class="feature-grid">
          <div v-for="f in features" :key="f.label" class="feature-row">
            <span class="feature-icon"><AppIcon :name="f.icon" :size="22" /></span>
            <div class="flex-1">
              <h3>{{ f.label }}</h3>
              <p>{{ f.description }}</p>
            </div>
            <StatusBadge
              :text="f.enabled ? '已开通' : '未开通'"
              :tone="f.enabled ? 'success' : 'warning'"
              :dot="false"
            />
          </div></div></template
      ><template v-else-if="tab === '使用额度'"
        ><div class="table-scroll quota-table">
          <table class="data-table">
            <thead>
              <tr>
                <th>资源类型</th>
                <th>已使用</th>
                <th>总额度</th>
                <th>使用率</th>
                <th>状态</th>
              </tr>
            </thead>
            <tbody>
              <tr v-for="q in quotas" :key="q.label">
                <td>{{ q.label }}</td>
                <td>{{ q.used }} {{ q.unit }}</td>
                <td>{{ q.total }} {{ q.unit }}</td>
                <td>{{ ((q.used / q.total) * 100).toFixed(1) }}%</td>
                <td><StatusBadge text="正常" /></td>
              </tr>
            </tbody>
          </table></div></template
      ><template v-else
        ><div class="timeline plan-timeline">
          <div
            v-for="log in store.logs.filter((l) => l.module === '套餐信息')"
            :key="log.id"
            class="timeline-item"
          >
            <strong>{{ log.action }}</strong>
            <p>{{ log.target }} · {{ log.after }}</p>
            <small>{{ log.time }}</small>
          </div>
          <div class="timeline-item">
            <strong>标准版预览套餐初始化</strong><small>2026-09-08 · 示例数据</small>
          </div>
        </div></template
      >
    </section>
    <UiDialog :open="requestOpen" title="申请套餐调整" @close="requestOpen = false"
      ><div class="page-stack">
        <div class="notice-box">
          <AppIcon name="help" />此操作仅记录前端预览申请，不会购买服务、变更实际订阅或扣费。
        </div>
        <label class="field"
          ><span>意向套餐</span
          ><select v-model="targetPlan" class="select">
            <option>企业版</option>
            <option>标准版扩容</option>
            <option>联系商务定制</option>
          </select></label
        ><label class="field"
          ><span>需求说明</span
          ><textarea
            v-model="note"
            class="textarea"
            maxlength="500"
            placeholder="描述所需成员、点位、设备或功能额度"
          />
        </label>
      </div>
      <template #footer
        ><button class="btn" @click="requestOpen = false">取消</button
        ><button class="btn btn-primary" @click="submit">记录申请</button></template
      ></UiDialog
    >
  </div>
</template>
<style scoped>
.plan-top {
  display: grid;
  grid-template-columns: 1fr 1.05fr;
  gap: 16px;
}
.current-plan {
  padding: 28px;
  background: linear-gradient(125deg, var(--color-primary-soft), var(--color-surface) 68%);
}
.plan-crown {
  width: 62px;
  height: 62px;
  display: grid;
  place-items: center;
  background: linear-gradient(130deg, var(--color-gradient-end), var(--color-primary));
  color: var(--color-on-primary);
  border-radius: 50%;
}
.current-plan h2 {
  font-size: 24px;
  margin-top: 4px;
  display: flex;
  align-items: center;
  gap: 12px;
}
.current-plan > p {
  font-size: 13px;
  color: var(--color-text-secondary);
  margin-top: 18px;
}
.plan-dates {
  display: grid;
  grid-template-columns: repeat(3, 1fr);
  gap: 12px;
  margin: 25px 0;
}
.plan-dates span {
  display: block;
  color: var(--color-text-muted);
  font-size: 12px;
  margin-bottom: 7px;
}
.plan-dates strong {
  font-size: 14px;
  font-weight: 500;
}
.preview-plan {
  display: block;
  color: var(--color-text-muted);
  font-size: 10px;
  margin-top: 18px;
}
.quota-overview {
  padding: 28px;
}
.quota-overview > .row-between > span {
  font-size: 12px;
}
.quota-cards {
  display: grid;
  grid-template-columns: 1fr 1fr;
  gap: 27px 24px;
  margin-top: 26px;
}
.quota-item .row-between {
  font-size: 12px;
}
.quota-item .icon {
  color: var(--color-primary);
}
.quota-item strong {
  display: block;
  font-size: 25px;
  margin: 11px 0 12px;
}
.quota-item strong small {
  font-size: 12px;
  color: var(--color-text-muted);
  font-weight: 400;
}
.plan-section-heading {
  margin: 25px 0 20px;
}
.plan-section-heading > span {
  font-size: 12px;
}
.feature-grid {
  display: grid;
  grid-template-columns: 1fr 1fr;
  gap: 13px 20px;
}
.feature-row {
  display: flex;
  align-items: center;
  gap: 14px;
  border: 1px solid var(--color-border);
  padding: 17px;
  border-radius: 9px;
}
.feature-row h3 {
  font-size: 14px;
}
.feature-row p {
  font-size: 12px;
  color: var(--color-text-muted);
  margin-top: 5px;
}
.feature-icon {
  height: 40px;
  width: 40px;
  border-radius: 12px;
  display: grid;
  place-items: center;
  flex-shrink: 0;
  background: var(--color-primary-soft);
  color: var(--color-primary);
}
.quota-table {
  margin-top: 24px;
}
.plan-timeline {
  margin-top: 28px;
}
@media (max-width: 1100px) {
  .plan-top {
    grid-template-columns: 1fr;
  }
}
@media (max-width: 767px) {
  .feature-grid {
    grid-template-columns: 1fr;
  }
  .current-plan,
  .quota-overview {
    padding: 20px;
  }
  .current-plan > .row {
    flex-wrap: wrap;
  }
  .plan-dates strong {
    font-size: 12px;
  }
  .feature-row p {
    font-size: 11px;
  }
}
</style>
