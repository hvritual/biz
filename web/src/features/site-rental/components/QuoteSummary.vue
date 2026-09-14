<script setup lang="ts">
import { computed } from 'vue'
import type { RentalQuote, RentalRule } from '@/types/siteRental'
import { money, summary } from '@/services/siteRental/model'
import StatusBadge from '@/components/ui/StatusBadge.vue'
import CustomerAlert from '@/components/customer/CustomerAlert.vue'
const props = defineProps<{ quote: RentalQuote; rule: RentalRule; trial?: boolean; confirmed?: boolean }>()
const cups = computed(() =>
  props.quote.contributions.every((x) => x.cups !== null)
    ? props.quote.contributions.reduce((n, x) => n + x.cups!, 0)
    : null,
)
const remaining = computed(() =>
  cups.value === null ? null : Math.max(props.rule.includedCups - cups.value, 0),
)
</script>
<template>
  <div class="rental-quote page-stack">
    <div class="rental-rule-caption">
      <strong>{{ summary(rule) }}</strong
      ><StatusBadge
        :text="trial ? '无副作用试算' : quote.blockers.length ? '待核对' : '用量快照可核对'"
        :tone="quote.blockers.length ? 'warning' : 'primary'"
      />
    </div>
    <div v-if="rule.scope === 'shared' && rule.minimumKind === 'included'" class="rental-stat-strip">
      <div>
        <span>整组共享额度</span><strong>{{ rule.includedCups.toLocaleString() }} <small>杯</small></strong>
      </div>
      <div>
        <span>计费组累计</span><strong>{{ cups?.toLocaleString() ?? '—' }} <small>杯</small></strong>
      </div>
      <div>
        <span>共享剩余</span><strong>{{ remaining?.toLocaleString() ?? '—' }} <small>杯</small></strong>
      </div>
      <div>
        <span>{{ confirmed ? '整组已确认对账金额' : trial ? '整组试算费用' : '整组预计费用' }}</span
        ><strong>{{ money(quote.totalCents) }}</strong>
      </div>
    </div>
    <CustomerAlert
      v-if="rule.scope === 'shared'"
      title="额度与费用只在计费组计算一次"
      description="下表展示各点位的用量贡献，不平均分配额度，也不重复生成点位应收。客户账单汇总方式不改变计算范围。"
    />
    <div class="table-scroll">
      <table class="data-table rental-contributions">
        <thead>
          <tr>
            <th>点位 / 归属</th>
            <th>{{ trial ? '试算输入' : '原始出杯' }}</th>
            <th>排除杯数</th>
            <th>可计费杯数</th>
            <th>口径状态</th>
          </tr>
        </thead>
        <tbody>
          <tr v-for="row in quote.contributions" :key="row.siteId">
            <td>
              <strong>{{ row.name }}</strong
              ><small>{{ row.siteId }}</small>
            </td>
            <td>{{ row.raw?.toLocaleString() ?? '—' }} 杯</td>
            <td>{{ row.excluded?.toLocaleString() ?? '—' }} 杯</td>
            <td class="rental-number">{{ row.cups?.toLocaleString() ?? '—' }} 杯</td>
            <td><StatusBadge :text="row.quality" :tone="row.cups === null ? 'warning' : 'neutral'" /></td>
          </tr>
        </tbody>
      </table>
    </div>
    <div class="rental-calculation">
      <h3>{{ rule.scope === 'shared' ? '计费组核算' : '点位独立核算' }}</h3>
      <div v-for="line in quote.lines" :key="line.key" class="rental-calculation-line">
        <div>
          <strong>{{ line.title }}</strong
          ><small v-if="rule.mode === 'included' && rule.minimumKind === 'included'"
            >基础费用 {{ money(line.baseCents) }} ＋ 超量 {{ line.overCups ?? '—' }} 杯 ×
            {{ money(rule.unitCents) }}</small
          ><small v-else-if="rule.mode === 'included'"
            >max（最低消费 {{ money(line.baseCents) }}，{{ line.cups ?? '—' }} 杯 ×
            {{ money(rule.unitCents) }}）</small
          ><small v-else-if="rule.mode === 'metered'"
            >{{ line.cups ?? '—' }} 杯 × {{ money(rule.unitCents) }}</small
          ><small v-else>完整账期固定费用；不按出杯量计租</small>
        </div>
        <strong>{{ money(line.totalCents) }}</strong>
      </div>
      <div class="rental-total">
        <span
          >{{ confirmed ? '已确认对账金额' : trial ? '试算合计' : '本期预计费用' }}
          <small>人民币 · 不含额外税费与调整</small></span
        ><strong>{{ money(quote.totalCents) }}</strong>
      </div>
    </div>
    <CustomerAlert v-if="quote.blockers.length" title="以下问题未解决，不能确认对账" tone="warning"
      ><p v-for="reason in quote.blockers" :key="reason">{{ reason }}</p></CustomerAlert
    >
    <p class="rental-help">
      {{
        trial
          ? '仅用于检查合同算法；输入杯数不是未来预测，不创建账单。'
          : '使用示例账期快照。预计费用、已确认对账与正式应收相互独立；本页不执行扣款。'
      }}
    </p>
  </div>
</template>
