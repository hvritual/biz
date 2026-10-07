<script setup lang="ts">
import { UiButton, UiInput, UiOption, UiSelect, UiTextarea } from '@/ui/base'
import StatusBadge from '@/ui/common/StatusBadge.vue'
import { backendStateTone, backendTermLabel } from '@/i18n/backend-terms'
import { useInitialSubscription } from '@/features/platform/composables/useInitialSubscription'

const props = defineProps<{ tenantId: string }>()
const emit = defineEmits<{ refresh: [] }>()

const {
  plans,
  salesScope,
  planCode,
  candidates,
  selectedVersionKey,
  previewReason,
  confirmReason,
  approved,
  preview,
  receipt,
  task,
  finalSubscription,
  verificationState,
  loadingPlans,
  loadingTargets,
  pending,
  errorMessage,
  statusMessage,
  selectedVersion,
  eligibleCandidates,
  provisioningStatus,
  receiptTone,
  versionKey,
  formatValidity,
  limitLabel,
  loadPublishedVersions,
  createPreview,
  confirmPreview,
  refreshResult,
  retryTask,
} = useInitialSubscription(() => props.tenantId, () => emit('refresh'))
</script>

<template>
  <section class="card initial-card" data-testid="platform-initial-subscription">
    <header class="section-header">
      <div>
        <h2>首次开通套餐</h2>
        <p>当前租户没有基础订阅。请选择精确已发布版本，确认后再以最终权益结果作为完成依据。</p>
      </div>
      <StatusBadge
        v-if="receipt"
        :text="backendTermLabel('receiptStatus', receipt.status)"
        :tone="receiptTone"
      />
    </header>

    <div class="guard-note">
      <strong>租户套餐权益与成员权限是两层独立控制。</strong>
      <span>完成首次开通不会自动给成员分配角色或操作权限。</span>
    </div>

    <div class="target-grid">
      <label class="field" for="initial-sales-scope">
        <span>适用范围</span>
        <UiInput id="initial-sales-scope" v-model="salesScope" class="input" placeholder="例如 rental、office、enterprise" />
        <small>必须是具体范围；系统会据此判断套餐版本是否可开通。</small>
      </label>

      <label class="field" for="initial-plan-code">
        <span>套餐</span>
        <UiSelect id="initial-plan-code" v-model="planCode" class="input" :disabled="loadingPlans">
          <UiOption value="">请选择套餐</UiOption>
          <UiOption v-for="plan in plans" :key="plan.planCode" :value="plan.planCode">
            {{ plan.name || backendTermLabel('plan', plan.planCode) }} · {{ plan.planCode }}
          </UiOption>
        </UiSelect>
        <small v-if="loadingPlans">正在读取平台套餐目录…</small>
      </label>

      <div class="target-action">
        <UiButton class="btn primary" type="button" :disabled="loadingTargets || !salesScope.trim() || !planCode" @click="loadPublishedVersions">
          {{ loadingTargets ? '正在检查…' : '检查已发布版本' }}
        </UiButton>
      </div>
    </div>

    <section v-if="candidates.length" class="versions-panel">
      <div class="section-subhead">
        <div><h3>精确版本</h3><p>只列出真实已发布版本；是否可用以当前资格检查结果为准。</p></div>
        <span>{{ eligibleCandidates.length }} 个可用</span>
      </div>
      <div class="version-list">
        <label
          v-for="candidate in candidates"
          :key="versionKey(candidate.version)"
          class="version-option"
          :class="{ selected: selectedVersionKey === versionKey(candidate.version), blocked: !candidate.eligible }"
        >
          <UiInput
            v-model="selectedVersionKey"
            type="radio"
            name="initial-plan-version"
            :value="versionKey(candidate.version)"
            :disabled="!candidate.eligible"
          />
          <span>
            <strong>{{ candidate.version.name || backendTermLabel('plan', candidate.version.planCode) }} · v{{ candidate.version.version }}</strong>
            <small>{{ candidate.eligible ? '当前范围可开通' : '当前范围不可开通' }}</small>
          </span>
        </label>
      </div>
    </section>

    <section v-if="selectedVersion" class="target-detail">
      <header>
        <div>
          <h3>{{ selectedVersion.name || backendTermLabel('plan', selectedVersion.planCode) }}</h3>
          <p>{{ selectedVersion.planCode }} · exact v{{ selectedVersion.version }}</p>
        </div>
        <StatusBadge :text="backendTermLabel('planState', selectedVersion.state)" :tone="backendStateTone('planState', selectedVersion.state)" />
      </header>

      <div class="facts-grid">
        <div><span>适用范围</span><strong>{{ salesScope }}</strong></div>
        <div><span>有效期</span><strong>{{ formatValidity(selectedVersion) }}</strong></div>
        <div><span>模块数</span><strong>{{ selectedVersion.terms?.modules?.length ?? 0 }}</strong></div>
        <div><span>价格处理</span><strong>{{ selectedVersion.terms?.priceRef ? '需要外部商业审批' : '无价格引用' }}</strong></div>
      </div>

      <article v-for="module in selectedVersion.terms?.modules ?? []" :key="module.moduleCode" class="module-row">
        <div class="module-title">
          <strong>{{ backendTermLabel('module', module.moduleCode) }}</strong>
          <span>{{ module.moduleCode }}</span>
        </div>
        <div class="term-grid">
          <div>
            <small>功能能力</small>
            <p v-if="!module.capabilityCodes?.length">无</p>
            <p v-for="capability in module.capabilityCodes ?? []" :key="capability">{{ backendTermLabel('entitlementKey', capability) }}</p>
          </div>
          <div>
            <small>额度</small>
            <p v-if="!module.quotas?.length">无</p>
            <p v-for="quota in module.quotas ?? []" :key="quota.key">{{ backendTermLabel('entitlementKey', quota.key) }}：{{ limitLabel(quota) }}</p>
          </div>
        </div>
      </article>

      <div v-if="!preview" class="preview-form">
        <label class="field wide" for="initial-preview-reason">
          <span>首次开通原因</span>
          <UiTextarea id="initial-preview-reason" v-model="previewReason" class="input" rows="2" placeholder="说明为什么为该租户开通此套餐版本" />
        </label>
        <UiButton class="btn primary" type="button" :disabled="pending" @click="createPreview">查看首次开通方案</UiButton>
      </div>
    </section>

    <section v-if="preview" class="preview-panel">
      <div class="section-subhead">
        <div><h3>首次开通方案</h3><p>目标 {{ preview.target?.planCode }} · exact v{{ preview.target?.version }}</p></div>
        <span>{{ preview.provisioningRequirements?.length ? '需要开通准备' : '可直接生效' }}</span>
      </div>

      <div class="facts-grid">
        <div><span>生效方式</span><strong>{{ backendTermLabel('effectiveMode', preview.mode) }}</strong></div>
        <div><span>预计权益来源版本</span><strong>{{ preview.projectedEntitlements?.sourceVersion ?? '待确认' }}</strong></div>
        <div><span>模块依赖</span><strong>{{ preview.dependencies?.length ?? 0 }} 项</strong></div>
        <div><span>准备任务</span><strong>{{ preview.provisioningRequirements?.length ?? 0 }} 项</strong></div>
      </div>

      <div class="confirm-box">
        <label class="check">
          <UiInput v-model="approved" type="checkbox" />
          <span>我已核对 exact 套餐版本、模块/能力/额度、适用范围和准备要求，并确认继续。</span>
        </label>
        <label class="field" for="initial-confirm-reason">
          <span>确认原因</span>
          <UiTextarea id="initial-confirm-reason" v-model="confirmReason" class="input" rows="2" placeholder="说明本次批准依据" />
        </label>
        <UiButton class="btn primary" type="button" :disabled="pending" @click="confirmPreview">确认首次开通</UiButton>
      </div>
    </section>

    <section v-if="receipt" class="result-panel">
      <div class="section-subhead">
        <div><h3>开通结果</h3><p>变更编号 {{ receipt.changeId }}</p></div>
        <StatusBadge :text="backendTermLabel('receiptStatus', receipt.status)" :tone="receiptTone" />
      </div>

      <div v-if="receipt.status === 'PROVISIONING'" class="processing-state">
        <strong>{{ provisioningStatus }}</strong>
        <span>目标套餐权益尚未正式生效。系统会继续使用同一准备任务，不会重复创建订阅。</span>
        <div class="result-actions">
          <UiButton class="btn" type="button" :disabled="pending" @click="refreshResult">重新读取结果</UiButton>
          <UiButton v-if="task?.retryAllowed" class="btn primary" type="button" :disabled="pending" @click="retryTask">恢复原准备任务</UiButton>
        </div>
      </div>

      <div v-else-if="verificationState === 'verified'" class="verified-state">
        <strong>首次开通已完成，最终权益已确认</strong>
        <span>当前订阅为 {{ finalSubscription?.planCode }} · v{{ finalSubscription?.planVersion }}；页面显示的是当前最终权益结果，不是预估结果。</span>
      </div>

      <div v-else-if="verificationState === 'failed'" class="verification-state">
        <strong>订阅回执已存在，最终权益仍待确认</strong>
        <span>不要再次提交首次开通。重新读取 Subscription、Change Receipt 和 Entitlement 后再判断结果。</span>
        <UiButton class="btn" type="button" :disabled="pending" @click="refreshResult">重新读取结果</UiButton>
      </div>

      <div v-else class="processing-state">
        <strong>结果待确认</strong>
        <span>当前不会显示租户已获得目标模块，直到最终权益确认完成。</span>
        <UiButton class="btn" type="button" :disabled="pending" @click="refreshResult">读取最新结果</UiButton>
      </div>
    </section>

    <p v-if="statusMessage" class="notice success" role="status">{{ statusMessage }}</p>
    <p v-if="errorMessage" class="notice danger" role="alert">{{ errorMessage }}</p>
  </section>
