<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { useRouter } from 'vue-router'
import { useI18n } from 'vue-i18n'
import { formatDateTime } from '@/i18n'
import { useEnterpriseStore } from '@/stores/enterprise'
import { usePersonalProfileStore } from '@/stores/personalProfile'
import { useUiStore } from '@/stores/ui'
import { UiButton } from '@/ui/base'
import AppIcon from '@/ui/common/AppIcon.vue'
import AvatarMark from '@/ui/common/AvatarMark.vue'
import PageHeading from '@/ui/common/PageHeading.vue'
import StatusBadge from '@/ui/common/StatusBadge.vue'
import PersonalSecurityPanel from '../components/security/PersonalSecurityPanel.vue'

const enterprise = useEnterpriseStore()
const personal = usePersonalProfileStore()
const ui = useUiStore()
const router = useRouter()
const { t } = useI18n()

const draftAvatar = ref('')
const saveError = ref('')

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

async function logout() {
  personal.clear()
  await enterprise.logout()
  window.location.assign(enterprise.loginHref())
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
    <PageHeading :title="t('personalProfile.title')" :description="t('personalProfile.description')" />

    <p v-if="enterprise.sourceKind !== 'api'" class="notice-box warning" role="note">
      <AppIcon name="lock" :size="16" />
      {{ t('personalProfile.apiOnly') }}
    </p>
    <p v-else-if="personal.loading && !personal.ready" class="notice-box" role="status">
      {{ t('personalProfile.loading') }}
    </p>
    <div v-if="personal.error" class="notice-box danger profile-error" role="alert">
      <span>{{ personal.error }}</span>
      <UiButton variant="outline" size="sm" @click="load">{{ t('personalProfile.retry') }}</UiButton>
    </div>

    <template v-if="profile">
      <div class="profile-layout">
        <section class="card identity-card" aria-labelledby="personal-profile-identity">
          <div class="identity-hero">
            <AvatarMark
              :name="profile.name || profile.username || profile.userId"
              :asset-ref="savedAvatar"
              :size="72"
            />
            <div>
              <p class="eyebrow">{{ t('personalProfile.accountSection') }}</p>
              <h2 id="personal-profile-identity">{{ profile.name || profile.username || profile.userId }}</h2>
              <p class="secondary">{{ profile.username || profile.userId }}</p>
            </div>
          </div>

          <dl class="detail-grid">
            <div><dt>{{ t('personalProfile.username') }}</dt><dd>{{ profile.username || t('common.unknown') }}</dd></div>
            <div><dt>{{ t('personalProfile.userId') }}</dt><dd class="mono">{{ profile.userId }}</dd></div>
            <div><dt>{{ t('personalProfile.registeredAt') }}</dt><dd>{{ displayDate(profile.registeredAt) }}</dd></div>
            <div><dt>{{ t('personalProfile.joinedAt') }}</dt><dd>{{ displayDate(profile.joinedAt) }}</dd></div>
          </dl>
        </section>

        <section class="card tenant-card" data-ui-region="scope" aria-labelledby="personal-profile-tenant">
          <div class="section-heading">
            <div>
              <p class="eyebrow">{{ t('personalProfile.tenantSection') }}</p>
              <h2 id="personal-profile-tenant">{{ profile.tenantName || profile.tenantId }}</h2>
            </div>
            <StatusBadge :text="enterprise.tenantId" tone="primary" :dot="false" />
          </div>
          <dl class="detail-grid">
            <div><dt>{{ t('personalProfile.roles') }}</dt><dd class="role-list"><span v-for="role in profile.roles" :key="role.roleId" class="role-chip">{{ role.roleName || role.roleId }}</span><span v-if="!profile.roles.length" class="secondary">{{ t('personalProfile.emptyRoles') }}</span></dd></div>
            <div><dt>{{ t('personalProfile.employeeId') }}</dt><dd>{{ profile.employeeId || t('common.unknown') }}</dd></div>
            <div><dt>{{ t('personalProfile.position') }}</dt><dd>{{ profile.position || t('common.unknown') }}</dd></div>
            <div><dt>{{ t('personalProfile.departmentId') }}</dt><dd class="mono">{{ profile.departmentId || t('common.unknown') }}</dd></div>
          </dl>
        </section>
      </div>

      <section class="card avatar-card" data-ui-region="form-workspace" aria-labelledby="personal-profile-avatar">
        <div class="section-heading">
          <div>
            <p class="eyebrow">{{ t('personalProfile.avatarSection') }}</p>
            <h2 id="personal-profile-avatar">{{ t('personalProfile.avatarCurrent') }}</h2>
            <p class="secondary">{{ t('personalProfile.avatarDescription') }}</p>
          </div>
          <AvatarMark :name="profile.name || profile.username || profile.userId" :asset-ref="draftAvatar || savedAvatar" :size="54" />
        </div>

        <div class="avatar-options" role="group" :aria-label="t('personalProfile.avatarSection')">
          <UiButton
            v-for="option in personal.avatarOptions"
            :key="option.assetRef"
            type="button"
            variant="outline"
            class="avatar-option"
            :class="{ selected: draftAvatar === option.assetRef }"
            :aria-label="t('personalProfile.avatarOption', { name: option.name })"
            :aria-pressed="draftAvatar === option.assetRef"
            :disabled="personal.saving"
            @click="draftAvatar = option.assetRef"
          >
            <AvatarMark :name="profile.name || profile.username || profile.userId" :asset-ref="option.assetRef" :size="44" />
            <span><strong>{{ option.name }}</strong><small>{{ option.assetRef }}</small></span>
            <AppIcon v-if="draftAvatar === option.assetRef" name="check" :size="16" />
          </UiButton>
        </div>
        <p class="secondary confirm-note"><AppIcon name="shield" :size="15" />{{ t('personalProfile.avatarConfirmNote') }}</p>
        <p v-if="saveError" class="form-error" role="alert">{{ saveError }}</p>
        <div class="form-footer" data-ui-region="form-actions">
          <UiButton
            class="btn btn-primary"
            :disabled="!avatarDirty || personal.saving"
            @click="saveAvatar"
          >
            <AppIcon name="check" :size="15" />
            {{ personal.saving ? t('personalProfile.savingAvatar') : t('personalProfile.saveAvatar') }}
          </UiButton>
        </div>
      </section>

      <div class="profile-layout lower-grid">
        <section class="card contact-card" aria-labelledby="personal-profile-contact">
          <p class="eyebrow">{{ t('personalProfile.contactSection') }}</p>
          <h2 id="personal-profile-contact">{{ t('personalProfile.contactSection') }}</h2>
          <dl class="detail-grid">
            <div><dt>{{ t('personalProfile.email') }}</dt><dd>{{ profile.email || t('personalProfile.unbound') }}</dd></div>
            <div><dt>{{ t('personalProfile.phone') }}</dt><dd>{{ profile.phone || t('personalProfile.unbound') }}</dd></div>
          </dl>
        </section>

        <section class="card security-card" aria-labelledby="personal-profile-security">
          <p class="eyebrow">{{ t('personalProfile.securitySection') }}</p>
          <h2 id="personal-profile-security">{{ t('personalProfile.securitySection') }}</h2>
          <div class="security-actions">
            <div class="security-row">
              <span><strong>{{ t('personalProfile.password') }}</strong><small>{{ t('personalProfile.passwordDescription') }}</small></span>
              <UiButton variant="outline" @click="router.push('/system/security')">{{ t('personalProfile.openSecurity') }}</UiButton>
            </div>
            <div class="security-row">
              <span><strong>{{ t('personalProfile.preferences') }}</strong><small>{{ t('personalProfile.preferencesDescription') }}</small></span>
              <UiButton variant="outline" disabled>{{ t('personalProfile.comingSoon') }}</UiButton>
            </div>
            <div class="security-row sign-out-row">
              <span><strong>{{ t('personalProfile.signOut') }}</strong><small>{{ t('personalProfile.signOutDescription') }}</small></span>
              <UiButton variant="outline" @click="logout">{{ t('personalProfile.signOut') }}</UiButton>
            </div>
          </div>
        </section>
      </div>
      <div data-ui-region="personal-security"><PersonalSecurityPanel /></div>
    </template>

    <div v-else-if="personal.ready && !personal.loading && enterprise.sourceKind === 'api' && !personal.error" class="card empty-state" role="status">
      <AppIcon name="user" :size="28" />
      <p>{{ t('personalProfile.profileUnavailable') }}</p>
    </div>
  </div>
</template>

<style scoped>
.personal-profile-page{gap:16px}.profile-layout{display:grid;grid-template-columns:minmax(0,1fr) minmax(0,1fr);gap:16px}.identity-card,.tenant-card,.contact-card,.security-card,.avatar-card{padding:22px}.identity-hero,.section-heading{display:flex;align-items:center;justify-content:space-between;gap:16px}.identity-hero{justify-content:flex-start}.identity-hero h2,.section-heading h2,.contact-card h2,.security-card h2{font-size:18px;letter-spacing:-.2px}.eyebrow{margin-bottom:5px;color:var(--color-primary);font-size:11px;font-weight:700;letter-spacing:.06em;text-transform:uppercase}.secondary{color:var(--color-text-secondary);font-size:var(--text-sm);line-height:1.6}.detail-grid{display:grid;grid-template-columns:repeat(2,minmax(0,1fr));gap:16px 24px;margin-top:22px}.detail-grid>div{min-width:0;padding-top:12px;border-top:1px solid var(--color-border)}.detail-grid dt{color:var(--color-text-muted);font-size:var(--text-xs)}.detail-grid dd{margin-top:6px;overflow-wrap:anywhere;font-size:var(--text-sm);font-weight:600}.role-list{display:flex;gap:6px;flex-wrap:wrap}.role-chip{display:inline-flex;padding:3px 7px;border-radius:999px;background:var(--color-primary-soft);color:var(--color-primary);font-size:11px;font-weight:650}.avatar-card{display:flex;flex-direction:column;gap:18px}.avatar-options{display:grid;grid-template-columns:repeat(4,minmax(0,1fr));gap:10px}.avatar-option{min-height:70px;height:auto;justify-content:flex-start;padding:10px;text-align:left}.avatar-option.selected{border-color:var(--color-primary);background:var(--color-primary-soft)}.avatar-option>span:nth-child(2){display:flex;min-width:0;flex:1;flex-direction:column;align-items:flex-start}.avatar-option strong{font-size:12px}.avatar-option small{max-width:100%;overflow:hidden;color:var(--color-text-muted);font-size:10px;text-overflow:ellipsis}.confirm-note{display:flex;align-items:center;gap:7px}.form-footer{display:flex;justify-content:flex-end;padding-top:16px;border-top:1px solid var(--color-border)}.security-actions{display:flex;flex-direction:column;margin-top:14px}.security-row{display:flex;align-items:center;justify-content:space-between;gap:16px;padding:13px 0;border-top:1px solid var(--color-border)}.security-row span{min-width:0}.security-row strong,.security-row small{display:block}.security-row strong{font-size:var(--text-sm)}.security-row small{margin-top:4px;color:var(--color-text-muted);font-size:var(--text-xs);line-height:1.5}.sign-out-row{margin-top:3px}.profile-error{display:flex;align-items:center;justify-content:space-between;gap:12px}.empty-state{display:flex;align-items:center;gap:12px;padding:24px;color:var(--color-text-secondary)}.mono{font-family:var(--font-mono,ui-monospace,SFMono-Regular,Menlo,monospace)}@media(max-width:1100px){.avatar-options{grid-template-columns:repeat(2,minmax(0,1fr))}}@media(max-width:900px){.profile-layout{grid-template-columns:1fr}}@media(max-width:600px){.identity-card,.tenant-card,.contact-card,.security-card,.avatar-card{padding:18px}.identity-hero,.section-heading{align-items:flex-start}.detail-grid{grid-template-columns:1fr;gap:10px}.avatar-options{grid-template-columns:1fr}.security-row{align-items:stretch;flex-direction:column}.security-row .inline-flex{width:100%}.form-footer .btn{width:100%}.profile-error{align-items:stretch;flex-direction:column}}
</style>
