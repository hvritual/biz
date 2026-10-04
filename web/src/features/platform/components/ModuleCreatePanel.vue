<script setup lang="ts">
import { computed } from 'vue'
import { UiInput, UiOption, UiSelect, UiTextarea } from '@/ui/base'
import AppIcon from '@/ui/common/AppIcon.vue'
import StatusBadge from '@/ui/common/StatusBadge.vue'
import { backendTermLabel } from '@/i18n/backend-terms'
import type { ModuleDTO } from '@/services/commercial/platformCommercial'
import type { PlatformModuleDefinitionDTO } from '@/services/commercial/platformModules'
import type { ModuleCreateDraft, ModuleResultState, ModuleScreen } from '../composables/useModuleManagement'

const props = defineProps<{
  definitions: PlatformModuleDefinitionDTO[]
  selectedDefinition: PlatformModuleDefinitionDTO | null
  missingDependencies: string[]
  draft: ModuleCreateDraft
  screen: ModuleScreen
  busy: boolean
  resultState: ModuleResultState
  resultMessage: string
  created: ModuleDTO | null
  definitionState: 'idle' | 'loading' | 'ready' | 'error'
  definitionError: string
}>()
const emit = defineEmits<{ draft: [value: ModuleCreateDraft] }>()
const scopeValues = computed(() => props.draft.salesScope.split(/[,，\n]/).map((value) => value.trim()).filter(Boolean))
function updateDraft(key: keyof ModuleCreateDraft, value: unknown) {
  emit('draft', { ...props.draft, [key]: String(value ?? '') })
}
</script>

