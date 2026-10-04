<script setup lang="ts">
import { computed } from 'vue'
import { UiInput, UiOption, UiSelect, UiTextarea } from '@/ui/base'
import AppIcon from '@/ui/common/AppIcon.vue'
import StatusBadge from '@/ui/common/StatusBadge.vue'
import { backendTermLabel } from '@/i18n/backend-terms'
import type { ModuleDTO, ModuleSalesStatus } from '@/services/commercial/platformCommercial'
import { moduleChangeOperation, operationDefinition, type ModuleChangeKind } from '@/services/commercial/moduleAccess'
import type { ModuleDraft, ModuleScreen, ModuleResultState } from '../composables/useModuleManagement'

const vm = defineProps<{
  selected: ModuleDTO; before: ModuleDTO | null; fresh: ModuleDTO | null;
  screen: ModuleScreen; kind: ModuleChangeKind; nextSales: ModuleSalesStatus;
  busy: boolean; resultState: ModuleResultState; resultMessage: string; draft: ModuleDraft;
}>()
const emit = defineEmits<{ draft: [value: ModuleDraft] }>()
const permissionRequirement = computed(() => operationDefinition(moduleChangeOperation[vm.kind])?.permissions.join('、') || '未取得权限定义')
const isStopSale = computed(() => vm.nextSales === 'MODULE_SALES_STATUS_RETIRED')
function updateDraft(key: keyof ModuleDraft, value: unknown) {
  if (typeof value !== 'string' && typeof value !== 'number') return
  const text = String(value)
  if (key === 'technicalStatus' && text !== 'MODULE_TECHNICAL_STATUS_READY'
    && text !== 'MODULE_TECHNICAL_STATUS_NOT_READY' && text !== 'MODULE_TECHNICAL_STATUS_DISABLED') return
  emit('draft', { ...vm.draft, [key]: text })
}
</script>