</template>

<style scoped>
.initial-card { overflow: hidden; }
.section-header, .section-subhead, .target-detail > header { display: flex; align-items: flex-start; justify-content: space-between; gap: 14px; }
.section-header { padding: 18px 20px; border-bottom: 1px solid var(--color-border); }
.section-header h2, .section-subhead h3, .target-detail h3 { margin: 0; font-size: 16px; }
.section-header p, .section-subhead p, .target-detail header p { margin: 5px 0 0; color: var(--color-text-muted); font-size: 12px; line-height: 1.5; }
.guard-note { display: grid; gap: 4px; margin: 16px 20px 0; padding: 12px; border: 1px solid var(--color-warning); border-radius: 8px; font-size: 12px; }
.guard-note span { color: var(--color-text-secondary); }
.target-grid { display: grid; grid-template-columns: minmax(180px, .8fr) minmax(240px, 1.2fr) auto; gap: 12px; align-items: end; padding: 18px 20px; }
.field { display: grid; gap: 6px; min-width: 0; font-size: 12px; color: var(--color-text-secondary); }
.field small { color: var(--color-text-muted); line-height: 1.4; }
.field.wide { width: 100%; }
.input { width: 100%; min-height: 36px; }
.target-action { padding-bottom: 1px; }
.btn.primary { background: var(--color-primary); border-color: var(--color-primary); color: var(--color-on-primary); }
.versions-panel, .target-detail, .preview-panel, .result-panel { border-top: 1px solid var(--color-border); }
.section-subhead { padding: 15px 20px; background: var(--color-surface-subtle); }
.section-subhead > span { color: var(--color-text-muted); font-size: 12px; white-space: nowrap; }
.version-list { display: grid; gap: 8px; padding: 14px 20px 18px; }
.version-option { display: flex; gap: 9px; align-items: flex-start; padding: 11px 12px; border: 1px solid var(--color-border); border-radius: 8px; cursor: pointer; }
.version-option.selected { border-color: var(--color-primary); background: var(--color-primary-soft); }
.version-option.blocked { opacity: .65; cursor: not-allowed; }
.version-option span { display: grid; gap: 4px; }
.version-option small { color: var(--color-text-muted); }
.target-detail > header { padding: 16px 20px; }
.facts-grid { display: grid; grid-template-columns: repeat(4, minmax(0, 1fr)); gap: 1px; background: var(--color-border); }
.facts-grid > div { min-width: 0; padding: 12px 14px; background: var(--color-surface); }
.facts-grid span { display: block; color: var(--color-text-muted); font-size: 11px; }
.facts-grid strong { display: block; margin-top: 4px; font-size: 13px; overflow-wrap: anywhere; }
.module-row { padding: 14px 20px; border-top: 1px solid var(--color-border); }
.module-title { display: flex; justify-content: space-between; gap: 12px; }
.module-title span { color: var(--color-text-muted); font-size: 11px; overflow-wrap: anywhere; }
.term-grid { display: grid; grid-template-columns: 1fr 1fr; gap: 16px; margin-top: 10px; }
.term-grid small { color: var(--color-text-muted); }
.term-grid p { margin: 5px 0 0; font-size: 12px; }
.preview-form, .confirm-box { display: grid; gap: 12px; padding: 16px 20px 20px; border-top: 1px solid var(--color-border); }
.check { display: flex; align-items: flex-start; gap: 8px; color: var(--color-text-secondary); font-size: 12px; line-height: 1.5; }
.processing-state, .verified-state, .verification-state { display: grid; gap: 6px; padding: 18px 20px; font-size: 13px; }
.processing-state span, .verified-state span, .verification-state span { color: var(--color-text-secondary); line-height: 1.5; }
.result-actions { display: flex; gap: 8px; flex-wrap: wrap; margin-top: 6px; }
.notice { margin: 0 20px 18px; padding: 10px 12px; border: 1px solid var(--color-border); border-radius: 8px; font-size: 12px; }
.notice.success { border-color: var(--color-success); }
.notice.danger { border-color: var(--color-danger); }
@media (max-width: 900px) {
  .target-grid { grid-template-columns: 1fr 1fr; }
  .target-action { grid-column: 1 / -1; }
  .facts-grid { grid-template-columns: repeat(2, minmax(0, 1fr)); }
}
@media (max-width: 680px) {
  .target-grid, .facts-grid, .term-grid { grid-template-columns: 1fr; }
  .section-header, .section-subhead, .target-detail > header { flex-direction: column; }
  .input { font-size: 16px; }
}
</style>
