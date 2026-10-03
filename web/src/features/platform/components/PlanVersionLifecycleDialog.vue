<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { UiButton, UiInput } from '@/ui/base'
import UiDialog from '@/ui/common/UiDialog.vue'
import StatusBadge from '@/ui/common/StatusBadge.vue'
import { backendStateTone, backendTermLabel } from '@/i18n/backend-terms'
import type { ModuleDTO, PlanVersionDTO } from '@/services/commercial/platformCommercial'

type LifecycleMode = 'preflight' | 'publish' | 'clone' | 'retire'

const props = withDefaults(
  defineProps<{
    open: boolean
    mode: LifecycleMode
    version?: PlanVersionDTO
    modules: ModuleDTO[]
    pending: boolean
    serverError?: string
  }>(),
  { version: undefined, serverError: '' },
)

const emit = defineEmits<{
  close: []
  advance: []
  publish: [reason: string]
  clone: [reason: string]
  retire: [reason: string]
}>()

const reason = ref('')
const localError = ref('')

const title = computed(() => ({
  preflight: '发布前检查',
  publish: '确认发布套餐版本',
  clone: '基于当前版本创建新版本',
  retire: '确认停售套餐版本',
}[props.mode]))

const statusText = computed(() => backendTermLabel('planState', props.version?.state))
const statusTone = computed(() => backendStateTone('planState', props.version?.state))

const knownModuleCodes = computed(() => new Set(props.modules.map((item) => item.moduleCode)))
const terms = computed(() => props.version?.terms)
const localChecks = computed(() => {
  const value = terms.value
  const modules = value?.modules ?? []
  const salesScopes = value?.salesScope ?? []
  const validityOk = value?.validityMode === 'unlimited'
    || (value?.validityMode === 'fixed_days'
      && Number.isInteger(Number(value?.validityDays))
      && Number(value?.validityDays) >= 1
      && Number(value?.validityDays) <= 36500)
  const moduleCodes = modules.map((item) => item.moduleCode).filter(Boolean)
  const modulesComplete = modules.every((item) => Boolean(item.moduleCode))
    && new Set(moduleCodes).size === moduleCodes.length
  const termsComplete = modules.every((item) =>
    (item.quotas ?? []).every((quota) =>
      Boolean(quota.key)
      && (quota.unlimited || (Number.isFinite(Number(quota.value)) && Number(quota.value) >= 0)))
    && (item.fields ?? []).every((field) => Boolean(field.key && field.action && field.mode)))
  const catalogState = modules.length && !props.modules.length
    ? null
    : modules.every((item) => knownModuleCodes.value.has(item.moduleCode))

  return [
    {
      label: '基础信息',
      detail: props.version?.planCode && props.version?.name && salesScopes.length
        ? '套餐代码、名称和销售范围已填写。'
        : '套餐代码、名称或销售范围仍不完整。',
      ok: Boolean(props.version?.planCode && props.version?.name && salesScopes.length),
      blocking: true,
    },
    {
      label: '有效期',
      detail: validityOk ? '有效期配置格式完整。' : '固定有效期需要填写 1～36500 天。',
      ok: validityOk,
      blocking: true,
    },
    {
      label: '模块与权益',
      detail: modulesComplete && termsComplete
        ? `已配置 ${modules.length} 个模块，额度和字段条款结构完整。`
        : '模块、额度或字段条款仍有未完成项。',
      ok: modulesComplete && termsComplete,
      blocking: true,
    },
    {
      label: '当前模块目录',
      detail: catalogState == null
        ? '当前模块目录暂未加载；发布时系统会再次核对。'
        : catalogState
          ? '页面已读取到当前模块目录中的对应模块。'
          : '存在当前目录未识别的模块；发布时系统会再次核对。',
      ok: catalogState,
      blocking: false,
    },
    {
      label: '依赖与可售状态',
      detail: '模块依赖、技术状态、可售状态和安全规则将在正式发布时再次校验。',
      ok: null,
      blocking: false,
    },
  ]
})

const localReady = computed(() =>
  localChecks.value.filter((item) => item.blocking).every((item) => item.ok === true))

const moduleSummary = computed(() => {
  const modules = props.version?.terms?.modules ?? []
  const capabilities = modules.reduce((sum, item) => sum + (item.capabilityCodes?.length ?? 0), 0)
  const quotas = modules.reduce((sum, item) => sum + (item.quotas?.length ?? 0), 0)
  return `${modules.length} 个模块 · ${capabilities} 项能力 · ${quotas} 项额度`
})

watch(
  () => [props.open, props.mode] as const,
  ([open, mode]) => {
    if (!open) return
    localError.value = ''
    reason.value = mode === 'publish'
      ? '发布当前套餐版本'
      : mode === 'clone'
        ? '基于当前版本创建新草稿'
        : mode === 'retire'
          ? '停止当前版本的新销售'
          : ''
  },
  { immediate: true },
)

