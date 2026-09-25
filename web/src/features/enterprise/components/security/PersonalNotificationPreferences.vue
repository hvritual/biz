<script setup lang="ts">
import { computed, onBeforeUnmount, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { UiButton, UiInput } from '@/ui/base'
import UiDialog from '@/ui/common/UiDialog.vue'
import AppIcon from '@/ui/common/AppIcon.vue'
import { useEnterpriseStore } from '@/stores/enterprise'
import { usePersonalProfileStore } from '@/stores/personalProfile'
import { sessionContext } from '@/services/runtime/api'
import { subscribeSessionContextChange } from '@/services/runtime/sessionCoordinator'
import type { PreferenceChannel } from '@/services/enterprise/notificationPreferencesRuntime'
import { useNotificationPreferences } from '../../composables/useNotificationPreferences'

const enterprise = useEnterpriseStore(), personal = usePersonalProfileStore(), { t } = useI18n()
const flow = useNotificationPreferences(() => enterprise.session)
const { snapshot, draft, selected, busy, dialogOpen, error, success, recovery, canEdit } = flow
const channels: PreferenceChannel[] = ['sms', 'email']
const reloadRequired = computed(() => error.value === 'sessionChanged' || error.value === 'signIn')
function retry() { if (reloadRequired.value) window.location.reload(); else void flow.load() }
watch(() => [enterprise.sourceKind, enterprise.session?.authenticated,
  enterprise.session ? sessionContext(enterprise.session) : ''], () => {
  flow.invalidate()
  if (enterprise.sourceKind === 'api') void flow.load()
}, { immediate: true, flush: 'sync' })
const unsubscribe = subscribeSessionContextChange(() => flow.invalidate('sessionChanged'))
function hidePage() { flow.invalidate() }
window.addEventListener('pagehide', hidePage)
onBeforeUnmount(() => { unsubscribe(); window.removeEventListener('pagehide', hidePage); flow.invalidate() })
</script>

<template>
  <section class="card preference-panel" aria-labelledby="notification-preference-heading" :aria-busy="busy">
    <div class="preference-heading">
      <AppIcon name="bell" :size="22" />
      <div><h2 id="notification-preference-heading">{{ t('notificationPreferences.title') }}</h2><p>{{ t('notificationPreferences.description') }}</p></div>
    </div>
    <p class="scope-note">{{ t('notificationPreferences.scope', { tenant: personal.profile?.tenantName, user: personal.profile?.name || personal.profile?.username }) }}</p>
    <p v-if="busy && !snapshot" role="status">{{ t('notificationPreferences.loading') }}</p>
    <p v-if="error" class="notice-box danger" role="alert">{{ t(`notificationPreferences.errors.${error}`) }}</p>
    <p v-if="success" class="notice-box success" role="status">{{ t('notificationPreferences.saved') }}</p>
    <template v-if="snapshot">
      <div v-for="channel in channels" :key="channel" class="preference-row">
        <label :for="`personal-preference-${channel}`">
          <strong>{{ t(`notificationPreferences.${channel}`) }}</strong>
          <small>{{ t(`notificationPreferences.states.${snapshot[channel].state}`) }}</small>
        </label>
        <div class="preference-value">
          <span>{{ t(snapshot[channel].allowed ? 'notificationPreferences.enabled' : 'notificationPreferences.disabled') }}</span>
          <UiInput :id="`personal-preference-${channel}`" type="checkbox" role="switch"
            :model-value="draft[channel]" :disabled="!canEdit"
            :aria-label="t(`notificationPreferences.${channel}`)"
            @update:model-value="flow.begin(channel, Boolean($event))" />
        </div>
      </div>
    </template>
    <p class="policy-note">{{ t('notificationPreferences.policyNote') }}</p>
    <div v-if="recovery" class="notice-box warning recovery-box" role="status">
      <p>{{ t(recovery === 'read' ? 'notificationPreferences.readbackPending' : 'notificationPreferences.uncertain') }}</p>
      <UiButton variant="outline" :disabled="busy" @click="flow.recover">{{ t('notificationPreferences.recover') }}</UiButton>
    </div>
    <UiButton v-else-if="!busy && (error || !snapshot)" variant="outline" @click="retry">
      {{ t(reloadRequired ? 'notificationPreferences.reload' : 'notificationPreferences.retry') }}
    </UiButton>
  </section>
  <UiDialog :open="dialogOpen" :title="t('notificationPreferences.confirmTitle')" @close="flow.close">
    <div class="preference-confirmation" :aria-busy="busy">
      <p>{{ t('notificationPreferences.scope', { tenant: personal.profile?.tenantName, user: personal.profile?.name || personal.profile?.username }) }}</p>
      <p v-if="selected" class="confirmation-impact">{{ t(selected.allowed ? 'notificationPreferences.confirmEnable' : 'notificationPreferences.confirmDisable', { channel: t(`notificationPreferences.${selected.channel}`) }) }}</p>
      <p>{{ t('notificationPreferences.confirmNote') }}</p>
      <p v-if="error" class="notice-box danger" role="alert">{{ t(`notificationPreferences.errors.${error}`) }}</p>
      <p v-if="recovery" class="notice-box warning">{{ t(recovery === 'read' ? 'notificationPreferences.readbackPending' : 'notificationPreferences.uncertain') }}</p>
      <p v-if="busy" role="status">{{ t('notificationPreferences.saving') }}</p>
    </div>
    <template #footer>
      <UiButton variant="outline" :disabled="busy" @click="flow.close">{{ t(recovery ? 'notificationPreferences.close' : 'notificationPreferences.cancel') }}</UiButton>
      <UiButton :disabled="busy" @click="recovery ? flow.recover() : flow.confirm()">{{ t(recovery ? 'notificationPreferences.recover' : 'notificationPreferences.confirm') }}</UiButton>
    </template>
  </UiDialog>
</template>

<style scoped>
.preference-panel { padding: 22px; display: flex; flex-direction: column; gap: 16px; }
.preference-heading { display: flex; align-items: flex-start; gap: 12px; }
h2 { font-size: 18px; }
p, small { color: var(--color-text-secondary); font-size: var(--text-sm); line-height: 1.6; }
.scope-note { overflow-wrap: anywhere; }
.preference-row { display: flex; align-items: center; justify-content: space-between; gap: 20px; padding-top: 16px; border-top: 1px solid var(--color-border); }
.preference-row label { min-width: 0; }
.preference-row strong, .preference-row small { display: block; }
.preference-row strong { font-size: var(--text-sm); }
.preference-row small { margin-top: 4px; }
.preference-value { display: flex; align-items: center; gap: 10px; flex-shrink: 0; font-size: var(--text-sm); }
.preference-value input { width: 22px; height: 22px; padding: 0; }
.policy-note { padding-top: 12px; border-top: 1px solid var(--color-border); }
.preference-panel .notice-box, .preference-confirmation .notice-box { display: block; }
.recovery-box, .preference-confirmation { display: flex; flex-direction: column; gap: 12px; }
.recovery-box .inline-flex { margin-top: 10px; }
.confirmation-impact { font-weight: 600; color: var(--color-text); }
@media(max-width:600px) { .preference-panel { padding:18px; } .preference-row { gap: 12px; } .preference-value { flex-direction: column; gap: 6px; } }
</style>
