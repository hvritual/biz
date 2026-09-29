<script setup lang="ts">
import { ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { changeOwnPassword, PasswordChangeError } from '@/services/runtime/accountSecurity'
import { useEnterpriseStore } from '@/stores/enterprise'
import { UiButton, UiInput } from '@/ui/base'
import UiDialog from '@/ui/common/UiDialog.vue'
import Notice from '@/ui/common/Notice.vue'

const props = defineProps<{ open: boolean }>()
const emit = defineEmits<{ close: [] }>()
const store = useEnterpriseStore()
const { t } = useI18n()
const currentPassword = ref('')
const newPassword = ref('')
const confirmPassword = ref('')
const saving = ref(false)
const error = ref('')
const changed = ref(false)

watch(() => props.open, (open) => {
  if (!open) return
  currentPassword.value = ''
  newPassword.value = ''
  confirmPassword.value = ''
  saving.value = false
  error.value = ''
  changed.value = false
})

function errorKey(cause: unknown) {
  if (!(cause instanceof PasswordChangeError)) return 'unavailable'
  if (cause.code === 'CURRENT_PASSWORD_INVALID') return 'current'
  if (cause.code === 'WEAK_PASSWORD') return 'weak'
  if (cause.code === 'PASSWORD_MISMATCH') return 'mismatch'
  if (cause.code === 'UNAUTHENTICATED') return 'unauthenticated'
  return 'unavailable'
}

async function submit() {
  error.value = ''
  if (!/^.{8,16}$/.test(newPassword.value) || !/[A-Z]/.test(newPassword.value) || !/\d/.test(newPassword.value)) {
    error.value = t('personalProfile.passwordDialog.errors.weak')
    return
  }
  if (newPassword.value !== confirmPassword.value) {
    error.value = t('personalProfile.passwordDialog.errors.mismatch')
    return
  }
  saving.value = true
  try {
    await changeOwnPassword({ currentPassword: currentPassword.value, newPassword: newPassword.value, confirmPassword: confirmPassword.value })
    currentPassword.value = ''
    newPassword.value = ''
    confirmPassword.value = ''
    changed.value = true
  } catch (cause) {
    error.value = t(`personalProfile.passwordDialog.errors.${errorKey(cause)}`)
  } finally {
    saving.value = false
  }
}

function close() { if (!saving.value) emit('close') }
function signIn() { window.location.assign(store.loginHref()) }
</script>

<template>
  <UiDialog :open="open" :title="t('personalProfile.passwordDialog.title')" width="520px" @close="close">
    <div class="password-dialog" :aria-busy="saving">
      <Notice v-if="changed" tone="success">{{ t('personalProfile.passwordDialog.success') }}</Notice>
      <form v-else id="personal-password-form" class="password-form" @submit.prevent="submit">
        <label class="field"><span class="required">{{ t('personalProfile.passwordDialog.current') }}</span><UiInput v-model="currentPassword" type="password" autocomplete="current-password" :disabled="saving" required :show-password-label="t('personalProfile.passwordDialog.show')" :hide-password-label="t('personalProfile.passwordDialog.hide')" /></label>
        <label class="field"><span class="required">{{ t('personalProfile.passwordDialog.next') }}</span><UiInput v-model="newPassword" type="password" autocomplete="new-password" maxlength="16" :disabled="saving" required :show-password-label="t('personalProfile.passwordDialog.show')" :hide-password-label="t('personalProfile.passwordDialog.hide')" /><small>{{ t('personalProfile.passwordDialog.requirements') }}</small></label>
        <label class="field"><span class="required">{{ t('personalProfile.passwordDialog.confirm') }}</span><UiInput v-model="confirmPassword" type="password" autocomplete="new-password" maxlength="16" :disabled="saving" required :show-password-label="t('personalProfile.passwordDialog.show')" :hide-password-label="t('personalProfile.passwordDialog.hide')" /></label>
      </form>
      <Notice v-if="error" tone="danger" role="alert">{{ error }}</Notice>
    </div>
    <template #footer>
      <UiButton variant="outline" :disabled="saving" @click="close">{{ t(changed ? 'common.close' : 'common.cancel') }}</UiButton>
      <UiButton v-if="changed" @click="signIn">{{ t('personalProfile.passwordDialog.signIn') }}</UiButton>
      <UiButton v-else form="personal-password-form" type="submit" :disabled="saving">{{ saving ? t('personalProfile.passwordDialog.saving') : t('personalProfile.passwordDialog.save') }}</UiButton>
    </template>
  </UiDialog>
</template>

<style scoped>
.password-dialog, .password-form { display: flex; flex-direction: column; gap: 16px; }
.field small { display: block; margin-top: 6px; color: var(--color-text-muted); font-size: var(--text-xs); line-height: 1.5; }
</style>