function submit() {
  localError.value = ''
  if (!props.version) {
    localError.value = '当前版本信息不可用，请关闭后重新读取。'
    return
  }
  if (props.mode === 'preflight') {
    if (!localReady.value) {
      localError.value = '请先修正页面能够确认的配置问题，再继续发布。'
      return
    }
    emit('advance')
    return
  }
  const normalizedReason = reason.value.trim()
  if (!normalizedReason) {
    localError.value = '请填写本次操作原因。'
    return
  }
  if (props.mode === 'publish') emit('publish', normalizedReason)
  if (props.mode === 'clone') emit('clone', normalizedReason)
  if (props.mode === 'retire') emit('retire', normalizedReason)
}
</script>

<template>
  <UiDialog :open="open" :title="title" width="760px" @close="emit('close')">
    <div v-if="version" class="lifecycle-dialog" :data-plan-version-dialog="mode">
      <div class="version-context">
        <div>
          <span>当前版本</span>
          <strong>{{ version.name || version.planCode }} · v{{ version.version }}</strong>
          <small>{{ version.planCode }}</small>
        </div>
        <StatusBadge :text="statusText" :tone="statusTone" />
      </div>

      <template v-if="mode === 'preflight'">
        <p class="lead">
          先检查页面能够确认的配置完整性。正式发布时系统仍会基于最新目录重新校验依赖、可售状态和安全规则。
        </p>
        <div class="check-list" data-plan-preflight>
          <div v-for="item in localChecks" :key="item.label" class="check-row">
            <span
              class="check-mark"
              :class="{ pass: item.ok === true, warning: item.ok === false, pending: item.ok == null }"
              aria-hidden="true"
            >{{ item.ok === true ? '✓' : item.ok === false ? '!' : '…' }}</span>
            <div>
              <strong>{{ item.label }}</strong>
              <p>{{ item.detail }}</p>
            </div>
          </div>
        </div>
        <div class="boundary-note">
          此检查不是“发布成功”证明；只有发布请求成功并重新读取到已发布状态，才视为本次操作完成。
        </div>
      </template>

      <template v-else-if="mode === 'publish'">
        <div class="impact warning">
          <strong>发布后内容不可直接覆盖</strong>
          <p>后续需要调整模块、权益或范围时，必须创建新的草稿版本；当前版本会作为历史版本保留。</p>
        </div>
        <div class="facts">
          <div><span>套餐</span><strong>{{ version.name || version.planCode }}</strong></div>
          <div><span>版本</span><strong>v{{ version.version }}</strong></div>
          <div><span>销售范围</span><strong>{{ version.terms?.salesScope?.join('、') || '未设置' }}</strong></div>
          <div><span>模块与权益</span><strong>{{ moduleSummary }}</strong></div>
        </div>
        <label class="reason-field">
          <span>发布原因</span>
          <UiInput v-model="reason" autocomplete="off" />
          <small>用于审计和后续问题追溯。</small>
        </label>
      </template>

      <template v-else-if="mode === 'clone'">
        <div class="impact info">
          <strong>原版本保持不变</strong>
          <p>系统会复制当前版本的完整条款并创建新的草稿。新版本可继续编辑，不会覆盖来源版本。</p>
        </div>
        <div class="clone-grid">
          <div class="source-card">
            <span>来源版本</span>
            <strong>{{ version.name || version.planCode }} · v{{ version.version }}</strong>
            <small>{{ statusText }} · {{ moduleSummary }}</small>
          </div>
          <div class="target-card">
            <span>新版本</span>
            <strong>创建后进入草稿状态</strong>
            <small>版本号由系统分配，页面不预先猜测。</small>
          </div>
        </div>
        <label class="reason-field">
          <span>创建原因</span>
          <UiInput v-model="reason" autocomplete="off" />
          <small>说明为什么需要从当前历史版本继续迭代。</small>
        </label>
      </template>

      <template v-else>
        <div class="impact danger">
          <strong>停售不是删除</strong>
          <p>停售后该版本不再用于新的销售选择；历史内容和已有引用继续保留，不会自动迁移到其他版本。</p>
        </div>
        <div class="facts">
          <div><span>套餐</span><strong>{{ version.name || version.planCode }}</strong></div>
          <div><span>版本</span><strong>v{{ version.version }}</strong></div>
          <div><span>当前状态</span><strong>{{ statusText }}</strong></div>
          <div><span>历史引用</span><strong>继续保留</strong></div>
        </div>
        <label class="reason-field">
          <span>停售原因</span>
          <UiInput v-model="reason" autocomplete="off" />
          <small>用于审计和后续历史查询。</small>
        </label>
      </template>

      <div v-if="localError || serverError" class="notice danger" role="alert">
        {{ localError || serverError }}
      </div>
    </div>

    <template #footer>
      <UiButton class="btn" :disabled="pending" @click="emit('close')">
        {{ mode === 'preflight' ? '返回编辑' : '取消' }}
      </UiButton>
      <UiButton
        class="btn"
        :class="{ primary: mode !== 'retire', danger: mode === 'retire' }"
        :disabled="pending || (mode === 'preflight' && !localReady)"
        data-plan-version-dialog-submit
        @click="submit"
      >
        {{
          pending
            ? '处理中…'
            : mode === 'preflight'
              ? '继续发布'
              : mode === 'publish'
                ? '确认发布'
                : mode === 'clone'
                  ? '创建新版本'
                  : '确认停售'
        }}
      </UiButton>
    </template>
  </UiDialog>
