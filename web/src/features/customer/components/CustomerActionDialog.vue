<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { useRouter } from 'vue-router'
import { useCustomerStore } from '@/stores/customer'
import { useUiStore } from '@/stores/ui'
import { actionDefinitions, initialValues } from '@/services/customer/forms'
import { closeRequirements, canArchive } from '@/services/customer/policy'
import { workKindNames } from '@/services/customer/seed'
import type { ActionId } from '@/services/customer/commands'
import type { FormValues } from '@/types/customer'
import UiDialog from '@/components/ui/UiDialog.vue'
import StatusBadge from '@/components/ui/StatusBadge.vue'
import CustomerAlert from './CustomerAlert.vue'
import FormFields from './FormFields.vue'
import EvidencePicker from './EvidencePicker.vue'
import PaymentSnapshot from './PaymentSnapshot.vue'
const props = defineProps<{
  open: boolean
  action: ActionId
  id: string
  selected: string[]
  defaults: FormValues
}>()
const emit = defineEmits<{ close: [] }>()
const store = useCustomerStore(),
  ui = useUiStore(),
  router = useRouter()
const values = ref<FormValues>({}),
  version = ref<number>(),
  error = ref(''),
  unsaved = ref(false),
  busy = ref(false),
  key = ref(''),
  initial = ref('')
