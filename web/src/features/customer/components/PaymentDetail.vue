<script setup lang="ts">
import { computed } from 'vue'
import type { WorkItem } from '@/types/customer'
import { useCustomerStore } from '@/stores/customer'
import { useCustomerActions } from '@/composables/customerActions'
import { receivableFor, workSources } from '@/services/customer/selectors'
import CustomerSection from './CustomerSection.vue'
import PaymentSnapshot from './PaymentSnapshot.vue'
import SourceRecords from './SourceRecords.vue'
const props = defineProps<{ work: WorkItem }>(),
  store = useCustomerStore(),
  actions = useCustomerActions()
const receivable = computed(() => receivableFor(store.snapshot, props.work))
const records = computed(() =>
  workSources(store.snapshot, props.work).filter((s) => s.kind === 'receivable' && s.verified),
)
</script>
<template>
  <div class="page-stack">
    <CustomerSection title="关联应收与核销进展" icon="order">
      <template #action
        ><button
          class="btn-link"
          :disabled="!work.writable || work.status === '已结束'"
          @click="actions.open('reschedule', work.id)"
        >
          核对回款结果
        </button></template
      >
      <PaymentSnapshot :work="work" />
      <dl class="customer-info-list" style="margin-top: 20px">
        <dt>应收记录</dt>
        <dd>{{ receivable?.id || '未关联' }} · {{ receivable?.title || '等待业务来源' }}</dd>
        <dt>客户</dt>
        <dd>{{ store.customerName(work.customerId) }}</dd>
        <dt>下一步行动</dt>
        <dd>{{ work.nextAction }}</dd>
        <dt>财务来源</dt>
        <dd>本地示例只读投影；不在客户事项中直接修改应收账本</dd>
      </dl>
    </CustomerSection>
    <CustomerSection title="回款沟通与责任" icon="phone">
      <dl class="customer-info-list">
        <dt>事项负责人</dt>
        <dd>{{ work.owner }}</dd>
        <dt>本次跟进</dt>
        <dd>{{ work.description }}</dd>
        <dt>下次行动</dt>
        <dd>
          {{ work.nextAt.replace('T', ' ') }}
          <button
            class="btn-link"
            :disabled="!work.writable || work.status === '已结束'"
            @click="actions.open('reschedule', work.id)"
          >
            调整安排
          </button>
        </dd>
        <dt>数据权限</dt>
        <dd><RouterLink to="/customers/restricted" class="btn-link">查看财务字段受限示例</RouterLink></dd>
      </dl>
    </CustomerSection>
    <CustomerSection title="财务业务来源" icon="file">
      <template #action
        ><button
          class="btn-link"
          :disabled="!work.writable || work.status === '已结束' || !records.length"
          @click="actions.open('link-source', work.id)"
        >
          核对最新核销记录
        </button></template
      >
      <SourceRecords :records="receivable ? [receivable] : []" />
    </CustomerSection>
  </div>
</template>
