<script setup lang="ts">
import { computed, nextTick, reactive, ref, watch } from 'vue'
import { UiButton, UiInput, UiOption, UiSelect } from '@/ui/base'
import AppIcon from '@/ui/common/AppIcon.vue'
import PageHeading from '@/ui/common/PageHeading.vue'
import MetricCard from '@/ui/common/MetricCard.vue'
import StatusBadge from '@/ui/common/StatusBadge.vue'
import UiDialog from '@/ui/common/UiDialog.vue'
import { backendErrorFallback, backendTermLabel } from '@/i18n/backend-terms'
import ModuleDefinitionPanel from '../components/ModuleDefinitionPanel.vue'
import ModuleCreatePanel from '../components/ModuleCreatePanel.vue'
import ModuleChangePanel from '../components/ModuleChangePanel.vue'
import { useModuleManagement } from '../composables/useModuleManagement'
import { currentAuthorizationAllows } from '@/services/runtime/authorization'
import { moduleChangeOperation } from '@/services/commercial/moduleAccess'
import '../styles/moduleManagement.css'

const vm = reactive(useModuleManagement())
const journeyPanel = ref<HTMLElement>()
const loadError = computed(() => vm.loadState === 'blocked' ? vm.errorMessage : backendErrorFallback('commercial'))
watch(() => vm.screen, async () => {
  await nextTick()
  journeyPanel.value?.focus()
})
const isStopSale = computed(() => vm.nextSales === 'MODULE_SALES_STATUS_RETIRED')
const canCreate = computed(() => currentAuthorizationAllows(moduleChangeOperation.create))
const canCurrentChange = computed(() => currentAuthorizationAllows(moduleChangeOperation[vm.kind]))
const heading = computed(() => ({
  detail: `模块详情 · ${vm.selected?.name || vm.selected?.moduleCode || ''}`,
  create: '新增模块', createConfirm: '确认新增模块', createResult: '模块创建结果',
  metadata: '修改基础配置', sales: isStopSale.value ? '确认模块停售' : '确认恢复模块销售',
  technical: '调整技术状态', result: '模块变更结果', conflict: '核对模块版本冲突', discard: '放弃本次编辑？',
}[vm.screen]))
const submitLabel = computed(() => vm.kind === 'metadata' ? '保存基础配置'
  : vm.kind === 'sales' ? (isStopSale.value ? '确认停售' : '确认恢复销售')
    : vm.draft.technicalStatus === 'MODULE_TECHNICAL_STATUS_DISABLED' ? '确认技术停用' : '确认应用技术状态')
</script>

