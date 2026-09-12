<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { useCustomerStore } from '@/stores/customer'
import {
  rentalState,
  activeRule,
  currentRules,
  reviewPeriod,
  modeNames,
  summary,
  money,
} from '@/services/siteRental/model'
import { quoteRental } from '@/services/siteRental/quote'
import PageHeading from '@/components/ui/PageHeading.vue'
import AppIcon from '@/components/ui/AppIcon.vue'
import StatusBadge from '@/components/ui/StatusBadge.vue'
import CustomerSection from '@/components/customer/CustomerSection.vue'
import CustomerAlert from '@/components/customer/CustomerAlert.vue'
import RuleEditor from '@/components/siteRental/RuleEditor.vue'
import QuoteSummary from '@/components/siteRental/QuoteSummary.vue'
import StatementDialog from '@/components/siteRental/StatementDialog.vue'
import '@/styles/siteRental.css'
const store = useCustomerStore(),
  route = useRoute(),
  router = useRouter(),
  period = ref(reviewPeriod),
  open = ref(false),
  editGroup = ref<string>(),
  error = ref(''),
  statementId = ref(''),
  detailTab = ref('计费核算')
const state = computed(() => rentalState(store.snapshot)),
  rows = computed(() => currentRules(state.value, period.value, store.snapshot.sources)),
  groupId = computed(() => String(route.params.groupId || '')),
  versions = computed(() =>
    state.value.rules.filter((x) => x.groupId === groupId.value).sort((a, b) => b.version - a.version),
  ),
  rule = computed(() => activeRule(state.value, groupId.value, period.value, store.snapshot.sources)),
  quote = computed(() =>
    rule.value ? quoteRental(store.snapshot, state.value, rule.value, period.value) : undefined,
  )