</template>

<style scoped>
.lifecycle-dialog { display: grid; gap: 18px; }
.version-context {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 16px;
  padding: 14px;
  border-radius: var(--radius-md);
  background: var(--color-surface-muted);
}
.version-context span,
.facts span,
.clone-grid span,
.reason-field > span {
  display: block;
  color: var(--color-text-muted);
  font-size: var(--text-xs);
}
.version-context strong { display: block; margin-top: 4px; font-size: var(--text-base); }
.version-context small { display: block; margin-top: 4px; color: var(--color-text-secondary); }
.lead { margin: 0; max-width: 72ch; color: var(--color-text-secondary); font-size: var(--text-sm); line-height: 1.6; text-wrap: pretty; }
.check-list { display: grid; gap: 10px; }
.check-row {
  display: flex;
  gap: 12px;
  align-items: flex-start;
  padding: 12px 14px;
  border: 1px solid var(--color-border);
  border-radius: var(--radius-sm);
}
.check-row strong { display: block; font-size: var(--text-sm); }
.check-row p { margin: 4px 0 0; color: var(--color-text-secondary); font-size: var(--text-xs); line-height: 1.55; }
.check-mark {
  flex: 0 0 auto;
  width: 24px;
  height: 24px;
  display: grid;
  place-items: center;
  border-radius: 50%;
  background: var(--color-surface-muted);
  color: var(--color-text-muted);
  font-weight: 700;
}
.check-mark.pass { background: var(--color-success-soft, var(--color-primary-soft)); color: var(--color-success); }
.check-mark.warning { background: var(--color-warning-soft, var(--color-surface-muted)); color: var(--color-warning); }
.check-mark.pending { background: var(--color-primary-soft); color: var(--color-primary); }
.boundary-note {
  padding: 12px 14px;
  border-radius: var(--radius-sm);
  background: var(--color-primary-soft);
  color: var(--color-text-secondary);
  font-size: var(--text-xs);
  line-height: 1.6;
}
.impact { padding: 14px; border: 1px solid var(--color-border); border-radius: var(--radius-md); }
.impact strong { font-size: var(--text-sm); }
.impact p { margin: 5px 0 0; color: var(--color-text-secondary); font-size: var(--text-xs); line-height: 1.6; }
.impact.warning { background: var(--color-warning-soft, var(--color-surface-muted)); }
.impact.info { background: var(--color-primary-soft); }
.impact.danger { background: var(--color-danger-soft); }
.facts,
.clone-grid {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 10px;
}
.facts > div,
.source-card,
.target-card {
  min-width: 0;
  padding: 12px;
  border-radius: var(--radius-sm);
  background: var(--color-surface-muted);
}
.facts strong,
.clone-grid strong { display: block; margin-top: 5px; font-size: var(--text-sm); overflow-wrap: anywhere; }
.clone-grid small { display: block; margin-top: 5px; color: var(--color-text-secondary); font-size: var(--text-xs); line-height: 1.5; }
.reason-field { display: grid; gap: 7px; }
.reason-field small { color: var(--color-text-muted); font-size: var(--text-xs); }
.notice { padding: 11px 13px; border: 1px solid var(--color-border); border-radius: var(--radius-sm); font-size: var(--text-sm); }
.notice.danger { border-color: var(--color-danger); background: var(--color-danger-soft); color: var(--color-danger); }
.btn.primary { background: var(--color-primary); border-color: var(--color-primary); color: var(--color-on-primary); }
.btn.danger { background: var(--color-danger); border-color: var(--color-danger); color: var(--color-on-primary); }
@media (max-width: 767px) {
  .version-context { align-items: stretch; flex-direction: column; }
  .facts, .clone-grid { grid-template-columns: 1fr; }
}
</style>