<template>
  <div class="page-stack module-management" data-testid="ce13-module-catalog" data-ui-template="ListPage">
    <div data-ui-region="page-heading">
      <PageHeading title="模块目录" description="统一管理模块基础配置、技术与销售状态，查看能力、权限及页面关联" />
    </div>
    <section v-if="vm.loadState === 'loading'" class="card module-state" aria-live="polite">
      <AppIcon name="refresh" :size="24" />
      <div>
        <strong>正在读取模块目录</strong>
        <p>正在获取当前账号可管理的模块信息。</p>
      </div>
    </section>
    <section v-else-if="vm.loadState === 'blocked' || vm.loadState === 'error'" class="card module-state" role="alert">
      <AppIcon name="shield" :size="24" />
      <div>
        <h2>{{ vm.loadState === 'blocked' ? '当前账号无平台商业管理权限' : '模块目录读取失败' }}</h2>
        <p>{{ loadError }}</p>
      </div>
      <UiButton class="btn" @click="vm.loadModules">{{ vm.loadState === 'blocked' ? '重新检查' : '重试' }}</UiButton>
    </section>
    <template v-else>
      <section class="module-metrics" aria-label="模块目录摘要" data-ui-region="metrics">
        <MetricCard label="模块总数" :value="vm.summary.total" icon="database" caption="当前平台模块记录" />
        <MetricCard label="可销售" :value="vm.summary.sellable" icon="crown" caption="当前目录中的销售状态" />
        <MetricCard label="技术就绪" :value="vm.summary.ready" icon="checks" caption="不等于运行验证已通过" />
        <MetricCard label="未就绪或停用" :value="vm.summary.notReady" icon="layers" caption="技术状态需单独关注" />
      </section>
      <section class="card module-query" data-ui-region="query" aria-label="模块目录查询">
        <div class="module-query-grid">
          <label class="field">
            <span>关键词</span>
            <UiInput v-model="vm.keyword" class="input" placeholder="模块名称 / 代码 / 分类" />
          </label>
          <label class="field">
            <span>技术状态</span>
            <UiSelect v-model="vm.technicalFilter" class="select">
              <UiOption value="">全部技术状态</UiOption>
              <UiOption value="MODULE_TECHNICAL_STATUS_READY">技术就绪</UiOption>
              <UiOption value="MODULE_TECHNICAL_STATUS_NOT_READY">未就绪</UiOption>
              <UiOption value="MODULE_TECHNICAL_STATUS_DISABLED">技术停用</UiOption>
            </UiSelect>
          </label>
          <label class="field">
            <span>销售状态</span>
            <UiSelect v-model="vm.salesFilter" class="select">
              <UiOption value="">全部销售状态</UiOption>
              <UiOption value="MODULE_SALES_STATUS_SELLABLE">可销售</UiOption>
              <UiOption value="MODULE_SALES_STATUS_RETIRED">已停售</UiOption>
            </UiSelect>
          </label>
          <UiButton class="btn module-query-reset" @click="vm.resetFilters">重置</UiButton>
        </div>
        <div class="module-query-summary" aria-live="polite">
          <span>当前显示 {{ vm.filteredModules.length }} / {{ vm.modules.length }} 个模块；筛选只影响当前列表。</span>
          <span>上次读取：{{ vm.readAt }}</span>
        </div>
        <div v-if="vm.keyword || vm.technicalFilter || vm.salesFilter" class="module-chips" aria-label="已选筛选条件">
          <UiButton v-if="vm.keyword" class="btn module-filter-chip" :aria-label="`清除关键词 ${vm.keyword}`" @click="vm.keyword = ''">
            关键词：{{ vm.keyword }}<AppIcon name="close" :size="14" />
          </UiButton>
          <UiButton v-if="vm.technicalFilter" class="btn module-filter-chip" aria-label="清除技术状态筛选" @click="vm.technicalFilter = ''">
            {{ backendTermLabel('technicalStatus', vm.technicalFilter) }}<AppIcon name="close" :size="14" />
          </UiButton>
          <UiButton v-if="vm.salesFilter" class="btn module-filter-chip" aria-label="清除销售状态筛选" @click="vm.salesFilter = ''">
            {{ backendTermLabel('salesStatus', vm.salesFilter) }}<AppIcon name="close" :size="14" />
          </UiButton>
        </div>
      </section>
      <section class="card module-catalog" data-ui-region="data">
        <div class="module-section-header">
          <div>
            <h2>模块目录</h2>
            <p>技术与销售状态分别操作；配置或技术变更会按目录规则自动停售。</p>
          </div>
          <div class="module-actions">
            <UiButton class="btn btn-primary" :disabled="!canCreate" :title="canCreate ? undefined : '当前账号没有新增模块权限'" @click="vm.openCreate"><AppIcon name="plus" :size="16" />新增模块</UiButton>
            <UiButton class="btn" @click="vm.loadModules"><AppIcon name="refresh" :size="16" />刷新</UiButton>
          </div>
        </div>
        <div class="module-table-scroll" tabindex="0" role="region" aria-label="模块列表，可横向滚动">
          <table class="data-table module-catalog-table">
            <thead>
              <tr>
                <th scope="col">模块</th><th scope="col">分类</th>
                <th scope="col">技术状态</th><th scope="col">销售状态</th>
                <th scope="col">能力治理</th><th scope="col">操作</th>
              </tr>
            </thead>
            <tbody>
              <tr v-if="!vm.filteredModules.length">
                <td colspan="6" class="module-empty">{{ vm.loadState === 'empty' ? '模块目录为空，当前没有平台模块记录。' : '没有符合条件的模块，请调整或清除筛选。' }}</td>
              </tr>
              <tr v-for="item in vm.filteredModules" :key="item.moduleCode">
                <td>
                  <strong>{{ item.name || backendTermLabel('module', item.moduleCode) }}</strong>
                  <small>版本 v{{ item.version }}</small>
                </td>
                <td>{{ backendTermLabel('moduleCategory', item.category) }}</td>
                <td><StatusBadge :text="backendTermLabel('technicalStatus', item.technicalStatus)" :tone="item.technicalStatus === 'MODULE_TECHNICAL_STATUS_READY' ? 'success' : 'warning'" /></td>
                <td><StatusBadge :text="backendTermLabel('salesStatus', item.salesStatus)" :tone="item.salesStatus === 'MODULE_SALES_STATUS_SELLABLE' ? 'success' : 'neutral'" /></td>
                <td>
                  <strong>{{ item.capabilityCodes?.length ?? 0 }} 项能力</strong>
                  <small>{{ item.dependencies?.length ?? 0 }} 项依赖</small>
                </td>
                <td><UiButton class="btn" @click="vm.openDetail(item)">查看详情</UiButton></td>
              </tr>
            </tbody>
          </table>
        </div>
      </section>
    </template>
    <UiDialog :open="vm.dialogOpen" :title="heading" :width="vm.screen === 'detail' || vm.screen === 'conflict' ? '1080px' : vm.screen.startsWith('create') ? '760px' : '680px'" @close="vm.requestClose">
      <div v-if="vm.screen.startsWith('create') || vm.selected" ref="journeyPanel" class="module-journey" tabindex="-1" :aria-label="heading" :aria-busy="vm.busy" data-testid="module-journey">
        <ModuleCreatePanel
          v-if="vm.screen.startsWith('create')" :definitions="vm.creatableDefinitions"
          :selected-definition="vm.selectedCreateDefinition" :missing-dependencies="vm.missingCreateDependencies"
          :draft="vm.createDraft" :screen="vm.screen" :busy="vm.busy" :result-state="vm.resultState"
          :result-message="vm.resultMessage" :created="vm.created" :definition-state="vm.definitionState"
          :definition-error="vm.definitionError" @draft="Object.assign(vm.createDraft, $event)" />
        <ModuleDefinitionPanel
          v-else-if="vm.screen === 'detail' && vm.selected" :module="vm.selected" :busy="vm.busy"
          :write-unresolved="vm.writeUnresolved" @change="vm.startChange" />
        <ModuleChangePanel
          v-else-if="vm.selected" :selected="vm.selected" :before="vm.before" :fresh="vm.fresh"
          :screen="vm.screen" :kind="vm.kind" :next-sales="vm.nextSales" :busy="vm.busy"
          :result-state="vm.resultState" :result-message="vm.resultMessage" :draft="vm.draft"
          @draft="Object.assign(vm.draft, $event)" />
        <div v-if="vm.actionError" class="module-notice danger" role="alert">{{ vm.actionError }}</div>
        <p v-if="vm.busy" class="module-note" role="status">{{ vm.readPending ? '正在读取系统最新状态…' : '正在提交，请勿重复操作…' }}</p>
      </div>
      <template #footer>
        <div class="module-dialog-footer">
          <template v-if="vm.screen === 'create'">
            <UiButton class="btn" :disabled="vm.busy" @click="vm.requestClose">取消</UiButton>
            <UiButton class="btn btn-primary" :disabled="vm.busy || vm.definitionState === 'loading'" @click="vm.continueCreate">下一步</UiButton>
          </template>
          <template v-else-if="vm.screen === 'createConfirm'">
            <UiButton class="btn" :disabled="vm.busy" @click="vm.screen = 'create'">返回修改</UiButton>
            <UiButton class="btn btn-primary" :disabled="vm.busy || vm.createUnresolved || !canCreate" @click="vm.submitCreate">确认创建</UiButton>
          </template>
          <template v-else-if="vm.screen === 'createResult'">
            <UiButton class="btn" :disabled="vm.busy" @click="vm.requestClose">关闭</UiButton>
            <UiButton v-if="vm.resultState !== 'confirmed'" class="btn" :disabled="vm.busy" @click="vm.refreshCreatedModule">重新读取最新状态</UiButton>
            <UiButton v-if="vm.created && vm.resultState === 'confirmed'" class="btn btn-primary" :disabled="vm.busy" @click="vm.backToDetail">查看模块详情</UiButton>
          </template>
          <template v-else-if="vm.screen === 'detail'">
            <UiButton class="btn" :disabled="vm.busy" @click="vm.requestClose">关闭</UiButton>
            <UiButton class="btn" :disabled="vm.busy" @click="vm.refreshModule">重新读取模块</UiButton>
          </template>
          <template v-else-if="vm.screen === 'discard'">
            <UiButton class="btn" @click="vm.discard">放弃编辑</UiButton>
            <UiButton class="btn btn-primary" @click="vm.continueEditing">继续编辑</UiButton>
          </template>
          <template v-else-if="vm.screen === 'conflict'">
            <UiButton class="btn" :disabled="vm.busy" @click="vm.backToDetail">返回详情</UiButton>
            <UiButton class="btn" :disabled="vm.busy" @click="vm.refreshModule">重新读取最新状态</UiButton>
            <UiButton class="btn btn-primary" :disabled="vm.busy || !vm.fresh" @click="vm.acceptFreshVersion">使用最新版本继续编辑</UiButton>
          </template>
          <template v-else-if="vm.screen === 'result'">
            <UiButton class="btn" :disabled="vm.busy" @click="vm.requestClose">关闭</UiButton>
            <UiButton v-if="vm.resultState !== 'confirmed'" class="btn" :disabled="vm.busy" @click="vm.refreshModule">重新读取最新状态</UiButton>
            <UiButton class="btn btn-primary" :disabled="vm.busy" @click="vm.backToDetail">返回模块详情</UiButton>
          </template>
          <template v-else>
            <UiButton class="btn" :disabled="vm.busy" @click="vm.requestClose">取消</UiButton>
            <UiButton :class="['btn', (vm.screen === 'sales' && isStopSale) || (vm.screen === 'technical' && vm.draft.technicalStatus === 'MODULE_TECHNICAL_STATUS_DISABLED') ? 'module-destructive' : 'btn-primary']" :disabled="vm.busy || !canCurrentChange" @click="vm.submit">{{ submitLabel }}</UiButton>
          </template>
        </div>
      </template>
    </UiDialog>
  </div>
</template>
