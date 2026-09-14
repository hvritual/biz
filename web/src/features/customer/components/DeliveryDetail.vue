<script setup lang="ts">
import { computed } from 'vue'
import { workSites, workSources } from '@/services/customer/selectors'
import type { WorkItem } from '@/types/customer'
import { useCustomerStore } from '@/stores/customer'
import { useCustomerActions } from '@/composables/customerActions'
import StatusBadge from '@/components/ui/StatusBadge.vue'
import MetricCard from '@/components/ui/MetricCard.vue'
import EmptyState from '@/components/ui/EmptyState.vue'
import CustomerSection from './CustomerSection.vue'
import CustomerAlert from './CustomerAlert.vue'
import SourceRecords from './SourceRecords.vue'
const props = defineProps<{ work: WorkItem }>()
const store = useCustomerStore(),
  actions = useCustomerActions()
const sites = computed(() => workSites(store.snapshot, props.work))
const records = computed(() => workSources(store.snapshot, props.work, true))
const allPassed = computed(() => sites.value.length > 0 && sites.value.every((s) => s.accepted && s.trial))
</script>
<template>
  <div class="page-stack">
    <div class="metric-grid customer-delivery-metrics">
      <MetricCard
        label="计划投放"
        :value="sites.length"
        unit="个点位"
        icon="site"
        caption="同一客户 · 本次交付范围"
      />
      <MetricCard
        label="验收通过"
        :value="sites.filter((s) => s.accepted).length"
        unit="个点位"
        icon="checks"
        tone="green"
        caption="关联有效验收记录"
      />
      <MetricCard
        label="待整改"
        :value="sites.filter((s) => !s.accepted).length"
        unit="个点位"
        icon="warning"
        tone="orange"
        caption="未通过不能整体完成"
      />
      <MetricCard
        label="事项截止"
        :value="work.deadline.slice(5)"
        icon="calendar"
        :caption="`${work.deadline.slice(0, 4)} 年 · 不随整改自动延期`"
      />
    </div>
    <CustomerSection title="本次投放范围与验收" icon="site">
      <template #action
        ><button
          class="btn-link"
          :disabled="!work.writable || work.status === '已结束'"
          @click="actions.open('link-source', work.id)"
        >
          关联验收记录
        </button></template
      >
      <div v-if="sites.length" class="table-scroll">
        <table class="data-table">
          <thead>
            <tr>
              <th>客户点位 / 投放关系</th>
              <th>执行人</th>
              <th>出杯数据验证</th>
              <th>培训确认</th>
              <th>验收状态</th>
            </tr>
          </thead>
          <tbody>
            <tr v-for="site in sites" :key="site.id">
              <td>
                <strong>{{ site.name }}</strong>
                <p class="subline">投放关系 {{ site.placement }} · {{ site.device }}</p>
              </td>
              <td>{{ site.owner }}</td>
              <td>
                <StatusBadge
                  :text="site.trial ? '通过' : '未通过'"
                  :tone="site.trial ? 'success' : 'danger'"
                />
              </td>
              <td>
                <StatusBadge
                  :text="site.training ? '已确认' : '未完成'"
                  :tone="site.training ? 'success' : 'neutral'"
                />
              </td>
              <td>
                <StatusBadge
                  :text="site.accepted ? '验收通过' : '待整改'"
                  :tone="site.accepted ? 'success' : 'warning'"
                />
              </td>
            </tr>
          </tbody>
        </table>
      </div>
      <EmptyState
        v-else
        title="尚未关联本次交付范围"
        description="请先关联当前客户和事项的投放资料，不能使用其他客户的验收结果。"
      />
      <CustomerAlert
        v-if="sites.some((s) => !s.accepted)"
        title="部分通过，不等于整批交付完成"
        description="未通过点位需整改后复验；已通过部分保留原结果。"
        tone="warning"
      />
    </CustomerSection>
    <div class="customer-columns">
      <CustomerSection title="交付要求" icon="checks">
        <div class="customer-checklist">
          <label
            ><input type="checkbox" :checked="sites.length > 0" disabled />投放范围归属当前客户与事项</label
          >
          <label
            ><input
              type="checkbox"
              :checked="sites.length > 0 && sites.every((s) => s.training)"
              disabled
            />现场安装、安全检查与培训完成</label
          >
          <label
            ><input type="checkbox" :checked="allPassed" disabled />全部点位的出杯与状态数据完成验证</label
          >
          <label
            ><input
              type="checkbox"
              :checked="work.status === '已结束' && work.resolution === '成功'"
              disabled
            />客户确认与有效投放关系已核验</label
          >
        </div>
      </CustomerSection>
      <CustomerSection title="关联资料" icon="link"><SourceRecords :records="records" /></CustomerSection>
    </div>
  </div>
</template>
