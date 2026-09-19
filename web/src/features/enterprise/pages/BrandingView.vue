<script setup lang="ts">
import { computed, onBeforeUnmount, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { UiButton, UiInput } from '@/ui/base'
import { useEnterpriseStore } from '@/stores/enterprise'
import { useUiStore } from '@/stores/ui'
import PageHeading from '@/ui/common/PageHeading.vue'
import StatusBadge from '@/ui/common/StatusBadge.vue'
import AppIcon from '@/ui/common/AppIcon.vue'
import {
  applyUiTheme,
  resolveTenantUiTheme,
  uiThemePresets,
  type UiThemePresetName,
} from '@/ui/base/theme'
import type { EnterpriseTenantBrandingDraft } from '@/services/enterprise/tenantBrandingRuntime'
import { currentAuthorizationAllows } from '@/services/runtime/authorization'

const store = useEnterpriseStore()
const ui = useUiStore()
const { t } = useI18n()
const presets = Object.keys(uiThemePresets) as UiThemePresetName[]
const draft = ref<EnterpriseTenantBrandingDraft>({ preset: 'blue', primary: '' })
const error = ref('')
const saving = ref(false)
const saveAttempted = ref(false)

const authoritativeDraft = computed<EnterpriseTenantBrandingDraft>(() => ({
  preset: store.branding?.preset ?? 'blue',
  primary: store.branding?.primary ?? '',
}))
const dirty = computed(() => JSON.stringify(draft.value) !== JSON.stringify(authoritativeDraft.value))
const customValid = computed(() => draft.value.preset !== 'custom' || /^#[0-9a-fA-F]{6}$/.test(draft.value.primary.trim()))
const canEdit = computed(() => Boolean(store.branding?.canManage) && (store.previewMode || currentAuthorizationAllows('tenant.branding.update')))

function applyDraft(value: EnterpriseTenantBrandingDraft) {
  error.value = ''
  try {
    applyUiTheme(resolveTenantUiTheme({
      preset: value.preset,
      primary: value.primary || undefined,
    }))
  } catch {
    error.value = t('branding.invalidPrimary')
  }
}

function restoreAuthoritative() {
  const value = authoritativeDraft.value
  applyUiTheme(resolveTenantUiTheme({ preset: value.preset, primary: value.primary || undefined }))
}

function selectPreset(preset: UiThemePresetName) {
  if (!canEdit.value) return
  saveAttempted.value = false
  draft.value = { preset, primary: '' }
  applyDraft(draft.value)
}

function selectCustom() {
  if (!canEdit.value) return
  saveAttempted.value = false
  draft.value = {
    preset: 'custom',
    primary: draft.value.primary || store.branding?.primary || uiThemePresets.blue.primary,
  }
  applyDraft(draft.value)
}

function updatePrimary() {
  saveAttempted.value = false
  if (draft.value.preset !== 'custom') draft.value = { preset: 'custom', primary: draft.value.primary }
  applyDraft(draft.value)
}

function cancel() {
  draft.value = { ...authoritativeDraft.value }
  saveAttempted.value = false
  error.value = ''
  restoreAuthoritative()
}

function resetDefault() {
  if (!canEdit.value) return
  saveAttempted.value = false
  draft.value = { preset: 'blue', primary: '' }
  applyDraft(draft.value)
}

async function save() {
  if (!canEdit.value || saving.value) return
  if (!customValid.value) {
    error.value = t('branding.invalidPrimary')
    return
  }
  saving.value = true
  saveAttempted.value = true
  error.value = ''
  try {
    await store.saveBranding(draft.value)
    draft.value = { ...authoritativeDraft.value }
    saveAttempted.value = false
    ui.toast(store.sourceKind === 'api' ? t('branding.saved') : t('branding.previewSaved'), 'success')
  } catch (caught) {
    error.value = caught instanceof Error ? caught.message : t('branding.saveFailed')
  } finally {
    saving.value = false
  }
}

watch(
  () => store.branding,
  () => {
    draft.value = { ...authoritativeDraft.value }
    error.value = ''
    saveAttempted.value = false
  },
  { immediate: true },
)

onBeforeUnmount(restoreAuthoritative)
</script>

<template>
  <div class="page-stack branding-page" data-enterprise-page="branding" data-ui-template="FormPage">
    <PageHeading :title="t('branding.title')" :description="t('branding.description')" />

    <p v-if="store.brandingLoading && !store.brandingReady" class="notice-box" role="status">
      {{ t('branding.loading') }}
    </p>
    <p v-if="store.brandingError" class="notice-box danger" role="alert">
      {{ store.brandingError }}
    </p>

    <div class="branding-layout">
      <form class="card branding-form" data-ui-region="form-workspace" @submit.prevent="save">
        <div class="row-between section-heading">
          <div>
            <h2>{{ t('branding.presetSection') }}</h2>
            <p>{{ t('branding.currentTheme') }} · {{ t(`branding.presets.${store.branding?.preset ?? 'blue'}`) }}</p>
          </div>
          <StatusBadge
            :text="canEdit ? t('branding.canManage') : t('branding.cannotManage')"
            :tone="canEdit ? 'success' : 'neutral'"
          />
        </div>

        <div v-if="!canEdit" class="notice-box warning" role="note">
          <AppIcon name="lock" :size="16" />{{ t('branding.readOnly') }}
        </div>

        <div v-if="canEdit" class="preset-grid" :aria-label="t('branding.presetSection')">
          <UiButton
            v-for="preset in presets"
            :key="preset"
            type="button"
            :disabled="!canEdit"
            :class="['preset-card', { selected: draft.preset === preset }]"
            :aria-pressed="draft.preset === preset"
            @click="selectPreset(preset)"
          >
            <span class="theme-swatch" :style="{ '--brand-swatch': uiThemePresets[preset].primary }" />
            <span>
              <strong>{{ t(`branding.presets.${preset}`) }}</strong>
              <small v-if="draft.preset === preset">{{ t('branding.selected') }}</small>
            </span>
            <AppIcon v-if="draft.preset === preset" name="check" :size="16" />
          </UiButton>
        </div>

        <section v-if="canEdit" class="custom-section">
          <div class="section-copy">
            <h2>{{ t('branding.customSection') }}</h2>
            <p>{{ t('branding.customDescription') }}</p>
          </div>
          <UiButton
            type="button"
            :disabled="!canEdit"
            :class="['custom-selector', { selected: draft.preset === 'custom' }]"
            :aria-pressed="draft.preset === 'custom'"
            @click="selectCustom"
          >
            {{ t('branding.presets.custom') }}
          </UiButton>
          <label class="field custom-primary-field">
            <span>{{ t('branding.customPrimary') }}</span>
            <div class="custom-inputs">
              <UiInput
                v-model="draft.primary"
                type="color"
                class="color-picker"
                :disabled="!canEdit"
                :aria-label="t('branding.customPrimary')"
                @input="updatePrimary"
              />
              <UiInput
                v-model="draft.primary"
                class="input mono"
                :disabled="!canEdit"
                :placeholder="t('branding.customPlaceholder')"
                :aria-invalid="draft.preset === 'custom' && !customValid"
                @input="updatePrimary"
              />
            </div>
          </label>
          <p v-if="draft.preset === 'custom' && !customValid" class="form-error" role="alert">
            {{ t('branding.invalidPrimary') }}
          </p>
        </section>

        <section class="preview-panel" aria-live="polite">
          <div class="section-copy">
            <h2>{{ t('branding.previewTitle') }}</h2>
            <p>{{ t('branding.previewDescription') }}</p>
          </div>
          <div class="preview-gradient">
            <strong>{{ t('branding.previewGradient') }}</strong>
            <span>{{ store.tenantId || '—' }}</span>
          </div>
          <div class="preview-controls">
            <UiButton type="button" class="btn btn-primary">{{ t('branding.previewButton') }}</UiButton>
            <a href="#branding-preview" @click.prevent>{{ t('branding.previewLink') }}</a>
            <StatusBadge :text="t('branding.previewBadge')" tone="primary" />
            <UiInput class="input preview-input" :placeholder="t('branding.previewInput')" />
          </div>
        </section>

        <p v-if="saveAttempted && error" class="notice-box warning">{{ t('branding.retryLocked') }}</p>
        <p v-if="error" class="form-error" role="alert">{{ error }}</p>

        <div class="form-footer" data-ui-region="form-actions">
          <template v-if="canEdit">
            <span v-if="dirty" class="muted flex-1">{{ t('branding.dirty') }}</span>
            <UiButton type="button" class="btn" :disabled="saving" @click="resetDefault">{{ t('branding.resetDefault') }}</UiButton>
            <UiButton type="button" class="btn" :disabled="!dirty || saving" @click="cancel">{{ t('branding.cancel') }}</UiButton>
            <UiButton class="btn btn-primary" type="submit" :disabled="!dirty || !customValid || saving"><AppIcon name="check" :size="15" />{{ saving ? t('branding.saving') : t('branding.save') }}</UiButton>
          </template>
          <span v-else class="muted flex-1">{{ t('branding.readOnly') }}</span>
        </div>
      </form>

      <aside class="branding-scope" data-ui-region="scope">
        <section class="card scope-card">
          <h2>{{ t('branding.currentTheme') }}</h2>
          <dl class="detail-list">
            <dt>{{ t('branding.tenant') }}</dt>
            <dd class="mono">{{ store.tenantId || '—' }}</dd>
            <dt>{{ t('branding.source') }}</dt>
            <dd>{{ store.sourceKind === 'api' ? t('branding.serverConfirmed') : t('branding.previewData') }}</dd>
            <dt>{{ t('branding.version') }}</dt>
            <dd>{{ store.branding?.version ?? '—' }}</dd>
            <dt>{{ t('branding.permission') }}</dt>
            <dd>{{ canEdit ? t('branding.canManage') : t('branding.cannotManage') }}</dd>
          </dl>
        </section>
        <section class="card scope-card default-card">
          <AppIcon name="help" :size="18" />
          <p>{{ t('branding.defaultNotice') }}</p>
        </section>
      </aside>
    </div>
  </div>
</template>

<style scoped>
.branding-page{gap:16px}.branding-layout{display:grid;grid-template-columns:minmax(0,1fr) 300px;gap:18px;align-items:start}.branding-form{padding:24px;display:flex;flex-direction:column;gap:24px}.section-heading,.section-copy{min-width:0}.section-heading p,.section-copy p{margin-top:5px;color:var(--color-text-muted);font-size:var(--text-sm);line-height:1.6}.preset-grid{display:grid;grid-template-columns:repeat(4,minmax(0,1fr));gap:10px}.preset-card{min-height:78px;display:flex;align-items:center;justify-content:flex-start;gap:11px;padding:12px;border:1px solid var(--color-border);border-radius:var(--radius-md);text-align:left}.preset-card:hover:not(:disabled),.preset-card.selected{border-color:var(--color-primary);background:var(--color-primary-soft)}.preset-card>span:nth-child(2){min-width:0;flex:1}.preset-card strong,.preset-card small{display:block}.preset-card small{margin-top:4px;color:var(--color-primary);font-size:10px}.theme-swatch{width:30px;height:30px;flex:0 0 30px;border-radius:9px;background:var(--brand-swatch);box-shadow:inset 0 0 0 1px var(--color-border)}.custom-section{display:grid;grid-template-columns:minmax(0,1fr) auto;gap:12px 16px;padding-top:22px;border-top:1px solid var(--color-border)}.custom-selector{align-self:start;padding:8px 12px;border:1px solid var(--color-border);border-radius:var(--radius-sm)}.custom-selector.selected{border-color:var(--color-primary);background:var(--color-primary-soft);color:var(--color-primary)}.custom-primary-field{grid-column:1/-1}.custom-inputs{display:flex;gap:10px}.color-picker{width:48px;min-width:48px;padding:3px}.custom-inputs .input{max-width:220px}.preview-panel{padding-top:22px;border-top:1px solid var(--color-border);display:flex;flex-direction:column;gap:14px}.preview-gradient{min-height:86px;padding:18px;border-radius:var(--radius-md);background:linear-gradient(115deg,var(--color-primary-soft),var(--color-gradient-end));display:flex;align-items:flex-end;justify-content:space-between;gap:12px}.preview-gradient strong{font-size:17px}.preview-gradient span{color:var(--color-text-secondary);font-size:var(--text-sm)}.preview-controls{display:flex;align-items:center;gap:12px;flex-wrap:wrap}.preview-controls>a{color:var(--color-primary);font-weight:600}.preview-input{max-width:220px}.form-footer{display:flex;align-items:center;justify-content:flex-end;gap:8px;padding-top:20px;border-top:1px solid var(--color-border)}.branding-scope{display:flex;flex-direction:column;gap:14px}.scope-card{padding:20px}.scope-card h2{font-size:15px;margin-bottom:14px}.scope-card .detail-list{grid-template-columns:100px 1fr}.default-card{display:flex;align-items:flex-start;gap:10px;color:var(--color-text-secondary);font-size:var(--text-sm);line-height:1.6}.default-card>.icon{flex:0 0 auto;color:var(--color-primary)}@media(max-width:1050px){.branding-layout{grid-template-columns:1fr}.branding-scope{display:grid;grid-template-columns:1fr 1fr}.preset-grid{grid-template-columns:repeat(2,minmax(0,1fr))}}@media(max-width:767px){.branding-form{padding:18px}.branding-scope{grid-template-columns:1fr}.preset-grid{grid-template-columns:1fr 1fr}.form-footer{align-items:stretch;flex-direction:column}.form-footer .flex-1{width:100%}.form-footer .btn{width:100%}.preview-controls{align-items:stretch;flex-direction:column}.preview-controls>*{width:100%;max-width:none}.custom-section{grid-template-columns:1fr}.custom-selector,.custom-primary-field{grid-column:1}.custom-inputs .input{max-width:none;flex:1}}@media(max-width:420px){.preset-grid{grid-template-columns:1fr}}
</style>
