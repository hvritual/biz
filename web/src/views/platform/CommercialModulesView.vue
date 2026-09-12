<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import AppIcon from '@/components/ui/AppIcon.vue'
import PageHeading from '@/components/ui/PageHeading.vue'
import StatusBadge from '@/components/ui/StatusBadge.vue'
import {
  CommercialApiError,
  listPlatformModules,
  type ModuleDTO,
  type ModuleSalesStatus,
  type ModuleTechnicalStatus,
} from '@/services/commercial/platformCommercial'

type LoadState = 'loading' | 'ready' | 'empty' | 'blocked' | 'error'

const modules = ref<ModuleDTO[]>([])
const loadState = ref<LoadState>('loading')
const errorMessage = ref('')

const summary = computed(() => ({
  total: modules.value.length,
  ready: modules.value.filter((item) => item.technicalStatus === 'MODULE_TECHNICAL_STATUS_READY').length,
  sellable: modules.value.filter((item) => item.salesStatus === 'MODULE_SALES_STATUS_SELLABLE').length,
}))

function technicalLabel(status: ModuleTechnicalStatus) {
  return {
    MODULE_TECHNICAL_STATUS_READY: '技术就绪',
    MODULE_TECHNICAL_STATUS_NOT_READY: '未就绪',
    MODULE_TECHNICAL_STATUS_DISABLED: '技术停用',
    MODULE_TECHNICAL_STATUS_UNSPECIFIED: '未声明',
  }[status]
}

function salesLabel(status: ModuleSalesStatus) {
  return {
    MODULE_SALES_STATUS_SELLABLE: '可销售',
    MODULE_SALES_STATUS_RETIRED: '已停售',
    MODULE_SALES_STATUS_UNSPECIFIED: '未声明',
  }[status]
}

async function loadModules() {
  loadState.value = 'loading'
  errorMessage.value = ''
  try {
    modules.value = await listPlatformModules()
    loadState.value = modules.value.length ? 'ready' : 'empty'
  } catch (error) {
    if (error instanceof CommercialApiError && ['unauthenticated', 'forbidden'].includes(error.code)) {
      loadState.value = 'blocked'
      errorMessage.value = error.message
      return
    }
    loadState.value = 'error'
    errorMessage.value = error instanceof Error ? error.message : '无法读取模块目录'
  }
}

onMounted(loadModules)
</script>

<template>
  <div class="page-stack" data-testid="ce13-module-catalog">
    <PageHeading
      title="平台商业管理"
      description="管理模块目录、套餐版本与租户权益；页面只读取服务端商业事实，不使用前端示例权益"
    />

    <section class="commercial-tabs" aria-label="平台商业管理导航">
      <RouterLink class="commercial-tab active" to="/platform/commercial/modules">模块目录</RouterLink>
      <RouterLink class="commercial-tab" to="/platform/commercial/plans">套餐版本</RouterLink>
      <span class="commercial-tab disabled" aria-disabled="true">租户权益 · 后续切片</span>
    </section>

    <section v-if="loadState === 'loading'" class="card state-card" aria-live="polite">
      <span class="state-icon"><AppIcon name="refresh" :size="20" /></span>
      <div><strong>正在读取真实模块目录</strong><p>请求 /v1/platform/modules，不加载本地 seed。</p></div>
    </section>

    <section v-else-if="loadState === 'blocked'" class="card state-card warning" role="alert">
      <span class="state-icon"><AppIcon name="shield" :size="20" /></span>
      <div class="flex-1">
        <strong>当前会话无平台商业访问权限</strong>
        <p>{{ errorMessage || '请使用已授权的平台 Web Session；浏览器不会降级使用平台 API Key。' }}</p>
        <p class="boundary-note">平台商业 API 已支持可信 web-session；401/403 表示当前身份未认证或缺少对应平台权限。</p>
      </div>
      <button class="btn" type="button" @click="loadModules">重新检查</button>
    </section>

    <section v-else-if="loadState === 'error'" class="card state-card danger" role="alert">
      <span class="state-icon"><AppIcon name="help" :size="20" /></span>
      <div class="flex-1"><strong>模块目录读取失败</strong><p>{{ errorMessage }}</p></div>
      <button class="btn" type="button" @click="loadModules">重试</button>
    </section>

    <template v-else>
      <section class="metric-grid" aria-label="模块目录摘要">
        <article class="card metric"><span>模块总数</span><strong>{{ summary.total }}</strong><small>来自服务端目录</small></article>
        <article class="card metric"><span>技术就绪</span><strong>{{ summary.ready }}</strong><small>READY</small></article>
        <article class="card metric"><span>可销售</span><strong>{{ summary.sellable }}</strong><small>SELLABLE</small></article>
      </section>

      <section v-if="loadState === 'empty'" class="card state-card">
        <span class="state-icon"><AppIcon name="database" :size="20" /></span>
        <div><strong>模块目录为空</strong><p>服务端返回成功，但当前没有平台模块记录。</p></div>
      </section>

      <section v-else class="card catalog-card">
        <div class="catalog-header">
          <div><h2>模块目录</h2><p>技术状态与销售状态独立展示，避免把“可运行”误判为“可售卖”。</p></div>
          <button class="btn" type="button" @click="loadModules"><AppIcon name="refresh" :size="15" />刷新</button>
        </div>
        <div class="table-scroll">
          <table class="data-table">
            <thead>
              <tr>
                <th>模块</th><th>分类</th><th>技术状态</th><th>销售状态</th><th>能力</th><th>依赖</th><th>版本</th>
              </tr>
            </thead>
            <tbody>
              <tr v-for="item in modules" :key="item.moduleCode">
                <td><strong>{{ item.name || item.moduleCode }}</strong><small class="module-code">{{ item.moduleCode }}</small></td>
                <td>{{ item.category || '—' }}</td>
                <td><StatusBadge :text="technicalLabel(item.technicalStatus)" :tone="item.technicalStatus === 'MODULE_TECHNICAL_STATUS_READY' ? 'success' : 'warning'" /></td>
                <td><StatusBadge :text="salesLabel(item.salesStatus)" :tone="item.salesStatus === 'MODULE_SALES_STATUS_SELLABLE' ? 'success' : 'neutral'" /></td>
                <td><span class="count-cell">{{ item.capabilityCodes?.length ?? 0 }}</span></td>
                <td>{{ item.dependencies?.length ? item.dependencies.join('、') : '无' }}</td>
                <td class="numeric">{{ item.version }}</td>
              </tr>
            </tbody>
          </table>
        </div>
      </section>
    </template>
  </div>