function edit(id?: string) {
  editGroup.value = id
  open.value = true
}
function generate(id: string) {
  try {
    store.refreshRental()
    const result = store.runRental({
      tenant: store.snapshot.tenant,
      expectedRevision: store.snapshot.revision,
      key: crypto.randomUUID(),
      action: { type: 'create-statement', groupId: id, period: period.value },
    })
    statementId.value = result.target
    error.value = ''
  } catch (e) {
    error.value = (e as Error).message
  }
}
watch(
  () => route.fullPath,
  () => {
    if (route.query.action === 'new-rule') {
      edit()
      void router.replace({ path: route.path })
    }
  },
  { immediate: true },
)
function bill(id: string) {
  const x = activeRule(state.value, id, period.value, store.snapshot.sources)
  return x ? money(quoteRental(store.snapshot, state.value, x, period.value).totalCents) : '—'
}
</script>
<template>
  <div class="rental-area page-stack">
    <PageHeading
      :title="groupId ? rule?.name || versions[0]?.name || '计费组详情' : '计费规则与共享组'"
      breadcrumb="点位租赁"
      description="先确定计算范围，再决定账单汇总；规则按生效版本追溯"
      ><div class="rental-actions">
        <RouterLink :to="groupId ? '/sites/groups' : '/sites'" class="btn">{{
          groupId ? '返回计费组' : '点位列表'
        }}</RouterLink
        ><button class="btn btn-primary" @click="edit(groupId || undefined)">
          <AppIcon name="plus" :size="16" />{{ groupId ? '创建规则变更' : '新建计费规则' }}
        </button>
      </div></PageHeading
    >
    <div class="rental-mode-guide">
      <div v-for="(title, mode) in modeNames" :key="mode">
        <strong>{{ title }}</strong>
        <p>
          {{
            mode === 'fixed'
              ? '按点位或在租设备计固定月费'
              : mode === 'metered'
                ? '按合同口径的可计费杯数核算'
                : '含杯数与最低消费明确区分'
          }}
        </p>
      </div>
    </div>
    <div class="rental-toolbar">
      <nav v-if="groupId" class="customer-tabs" aria-label="计费组详情">
        <button
          v-for="name in ['计费核算', '版本履历']"
          :key="name"
          :class="{ active: detailTab === name }"
          @click="detailTab = name"
        >
          {{ name }}
        </button>
      </nav>
      <p v-else class="rental-help">计费币种：CNY · 自然月 · 规则不会改变 SaaS 套餐配额</p>
      <label class="rental-inline-label"
        >查看账期 <input v-model="period" type="month" aria-label="计费组账期"
      /></label>
    </div>
    <template v-if="!groupId">
      <CustomerAlert
        title="独立计算 ≠ 分别出账；共享额度 ≠ 平均分配"
        description="同合同指定点位共享时，先汇总可计费杯数，再扣减整组额度；各点位展示用量贡献。"
      />
      <section class="card">
        <div class="table-scroll">
          <table class="data-table rental-group-table">
            <thead>
              <tr>
                <th>计费组 / 客户</th>
                <th>计费模式与约定</th>
                <th>计算范围</th>
                <th>版本与合同</th>
                <th>本期预计费用</th>
                <th>操作</th>
              </tr>
            </thead>
            <tbody>
              <tr v-for="item in rows" :key="item.id">
                <td>
                  <RouterLink :to="`/sites/groups/${item.groupId}`" class="rental-name">{{
                    item.name
                  }}</RouterLink
                  ><small>{{ store.customerName(item.customerId) }}</small>
                </td>
                <td>
                  <StatusBadge :text="modeNames[item.mode]" tone="primary" /><small>{{
                    summary(item)
                  }}</small>
                </td>
                <td>
                  <strong>{{ item.scope === 'shared' ? '共享额度' : '独立计算' }}</strong
                  ><small>{{ item.siteIds.length }} 个指定点位 · {{ item.billPresentation }}</small>
                </td>
                <td>
                  <strong>v{{ item.version }} · {{ item.effectiveFrom }}</strong
                  ><small>{{ item.contractId }}</small>
                </td>
                <td class="rental-number">{{ bill(item.groupId) }}</td>
                <td>
                  <RouterLink :to="`/sites/groups/${item.groupId}`" class="btn-link">查看</RouterLink>
                  <button class="btn-link" @click="edit(item.groupId)">变更</button>
                </td>
              </tr>
            </tbody>
          </table>
        </div>
        <p v-if="!rows.length" class="rental-empty">当前账期没有已发布规则；草稿不会参与核算。</p>
      </section>
      <CustomerSection v-if="Object.keys(state.drafts).length" title="未发布草稿" icon="file"
        ><div v-for="draft in state.drafts" :key="draft.rule.groupId" class="rental-history-row">
          <div>
            <strong>{{ draft.rule.name }}</strong>
            <p>{{ draft.rule.effectiveFrom }} · 未生效</p>
          </div>
          <button class="btn" @click="edit(draft.rule.groupId)">继续编辑</button>
        </div></CustomerSection
      >
      <CustomerSection
        v-if="state.rules.some((x) => x.effectiveFrom > `${period}-01`)"
        title="预约生效版本"
        icon="clock"
        ><div
          v-for="item in state.rules.filter((x) => x.effectiveFrom > `${period}-01`)"
          :key="item.id"
          class="rental-history-row"
        >
          <div>
            <strong>{{ item.name }} · v{{ item.version }}</strong>
            <p>{{ item.effectiveFrom }} 起 · {{ summary(item) }}</p>
          </div>
          <RouterLink :to="`/sites/groups/${item.groupId}`" class="btn-link">查看版本</RouterLink>
        </div></CustomerSection
      >
    </template>
    <template v-else-if="versions.length">
      <CustomerSection
        v-if="detailTab === '计费核算' && rule && quote"
        title="用量贡献与计费结果"
        icon="layers"
        ><template #action
          ><button class="btn btn-primary" @click="generate(groupId)">生成 / 查看对账草稿</button></template
        ><QuoteSummary :quote="quote" :rule="rule" />
        <dl class="rental-definition">
          <dt>结算主体</dt>
          <dd>{{ rule.billingOwner }} · {{ rule.billPresentation }}</dd>
          <dt>生效规则</dt>
          <dd>{{ rule.contractId }} · v{{ rule.version }} · {{ rule.effectiveFrom }}</dd>
          <dt>约定依据</dt>
          <dd>{{ rule.reason }}</dd>
        </dl></CustomerSection
      >
      <CustomerSection v-else-if="detailTab === '版本履历'" title="规则及成员范围的版本履历" icon="clock"
        ><CustomerAlert
          title="新版本不覆盖旧版本"
          description="计费组成员与条款一起版本化。查询历史账期按当时有效规则，已确认对账保留原始快照。"
        />
        <div v-for="item in versions" :key="item.id" class="rental-version">
          <div>
            <StatusBadge
              :text="
                item.effectiveFrom > `${period}-01`
                  ? '预约生效'
                  : item.id === rule?.id
                    ? '该账期生效'
                    : '历史版本'
              "
              :tone="item.id === rule?.id ? 'success' : 'neutral'"
            />
            <h3>v{{ item.version }} · {{ item.effectiveFrom }} 起</h3>
            <p>{{ summary(item) }}</p>
            <small>{{ item.scope === 'shared' ? '共享' : '独立' }} · {{ item.siteIds.join(' / ') }}</small>
            <p>{{ item.reason }}</p>
          </div>
          <strong>{{ item.contractId }}</strong>
        </div></CustomerSection
      >
      <CustomerAlert
        v-else
        title="该账期没有有效计费规则"
        description="已到期合同停止参与新账期核算；预约版本不提前生效。历史规则仍可在版本履历查看。"
      /> </template
    ><CustomerAlert v-else title="计费组不存在或不在当前租户" tone="warning" />
    <p v-if="error" class="rental-error" role="alert">{{ error }}</p>
    <RuleEditor
      :open="open"
      :group-id="editGroup"
      @close="open = false"
      @published="(id) => router.push(`/sites/groups/${id}`)"
    /><StatementDialog :id="statementId" :open="!!statementId" @close="statementId = ''" />
  </div>
</template>
