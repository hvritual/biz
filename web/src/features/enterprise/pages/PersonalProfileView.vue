<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { formatDateTime } from '@/i18n'
import { useEnterpriseStore } from '@/stores/enterprise'
import { usePersonalProfileStore } from '@/stores/personalProfile'
import { useUiStore } from '@/stores/ui'
import { UiButton } from '@/ui/base'
import AppIcon from '@/ui/common/AppIcon.vue'
import AvatarMark from '@/ui/common/AvatarMark.vue'
import Notice from '@/ui/common/Notice.vue'
import PersonalSecurityPanel from '../components/security/PersonalSecurityPanel.vue'
import PersonalNotificationPreferences from '../components/security/PersonalNotificationPreferences.vue'
import PersonalPasswordDialog from '../components/security/PersonalPasswordDialog.vue'

const enterprise = useEnterpriseStore()
const personal = usePersonalProfileStore()
const ui = useUiStore()
const { t } = useI18n()

const draftAvatar = ref('')
const saveError = ref('')
const avatarEditorOpen = ref(false)
const passwordDialogOpen = ref(false)

const profile = computed(() => personal.profile)
const savedAvatar = computed(() => profile.value?.avatarAssetRef ?? 'avatar:coffee-blue')
const avatarDirty = computed(() => Boolean(draftAvatar.value) && draftAvatar.value !== savedAvatar.value)
const timeZone = computed(() => enterprise.session?.active_tenant_timezone || 'UTC')

function displayDate(value: string) {
  if (!value) return t('common.unknown')
  return formatDateTime(value, timeZone.value)
}

async function load() {
  saveError.value = ''
  if (enterprise.sourceKind !== 'api') {
    personal.clear()
    personal.ready = true
    return
  }
  await personal.refresh(enterprise.session)
}

async function saveAvatar() {
  const session = enterprise.session
  if (!session || !avatarDirty.value || personal.saving) return
  saveError.value = ''
  try {
    const saved = await personal.saveAvatar(session, draftAvatar.value)
    if (saved) {
      draftAvatar.value = personal.profile?.avatarAssetRef ?? ''
      ui.toast(t('personalProfile.avatarSaved'), 'success')
    }
  } catch (caught) {
    saveError.value = caught instanceof Error ? caught.message : t('personalProfile.loadFailed')
  }
}

watch(
  () => [
    enterprise.sourceKind,
    enterprise.session?.authenticated,
    enterprise.session?.actor_kind,
    enterprise.session?.user_id,
    enterprise.session?.active_tenant_id,
    enterprise.session?.context_version,
  ],
  () => {
    void load()
  },
  { immediate: true },
)

watch(
  () => personal.profile?.avatarAssetRef,
  (value) => {
    draftAvatar.value = value ?? ''
    saveError.value = ''
  },
  { immediate: true },
)
</script>

<template>
  <div class="page-stack personal-profile-page" data-enterprise-page="personal-profile" data-ui-template="FormPage">
    <Notice v-if="enterprise.sourceKind !== 'api'" tone="warning">
      <AppIcon name="lock" :size="16" />
      {{ t('personalProfile.apiOnly') }}
    </Notice>
    <Notice v-else-if="personal.loading && !personal.ready">
      {{ t('personalProfile.loading') }}
    </Notice>
    <Notice v-if="personal.error" class="profile-error" tone="danger">
      <span>{{ personal.error }}</span>
      <UiButton variant="outline" size="sm" @click="load">{{ t('personalProfile.retry') }}</UiButton>
    </Notice>

    <template v-if="profile">
      <section class="profile-welcome" data-ui-region="page-heading" aria-labelledby="personal-profile-welcome">
        <h1 id="personal-profile-welcome">{{ t('personalProfile.welcomeBack', { name: profile.name || profile.username || profile.userId }) }}</h1>
        <p>{{ t('personalProfile.welcomeDescription') }}</p>

        <section class="card basic-information" data-ui-region="form-workspace" aria-labelledby="personal-profile-information">
          <h2 id="personal-profile-information">{{ t('personalProfile.basicInformation') }}</h2>
          <div class="basic-information-body">
            <div class="profile-identity">
              <AvatarMark :name="profile.name || profile.username || profile.userId" :asset-ref="savedAvatar" :size="84" />
              <div><strong>{{ profile.name || profile.username || profile.userId }}</strong><small>{{ profile.username || profile.userId }}</small></div>
            </div>
            <dl class="profile-facts" data-ui-region="scope">
              <div><dt>{{ t('personalProfile.username') }}</dt><dd>{{ profile.username || t('common.unknown') }}</dd></div>
              <div><dt>{{ t('personalProfile.accountRole') }}</dt><dd class="role-list"><span v-for="role in profile.roles" :key="role.roleId" class="role-chip">{{ role.roleName || role.roleId }}</span><span v-if="!profile.roles.length">{{ t('personalProfile.emptyRoles') }}</span></dd></div>
              <div><dt>{{ t('personalProfile.enterpriseName') }}</dt><dd>{{ profile.tenantName || profile.tenantId }}</dd></div>
              <div><dt>{{ t('personalProfile.phone') }}</dt><dd>{{ profile.phone || t('personalProfile.unbound') }}</dd></div>
              <div><dt>{{ t('personalProfile.email') }}</dt><dd>{{ profile.email || t('personalProfile.unbound') }}</dd></div>
              <div><dt>{{ t('personalProfile.registeredAt') }}</dt><dd>{{ displayDate(profile.registeredAt) }}</dd></div>
            </dl>
          </div>
          <div class="avatar-actions" data-ui-region="form-actions">
            <UiButton variant="outline" @click="avatarEditorOpen = !avatarEditorOpen">{{ t('personalProfile.avatarSection') }}</UiButton>
          </div>
          <div v-if="avatarEditorOpen" class="avatar-editor" aria-labelledby="personal-profile-avatar">
            <p id="personal-profile-avatar" class="secondary">{{ t('personalProfile.avatarDescription') }}</p>
            <div class="avatar-options" role="group" :aria-label="t('personalProfile.avatarSection')">
              <UiButton v-for="option in personal.avatarOptions" :key="option.assetRef" type="button" variant="outline" class="avatar-option" :class="{ selected: draftAvatar === option.assetRef }" :aria-label="t('personalProfile.avatarOption', { name: option.name })" :aria-pressed="draftAvatar === option.assetRef" :disabled="personal.saving" @click="draftAvatar = option.assetRef">
                <AvatarMark :name="profile.name || profile.username || profile.userId" :asset-ref="option.assetRef" :size="40" />
                <span><strong>{{ option.name }}</strong><small>{{ option.assetRef }}</small></span>
                <AppIcon v-if="draftAvatar === option.assetRef" name="check" :size="16" />
              </UiButton>
            </div>
            <p class="secondary confirm-note"><AppIcon name="shield" :size="15" />{{ t('personalProfile.avatarConfirmNote') }}</p>
            <p v-if="saveError" class="form-error" role="alert">{{ saveError }}</p>
            <div class="form-footer"><UiButton class="btn btn-primary" :disabled="!avatarDirty || personal.saving" @click="saveAvatar"><AppIcon name="check" :size="15" />{{ personal.saving ? t('personalProfile.savingAvatar') : t('personalProfile.saveAvatar') }}</UiButton></div>
          </div>
        </section>
      </section>
      <div data-ui-region="personal-security"><PersonalSecurityPanel @password="passwordDialogOpen = true" /></div>
      <div data-ui-region="notification-preferences"><PersonalNotificationPreferences /></div>
      <PersonalPasswordDialog :open="passwordDialogOpen" @close="passwordDialogOpen = false" />
    </template>

    <div v-else-if="personal.ready && !personal.loading && enterprise.sourceKind === 'api' && !personal.error" class="card empty-state" role="status">
      <AppIcon name="user" :size="28" />
      <p>{{ t('personalProfile.profileUnavailable') }}</p>
    </div>
  </div>
