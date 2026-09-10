<script setup lang="ts">
import { computed } from 'vue'
import type { WorkItem } from '@/types/customer'
import { useCustomerStore } from '@/stores/customer'
import { receivableFor } from '@/services/customer/selectors'
import CustomerAlert from './CustomerAlert.vue'
const props = defineProps<{ work: WorkItem }>(),
  store = useCustomerStore()
const source = computed(() => receivableFor(store.snapshot, props.work))
const money = (n: unknown) =>
  n === undefined ? '—' : Number(n).toLocaleString('zh-CN', { minimumFractionDigits: 2 })
</script>
<template>
  <div>
    <div class="customer-mini-metrics">
      <div>
        <small>本期应收（CNY）</small><strong>{{ money(source?.facts.due) }}</strong>
      </div>
      <div>
        <small>已核销（CNY）</small><strong>{{ money(source?.facts.paid) }}</strong>
      </div>
      <div>
        <small>未核销（CNY）</small><strong>{{ money(source?.facts.balance) }}</strong>
      </div>
    </div>
    <CustomerAlert
      v-if="!source"
      title="尚未关联本事项的应收来源"
      description="金额未知，不能用其他客户的应收或零值代替。先关联同客户、同事项的财务来源。"
      tone="warning"
    />
    <CustomerAlert
      v-else
      :title="
        Number(source.facts.balance) > 0 ? '仍有未核销余额，不能成功关闭' : '核销条件已满足，还需提交事项验收'
      "
      :description="`${source.id} · ${source.state}。只读财务投影，客户付款截图或口头确认不等于有效核销。`"
      :tone="Number(source.facts.balance) > 0 ? 'warning' : 'success'"
    />
  </div>
</template>