const definition = computed(() => {
  const base = actionDefinitions[props.action]
  return props.action === 'reschedule' && work.value?.kind === 'payment'
    ? {
        ...base!,
        title: '核对回款结果',
        primary: '保存跟进',
        description: '核对财务只读来源并安排下一次跟进，不能直接修改核销金额。',
      }
    : base
})
const work = computed(() => store.snapshot.work.find((w) => w.id === props.id))
const customer = computed(() =>
  store.snapshot.customers.find((c) => c.id === (work.value?.customerId || props.id)),
)
const currentVersion = computed(
  () =>
    work.value?.version ??
    customer.value?.version ??
    store.snapshot.plans.find((p) => p.id === props.id)?.version,
)
const duplicate = computed(() =>
  props.action === 'create-customer'
    ? store.snapshot.customers.find((c) => c.name.trim() === String(values.value.name || '').trim())
    : undefined,
)
const blockers = computed(() => (props.action === 'archive' ? canArchive(store.snapshot, props.id) : []))
const sources = computed(() =>
  store.snapshot.sources.filter(
    (s) => s.customerId === work.value?.customerId && (!s.facts.workId || s.facts.workId === work.value?.id),
  ),
)
const relevantSources = computed(() =>
  sources.value.filter((s) =>
    ({
      delivery: ['acceptance', 'placement'],
      service: ['service', 'recovery', 'confirmation'],
      payment: ['receivable'],
      renewal: ['contract'],
      return: ['recovery', 'settlement', 'termination'],
      visit: ['confirmation'],
      improvement: ['confirmation', 'acceptance'],
    })[work.value?.kind || 'visit'].includes(s.kind),
  ),
)
const missing = computed(() => {
  if (props.action !== 'accept' || !work.value) return []
  try {
    return closeRequirements(
      store.snapshot,
      work.value,
      String(values.value.evidence || '')
        .split(',')
        .filter(Boolean),
    )
  } catch (e) {
    return [(e as Error).message]
  }
})
const labels = computed(() =>
  Object.fromEntries([
    ...Object.entries(workKindNames),
    ...store.snapshot.customers.map((c) => [c.id, `${c.name} · ${c.id}`]),
    ...store.snapshot.work.map((w) => [w.id, `${w.id} · ${w.title}`]),
    ...store.snapshot.contacts.map((c) => [c.id, `${c.name} · ${c.role}`]),
    ...sources.value.map((s) => [s.id, `${s.id} · ${s.title}`]),
  ]),
)
const fields = computed(
  () =>
    definition.value?.fields
      .filter((f) => f.key !== 'evidence')
      .filter(
        (f) =>
          !(
            ['visit', 'recap', 'nonrenewal', 'triage'].includes(props.action) &&
            ['title', 'owner', 'deadline', 'nextAt', 'nextAction'].includes(f.key) &&
            !values.value.followup &&
            !(props.action === 'triage' && values.value.result === '创建经营检查事项')
          ),
      )
      .map((f) =>
        f.key === 'sourceId'
          ? { ...f, options: sources.value.filter((s) => s.verified).map((s) => s.id) }
          : f.key === 'customerId'
            ? { ...f, options: store.snapshot.customers.filter((c) => !c.archived).map((c) => c.id) }
            : f,
      ) || [],
)
watch(
  () => [props.open, props.action, props.id] as const,
  () => {
    if (!props.open) return
    values.value = {
      ...initialValues(props.action, props.id, store.snapshot),
      ...store.snapshot.drafts[`${props.action}:${props.id}`],
      ...props.defaults,
    }
    version.value = currentVersion.value
    error.value = ''
    unsaved.value = false
    busy.value = false
    key.value = crypto.randomUUID()
    initial.value = JSON.stringify(values.value)
  },
  { immediate: true },
)
function close(force = false) {
  if (!force && JSON.stringify(values.value) !== initial.value) {
    unsaved.value = true
    return
  }
  emit('close')
}
function saveDraft() {
  try {
    store.saveDraft(`${props.action}:${props.id}`, values.value)
    initial.value = JSON.stringify(values.value)
    ui.toast('草稿已保存在当前租户的本地预览中')
    emit('close')
  } catch (e) {
    error.value = (e as Error).message
  }
}
function submit() {
  busy.value = true
  error.value = ''
  try {
    const result = store.run({
      action: props.action,
      id: props.id,
      values: values.value,
      version: version.value,
      selected: props.selected,
      key: key.value,
    })
    emit('close')
    ui.toast(result.detail)
    if (result.navigate) void router.push(result.navigate)
  } catch (e) {
    error.value = (e as Error).message
  } finally {
    busy.value = false
  }
}
function useExisting() {
  if (duplicate.value) {
    emit('close')
    void router.push(`/customers/accounts/${duplicate.value.id}`)
  }
}
</script>
<template>
  <UiDialog
    :open="open"
    :title="definition?.title || '业务操作'"
    :width="definition?.drawer ? '620px' : '700px'"
    :drawer="definition?.drawer"
    @close="close()"
    ><div v-if="definition" class="customer-dialog">
      <p class="dialog-description">{{ definition.description }}</p>
      <div v-if="work || customer" class="customer-form-context">
        <div class="row-between">
          <strong>{{ work?.id || customer?.id }} · {{ work?.title || customer?.name }}</strong
          ><StatusBadge v-if="work" :text="work.status" tone="primary" />
        </div>
        <p class="customer-help">
          {{ customer?.name
          }}<template v-if="work">
            · {{ workKindNames[work.kind] }} · 流程 v{{ work.workflowVersion }} · 第
            {{ work.cycle }} 轮处理</template
          ><template v-else> · 资料版本 v{{ customer?.version }}</template>
        </p>
      </div>
      <CustomerAlert
        v-if="duplicate"
        title="发现已有同名客户，建议直接使用"
        :description="`${duplicate.name} · ${duplicate.id} · ${duplicate.owner}`"
        tone="warning"
        ><button class="btn-link" type="button" @click="useExisting">使用已有客户</button></CustomerAlert
      >
      <CustomerAlert
        v-if="blockers.length"
        title="当前不能归档"
        :description="blockers.join('；')"
        tone="danger"
      />
      <div v-if="action === 'assign'" class="customer-form-context">
        <strong>本次选择 {{ selected.length }} 项</strong>
        <div v-for="selectedId in selected" :key="selectedId" class="row-between">
          <span>{{ selectedId }} · {{ store.snapshot.work.find((w) => w.id === selectedId)?.title }}</span
          ><StatusBadge
            :text="store.snapshot.work.find((w) => w.id === selectedId)?.writable ? '可分配' : '无编辑权限'"
            :tone="store.snapshot.work.find((w) => w.id === selectedId)?.writable ? 'success' : 'danger'"
          />
        </div>
      </div>
      <PaymentSnapshot
        v-if="action === 'reschedule' && work?.kind === 'payment'"
        :work="work"
        style="margin-bottom: 20px"
      />
      <form id="customer-action-form" @submit.prevent="submit">
        <EvidencePicker
          v-if="action === 'accept'"
          :model-value="String(values.evidence || '')"
          :sources="relevantSources"
          @update:model-value="values.evidence = $event"
        /><CustomerAlert
          v-if="missing.length"
          title="还不能成功关闭"
          :description="missing.join('；')"
          tone="warning"
        />
        <div v-if="missing.length || duplicate || blockers.length" style="height: 16px" />
        <FormFields v-model="values" :fields="fields" :labels="labels" />
        <CustomerAlert
          v-if="definition.note || definition.warning"
          :title="definition.warning || '业务边界'"
          :description="definition.note"
          :tone="definition.warning ? 'warning' : 'primary'"
        />
        <div v-if="error" class="form-error" role="alert">{{ error }}</div>
        <CustomerAlert
          v-if="error.includes('草稿已保留')"
          class="conflict"
          title="并发更新，未覆盖任何记录"
          :description="`你的编辑版本 v${version} · 最新版本 v${currentVersion}`"
          tone="warning"
          ><button
            class="btn"
            type="button"
            @click="
              () => {
                version = currentVersion
                error = ''
              }
            "
          >
            读取最新版本并保留草稿
          </button></CustomerAlert
        >
        <CustomerAlert
          v-if="unsaved"
          title="存在未保存的修改"
          description="保存草稿后关闭，或明确放弃本次修改。"
          tone="warning"
          class="unsaved"
          ><div class="row" style="margin-top: 10px">
            <button class="btn" type="button" @click="unsaved = false">继续编辑</button
            ><button class="btn btn-danger" type="button" @click="close(true)">放弃修改并关闭</button>
          </div></CustomerAlert
        >
      </form>
    </div>
    <template #footer
      ><button class="btn" @click="close()">取消</button
      ><button v-if="definition?.fields.length" class="btn" @click="saveDraft">保存草稿</button
      ><button
        type="submit"
        form="customer-action-form"
        class="btn btn-primary"
        :disabled="busy || Boolean(duplicate) || blockers.length > 0 || missing.length > 0"
      >
        {{ busy ? '正在保存…' : definition?.primary }}
      </button></template
    ></UiDialog
  >
</template>
