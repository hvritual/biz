<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { useCustomerStore } from '@/stores/customer'
import { rentalState } from '@/services/siteRental/model'
import type { RentalAction } from '@/services/siteRental/commands'
import UiDialog from '@/components/ui/UiDialog.vue'
import StatusBadge from '@/components/ui/StatusBadge.vue'
import CustomerAlert from '@/components/customer/CustomerAlert.vue'
import QuoteSummary from './QuoteSummary.vue'
const props = defineProps<{ open: boolean; id: string }>(),
  emit = defineEmits<{ close: [] }>(),
  store = useCustomerStore()
const statement = computed(() => rentalState(store.snapshot).statements.find((x) => x.id === props.id)),
  reason = ref(''),
  error = ref(''),
  revision = ref(0),
  tenant = ref('')
watch(
  () => [props.open, props.id],
  () => {
    if (props.open) {
      store.refreshRental()
      revision.value = store.snapshot.revision
      tenant.value = store.snapshot.tenant
      reason.value = ''
      error.value = ''
    }
  },
  { immediate: true },
)
function run(type: 'refresh-statement' | 'confirm-statement' | 'dispute' | 'resolve-dispute') {
  try {
    const action: RentalAction = { type, statementId: props.id, reason: reason.value }
    store.runRental({
      tenant: tenant.value,
      expectedRevision: revision.value,
      key: crypto.randomUUID(),
      action,
    })
    revision.value = store.snapshot.revision
    error.value = ''
    reason.value = ''
  } catch (e) {
    error.value = (e as Error).message
  }
}
</script>
<template>
  <UiDialog :open="open" title="点位租赁对账核对" width="860px" @close="emit('close')"
    ><div v-if="statement" class="rental-form page-stack">
      <div class="rental-rule-caption">
        <div>
          <h3>{{ statement.rule.name }} · {{ statement.period }}</h3>
          <p>{{ statement.id }} · {{ statement.rule.contractId }} · 规则 v{{ statement.rule.version }}</p>
        </div>
        <StatusBadge
          :text="statement.state"
          :tone="
            statement.state === '已确认'
              ? 'success'
              : statement.state === '异议处理中'
                ? 'warning'
                : 'primary'
          "
        />
      </div>
      <CustomerAlert
        v-if="statement.state === '已确认'"
        title="已确认快照锁定，后到数据不得静默覆盖"
        :description="`确认时间 ${statement.confirmedAt}。差异需另走调整流程；本轮没有真实记账。`"
        tone="success"
      />
      <CustomerAlert
        v-else-if="statement.state === '异议处理中'"
        title="异议未结束，确认已阻断"
        description="先记录处理结论，再重新核对来源；通知已读或事项结束不能代替对账确认。"
        tone="warning"
      />
      <QuoteSummary
        :quote="statement.quote"
        :rule="statement.rule"
        :confirmed="statement.state === '已确认'"
      />
      <label v-if="statement.state !== '已确认'"
        >核对依据 / 异议处理说明 <b>*</b
        ><textarea v-model="reason" rows="2" placeholder="请记录数据口径、核对结果或异议结论" />
      </label>
      <p v-if="error" role="alert" class="rental-error">{{ error }}</p>
      <details>
        <summary>操作记录（{{ statement.history.length }}）</summary>
        <div v-for="(h, i) in statement.history" :key="i" class="rental-history-row">
          <strong>{{ h.action }}</strong
          ><span>{{ h.reason }}</span
          ><small>{{ h.time }}</small>
        </div>
      </details>
    </div>
    <p v-else role="alert">对账单不存在或不在当前租户。</p>
    <template #footer
      ><button class="btn" @click="emit('close')">关闭</button
      ><template v-if="statement?.state === '草稿'"
        ><button class="btn" @click="run('refresh-statement')">刷新来源</button
        ><button class="btn" @click="run('dispute')">登记异议</button
        ><button
          class="btn btn-primary"
          :disabled="!!statement.quote.blockers.length"
          @click="run('confirm-statement')"
        >
          确认对账并冻结
        </button></template
      ><button
        v-if="statement?.state === '异议处理中'"
        class="btn btn-primary"
        @click="run('resolve-dispute')"
      >
        记录解决结论
      </button></template
    >
  </UiDialog>
</template>
