<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { UiButton, UiOption, UiSelect } from '@/ui/base'
import UiDialog from '@/ui/common/UiDialog.vue'
import Notice from '@/ui/common/Notice.vue'
import { prepareMemberPermissionCopy } from '@/services/memberPolicy'
import { useEnterpriseStore } from '@/stores/enterprise'
import { useUiStore } from '@/stores/ui'

const props = defineProps<{ open: boolean; targets: { id: string; version: number }[] }>()
const emit = defineEmits<{ close: []; saved: [] }>()
const store = useEnterpriseStore()
const ui = useUiStore()
const { t } = useI18n()
const sourceId = ref('')
const busy = ref(false)
const error = ref('')
const targetMembers = computed(() => props.targets.flatMap((target) => {
  const member = store.members.find((item) => item.id === target.id)
  return member ? [member] : []
}))
const sourceCandidates = computed(() => store.members.filter((member) =>
  member.status !== 'removed' && !props.targets.some((target) => target.id === member.id),
))
const source = computed(() => sourceCandidates.value.find((member) => member.id === sourceId.value) ?? null)
const policyError = computed(() => {
  if (!sourceId.value) return ''
  try {
    prepareMemberPermissionCopy(store.members, store.roles, sourceId.value, props.targets)
    return ''
  } catch (cause) {
    return cause instanceof Error ? cause.message : t('members.action.failure')
  }
})
const roleNames = (ids: string[]) => ids.length ? ids.map(store.roleName).join('、') : t('members.copyPermissions.noRoles')

watch(() => [props.open, props.targets] as const, () => {
  sourceId.value = ''
  busy.value = false
  error.value = ''
}, { deep: true })

async function submit() {
  error.value = ''
  if (!sourceId.value) {
    error.value = t('members.copyPermissions.sourceRequired')
    return
  }
  if (policyError.value) {
    error.value = policyError.value
    return
  }
  busy.value = true
  try {
    await store.copyMemberPermissions(sourceId.value, props.targets)
    ui.toast(
      store.previewMode
        ? t('members.copyPermissions.previewToast', { count: props.targets.length })
        : t('members.copyPermissions.serverToast'),
      'success',
    )
    emit('saved')
  } catch (cause) {
    error.value = cause instanceof Error ? cause.message : t('members.action.failure')
  } finally {
    busy.value = false
  }
}
</script>

<template>
  <UiDialog :open="open" :title="t('members.copyPermissions.title')" width="680px" @close="emit('close')">
    <div class="copy-permissions-stack" :aria-busy="busy">
      <Notice>
        {{ t('members.copyPermissions.description') }}
      </Notice>
      <label class="field">
        <span class="required">{{ t('members.copyPermissions.source') }}</span>
        <UiSelect v-model="sourceId" :aria-label="t('members.copyPermissions.source')">
          <UiOption value="">{{ t('members.copyPermissions.sourcePlaceholder') }}</UiOption>
          <UiOption v-for="member in sourceCandidates" :key="member.id" :value="member.id">
            {{ member.name }} · {{ roleNames(member.roleIds) }}
          </UiOption>
        </UiSelect>
      </label>
      <section class="copy-summary" :aria-label="t('members.copyPermissions.targets')">
        <div class="summary-heading"><h3>{{ t('members.copyPermissions.targets') }}</h3><span>{{ targetMembers.length }}</span></div>
        <ul class="member-summary-list">
          <li v-for="member in targetMembers" :key="member.id">
            <strong>{{ member.name }}</strong>
            <span>{{ t('members.copyPermissions.targetRoles', { name: member.name }) }}：{{ roleNames(member.roleIds) }}</span>
          </li>
        </ul>
      </section>
      <section v-if="source" class="copy-summary" :aria-label="t('members.copyPermissions.sourceRoles')">
        <div class="summary-heading"><h3>{{ t('members.copyPermissions.sourceRoles') }}</h3></div>
        <p>{{ source.name }}：{{ roleNames(source.roleIds) }}</p>
      </section>
      <Notice tone="warning">{{ t('members.copyPermissions.review') }}</Notice>
      <p v-if="policyError || error" class="form-error" role="alert">{{ policyError || error }}</p>
    </div>
    <template #footer>
      <UiButton variant="outline" :disabled="busy" @click="emit('close')">{{ t('common.cancel') }}</UiButton>
      <UiButton :disabled="busy || !sourceId || Boolean(policyError)" @click="submit">
        {{ busy ? t('common.processing') : t('members.copyPermissions.confirm') }}
      </UiButton>
    </template>
  </UiDialog>
</template>

<style scoped>
.copy-permissions-stack { display: flex; flex-direction: column; gap: 16px; }
.copy-summary { padding: 14px; border: 1px solid var(--color-border); border-radius: var(--radius-md); }
.summary-heading { display: flex; align-items: center; justify-content: space-between; gap: 12px; }
.summary-heading h3 { font-size: var(--text-sm); }
.summary-heading span { color: var(--color-text-secondary); font-size: var(--text-sm); }
.member-summary-list { display: flex; flex-direction: column; gap: 10px; margin: 12px 0 0; padding: 0; list-style: none; }
.member-summary-list li { display: flex; flex-direction: column; gap: 3px; }
.member-summary-list strong { font-size: var(--text-sm); }
.member-summary-list span, .copy-summary p { color: var(--color-text-secondary); font-size: var(--text-sm); line-height: 1.6; }
.copy-summary p { margin-top: 10px; }
</style>
