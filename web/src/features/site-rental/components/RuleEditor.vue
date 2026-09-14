<script setup lang="ts">
import { ref, watch, computed } from 'vue'
import { useCustomerStore } from '@/stores/customer'
import { rentalState, modeNames, nextPeriod, summary } from '@/services/siteRental/model'
import { draftFingerprint, validateRule } from '@/services/siteRental/policy'
import { quoteRental } from '@/services/siteRental/quote'
import type { RentalDraft, RentalQuote, RentalRule } from '@/types/siteRental'
import UiDialog from '@/components/ui/UiDialog.vue'
import CustomerAlert from '@/components/customer/CustomerAlert.vue'
import QuoteSummary from './QuoteSummary.vue'
const props = defineProps<{ open: boolean; groupId?: string }>(),
  emit = defineEmits<{ close: []; published: [id: string] }>()
const store = useCustomerStore(),
  draft = ref<RentalDraft>(),
  step = ref(1),
  error = ref(''),
  discard = ref(false),
  initial = ref(''),
  revision = ref(0),
  tenant = ref(''),
  quote = ref<RentalQuote>(),
  base = ref(''),
  price = ref('')
const state = computed(() => rentalState(store.snapshot)),
  sites = computed(() =>
    state.value.profiles.filter(
      (x) => x.customerId === draft.value?.rule.customerId && x.kind === 'site' && x.phase !== '已撤场',
    ),
  )