</template>

<style scoped>
.commercial-tabs {
  display: flex;
  gap: 4px;
  border-bottom: 1px solid var(--color-border);
}
.commercial-tab {
  padding: 10px 14px;
  font-size: 13px;
  color: var(--color-text-secondary);
  text-decoration: none;
  border-bottom: 2px solid transparent;
}
.commercial-tab.active {
  color: var(--color-primary);
  border-bottom-color: var(--color-primary);
  font-weight: 600;
}
.commercial-tab.disabled { color: var(--color-text-muted); cursor: not-allowed; }
.state-card {
  min-height: 112px;
  display: flex;
  align-items: center;
  gap: 14px;
  padding: 22px;
}
.state-card p { margin: 5px 0 0; color: var(--color-text-secondary); font-size: 13px; }
.state-card.warning { border-color: var(--color-warning, #d9a441); }
.state-card.danger { border-color: var(--color-danger, #d14343); }
.state-icon {
  width: 38px;
  height: 38px;
  display: grid;
  place-items: center;
  border-radius: 10px;
  background: var(--color-primary-soft);
  color: var(--color-primary);
  flex: 0 0 auto;
}
.boundary-note { font-size: 11px !important; color: var(--color-text-muted) !important; }
.metric-grid { display: grid; grid-template-columns: repeat(3, minmax(0, 1fr)); gap: 12px; }
.metric { padding: 18px; }
.metric span, .metric small { display: block; color: var(--color-text-muted); font-size: 12px; }
.metric strong { display: block; margin: 7px 0 3px; font-size: 25px; }
.catalog-card { overflow: hidden; }
.catalog-header { display: flex; align-items: center; justify-content: space-between; gap: 16px; padding: 20px; border-bottom: 1px solid var(--color-border); }
.catalog-header h2 { margin: 0; font-size: 16px; }
.catalog-header p { margin: 5px 0 0; color: var(--color-text-muted); font-size: 12px; }
.module-code { display: block; margin-top: 4px; color: var(--color-text-muted); font-family: ui-monospace, SFMono-Regular, Menlo, monospace; }
.count-cell { display: inline-flex; min-width: 28px; justify-content: center; }
.flex-1 { flex: 1; }
@media (max-width: 760px) {
  .metric-grid { grid-template-columns: 1fr; }
  .commercial-tabs { overflow-x: auto; }
  .commercial-tab { white-space: nowrap; }
  .state-card, .catalog-header { align-items: flex-start; flex-wrap: wrap; }
}
</style>
