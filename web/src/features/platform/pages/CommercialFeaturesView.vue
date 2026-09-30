<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { UiButton, UiInput, UiOption, UiSelect, UiTextarea } from '@/ui/base'
import PageHeading from '@/ui/common/PageHeading.vue'
import StatusBadge from '@/ui/common/StatusBadge.vue'
import { CommercialApiError, type CommercialFeatureDTO } from '@/services/commercial/platformCommercial'
import { createPlatformCommercialFeature, listPlatformCommercialFeatures, transitionPlatformCommercialFeature } from '@/services/commercial/platformFeatures'

const features = ref<CommercialFeatureDTO[]>([])
const loading = ref(true)
const error = ref('')
const pending = ref(false)
const selectedCode = ref('')
const message = ref('')
const featureCode = ref('')
const featureName = ref('')
const moduleCode = ref('')
const capabilityCodes = ref('')
const reason = ref('')
const replacementCode = ref('')
const migrationState = ref('REQUIRED')
const selected = computed(() => features.value.find((feature) => feature.featureCode === selectedCode.value) ?? null)
const impact = computed(() => selected.value?.referenceImpact)

function errorMessage(cause: unknown) {
  if (cause instanceof CommercialApiError && cause.code === 'forbidden') return '当前账号没有管理商业功能的权限。'
  if (cause instanceof CommercialApiError && cause.code === 'unauthenticated') return '登录会话已失效，请重新登录。'
  return '商业功能事实暂不可用，请稍后重试。'
}
async function load() {
  loading.value = true; error.value = ''
  try {
    features.value = await listPlatformCommercialFeatures()
    if (!selectedCode.value && features.value[0]) selectedCode.value = features.value[0].featureCode
    return true
  } catch (cause) { error.value = errorMessage(cause); return false } finally { loading.value = false }
}
async function confirmReadback(featureCode: string) {
  if (!await load()) return null
  const confirmed = features.value.find((feature) => feature.featureCode === featureCode) ?? null
  if (!confirmed) error.value = '操作已提交，但尚未读取到最新功能状态；请重新读取后再继续。'
  return confirmed
}
function select(feature: CommercialFeatureDTO) {
  selectedCode.value = feature.featureCode; reason.value = ''; message.value = ''; replacementCode.value = ''
}
async function create() {
  if (pending.value || !featureCode.value.trim() || !featureName.value.trim() || !moduleCode.value.trim() || !capabilityCodes.value.trim() || !reason.value.trim()) { error.value = '请填写功能代码、名称、模块能力和变更原因。'; return }
  pending.value = true; error.value = ''; message.value = ''
  try {
    const created = await createPlatformCommercialFeature({ featureCode: featureCode.value, name: featureName.value, moduleRefs: [{ moduleCode: moduleCode.value.trim(), capabilityCodes: capabilityCodes.value.split(/[\s,]+/).filter(Boolean) }], reason: reason.value })
    const confirmed = await confirmReadback(created.featureCode)
    if (confirmed) { select(confirmed); message.value = '已创建草稿；发布前会再次核对模块技术和销售事实。' }
    featureCode.value = ''; featureName.value = ''; moduleCode.value = ''; capabilityCodes.value = ''
  } catch (cause) { error.value = errorMessage(cause) } finally { pending.value = false }
}
async function transition(action: 'publish' | 'stop-selling' | 'sunset' | 'complete-migration' | 'retire') {
  const feature = selected.value
  if (!feature || pending.value || !reason.value.trim()) { error.value = '请选择功能并填写本次生命周期变更原因。'; return }
  if (action === 'sunset' && !replacementCode.value.trim()) { error.value = '规划 Sunset 前必须指定替代商业功能。'; return }
  pending.value = true; error.value = ''; message.value = ''
  try {
    const updated = await transitionPlatformCommercialFeature(feature, action, reason.value, { replacementCode: replacementCode.value, migrationState: migrationState.value })
    const confirmed = await confirmReadback(updated.featureCode)
    if (confirmed) { select(confirmed); message.value = '生命周期状态已确认并重新读取。' }
  } catch (cause) { error.value = errorMessage(cause) } finally { pending.value = false }
}
onMounted(load)
</script>