const contracts = computed(() =>
  store.snapshot.sources.filter(
    (x) => x.kind === 'contract' && x.customerId === draft.value?.rule.customerId && x.verified,
  ),
)
const previous = computed(
  () => state.value.rules.filter((x) => x.groupId === props.groupId).sort((a, b) => b.version - a.version)[0],
)
watch(
  () => props.open,
  (open) => {
    if (!open) return
    store.refreshRental()
    const saved = props.groupId ? state.value.drafts[props.groupId] : undefined
    const old = previous.value
    const rule: RentalRule = old
      ? {
          ...JSON.parse(JSON.stringify(old)),
          id: '',
          version: old.version + 1,
          effectiveFrom: nextPeriod(),
          reason: '',
        }
      : {
          id: '',
          groupId: `BG-${crypto.randomUUID().slice(0, 8)}`,
          version: 1,
          name: '',
          customerId: '',
          contractId: '',
          siteIds: [],
          mode: 'fixed',
          scope: '' as RentalRule['scope'],
          fixedUnit: 'site',
          baseCents: 0,
          unitCents: 160,
          includedCups: 1000,
          minimumKind: 'included',
          billingOwner: '',
          billPresentation: '客户汇总',
          effectiveFrom: nextPeriod(),
          reason: '',
        }
    draft.value = saved ? JSON.parse(JSON.stringify(saved)) : { rule, samples: {}, testedFingerprint: '' }
    for (const id of draft.value!.rule.siteIds)
      draft.value!.samples[id] ??= state.value.usage.find((x) => x.siteId === id)?.raw ?? 0
    base.value = String(draft.value!.rule.baseCents / 100)
    price.value = String(draft.value!.rule.unitCents / 100)
    revision.value = store.snapshot.revision
    tenant.value = store.snapshot.tenant
    initial.value = JSON.stringify(draft.value)
    step.value = 1
    error.value = ''
    discard.value = false
    quote.value = undefined
  },
  { immediate: true },
)
function customerChanged() {
  if (!draft.value) return
  draft.value.rule.contractId = ''
  draft.value.rule.siteIds = []
  draft.value.rule.billingOwner = store.customerName(draft.value.rule.customerId)
}
function cents(value: string) {
  if (!/^\d+(\.\d{1,2})?$/.test(value)) throw new Error('金额最多保留两位小数，不允许负数或空值')
  return Math.round(Number(value) * 100)
}
function sync() {
  if (!draft.value) return
  draft.value.rule.baseCents = cents(base.value)
  draft.value.rule.unitCents = cents(price.value)
  draft.value.rule.includedCups = Number(draft.value.rule.includedCups)
  for (const id of draft.value.rule.siteIds) draft.value.samples[id] ??= 0
}
function preview() {
  try {
    sync()
    validateRule(store.snapshot, state.value, draft.value!.rule)
    quote.value = quoteRental(
      store.snapshot,
      state.value,
      draft.value!.rule,
      draft.value!.rule.effectiveFrom.slice(0, 7),
      draft.value!.samples,
    )
    draft.value!.testedFingerprint = draftFingerprint(draft.value!)
    error.value = ''
    step.value = 2
  } catch (e) {
    error.value = (e as Error).message
  }
}
function save(publish = false) {
  try {
    sync()
    const result = store.runRental({
      tenant: tenant.value,
      expectedRevision: revision.value,
      key: crypto.randomUUID(),
      action: publish
        ? { type: 'publish-rule', draft: draft.value! }
        : { type: 'save-draft', draft: draft.value! },
    })
    if (publish) {
      emit('published', result.target)
      emit('close')
    } else {
      revision.value = store.snapshot.revision
      initial.value = JSON.stringify(draft.value)
      error.value = ''
      emit('close')
    }
  } catch (e) {
    error.value = (e as Error).message
  }
}
function close() {
  if (
    draft.value &&
    (JSON.stringify(draft.value) !== initial.value ||
      base.value !== String(draft.value.rule.baseCents / 100) ||
      price.value !== String(draft.value.rule.unitCents / 100))
  )
    discard.value = true
  else emit('close')
}
function reread() {
  store.refreshRental()
  revision.value = store.snapshot.revision
  step.value = 1
  if (draft.value) draft.value.testedFingerprint = ''
  error.value = '已重新读取。请检查当前版本、生效范围并重新试算。'
}
</script>
<template>
  <UiDialog
    :open="open"
    :title="groupId ? '变更计费规则' : '新建计费规则'"
    width="790px"
    drawer
    @close="close"
  >
    <div v-if="draft" class="rental-form page-stack">
      <div class="rental-step-tabs">
        <span :class="{ active: step === 1 }">1 配置与试算输入</span
        ><span :class="{ active: step === 2 }">2 影响确认与预约发布</span>
      </div>
      <template v-if="step === 1">
        <CustomerAlert
          title="规则属于合同，不是点位永久属性"
          description="先明确计费范围，再试算。发布新版本不重算历史；本轮月中起租、停用和变更需要人工核对约定。"
        />
        <div class="rental-field-grid">
          <label class="rental-full"
            >计费规则 / 组名称 <b>*</b><input v-model="draft.rule.name" required maxlength="80"
          /></label>
          <label
            >关联客户 <b>*</b
            ><select v-model="draft.rule.customerId" :disabled="!!previous" @change="customerChanged">
              <option value="" disabled>请选择客户</option>
              <option
                v-for="c in store.snapshot.customers.filter((x) => !x.archived)"
                :key="c.id"
                :value="c.id"
              >
                {{ c.name }}
              </option>
            </select></label
          >
          <label
            >有效合同来源 <b>*</b
            ><select v-model="draft.rule.contractId" :disabled="!!previous">
              <option value="" disabled>请选择已核验合同</option>
              <option v-for="c in contracts" :key="c.id" :value="c.id">{{ c.id }} · {{ c.title }}</option>
            </select></label
          >
          <label
            >计费模式 <b>*</b
            ><select v-model="draft.rule.mode" @change="draft.rule.scope = '' as RentalRule['scope']">
              <option v-for="(label, id) in modeNames" :key="id" :value="id">{{ label }}</option>
            </select></label
          >
          <label
            >计费范围 <b>*</b
            ><select v-model="draft.rule.scope">
              <option disabled value="">必须明确选择</option>
              <option value="independent">各点位独立计算</option>
              <option v-if="draft.rule.mode === 'included'" value="shared">同合同指定点位共享</option>
            </select></label
          >
          <label v-if="draft.rule.mode === 'included'" class="rental-full"
            >保底算法 <b>*</b
            ><select v-model="draft.rule.minimumKind">
              <option value="included">基础费用含杯数 ＋ 超量计费</option>
              <option value="minimum-spend">最低消费金额（与含杯数算法分开）</option>
            </select></label
          >
          <label v-if="draft.rule.mode === 'fixed'"
            >月租收费单位<select v-model="draft.rule.fixedUnit">
              <option value="site">元 / 点位 / 月</option>
              <option value="device">元 / 在租设备 / 月</option>
            </select></label
          >
          <label v-if="draft.rule.mode !== 'metered'"
            >{{
              draft.rule.mode === 'fixed'
                ? '固定月租'
                : draft.rule.minimumKind === 'included'
                  ? '基础费用'
                  : '最低消费金额'
            }}（元）<input v-model="base" type="number" min="0" step="0.01"
          /></label>
          <label v-if="draft.rule.mode !== 'fixed'"
            >{{
              draft.rule.mode === 'included' && draft.rule.minimumKind === 'included'
                ? '超量单价'
                : '可计费单价'
            }}（元 / 杯）<input v-model="price" type="number" min="0.01" step="0.01"
          /></label>
          <label v-if="draft.rule.mode === 'included' && draft.rule.minimumKind === 'included'"
            >{{ draft.rule.scope === 'shared' ? '整组共享' : '每个点位独立' }}含杯额度（杯）<input
              v-model.number="draft.rule.includedCups"
              type="number"
              min="1"
              step="1"
          /></label>
        </div>
        <fieldset class="rental-site-selector">
          <legend>指定点位与试算杯数 <b>*</b></legend>
          <p>试算输入不作为未来预测。只列同客户实际服务点位；共享组不平均分配额度。</p>
          <div v-for="site in sites" :key="site.id" class="rental-selector-row">
            <label
              ><input v-model="draft.rule.siteIds" type="checkbox" :value="site.id" /> {{ site.name }}
              <small>{{ site.id }}</small></label
            ><input
              v-if="draft.rule.siteIds.includes(site.id)"
              v-model.number="draft.samples[site.id]"
              type="number"
              min="0"
              step="1"
              :aria-label="`${site.id} 试算杯数`"
              placeholder="试算杯数"
            />
          </div>
          <p v-if="!sites.length">请先选择客户。</p>
        </fieldset>
        <div class="rental-field-grid">
          <label>结算主体 <b>*</b><input v-model="draft.rule.billingOwner" /></label
          ><label
            >账单汇总方式<select v-model="draft.rule.billPresentation">
              <option>客户汇总</option>
              <option>分别结算</option>
            </select></label
          ><label
            >预约生效日期 <b>*</b><input v-model="draft.rule.effectiveFrom" type="date" /><small
              >仅支持下一账期或以后的月初</small
            ></label
          ><label>币种与账期<input value="CNY · 自然月 · Asia/Shanghai" readonly /></label
          ><label class="rental-full"
            >合同约定 / 变更依据 <b>*</b
            ><textarea v-model="draft.rule.reason" rows="2" placeholder="记录已确认的条款与本次变更原因" />
          </label>
        </div>
      </template>
      <template v-else-if="quote">
        <CustomerAlert
          title="确认变更范围后再发布"
          :description="`v${draft.rule.version} · ${draft.rule.effectiveFrom} 生效 · ${draft.rule.siteIds.length} 个点位；旧账期保持原规则。`"
        />
        <div class="rental-diff">
          <div>
            <span>当前约定</span><strong>{{ previous ? summary(previous) : '尚未建立' }}</strong>
          </div>
          <div>
            <span>本次约定</span><strong>{{ summary(draft.rule) }}</strong>
          </div>
        </div>
        <QuoteSummary :quote="quote" :rule="draft.rule" trial />
        <p class="rental-help">
          结算主体：{{ draft.rule.billingOwner }} · {{ draft.rule.billPresentation }}。不生成正式应收，不修改
          SaaS 套餐额度。
        </p>
      </template>
      <p v-if="error" class="rental-error" role="alert">
        {{ error }} <button class="btn-link" @click="reread">重新读取</button>
      </p>
      <CustomerAlert v-if="discard" title="草稿尚未保存" tone="warning"
        ><button class="btn" @click="discard = false">继续编辑</button>
        <button class="btn btn-danger" @click="emit('close')">放弃修改</button></CustomerAlert
      >
    </div>
    <template #footer
      ><button class="btn" @click="close">取消</button
      ><button class="btn" @click="save(false)">保存规则草稿</button
      ><button v-if="step === 1" class="btn btn-primary" @click="preview">试算并预览影响</button
      ><template v-else
        ><button class="btn" @click="step = 1">返回配置</button
        ><button class="btn btn-primary" @click="save(true)">确认预约发布</button></template
      ></template
    >
  </UiDialog>
</template>
