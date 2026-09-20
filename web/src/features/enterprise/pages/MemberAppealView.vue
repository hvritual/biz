<script setup lang="ts">
import { onMounted, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { UiButton } from '@/ui/base'
import { CommercialApiError } from '@/services/commercial/platformCommercial'
import { redirectToTrustedLogin } from '@/services/runtime/authorization'
import {
  listMemberAppeals,
  submitMemberAppeal,
  type MemberAppealEligibility,
  type MemberAppealReceipt,
} from '@/services/runtime/memberAppeal'

const { t } = useI18n()
const items = ref<MemberAppealEligibility[]>([])
const loading = ref(true)
const error = ref('')
const target = ref<MemberAppealEligibility | null>(null)
const reason = ref('')
const submitting = ref(false)
const receipt = ref<MemberAppealReceipt | null>(null)

async function load() {
  loading.value = true
  error.value = ''
  try {
    items.value = await listMemberAppeals()
  } catch (cause) {
    if (cause instanceof CommercialApiError && cause.status === 401) {
      error.value = t('members.appeal.loginRequired')
    } else {
      error.value = t('members.appeal.loadFailed')
    }
  } finally {
    loading.value = false
  }
}

function choose(item: MemberAppealEligibility) {
  target.value = item
  receipt.value = null
  reason.value = ''
  error.value = ''
}

async function submit() {
  if (!target.value || !reason.value.trim()) {
    error.value = t('members.appeal.reasonRequired')
    return
  }
  submitting.value = true
  error.value = ''
  try {
    receipt.value = await submitMemberAppeal(target.value.tenant_id, reason.value)
    await load()
  } catch (cause) {
    if (cause instanceof CommercialApiError) {
      if (cause.status === 429) error.value = t('members.appeal.rateLimited')
      else if (cause.status === 409) error.value = t('members.appeal.notEligible')
      else if (cause.status === 401) error.value = t('members.appeal.loginRequired')
      else error.value = t('members.appeal.submitFailed')
    } else {
      error.value = t('members.appeal.submitFailed')
    }
  } finally {
    submitting.value = false
  }
}

onMounted(load)
</script>

<template>
  <section class="appeal-page">
    <div class="card appeal-card" data-member-appeal-page>
      <header class="appeal-heading">
        <div>
          <span class="eyebrow">{{ t('members.appeal.eyebrow') }}</span>
          <h1>{{ t('members.appeal.title') }}</h1>
          <p>{{ t('members.appeal.description') }}</p>
        </div>
        <UiButton class="btn" :disabled="loading" @click="load">{{ t('common.refresh') }}</UiButton>
      </header>

      <div v-if="error" class="appeal-alert" role="alert"><span>{{ error }}</span><UiButton v-if="error === t('members.appeal.loginRequired')" class="btn" @click="redirectToTrustedLogin">{{ t('common.login') }}</UiButton></div>
      <p v-if="loading" class="appeal-muted">{{ t('common.loading') }}</p>

      <div v-else-if="!items.length" class="appeal-empty">
        <strong>{{ t('members.appeal.noneTitle') }}</strong>
        <p>{{ t('members.appeal.noneDescription') }}</p>
        <RouterLink class="btn" to="/dashboard">{{ t('members.appeal.back') }}</RouterLink>
      </div>

      <div v-else class="appeal-list">
        <article v-for="item in items" :key="item.tenant_id" class="appeal-item">
          <div>
            <strong>{{ item.tenant_name }}</strong>
            <span>{{ t(`members.statuses.${item.status}`) }}</span>
            <small v-if="item.appeal_state">
              {{ t('members.appeal.currentState', { state: item.appeal_state }) }}
            </small>
          </div>
          <UiButton class="btn" @click="choose(item)">{{ t('members.appeal.request') }}</UiButton>
        </article>
      </div>

      <div v-if="target" class="appeal-form">
        <h2>{{ t('members.appeal.formTitle', { tenant: target.tenant_name }) }}</h2>
        <p>{{ t('members.appeal.formHint') }}</p>
        <label>
          <span>{{ t('members.appeal.reason') }}</span>
          <textarea
            v-model="reason"
            class="input appeal-reason"
            maxlength="500"
            :placeholder="t('members.appeal.reasonPlaceholder')"
          />
        </label>
        <div class="appeal-actions">
          <UiButton class="btn" :disabled="submitting" @click="target=null">{{ t('common.cancel') }}</UiButton>
          <UiButton class="btn btn-primary" :disabled="submitting || !reason.trim()" @click="submit">
            {{ submitting ? t('common.processing') : t('members.appeal.submit') }}
          </UiButton>
        </div>
      </div>

      <div v-if="receipt" class="appeal-receipt" role="status">
        <strong>{{ t('members.appeal.accepted') }}</strong>
        <p>{{ t('members.appeal.acceptedHint') }}</p>
        <dl>
          <div><dt>{{ t('members.appeal.receiptId') }}</dt><dd>{{ receipt.appeal_id }}</dd></div>
          <div><dt>{{ t('members.appeal.receiptState') }}</dt><dd>{{ receipt.state }}</dd></div>
          <div><dt>{{ t('members.appeal.receiptTime') }}</dt><dd>{{ receipt.submitted_at }}</dd></div>
        </dl>
      </div>
    </div>
  </section>
</template>

<style scoped>
.appeal-page{max-width:760px;margin:48px auto;padding:0 20px}.appeal-card{padding:24px;display:flex;flex-direction:column;gap:20px}.appeal-heading{display:flex;justify-content:space-between;align-items:flex-start;gap:20px}.appeal-heading h1{margin-top:4px;font-size:24px}.appeal-heading p,.appeal-muted,.appeal-empty p,.appeal-form p,.appeal-receipt p{margin-top:8px;color:var(--color-text-secondary);line-height:1.65}.eyebrow{font-size:11px;color:var(--color-primary);font-weight:700}.appeal-alert{padding:10px 12px;border-radius:var(--radius-sm);background:var(--color-danger-soft);color:var(--color-danger)}.appeal-list{display:flex;flex-direction:column;gap:10px}.appeal-item{display:flex;align-items:center;justify-content:space-between;gap:16px;padding:14px;border:1px solid var(--color-border);border-radius:var(--radius-md)}.appeal-item strong,.appeal-item span,.appeal-item small{display:block}.appeal-item span,.appeal-item small{margin-top:4px;color:var(--color-text-muted);font-size:12px}.appeal-form{padding-top:18px;border-top:1px solid var(--color-border)}.appeal-form label>span{display:block;margin:14px 0 7px;font-size:12px;font-weight:600}.appeal-reason{width:100%;min-height:120px;padding:10px;resize:vertical}.appeal-actions{display:flex;justify-content:flex-end;gap:10px;margin-top:14px}.appeal-receipt{padding:14px;border-radius:var(--radius-md);background:var(--color-success-soft)}.appeal-receipt dl{margin-top:12px;display:grid;gap:8px}.appeal-receipt dl div{display:grid;grid-template-columns:120px 1fr;gap:12px;font-size:12px}.appeal-receipt dt{color:var(--color-text-muted)}.appeal-receipt dd{overflow-wrap:anywhere}@media(max-width:640px){.appeal-page{margin:20px auto;padding:0 12px}.appeal-card{padding:18px}.appeal-heading,.appeal-item{align-items:stretch;flex-direction:column}.appeal-receipt dl div{grid-template-columns:1fr}}
</style>
