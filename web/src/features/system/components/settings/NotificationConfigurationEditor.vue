<script setup lang="ts">
import { useI18n } from 'vue-i18n'
import { UiButton, UiInput, UiTextarea } from '@/ui/base'
import UiDialog from '@/ui/common/UiDialog.vue'
import NotificationDirectoryPicker from './NotificationDirectoryPicker.vue'
import type { TrustedSession } from '@/services/runtime/api'
import { messageLevels, type ConfigurationDraft, type MessageChannel, type MessageConfiguration } from '@/services/enterprise/notificationConfigurationRuntime'
const props = defineProps<{
  open: boolean; session: TrustedSession | null; draft: ConfigurationDraft; selected: MessageConfiguration | null
  channels: readonly MessageChannel[]; busy: boolean; recovery: '' | 'read' | 'write'; error: string; mustReload: boolean
}>()
const emit = defineEmits<{ change: [draft: ConfigurationDraft]; close: []; confirm: []; recover: []; reload: [] }>()
const { t } = useI18n()
function change(values: Partial<ConfigurationDraft>) { emit('change', { ...props.draft, ...values }) }
function toggle(field: 'levels' | 'channels', value: string, checked: boolean) {
  change({ [field]: checked ? [...props.draft[field], value] : props.draft[field].filter(item => item !== value) })
}
</script>
<template>
  <UiDialog :open="open" :title="t(selected ? 'notificationConfiguration.editTitle' : 'notificationConfiguration.createTitle')" width="780px" @close="emit('close')">
    <div class="notification-editor" :aria-busy="busy">
      <p v-if="error" class="notice-box danger" role="alert">{{ t(`notificationConfiguration.errors.${error}`) }}</p>
      <p v-if="recovery" class="notice-box warning" role="status">{{ t(recovery === 'read' ? 'notificationConfiguration.readPending' : 'notificationConfiguration.uncertain') }}</p>
      <fieldset :disabled="busy || Boolean(recovery) || mustReload" class="editor-fields">
        <div v-if="selected" class="fixed-scope"><strong>{{ selected.groupName || selected.groupId }} · {{ t(`notificationConfiguration.${selected.level}`) }}</strong><p>{{ t('notificationConfiguration.identityNote') }}</p></div>
        <NotificationDirectoryPicker v-else kind="groups" :session="session" :model-value="draft.groupId ? [draft.groupId] : []" :label="t('notificationConfiguration.group')" :disabled="busy || Boolean(recovery) || mustReload" @update:model-value="change({ groupId: $event[0] ?? '' })" />
        <fieldset v-if="!selected" class="choices"><legend>{{ t('notificationConfiguration.level') }}</legend>
          <label v-for="level in messageLevels" :key="level"><UiInput type="checkbox" :model-value="draft.levels.includes(level)" :aria-label="t(`notificationConfiguration.${level}`)" @update:model-value="toggle('levels', level, Boolean($event))" />{{ t(`notificationConfiguration.${level}`) }}</label>
          <p>{{ t('notificationConfiguration.levelsNote') }}</p>
        </fieldset>
        <fieldset class="choices"><legend>{{ t('notificationConfiguration.channelLabel') }}</legend>
          <label v-for="channel in channels" :key="channel.code" :class="{ unavailable: !channel.configurable }"><UiInput type="checkbox" :model-value="draft.channels.includes(channel.code)" :disabled="!channel.configurable" :aria-label="channel.name" @update:model-value="toggle('channels', channel.code, Boolean($event))" /><span>{{ channel.name }}<small v-if="!channel.configurable">{{ channel.unavailableReason }}</small></span></label>
        </fieldset>
        <div class="contact-columns">
          <NotificationDirectoryPicker kind="recipients" :session="session" :model-value="draft.primaryUserId ? [draft.primaryUserId] : []" :label="t('notificationConfiguration.primary')" :excluded="[draft.secondaryUserId, ...draft.additionalUserIds]" :disabled="busy || Boolean(recovery) || mustReload" @update:model-value="change({ primaryUserId: $event[0] ?? '' })" />
          <NotificationDirectoryPicker kind="recipients" :session="session" :model-value="draft.secondaryUserId ? [draft.secondaryUserId] : []" :label="t('notificationConfiguration.secondary')" :excluded="[draft.primaryUserId, ...draft.additionalUserIds]" :disabled="busy || Boolean(recovery) || mustReload" @update:model-value="change({ secondaryUserId: $event[0] ?? '' })" />
        </div>
        <NotificationDirectoryPicker kind="recipients" :session="session" :model-value="draft.additionalUserIds" :label="t('notificationConfiguration.additional')" multiple :excluded="[draft.primaryUserId, draft.secondaryUserId]" :disabled="busy || Boolean(recovery) || mustReload" @update:model-value="change({ additionalUserIds: $event })" />
        <label class="notes"><span>{{ t('notificationConfiguration.notes') }}</span><UiTextarea :model-value="draft.notes" :aria-label="t('notificationConfiguration.notes')" maxlength="500" @update:model-value="change({ notes: String($event ?? '') })" /></label>
        <p>{{ t('notificationConfiguration.fieldRules') }}</p>
      </fieldset>
    </div>
    <template #footer>
      <UiButton variant="outline" :disabled="busy" @click="emit('close')">{{ t(recovery ? 'notificationConfiguration.close' : 'notificationConfiguration.cancel') }}</UiButton>
      <UiButton v-if="recovery" :disabled="busy" @click="emit('recover')">{{ t('notificationConfiguration.recover') }}</UiButton>
      <UiButton v-else-if="mustReload" :disabled="busy" @click="emit('reload')">{{ t('notificationConfiguration.retry') }}</UiButton>
      <UiButton v-else :disabled="busy || !draft.groupId || !draft.primaryUserId || !draft.channels.length || !draft.levels.length" @click="emit('confirm')">{{ t(busy ? 'notificationConfiguration.saving' : 'notificationConfiguration.save') }}</UiButton>
    </template>
  </UiDialog>
</template>
<style scoped>
.notification-editor,.editor-fields { display:flex; flex-direction:column; gap:20px; }
.editor-fields,.choices { margin:0; padding:0; border:0; min-width:0; }
.contact-columns { display:grid; grid-template-columns:repeat(2,minmax(0,1fr)); gap:20px; }
.choices { display:flex; flex-wrap:wrap; gap:12px; }
.choices legend { float:left; width:100%; margin-bottom:8px; font-weight:600; font-size:var(--text-sm); }
.choices label { display:flex; align-items:center; gap:8px; padding:10px; border:1px solid var(--color-border); border-radius:var(--radius-sm); }
.choices input { width:18px; height:18px; padding:0; }
.choices small { display:block; max-width:230px; color:var(--color-text-muted); margin-top:4px; }
p,.choices label,.notes { font-size:var(--text-sm); line-height:1.6; }
p { color:var(--color-text-secondary); }
.notes span { display:block; font-weight:600; margin-bottom:8px; }
.notes textarea { min-height:90px; }
.notice-box { display:block; margin:0; }
.fixed-scope { padding:12px; border-radius:var(--radius-sm); background:var(--color-surface-soft); overflow-wrap:anywhere; }
@media(max-width:640px) { .contact-columns { grid-template-columns:1fr; } .choices label { width:100%; } }
</style>
