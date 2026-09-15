<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { UiButton, UiInput, UiOption, UiSelect, UiTextarea } from '@/ui/base'
import AppIcon from '@/ui/common/AppIcon.vue'
import PageHeading from '@/ui/common/PageHeading.vue'
import StatusBadge from '@/ui/common/StatusBadge.vue'
import { useUiStore } from '@/stores/ui'
import {
  getEnterpriseTenantProfile,
  readEnterpriseTenantProfileSession,
  tenantProfileRequestId,
  tenantProfileRuntimeError,
  updateEnterpriseTenantProfile,
  type EnterpriseTenantProfile,
  type EnterpriseTenantProfileDraft,
} from '@/services/enterprise/tenantProfileRuntime'
import type { TrustedSession } from '@/services/runtime/api'

const ui = useUiStore()
const session = ref<TrustedSession | null>(null)
const profile = ref<EnterpriseTenantProfile | null>(null)
const draft = ref<EnterpriseTenantProfileDraft>(emptyDraft())
const loading = ref(true)
const saving = ref(false)
const error = ref('')
const confirmed = ref('')
const writeKey = ref('')

function emptyDraft(): EnterpriseTenantProfileDraft {
  return {
    name: '',
    shortName: '',
    industry: '',
    companySize: '',
    timezone: 'Asia/Shanghai',
    contactName: '',
    phone: '',
    email: '',
    address: '',
    description: '',
    logoAssetRef: '',
  }
}

function toDraft(value: EnterpriseTenantProfile): EnterpriseTenantProfileDraft {
  return {
    name: value.name,
    shortName: value.shortName,
    industry: value.industry,
    companySize: value.companySize,
    timezone: value.timezone,
    contactName: value.contactName,
    phone: value.phone,
    email: value.email,
    address: value.address,
    description: value.description,
    logoAssetRef: value.logoAssetRef,
  }
}

const changed = computed(
  () => profile.value !== null && JSON.stringify(draft.value) !== JSON.stringify(toDraft(profile.value)),
)

async function load() {
  loading.value = true
  error.value = ''
  confirmed.value = ''
  try {
    const trusted = await readEnterpriseTenantProfileSession()
    session.value = trusted
    const value = await getEnterpriseTenantProfile(trusted)
    profile.value = value
    draft.value = toDraft(value)
  } catch (cause) {
    error.value = tenantProfileRuntimeError(cause)
  } finally {
    loading.value = false
  }
}

function validate() {
  if (!draft.value.name.trim()) return '请填写企业名称。'
  if (!draft.value.shortName.trim()) return '请填写企业简称。'
  if (!draft.value.timezone.trim()) return '请选择默认时区。'
  if (draft.value.email && !/^[^\s@]+@[^\s@]+\.[^\s@]+$/.test(draft.value.email)) return '请填写有效的企业邮箱。'
  if (draft.value.logoAssetRef.toLowerCase().startsWith('data:')) return '生产模式不接受 DataURL Logo，请使用资产服务引用。'
  return ''
}

async function save() {
  error.value = validate()
  confirmed.value = ''
  const trusted = session.value
  const current = profile.value
  if (error.value || !trusted || !current) return
  saving.value = true
  if (!writeKey.value) writeKey.value = tenantProfileRequestId(current.tenantId || 'active')
  try {
    await updateEnterpriseTenantProfile(trusted, current, draft.value, writeKey.value)
    const readback = await getEnterpriseTenantProfile(trusted)
    profile.value = readback
    draft.value = toDraft(readback)
    writeKey.value = ''
    confirmed.value = `企业资料已由服务端确认 · v${readback.version}`
    ui.toast('企业资料已由服务端确认。')
  } catch (cause) {
    error.value = tenantProfileRuntimeError(cause)
  } finally {
    saving.value = false
  }
}

function cancel() {
  if (!profile.value) return
  draft.value = toDraft(profile.value)
  error.value = ''
  writeKey.value = ''
}

function upload() {
  ui.toast('API 模式禁止 DataURL Logo；Logo 文件需先进入资产服务，再保存 logo_asset_ref。', 'info')
}

onMounted(load)
</script>

