<script setup lang="ts">
import { computed, onBeforeUnmount, ref, shallowRef, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { UiButton, UiInput, UiOption, UiSelect } from '@/ui/base'
import { sessionContext, type TrustedSession } from '@/services/runtime/api'
import { listMessageDirectory, notificationConfigurationErrorKey, type MessageDirectoryEntry, type MessagePage } from '@/services/enterprise/notificationConfigurationRuntime'
const props = withDefaults(defineProps<{
  kind: 'groups' | 'recipients'; session: TrustedSession | null; label: string; modelValue: readonly string[]
  multiple?: boolean; disabled?: boolean; excluded?: readonly string[]
}>(), { multiple: false, disabled: false, excluded: () => [] })
const emit = defineEmits<{ 'update:modelValue': [value: string[]] }>()
const { t } = useI18n(), search = ref(''), applied = ref(''), currentPage = ref(1), busy = ref(false), error = ref('')
const result = shallowRef<MessagePage<MessageDirectoryEntry> | null>(null), names = ref<Record<string, string>>({})
const options = computed(() => result.value?.items ?? [])
const currentValue = computed(() => props.multiple ? '' : props.modelValue[0] ?? '')
const offPage = computed(() => props.modelValue.filter(id => !options.value.some(row => row.id === id)))
let sequence = 0
function selectionLabel(id: string) { return names.value[id] ?? t('notificationConfiguration.unavailableSelection', { id }) }
async function load() {
  if (!props.session) return
  const token = ++sequence, session = { ...props.session }, key = sessionContext(session)
  busy.value = true; error.value = ''; result.value = null
  try {
    const page = await listMessageDirectory(session, props.kind, applied.value, currentPage.value, 20)
    if (token !== sequence || !props.session || sessionContext(props.session) !== key) return
    result.value = page
    for (const row of page.items) names.value[row.id] = row.name
  } catch (cause) { if (token === sequence) error.value = notificationConfigurationErrorKey(cause) }
  finally { if (token === sequence) busy.value = false }
}
function select(value: unknown) {
  if (props.disabled || busy.value) return
  const id = String(value ?? '')
  if (!id) { if (!props.multiple) emit('update:modelValue', []); return }
  if (props.excluded.includes(id) || !options.value.some(row => row.id === id)) return
  if (props.multiple) {
    if (!props.modelValue.includes(id) && props.modelValue.length < 100) emit('update:modelValue', [...props.modelValue, id])
  } else emit('update:modelValue', [id])
}
function remove(id: string) { if (!props.disabled) emit('update:modelValue', props.modelValue.filter(value => value !== id)) }
function find() { applied.value = search.value; currentPage.value = 1; void load() }
function turn(direction: number) { currentPage.value += direction; void load() }
watch(() => [props.kind, props.session ? sessionContext(props.session) : ''], () => {
  sequence++; search.value = ''; applied.value = ''; currentPage.value = 1; result.value = null; names.value = {}; error.value = ''
  if (props.session) void load()
}, { immediate: true, flush: 'sync' })
onBeforeUnmount(() => sequence++)
</script>
<template>
  <div class="directory-picker" :aria-label="props.label" role="group" :aria-busy="busy">
    <strong class="field-label">{{ props.label }}</strong>
    <div class="directory-search">
      <UiInput v-model="search" :aria-label="`${props.label} ${t('notificationConfiguration.directoryQuery')}`" :placeholder="t('notificationConfiguration.directoryQuery')" :disabled="disabled || busy" maxlength="200" @keydown.enter.prevent="find" />
      <UiButton variant="outline" :disabled="disabled || busy" @click="find">{{ t('notificationConfiguration.search') }}</UiButton>
    </div>
    <UiSelect :model-value="currentValue" :aria-label="props.label" :disabled="disabled || busy || Boolean(error)" :placeholder="t('notificationConfiguration.choose')" @update:model-value="select">
      <UiOption value="">{{ t('notificationConfiguration.optional') }}</UiOption>
      <UiOption v-for="id in offPage" :key="`selected-${id}`" :value="id" disabled>{{ selectionLabel(id) }}</UiOption>
      <UiOption v-for="row in options" :key="row.id" :value="row.id" :disabled="excluded.includes(row.id) || (multiple && modelValue.includes(row.id))">{{ row.name }}</UiOption>
    </UiSelect>
    <div v-if="multiple && modelValue.length" class="selected-recipients">
      <UiButton v-for="id in modelValue" :key="id" size="sm" variant="outline" :disabled="disabled" :aria-label="t('notificationConfiguration.removeChoice', { name: selectionLabel(id) })" @click="remove(id)">{{ selectionLabel(id) }} ×</UiButton>
    </div>
    <p v-if="error" class="text-danger" role="alert">{{ t(`notificationConfiguration.errors.${error}`) }} <UiButton size="sm" variant="outline" :disabled="disabled || busy" @click="load">{{ t('notificationConfiguration.retry') }}</UiButton></p>
    <div v-else-if="result" class="directory-pagination">
      <small>{{ t('notificationConfiguration.directoryTotal', { total: result.total, page: result.page }) }}</small>
      <UiButton size="sm" variant="ghost" :disabled="disabled || busy || currentPage <= 1" @click="turn(-1)">{{ t('notificationConfiguration.previous') }}</UiButton>
      <UiButton size="sm" variant="ghost" :disabled="disabled || busy || currentPage * 20 >= result.total" @click="turn(1)">{{ t('notificationConfiguration.next') }}</UiButton>
    </div>
  </div>
</template>
<style scoped>
.directory-picker { display:flex; flex-direction:column; gap:8px; min-width:0; }
.field-label { font-size:var(--text-sm); }
.directory-search { display:flex; gap:8px; }
.directory-search input { min-width:0; flex:1; }
.directory-pagination { display:flex; align-items:center; flex-wrap:wrap; gap:4px; }
.directory-pagination small { flex:1; min-width:90px; color:var(--color-text-muted); font-size:var(--text-xs); }
.selected-recipients { display:flex; flex-wrap:wrap; gap:6px; }
.selected-recipients button { max-width:100%; white-space:normal; overflow-wrap:anywhere; height:auto; min-height:32px; }
p { font-size:var(--text-sm); line-height:1.5; }
</style>