<template>
  <div class="module-create-panel">
    <template v-if="screen === 'create'">
      <div class="module-notice">
        <strong>从已注册定义创建模块目录记录</strong>
        <p>模块代码、能力、额度、字段和依赖来自当前运行注册表，只读。创建目录记录不会新增技术能力，也不会给租户或成员授予权限。</p>
      </div>
      <div v-if="definitionState === 'loading'" class="module-state-inline" role="status">
        <AppIcon name="refresh" :size="18" />正在读取可创建模块定义…
      </div>
      <div v-else-if="definitionState === 'error'" class="module-notice danger" role="alert">{{ definitionError }}</div>
      <label class="field">
        <span>模块定义 <span class="module-required">*</span></span>
        <UiSelect :model-value="draft.moduleCode" class="select" :disabled="busy || definitionState !== 'ready'" @update:model-value="updateDraft('moduleCode', $event)">
          <UiOption value="">请选择尚未创建的模块定义</UiOption>
          <UiOption v-for="definition in definitions" :key="definition.moduleCode" :value="definition.moduleCode">
            {{ backendTermLabel('module', definition.moduleCode) }} · {{ definition.moduleCode }}
          </UiOption>
        </UiSelect>
        <small v-if="definitionState === 'ready' && !definitions.length">当前注册表没有可新增的模块定义。</small>
      </label>
      <section v-if="selectedDefinition" class="module-route-detail">
        <div class="module-section-header compact">
          <div>
            <h4>{{ backendTermLabel('module', selectedDefinition.moduleCode) }}</h4>
            <code>{{ selectedDefinition.moduleCode }}</code>
          </div>
          <StatusBadge :text="selectedDefinition.implementationReady ? '实现已就绪' : '实现未就绪'" :tone="selectedDefinition.implementationReady ? 'success' : 'warning'" />
        </div>
        <div class="module-governance compact-grid">
          <section><h4>能力</h4><div class="module-chips"><span v-for="code in selectedDefinition.capabilityCodes" :key="code" class="module-tag">{{ backendTermLabel('entitlementKey', code) }}</span></div></section>
          <section><h4>依赖</h4><p v-if="!selectedDefinition.dependencies.length">无</p><div class="module-chips"><span v-for="code in selectedDefinition.dependencies" :key="code" class="module-tag">{{ backendTermLabel('module', code) }}</span></div></section>
          <section><h4>额度定义</h4><p v-if="!selectedDefinition.quotaSchemaKeys.length">未配置</p><div class="module-chips"><span v-for="code in selectedDefinition.quotaSchemaKeys" :key="code" class="module-tag">{{ backendTermLabel('entitlementKey', code) }}</span></div></section>
          <section><h4>字段规则</h4><p v-if="!selectedDefinition.fieldPolicySchemaKeys.length">未配置</p><div class="module-chips"><span v-for="code in selectedDefinition.fieldPolicySchemaKeys" :key="code" class="module-tag">{{ backendTermLabel('entitlementKey', code) }}</span></div></section>
        </div>
        <div v-if="missingDependencies.length" class="module-notice warning" role="alert">依赖模块尚未创建：{{ missingDependencies.join('、') }}。请先创建依赖模块。</div>
      </section>
      <div class="module-form-grid">
        <label class="field"><span>模块名称 <span class="module-required">*</span></span><UiInput :model-value="draft.name" class="input" :disabled="busy" maxlength="200" @update:model-value="updateDraft('name', $event)" /></label>
        <label class="field"><span>分类 <span class="module-required">*</span></span><UiInput :model-value="draft.category" class="input" :disabled="busy" maxlength="200" @update:model-value="updateDraft('category', $event)" /></label>
        <label class="field module-full"><span>销售范围</span><UiInput :model-value="draft.salesScope" class="input" :disabled="busy" placeholder="多个范围使用逗号分隔" @update:model-value="updateDraft('salesScope', $event)" /></label>
      </div>
      <label class="field">
        <span>创建原因 <span class="module-required">*</span></span>
        <UiTextarea :model-value="draft.reason" class="textarea" :disabled="busy" maxlength="500" placeholder="说明为什么要将该已实现定义纳入平台模块目录" @update:model-value="updateDraft('reason', $event)" />
        <small class="module-char-count">{{ draft.reason.length }} / 500</small>
      </label>
    </template>

    <template v-else-if="screen === 'createConfirm' && selectedDefinition">
      <div class="module-command-identity">
        <span class="module-icon"><AppIcon name="database" :size="24" /></span>
        <div><h3>{{ draft.name }}</h3><p><code>{{ draft.moduleCode }}</code> · 新目录记录 · 平台全局</p></div>
      </div>
      <div class="module-notice warning">
        <strong>确认创建边界</strong>
        <p>创建后初始销售状态为停售；技术状态由注册定义决定。后续若需销售，仍需满足运行验证并显式恢复销售。</p>
      </div>
      <dl class="module-facts">
        <div><dt>分类</dt><dd>{{ draft.category }}</dd></div>
        <div><dt>实现状态</dt><dd>{{ selectedDefinition.implementationReady ? '实现已就绪' : '实现未就绪' }}</dd></div>
        <div class="module-full"><dt>销售范围</dt><dd>{{ scopeValues.join('、') || '未配置' }}</dd></div>
        <div class="module-full"><dt>创建原因</dt><dd>{{ draft.reason }}</dd></div>
      </dl>
    </template>

    <template v-else-if="screen === 'createResult'">
      <div class="module-result-heading">
        <span class="module-icon" :class="resultState"><AppIcon :name="resultState === 'confirmed' ? 'checks' : 'shield'" :size="28" /></span>
        <div><h3>{{ resultState === 'confirmed' ? '模块已创建并确认' : resultState === 'denied' ? '未获创建权限' : '创建结果尚未确认' }}</h3><p>{{ created?.name || draft.name }} · {{ created?.moduleCode || draft.moduleCode }}</p></div>
      </div>
      <div class="module-notice" :class="resultState === 'confirmed' ? 'success' : 'warning'" role="status">{{ resultMessage }}</div>
      <dl v-if="created" class="module-facts">
        <div><dt>目录版本</dt><dd>v{{ created.version }}</dd></div>
        <div><dt>技术状态</dt><dd>{{ backendTermLabel('technicalStatus', created.technicalStatus) }}</dd></div>
        <div><dt>销售状态</dt><dd>{{ backendTermLabel('salesStatus', created.salesStatus) }}</dd></div>
        <div><dt>分类</dt><dd>{{ backendTermLabel('moduleCategory', created.category) }}</dd></div>
      </dl>
      <p class="module-note">创建模块目录记录不等于租户获得能力、成员获得权限或模块可立即销售。</p>
    </template>
  </div>
</template>