<template>
  <div class="page-stack" data-testid="ce290-commercial-features" data-ui-template="WorkbenchPage">
    <PageHeading title="商业功能" description="管理面向客户的功能发布、停售、迁移与退役；模块和权益始终以当前产品事实为准。" />
    <section v-if="loading" class="card state-card" role="status">正在读取商业功能目录…</section>
    <section v-else-if="error && !features.length" class="card state-card error" role="alert">{{ error }}<UiButton class="btn" @click="load">重新读取</UiButton></section>
    <template v-else>
      <section class="card panel" data-ui-region="query">
        <h2>新建商业功能</h2><p>功能通过稳定模块代码和能力代码引用，不按名称复制或关联技术定义。</p>
        <div class="form-grid"><label><span>功能代码</span><UiInput v-model="featureCode" placeholder="marketing-campaigns" /></label><label><span>客户名称</span><UiInput v-model="featureName" placeholder="营销活动" /></label><label><span>技术模块</span><UiInput v-model="moduleCode" placeholder="access-management" /></label><label><span>能力代码</span><UiInput v-model="capabilityCodes" placeholder="tenant.lifecycle" /></label></div>
        <label class="reason"><span>创建原因</span><UiTextarea v-model="reason" maxlength="500" placeholder="说明此功能的产品边界" /></label><UiButton class="btn primary" :disabled="pending" @click="create">创建草稿</UiButton>
      </section>
      <section class="card panel" data-ui-region="data"><div class="section-head"><div><h2>功能目录</h2><p>停售不会撤销现有租户权益；退役前必须完成迁移且无有效引用。</p></div><UiButton class="btn" @click="load">刷新</UiButton></div>
        <div v-if="!features.length" class="empty">当前没有商业功能。</div><div v-else class="feature-list"><UiButton v-for="feature in features" :key="feature.featureCode" type="button" :class="['feature-row',{selected:feature.featureCode===selectedCode}]" @click="select(feature)"><span><strong>{{ feature.name }}</strong><small>{{ feature.featureCode }} · v{{ feature.version }}</small></span><StatusBadge :text="feature.productState" :tone="feature.productState==='PUBLISHED'?'success':'warning'" /></UiButton></div>
      </section>
      <section v-if="selected" class="card panel" data-ui-region="detail"><div class="section-head"><div><h2>{{ selected.name }}</h2><p class="code">{{ selected.featureCode }} · v{{ selected.version }}</p></div><div class="badges"><StatusBadge :text="selected.salesState" :tone="selected.salesState==='SELLABLE'?'success':'neutral'" /><StatusBadge :text="selected.runtimeState" :tone="selected.runtimeState==='STOPPED'?'danger':'neutral'" /></div></div>
        <div class="facts"><div><span>产品状态</span><strong>{{ selected.productState }}</strong></div><div><span>迁移状态</span><strong>{{ selected.migrationState }}</strong></div><div><span>已发布套餐引用</span><strong>{{ impact?.publishedPlans ?? 0 }}</strong></div><div><span>有效订阅</span><strong>{{ impact?.activeSubscriptions ?? 0 }}</strong></div></div>
        <div class="refs"><strong>模块与能力引用</strong><p v-for="ref in selected.moduleRefs" :key="ref.moduleCode">{{ ref.moduleCode }}：{{ ref.capabilityCodes.join('、') }}</p></div>
        <label class="reason"><span>生命周期变更原因</span><UiTextarea v-model="reason" maxlength="500" placeholder="说明此操作的业务原因" /></label><div class="actions"><UiButton v-if="selected.productState==='DRAFT'||selected.productState==='PILOT'" class="btn primary" :disabled="pending" @click="transition('publish')">发布功能</UiButton><UiButton v-if="selected.salesState==='SELLABLE'" class="btn" :disabled="pending" @click="transition('stop-selling')">停止新售</UiButton><template v-if="selected.salesState==='STOP_SELL' && selected.productState!=='DEPRECATED'"><UiInput v-model="replacementCode" placeholder="替代功能代码" /><UiSelect v-model="migrationState"><UiOption value="RECOMMENDED">建议迁移</UiOption><UiOption value="REQUIRED">必须迁移</UiOption></UiSelect><UiButton class="btn" :disabled="pending" @click="transition('sunset')">规划 Sunset</UiButton></template><UiButton v-if="selected.productState==='DEPRECATED' && selected.migrationState!=='COMPLETE'" class="btn" :disabled="pending" @click="transition('complete-migration')">确认迁移完成</UiButton><UiButton v-if="selected.migrationState==='COMPLETE'" class="btn destructive" :disabled="pending" @click="transition('retire')">最终退役</UiButton></div>
      </section>
      <p v-if="error" class="notice error" role="alert">{{ error }}</p><p v-if="message" class="notice" role="status">{{ message }}</p>
    </template>
  </div>
</template>

<style scoped>
.panel{padding:20px}.panel h2{margin:0;font-size:16px;line-height:1.25}.panel>p,.section-head p,.refs p{color:var(--color-text-muted);font-size:13px;line-height:1.55;max-width:68ch;text-wrap:pretty}.form-grid,.facts{display:grid;grid-template-columns:repeat(2,minmax(0,1fr));gap:12px;margin:14px 0}.form-grid label,.reason{display:grid;gap:6px;color:var(--color-text-secondary);font-size:13px}.reason{margin:14px 0}.section-head{display:flex;justify-content:space-between;gap:16px}.feature-list{display:grid;gap:8px;margin-top:14px}.feature-row{display:flex;justify-content:space-between;gap:12px;text-align:left;padding:13px;border:1px solid var(--color-border);border-radius:9px;background:var(--color-surface);color:var(--color-text-primary)}.feature-row.selected{border-color:var(--color-primary);background:var(--color-primary-soft)}.feature-row small,.code{display:block;margin-top:4px;overflow-wrap:break-word;font-variant-numeric:tabular-nums}.facts>div{padding:11px;border-radius:8px;background:var(--color-surface-muted)}.facts span{display:block;color:var(--color-text-muted);font-size:12px}.facts strong{display:block;margin-top:4px;font-variant-numeric:tabular-nums}.refs{padding:12px;border:1px solid var(--color-border);border-radius:8px}.refs p{margin:5px 0 0;overflow-wrap:break-word}.actions{display:flex;flex-wrap:wrap;gap:8px;align-items:center}.badges{display:flex;gap:8px;align-items:flex-start}.state-card,.empty,.notice{padding:20px}.error{color:var(--color-danger)}@media(max-width:700px){.form-grid,.facts{grid-template-columns:1fr}.section-head{align-items:flex-start;flex-direction:column}.actions{align-items:stretch}.actions .btn{width:100%}}
</style>
