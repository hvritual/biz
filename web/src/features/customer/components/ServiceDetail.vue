<script setup lang="ts">
import { computed } from 'vue'
import type { WorkItem } from '@/types/customer'
import { useCustomerStore } from '@/stores/customer'
import { useCustomerActions } from '@/composables/customerActions'
import { workSources } from '@/services/customer/selectors'
import CustomerSection from './CustomerSection.vue'
import CustomerAlert from './CustomerAlert.vue'
import SourceRecords from './SourceRecords.vue'
import StatusBadge from '@/components/ui/StatusBadge.vue'
const props = defineProps<{ work: WorkItem }>(),
  store = useCustomerStore(),
  actions = useCustomerActions()
const records = computed(() =>
  workSources(store.snapshot, props.work).filter((s) =>
    ['service', 'recovery', 'confirmation'].includes(s.kind),
  ),
)
const order = computed(() => records.value.find((s) => s.kind === 'service'))
const runtime = computed(() => records.value.find((s) => s.kind === 'recovery' && s.facts.recovered))
const confirmation = computed(() => records.value.find((s) => s.kind === 'confirmation'))
</script>
<template>
  <div class="page-stack">
    <CustomerSection title="服务恢复验证" icon="operations">
      <template #action><RouterLink to="/customers/sla" class="btn-link">查看服务时限</RouterLink></template>
      <CustomerAlert
        :title="order?.facts.workComplete ? '工单处理完成，仍需要验证恢复' : '服务依据待核对'"
        description="客户事项只协调与验证，不重建运维工单执行状态。"
      />
      <div class="customer-mini-metrics" style="margin-top: 18px">
        <div>
          <small>运维工单</small><strong style="font-size: 20px">{{ order?.id || '未关联' }}</strong
          ><StatusBadge
            :text="order?.state || '等待业务来源'"
            :tone="order?.facts.workComplete ? 'success' : 'neutral'"
          />
        </div>
        <div>
          <small>首次响应时限</small
          ><strong>{{ store.snapshot.sla.responseMinutes }} <small>分钟</small></strong>
        </div>
        <div>
          <small>服务恢复时限</small
          ><strong>{{ store.snapshot.sla.recoveryHours }} <small>小时</small></strong>
        </div>
      </div>
      <dl class="customer-info-list">
        <dt>问题范围</dt>
        <dd>{{ store.customerName(work.customerId) }} · {{ work.description }}</dd>
        <dt>处理说明</dt>
        <dd>{{ order ? `${order.title} · ${order.state}` : '尚无本事项的运维处理结果' }}</dd>
        <dt>恢复验证</dt>
        <dd>
          {{
            runtime
              ? `${runtime.id}：连续在线 ${runtime.facts.onlineMinutes} 分钟，验证出杯 ${runtime.facts.validCups} 杯`
              : '尚无可核对的连续运行验证'
          }}
        </dd>
        <dt>客户反馈</dt>
        <dd>
          {{
            confirmation
              ? `${confirmation.id} · ${confirmation.title} · ${confirmation.state}`
              : '尚未关联本次客户确认'
          }}
        </dd>
      </dl>
    </CustomerSection>
    <CustomerSection title="恢复验收清单" icon="checks">
      <div class="customer-checklist">
        <label v-for="record in records" :key="record.id"
          ><input type="checkbox" :checked="work.evidenceIds.includes(record.id)" disabled />{{ record.title
          }}<button
            class="btn-link"
            :disabled="!work.writable || work.status === '已结束'"
            @click="actions.open('link-source', work.id, [], { sourceId: record.id })"
          >
            关联 {{ record.id }}
          </button></label
        >
        <p v-if="!records.length" class="customer-help">
          暂无同客户、同事项下的服务依据，不展示其他客户的工单。
        </p>
      </div>
      <CustomerAlert
        title="验证失败可退回处理"
        description="保留原工单与验证证据，在原客户事项中继续跟踪，不新建重复故障。"
        tone="warning"
        style="margin-top: 18px"
      />
    </CustomerSection>
    <CustomerSection title="已关联服务记录" icon="link"
      ><SourceRecords :records="records.filter((s) => work.evidenceIds.includes(s.id))"
    /></CustomerSection>
  </div>
</template>