<template>
  <div class="module-change-panel">
    <template v-if="['metadata', 'sales', 'technical'].includes(vm.screen)">
      <div class="module-command-identity">
        <span class="module-icon"><AppIcon name="database" :size="24" /></span>
        <div>
          <h3>{{ vm.before?.name }}</h3>
          <p><code>{{ vm.before?.moduleCode }}</code> · 版本 v{{ vm.before?.version }} · 平台全局</p>
        </div>
      </div>
      <p class="module-permission">
        本操作要求：<code>{{ permissionRequirement }}</code><br />
        系统校验权限、版本和业务前置条件；页面打开不代表允许提交。
      </p>
      <div v-if="vm.screen === 'metadata'" class="module-form-grid">
        <label class="field">
          <span>模块名称 <span class="module-required">*</span></span>
          <UiInput :model-value="vm.draft.name" class="input" :disabled="vm.busy" maxlength="200" @update:model-value="updateDraft('name', $event)" />
        </label>
        <label class="field">
          <span>分类 <span class="module-required">*</span></span>
          <UiInput :model-value="vm.draft.category" class="input" :disabled="vm.busy" maxlength="200" @update:model-value="updateDraft('category', $event)" />
        </label>
        <label class="field module-full">
          <span>销售范围</span>
          <UiInput :model-value="vm.draft.salesScope" class="input" :disabled="vm.busy" placeholder="多个范围使用逗号分隔" @update:model-value="updateDraft('salesScope', $event)" />
          <small>保留系统定义的范围标识，不在这里创建新的租户类型。</small>
        </label>
      </div>
      <template v-else-if="vm.screen === 'sales'">
        <div class="module-transition">
          <div><span>当前销售状态</span><StatusBadge :text="backendTermLabel('salesStatus', vm.before?.salesStatus ?? '')" /></div>
          <span aria-hidden="true">→</span>
          <div><span>目标销售状态</span><StatusBadge :text="backendTermLabel('salesStatus', vm.nextSales)" :tone="isStopSale ? 'warning' : 'success'" /></div>
        </div>
        <div class="module-notice warning">
          <strong>{{ isStopSale ? '停售说明' : '恢复销售说明' }}</strong>
          <p>本次仅变更模块销售状态，不创建新的目录版本，也不修改技术状态或成员角色。现有订阅及业务运行影响需按实际规则核对，本页不承诺所有租户不受影响。</p>
        </div>
      </template>
      <template v-else>
        <label class="field">
          <span>目标技术状态</span>
          <UiSelect :model-value="vm.draft.technicalStatus" class="select" :disabled="vm.busy" @update:model-value="updateDraft('technicalStatus', $event)">
            <UiOption value="MODULE_TECHNICAL_STATUS_NOT_READY">未就绪</UiOption>
            <UiOption value="MODULE_TECHNICAL_STATUS_READY">技术就绪</UiOption>
            <UiOption value="MODULE_TECHNICAL_STATUS_DISABLED">技术停用</UiOption>
          </UiSelect>
        </label>
        <div class="module-notice" :class="{ warning: vm.draft.technicalStatus === 'MODULE_TECHNICAL_STATUS_DISABLED' }">
          <strong>{{ vm.draft.technicalStatus === 'MODULE_TECHNICAL_STATUS_DISABLED' ? '确认全局技术停用影响' : '技术状态变更会自动停售' }}</strong>
          <p>当前：{{ backendTermLabel('technicalStatus', vm.before?.technicalStatus ?? '') }}。该操作可能影响使用此模块的业务；受影响对象尚未由接口提供，请核对后提交。技术状态变更会生成新目录版本并自动停售。技术就绪不等于运行验证通过，也不会自动恢复销售。</p>
        </div>
      </template>
      <label class="field">
        <span>变更原因 <span class="module-required">*</span></span>
        <UiTextarea :model-value="vm.draft.reason" class="textarea" :disabled="vm.busy" maxlength="500" placeholder="说明本次调整的目的、影响与核对情况" @update:model-value="updateDraft('reason', $event)" />
        <small class="module-char-count">{{ vm.draft.reason.length }} / 500</small>
      </label>
    </template>
    <template v-else-if="vm.screen === 'result'">
      <div class="module-result-heading">
        <span class="module-icon" :class="vm.resultState"><AppIcon :name="vm.resultState === 'confirmed' ? 'checks' : 'shield'" :size="28" /></span>
        <div>
          <h3>{{ vm.resultState === 'confirmed' ? '变更已确认生效' : vm.resultState === 'denied' ? '未获操作授权' : '结果尚未确认' }}</h3>
          <p>{{ vm.selected.name }} · {{ vm.selected.moduleCode }}</p>
        </div>
      </div>
      <div class="module-notice" :class="vm.resultState === 'confirmed' ? 'success' : 'warning'" role="status">{{ vm.resultMessage }}</div>
      <dl class="module-facts">
        <div><dt>提交前版本</dt><dd>v{{ vm.before?.version ?? '未知' }}</dd></div>
        <div><dt>最近确认版本</dt><dd>{{ vm.fresh ? `v${vm.fresh.version}` : '尚未确认' }}</dd></div>
        <div><dt>技术状态</dt><dd>{{ vm.fresh ? backendTermLabel('technicalStatus', vm.fresh.technicalStatus) : '尚未确认' }}</dd></div>
        <div><dt>销售状态</dt><dd>{{ vm.fresh ? backendTermLabel('salesStatus', vm.fresh.salesStatus) : '尚未确认' }}</dd></div>
        <div class="module-full"><dt>原变更原因</dt><dd>{{ vm.draft.reason }}</dd></div>
      </dl>
      <p class="module-note">模块变更不等于租户已获得能力或成员已获权限。原状态、回执与最新状态分别保留；本页不提供未经验证的撤销操作。</p>
    </template>
    <template v-else-if="vm.screen === 'conflict'">
      <div class="module-notice warning">
        <strong>其他操作已改变模块版本，尚未重新提交</strong>
        <p>原草稿保留。核对最新状态后，明确选择以新版本继续；不会静默覆盖。</p>
      </div>
      <div class="module-table-scroll" tabindex="0" role="region" aria-label="模块版本对比">
        <table class="module-definition-table">
          <thead><tr><th>字段</th><th>原版本</th><th>最新状态</th></tr></thead>
          <tbody>
            <tr><td>版本</td><td>v{{ vm.before?.version }}</td><td>{{ vm.fresh ? `v${vm.fresh.version}` : '未读取' }}</td></tr>
            <tr><td>模块名称</td><td>{{ vm.before?.name }}</td><td>{{ vm.fresh?.name || '未读取' }}</td></tr>
            <tr><td>分类</td><td>{{ vm.before?.category }}</td><td>{{ vm.fresh?.category || '未读取' }}</td></tr>
            <tr><td>销售范围</td><td>{{ vm.before?.salesScope?.join('、') || '未配置' }}</td><td>{{ vm.fresh ? vm.fresh.salesScope?.join('、') || '未配置' : '未读取' }}</td></tr>
            <tr><td>技术状态</td><td>{{ backendTermLabel('technicalStatus', vm.before?.technicalStatus ?? '') }}</td><td>{{ vm.fresh ? backendTermLabel('technicalStatus', vm.fresh.technicalStatus) : '未读取' }}</td></tr>
            <tr><td>销售状态</td><td>{{ backendTermLabel('salesStatus', vm.before?.salesStatus ?? '') }}</td><td>{{ vm.fresh ? backendTermLabel('salesStatus', vm.fresh.salesStatus) : '未读取' }}</td></tr>
          </tbody>
        </table>
      </div>
      <section class="module-route-detail">
        <h4>保留的目标草稿</h4>
        <p v-if="vm.kind === 'metadata'">{{ vm.draft.name }} · {{ vm.draft.category }} · {{ vm.draft.salesScope || '未配置销售范围' }}</p>
        <p v-else>{{ vm.kind === 'sales' ? backendTermLabel('salesStatus', vm.nextSales) : backendTermLabel('technicalStatus', vm.draft.technicalStatus) }}</p>
        <p>原因：{{ vm.draft.reason }}</p>
      </section>
    </template>
    <div v-else class="module-notice warning">
      <strong>尚未提交的输入将被放弃</strong>
      <p>退出编辑不会撤销已执行的其他操作。继续编辑可保留当前输入。</p>
    </div>
  </div>
</template>
