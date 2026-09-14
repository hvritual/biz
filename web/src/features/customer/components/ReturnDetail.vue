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
    ['recovery', 'settlement', 'termination'].includes(s.kind),
  ),
)
const recovery = computed(() => records.value.find((s) => s.kind === 'recovery' && s.facts.returned))
const termination = computed(() =>
  records.value.find((s) => s.kind === 'termination' && props.work.evidenceIds.includes(s.id)),
)
</script>
<template>
  <div class="page-stack">
    <CustomerSection title="退租回收与结算" icon="back">
      <div class="customer-mini-metrics">
        <div>
          <small>本次回收设备</small><strong>{{ recovery?.facts.count ?? '—' }} <small>台</small></strong
          ><small>仅本次回收记录范围</small>
        </div>
        <div>
          <small>回收记录</small><strong style="font-size: 20px">{{ recovery?.id || '未关联' }}</strong
          ><StatusBadge
            :text="recovery?.state || '等待业务来源'"
            :tone="recovery?.verified ? 'success' : 'neutral'"
          />
        </div>
        <div>
          <small>处理结果</small
          ><strong style="font-size: 20px">{{
            work.status === '已结束' ? '处置已完成' : '待核对结算'
          }}</strong>
        </div>
      </div>
      <dl class="customer-info-list">
        <dt>关联客户</dt>
        <dd>{{ store.customerName(work.customerId) }}</dd>
        <dt>点位范围</dt>
        <dd>{{ recovery?.facts.scope || '以关联回收与投放记录核对，尚未确认' }}</dd>
        <dt>责任人</dt>
        <dd>{{ work.owner }} · 物流、运维与财务协作</dd>
        <dt>关闭依据</dt>
        <dd>实物回收 + 有效结算 + 投放关系终止</dd>
      </dl>
      <div class="customer-checklist" style="margin-top: 24px">
        <label v-for="record in records" :key="record.id"
          ><input type="checkbox" :checked="work.evidenceIds.includes(record.id)" disabled />{{ record.title
          }}<button
            class="btn-link"
            :disabled="!work.writable || work.status === '已结束'"
            @click="actions.open('link-source', work.id, [], { sourceId: record.id })"
          >
            核对 {{ record.id }}
          </button></label
        >
        <p v-if="!records.length" class="customer-help">尚无本事项回收、结算或终止记录。</p>
      </div>
    </CustomerSection>
    <CustomerSection title="终止后的设备数据范围" icon="shield">
      <CustomerAlert
        :title="termination ? `已记录投放终止时点：${termination.facts.end}` : '投放终止日期待来源核验'"
        description="终止之后的新运行数据不再共享给原客户；原投放期间的历史数据按授权保留。事项关联设备不等于获得全部历史权限。"
        :tone="termination ? 'success' : 'warning'"
      />
      <dl class="customer-info-list" style="margin-top: 20px">
        <dt>历史数据</dt>
        <dd>原投放有效期间可查看，不能借此读取后续租户数据。</dd>
        <dt>新数据访问</dt>
        <dd>终止后停止，不因旧事项未删除而继续保留。</dd>
        <dt>其他合作</dt>
        <dd>本次仅结束指定范围投放，客户其他合同和点位不受影响。</dd>
      </dl>
    </CustomerSection>
    <CustomerSection title="回收、结算与终止记录" icon="link"
      ><SourceRecords :records="records.filter((s) => work.evidenceIds.includes(s.id))"
    /></CustomerSection>
  </div>
</template>