<template>
  <div class="page-stack" data-enterprise-company-source="server">
    <PageHeading title="企业信息" description="维护当前租户的权威企业资料；保存后必须从服务端回读确认" />
    <p v-if="confirmed" class="notice-box" role="status">{{ confirmed }}</p>
    <p v-if="error" class="form-error" role="alert">{{ error }}</p>
    <section v-if="loading" class="card panel-pad">正在读取服务端企业资料…</section>
    <div v-else-if="profile" class="split-layout">
      <form class="card panel-pad company-form" @submit.prevent="save">
        <div class="row-between block-title">
          <h2>基本信息</h2>
          <StatusBadge text="服务端权威资料" />
        </div>
        <div class="form-grid">
          <label class="field"><span class="required">企业名称</span><UiInput v-model="draft.name" class="input" required maxlength="200" /></label>
          <label class="field"><span class="required">企业简称</span><UiInput v-model="draft.shortName" class="input" required maxlength="80" /></label>
          <label class="field"><span>所属行业</span><UiInput v-model="draft.industry" class="input" maxlength="120" /></label>
          <label class="field"><span>企业规模</span><UiSelect v-model="draft.companySize" class="select"><UiOption value="">未设置</UiOption><UiOption value="1–10 人">1–10 人</UiOption><UiOption value="10–50 人">10–50 人</UiOption><UiOption value="50–200 人">50–200 人</UiOption><UiOption value="200 人以上">200 人以上</UiOption></UiSelect></label>
          <label class="field"><span class="required">默认时区</span><UiSelect v-model="draft.timezone" class="select"><UiOption value="Asia/Shanghai">中国标准时间 · UTC+08:00</UiOption><UiOption value="Asia/Taipei">台北时间 · UTC+08:00</UiOption><UiOption value="UTC">协调世界时 · UTC</UiOption><UiOption value="Europe/Berlin">欧洲柏林 · 按当地夏令时规则</UiOption></UiSelect></label>
          <div class="field">
            <span>企业 Logo</span>
            <div class="logo-control">
              <span class="logo-placeholder"><AppIcon name="company" :size="24" /></span>
              <UiButton class="btn" type="button" @click="upload"><AppIcon name="upload" :size="15" />资产服务上传</UiButton>
            </div>
            <small v-if="draft.logoAssetRef" class="mono">{{ draft.logoAssetRef }}</small>
            <small v-else>当前未绑定生产资产；不使用浏览器 DataURL 冒充上传。</small>
          </div>
          <label class="field full-width"><span>企业简介</span><UiTextarea v-model="draft.description" class="textarea" rows="3" maxlength="1000" /></label>
        </div>
        <section class="form-section">
          <h3>联系信息</h3>
          <div class="form-grid">
            <label class="field"><span>联系人</span><UiInput v-model="draft.contactName" class="input" maxlength="100" /></label>
            <label class="field"><span>联系电话</span><UiInput v-model="draft.phone" class="input" maxlength="40" /></label>
            <label class="field"><span>企业邮箱</span><UiInput v-model="draft.email" class="input" type="email" maxlength="320" /></label>
            <label class="field"><span>联系地址</span><UiInput v-model="draft.address" class="input" maxlength="500" /></label>
          </div>
        </section>
        <div class="form-footer">
          <span v-if="changed" class="muted flex-1">有尚未保存的修改</span>
          <UiButton class="btn" type="button" :disabled="saving" @click="cancel">取消修改</UiButton>
          <UiButton class="btn btn-primary" type="submit" :disabled="saving || !changed"><AppIcon name="check" :size="15" />{{ saving ? '保存中…' : '保存并回读确认' }}</UiButton>
        </div>
      </form>
      <aside class="side-summary">
        <section class="card panel-pad">
          <h2>权威资料概览</h2>
          <div class="company-identity">
            <span><AppIcon name="company" :size="28" /></span>
            <h3>{{ profile.name }}</h3>
            <p>{{ profile.industry || '未设置行业' }}</p>
          </div>
          <dl class="detail-list">
            <dt>租户标识</dt><dd class="mono">{{ profile.tenantId }}</dd>
            <dt>资料版本</dt><dd class="mono">v{{ profile.version }}</dd>
            <dt>默认时区</dt><dd>{{ profile.timezone }}</dd>
            <dt>数据来源</dt><dd>Tenant Profile API</dd>
          </dl>
          <div class="divider" />
          <div class="notice-box"><AppIcon name="help" :size="16" />本页租户范围来自可信会话，不接受前端提交 tenant_id；Logo 只保存资产引用。</div>
        </section>
      </aside>
    </div>
  </div>
</template>

<style scoped>
.company-form { padding: 28px; }
.logo-control { display: flex; align-items: center; gap: 16px; }
.logo-placeholder { width: 48px; height: 48px; display: grid; place-items: center; border-radius: 14px; color: var(--color-primary); background: var(--color-primary-soft); }
.company-identity { text-align: center; padding: 24px 5px; }
.company-identity > span { width: 64px; height: 64px; display: grid; place-items: center; background: var(--color-primary-soft); border-radius: 20px; color: var(--color-primary); margin: 0 auto 18px; }
.company-identity h3 { font-size: 15px; }
.company-identity p { font-size: 12px; color: var(--color-text-muted); margin-top: 8px; }
.side-summary .detail-list { grid-template-columns: 85px 1fr; }
@media (max-width: 767px) { .company-form { padding: 20px; } }
</style>
