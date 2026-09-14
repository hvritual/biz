<script setup lang="ts">
import { computed, ref } from 'vue'
import { useRoute } from 'vue-router'
import { useCustomerStore } from '@/stores/customer'
import { rentalState, currentRules, reviewPeriod, money } from '@/services/siteRental/model'
import PageHeading from '@/components/ui/PageHeading.vue'
import CustomerAlert from '@/components/customer/CustomerAlert.vue'
import StatusBadge from '@/components/ui/StatusBadge.vue'
import EmptyState from '@/components/ui/EmptyState.vue'
import StatementDialog from '@/components/siteRental/StatementDialog.vue'
import '@/styles/siteRental.css'
const store = useCustomerStore(),
  route = useRoute(),
  state = computed(() => rentalState(store.snapshot)),
  period = ref(reviewPeriod),
  group = ref(String(route.query.group || '')),
  openId = ref(''),
  error = ref('')
const rows = computed(() =>
  state.value.statements.filter(
    (x) => x.period === period.value && (!group.value || x.groupId === group.value),
  ),
)
function create() {
  try {
    if (!group.value) throw new Error('请先选择计费组')
    store.refreshRental()
    const result = store.runRental({
      tenant: store.snapshot.tenant,
      expectedRevision: store.snapshot.revision,
      key: crypto.randomUUID(),
      action: { type: 'create-statement', groupId: group.value, period: period.value },
    })
    openId.value = result.target
    error.value = ''
  } catch (e) {
    error.value = (e as Error).message
  }
}
</script>
<template>
  <div class="rental-area page-stack">
    <PageHeading
      title="租赁对账"
      breadcrumb="点位管理"
      description="用量核对 → 异议处理 → 确认快照；正式应收与收款仍由财务模块管理"
      ><RouterLink to="/sites/groups" class="btn">查看计费规则</RouterLink></PageHeading
    ><CustomerAlert
      title="对账确认不等于已经收款"
      description="这里核对本地示例合同与用量。草稿可更新，确认后不可覆盖；未向财务系统记账，也未发送客户账单。"
    />
    <div class="card rental-filters">
      <select v-model="group" aria-label="对账计费组">
        <option value="">请选择计费组</option>
        <option
          v-for="rule in currentRules(state, period, store.snapshot.sources)"
          :key="rule.groupId"
          :value="rule.groupId"
        >
          {{ rule.name }}
        </option></select
      ><input v-model="period" type="month" aria-label="对账账期" /><button
        class="btn btn-primary"
        @click="create"
      >
        生成 / 查看对账草稿
      </button>
    </div>
    <p v-if="error" class="rental-error" role="alert">{{ error }}</p>
    <section class="card">
      <div class="table-scroll">
        <table class="data-table">
          <thead>
            <tr>
              <th>对账单 / 计费组</th>
              <th>客户与合同</th>
              <th>账期 / 规则</th>
              <th>状态</th>
              <th>对账金额</th>
              <th>操作</th>
            </tr>
          </thead>
          <tbody>
            <tr v-for="row in rows" :key="row.id">
              <td>
                <strong>{{ row.rule.name }}</strong
                ><small>{{ row.id }}</small>
              </td>
              <td>
                {{ row.rule.billingOwner }}<small>{{ row.rule.contractId }}</small>
              </td>
              <td>
                {{ row.period
                }}<small>v{{ row.rule.version }} · {{ row.rule.scope === 'shared' ? '共享' : '独立' }}</small>
              </td>
              <td>
                <StatusBadge
                  :text="row.state"
                  :tone="
                    row.state === '已确认' ? 'success' : row.state === '异议处理中' ? 'warning' : 'primary'
                  "
                /><small v-if="row.quote.blockers.length">{{ row.quote.blockers.length }} 项待核对</small>
              </td>
              <td class="rental-number">{{ money(row.quote.totalCents) }}<small>非正式应收</small></td>
              <td><button class="btn-link" @click="openId = row.id">核对详情</button></td>
            </tr>
          </tbody>
        </table>
      </div>
      <EmptyState
        v-if="!rows.length"
        title="本账期尚无对账记录"
        description="选择有效计费组后生成草稿；同一组同一账期只保留一份。"
      />
    </section>
    <StatementDialog :id="openId" :open="!!openId" @close="openId = ''" />
  </div>
</template>