</template>

<style scoped>
.personal-profile-page{gap:16px}.profile-welcome{padding:26px 18px 18px;border-radius:var(--radius-md);background:linear-gradient(108deg,var(--color-primary-soft),var(--color-surface-soft))}.profile-welcome>h1{font-size:26px;letter-spacing:-.5px}.profile-welcome>p{margin-top:8px;color:var(--color-text-secondary);font-size:var(--text-sm)}.basic-information{margin-top:22px;padding:20px}.basic-information>h2{font-size:18px}.basic-information-body{display:grid;grid-template-columns:minmax(210px,.8fr) minmax(0,2.2fr);align-items:center;gap:24px;margin-top:18px}.profile-identity{display:flex;align-items:center;gap:16px}.profile-identity strong,.profile-identity small{display:block}.profile-identity strong{font-size:var(--text-lg)}.profile-identity small{margin-top:5px;color:var(--color-text-secondary);font-size:var(--text-sm)}.profile-facts{display:grid;grid-template-columns:repeat(3,minmax(0,1fr));gap:18px 28px}.profile-facts>div{min-width:0}.profile-facts dt{color:var(--color-text-secondary);font-size:var(--text-sm)}.profile-facts dd{margin-top:6px;overflow-wrap:anywhere;font-size:var(--text-sm);font-weight:600}.role-list{display:flex;gap:6px;flex-wrap:wrap}.role-chip{display:inline-flex;padding:3px 7px;border-radius:999px;background:var(--color-primary-soft);color:var(--color-primary);font-size:11px;font-weight:650}.avatar-actions{display:flex;justify-content:flex-end;margin-top:18px;padding-top:14px;border-top:1px solid var(--color-border)}.avatar-editor{display:flex;flex-direction:column;gap:14px;margin-top:14px;padding-top:16px;border-top:1px solid var(--color-border)}.secondary{color:var(--color-text-secondary);font-size:var(--text-sm);line-height:1.6}.avatar-options{display:grid;grid-template-columns:repeat(4,minmax(0,1fr));gap:10px}.avatar-option{min-height:66px;height:auto;justify-content:flex-start;padding:10px;text-align:left}.avatar-option.selected{border-color:var(--color-primary);background:var(--color-primary-soft)}.avatar-option>span:nth-child(2){display:flex;min-width:0;flex:1;flex-direction:column;align-items:flex-start}.avatar-option strong{font-size:12px}.avatar-option small{max-width:100%;overflow:hidden;color:var(--color-text-muted);font-size:10px;text-overflow:ellipsis}.confirm-note{display:flex;align-items:center;gap:7px}.form-footer{display:flex;justify-content:flex-end;padding-top:14px;border-top:1px solid var(--color-border)}.profile-error{display:flex;align-items:center;justify-content:space-between;gap:12px}.empty-state{display:flex;align-items:center;gap:12px;padding:24px;color:var(--color-text-secondary)}@media(max-width:980px){.basic-information-body{grid-template-columns:1fr}.profile-facts{grid-template-columns:repeat(2,minmax(0,1fr))}}@media(max-width:600px){.profile-welcome{padding:20px 14px 14px}.profile-welcome>h1{font-size:22px}.basic-information{margin-top:16px;padding:16px}.profile-facts,.avatar-options{grid-template-columns:1fr}.avatar-actions .inline-flex,.form-footer .inline-flex{width:100%}.profile-error{align-items:stretch;flex-direction:column}}
</style>
