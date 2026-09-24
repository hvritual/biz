<script setup lang="ts">
import { computed, onBeforeUnmount, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { useEnterpriseStore } from '@/stores/enterprise'
import { usePersonalProfileStore } from '@/stores/personalProfile'
import { publishSessionContextChange, subscribeSessionContextChange } from '@/services/runtime/sessionCoordinator'
import { sessionContext } from '@/services/runtime/api'
import { UiButton, UiInput, UiOption, UiSelect } from '@/ui/base'
import UiDialog from '@/ui/common/UiDialog.vue'
import AppIcon from '@/ui/common/AppIcon.vue'
import { usePersonalSecurity } from '../../composables/usePersonalSecurity'

const enterprise = useEnterpriseStore(), personal = usePersonalProfileStore()
const { t } = useI18n()
const profile = computed(() => personal.profile)
const hasBoundContact = computed(() => Boolean(profile.value?.email || profile.value?.phone))
const flow = usePersonalSecurity({
  currentSession: () => enterprise.session,
  currentProfile: () => personal.profile,
  applyProfile: (value) => { personal.profile = value },
  deleted: () => {
    personal.clear()
    publishSessionContextChange((enterprise.session?.context_version ?? 0) + 1)
    window.location.assign(enterprise.loginHref())
  },
})
const { open, busy, mode, channel, password, destination, code, confirmed, error, success, uncertain,
  challenge, receipt, seconds, expired, confirmationPending } = flow
const title = computed(() => t(mode.value === 'deletion' ? 'personalSecurity.deletionTitle' : 'personalSecurity.contactTitle'))
const selectedBound = computed(() => (channel.value === 'email' ? profile.value?.email : profile.value?.phone) || '')
const deliveryKey = computed(() => {
  const status = receipt.value?.notification_state ?? challenge.value?.delivery_state
  return status === 'DELIVERED' ? 'delivered' : status === 'FAILED' ? 'deliveryFailed' : 'queued'
})
function closeDialog() { if (!busy.value) flow.close() }
function beginDeletion() { flow.begin('deletion', profile.value?.email ? 'email' : 'sms') }
watch(() => enterprise.session ? sessionContext(enterprise.session) : '', () => flow.close(), { flush: 'sync' })
const unsubscribe = subscribeSessionContextChange(() => flow.close())
function pageHidden() { flow.close() }
window.addEventListener('pagehide', pageHidden)
onBeforeUnmount(() => { unsubscribe(); window.removeEventListener('pagehide', pageHidden) })
</script>

<template>
  <section class="card personal-security-panel" aria-labelledby="personal-security-heading">
    <div class="security-heading">
      <AppIcon name="shield" :size="22" />
      <div><h2 id="personal-security-heading">{{ t('personalSecurity.title') }}</h2><p>{{ t('personalSecurity.description') }}</p></div>
    </div>
    <div class="security-command-row">
      <UiButton variant="outline" :disabled="!profile" @click="flow.begin('contact', 'email')">{{ t('personalSecurity.changeEmail') }}</UiButton>
      <UiButton variant="outline" :disabled="!profile" @click="flow.begin('contact', 'sms')">{{ t('personalSecurity.changePhone') }}</UiButton>
    </div>
    <div class="deletion-row">
      <div><strong>{{ t('personalSecurity.deletion') }}</strong><p>{{ t('personalSecurity.ownerHint') }}</p></div>
      <UiButton variant="outline" :disabled="!hasBoundContact" @click="beginDeletion">{{ t('personalSecurity.deletion') }}</UiButton>
    </div>
    <p v-if="!hasBoundContact" class="secondary">{{ t('personalSecurity.noBound') }}</p>
  </section>

  <UiDialog :open="open" :title="title" @close="closeDialog">
    <div class="security-dialog" :aria-busy="busy">
      <dl class="scope-summary">
        <div><dt>{{ t('personalSecurity.currentTenant') }}</dt><dd>{{ profile?.tenantName }} <small>{{ profile?.tenantId }}</small></dd></div>
        <div><dt>{{ t('personalSecurity.currentAccount') }}</dt><dd>{{ profile?.name || profile?.username }} <small>{{ profile?.userId }}</small></dd></div>
      </dl>
      <div v-if="mode === 'deletion'" class="notice-box warning" role="note">
        <p>{{ t('personalSecurity.deletionScope') }}</p><p>{{ t('personalSecurity.otherTenants') }}</p>
      </div>
      <p v-else class="secondary">{{ t('personalSecurity.contactScope') }}</p>

      <p v-if="error" class="notice-box danger" role="alert">{{ t(`personalSecurity.errors.${error}`) }}</p>
      <p v-if="success" class="notice-box success" role="status">{{ t('personalSecurity.saved') }}</p>
      <p v-if="uncertain" class="notice-box warning" role="status">{{ t('personalSecurity.uncertain') }}</p>
      <template v-if="confirmationPending">
        <p class="notice-box warning" role="status">{{ t('personalSecurity.confirmNote') }}</p>
        <UiButton :disabled="busy" @click="flow.recover">{{ busy ? t('personalSecurity.checking') : t('personalSecurity.checkAgain') }}</UiButton>
      </template>
      <p v-if="receipt || challenge" class="secondary" role="status">{{ t('personalSecurity.notification') }} · {{ t(`personalSecurity.${deliveryKey}`) }}</p>

      <form v-if="!success && !confirmationPending && !uncertain" class="security-form" autocomplete="off" @submit.prevent="flow.send">
        <template v-if="mode === 'contact'">
          <template v-if="!challenge">
            <label for="self-security-destination">{{ t('personalSecurity.newContact') }} · {{ t(`personalSecurity.${channel}`) }}</label>
            <UiInput id="self-security-destination" v-model="destination" :type="channel === 'email' ? 'email' : 'tel'" autocomplete="off" :disabled="busy" required />
          </template>
          <label for="self-security-password">{{ t('personalSecurity.password') }}</label>
          <UiInput id="self-security-password" v-model="password" type="password" autocomplete="off" :disabled="busy" aria-describedby="self-security-password-hint" />
          <small id="self-security-password-hint">{{ t('personalSecurity.passwordHint') }}</small>
        </template>
        <template v-else>
          <label for="self-security-channel">{{ t('personalSecurity.channel') }}</label>
          <UiSelect id="self-security-channel" v-model="channel" :disabled="busy || Boolean(challenge)">
            <UiOption v-if="profile?.email" value="email">{{ t('personalSecurity.email') }}</UiOption>
            <UiOption v-if="profile?.phone" value="sms">{{ t('personalSecurity.sms') }}</UiOption>
          </UiSelect>
          <p>{{ t('personalSecurity.bound') }}：{{ selectedBound }}</p>
        </template>
        <UiButton type="submit" variant="outline" :disabled="busy || seconds > 0 || (mode === 'contact' && !password)">
          {{ seconds > 0 ? t('personalSecurity.countdown', { seconds }) : t(challenge ? 'personalSecurity.resend' : 'personalSecurity.send') }}
        </UiButton>
      </form>

      <form v-if="challenge && !success && !confirmationPending && !uncertain" class="security-form" @submit.prevent="flow.complete">
        <p class="secondary">{{ t('personalSecurity.codeAccepted') }}</p>
        <p>{{ t('personalSecurity.destination') }}：{{ challenge.masked_destination }}</p>
        <p v-if="expired" class="notice-box warning" role="status">{{ t('personalSecurity.expired') }}</p>
        <label for="self-security-code">{{ t('personalSecurity.otp') }}</label>
        <UiInput id="self-security-code" v-model="code" inputmode="numeric" autocomplete="one-time-code" maxlength="10" :disabled="busy || expired" required />
        <label v-if="mode === 'deletion'" class="confirmation-label" for="self-security-confirmation">
          <UiInput id="self-security-confirmation" v-model="confirmed" type="checkbox" :disabled="busy" />
          <span>{{ t('personalSecurity.confirmDeletion') }}</span>
        </label>
        <UiButton type="submit" :variant="mode === 'deletion' ? 'destructive' : 'default'" :disabled="busy || expired || !code || (mode === 'deletion' && !confirmed)">
          {{ busy ? t('personalSecurity.submitting') : t(mode === 'deletion' ? 'personalSecurity.confirmDelete' : 'personalSecurity.confirmContact') }}
        </UiButton>
      </form>
    </div>
    <template #footer><UiButton variant="outline" :disabled="busy" @click="closeDialog">{{ t(success || uncertain ? 'personalSecurity.close' : 'personalSecurity.cancel') }}</UiButton></template>
  </UiDialog>
</template>

<style scoped>
.personal-security-panel { padding: 22px; }
.security-heading { display: flex; align-items: flex-start; gap: 12px; }
h2 { font-size: 18px; }
p, small, dt { color: var(--color-text-secondary); font-size: var(--text-sm); line-height: 1.6; }
.security-command-row { display: flex; gap: 10px; flex-wrap: wrap; margin-top: 18px; }
.deletion-row { display: flex; align-items: center; justify-content: space-between; gap: 20px; border-top: 1px solid var(--color-border); margin-top: 20px; padding-top: 18px; }
.deletion-row strong { color: var(--color-danger); }
.security-dialog, .security-form { display: flex; flex-direction: column; gap: 12px; }
.security-form { border-top: 1px solid var(--color-border); padding-top: 16px; }
.security-form > label { font-size: var(--text-sm); font-weight: 600; }
.scope-summary { display: grid; grid-template-columns: 1fr 1fr; gap: 16px; padding: 14px; border-radius: var(--radius-sm); background: var(--color-surface-soft); }
.scope-summary dd { overflow-wrap: anywhere; font-size: var(--text-sm); font-weight: 600; }
.scope-summary small { display: block; font-weight: 400; }
.notice-box { display: block; }
.confirmation-label { display: flex; align-items: flex-start; gap: 10px; }
.confirmation-label input { width: 18px; height: 18px; flex: 0 0 18px; margin-top: 3px; }
@media(max-width:600px) { .personal-security-panel { padding:18px; } .deletion-row { align-items:stretch; flex-direction:column; gap:12px; } .scope-summary { grid-template-columns:1fr; } .security-command-row { flex-direction:column; } }
</style>
