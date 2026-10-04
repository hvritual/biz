<script setup lang="ts">
import { computed, ref } from 'vue'
import { useRouter } from 'vue-router'
import { UiButton } from '@/ui/base'
import AppIcon from '@/ui/common/AppIcon.vue'
import StatusBadge from '@/ui/common/StatusBadge.vue'
import { backendTermLabel } from '@/i18n/backend-terms'
import type { ModuleDTO } from '@/services/commercial/platformCommercial'
import type { ExistingModuleChangeKind } from '../composables/useModuleManagement'
import { moduleAccessDefinitions, moduleChangeOperation, moduleMappingVersion, operationDefinition } from '@/services/commercial/moduleAccess'

const props = defineProps<{ module: ModuleDTO; busy: boolean; writeUnresolved: boolean }>()
const emit = defineEmits<{ change: [kind: ExistingModuleChangeKind] }>()
const router = useRouter()
const tab = ref('overview')
const routeSelection = ref('')
const tabs = [
  { id: 'overview', label: '概览' }, { id: 'permissions', label: '能力与权限' },
  { id: 'pages', label: '关联页面' }, { id: 'verification', label: '可用性核验' },
]
const definitions = computed(() => moduleAccessDefinitions(props.module, router.getRoutes()))
const selectedPage = computed(() => definitions.value.pages.find((page) => page.path === routeSelection.value))
const managementDefinitions = computed(() => ['commercial.module.get', ...Object.values(moduleChangeOperation)]
  .map(operationDefinition).filter((item) => item !== undefined))
function changeTab(id: string) { tab.value = id; routeSelection.value = '' }
function navigateTabs(event: KeyboardEvent) {
  const index = tabs.findIndex((item) => item.id === tab.value)
  const next = event.key === 'Home' ? 0 : event.key === 'End' ? tabs.length - 1
    : event.key === 'ArrowRight' ? (index + 1) % tabs.length : event.key === 'ArrowLeft' ? (index + tabs.length - 1) % tabs.length : -1
  if (next < 0) return
  event.preventDefault()
  changeTab(tabs[next]!.id)
  const buttons = (event.currentTarget as HTMLElement).querySelectorAll<HTMLElement>('[role="tab"]')
  buttons[next]?.focus()
}
</script>

<template>
  <div class="module-definition-panel">
    <div class="module-identity">
      <div class="module-identity-name">
        <span class="module-icon"><AppIcon name="database" :size="28" /></span>
        <div>
          <h3>{{ module.name }}</h3>
          <code>{{ module.moduleCode }}</code>
        </div>
      </div>
      <div>
        <span>当前版本</span>
        <strong class="numeric">v{{ module.version }}</strong>
      </div>
      <div>
        <span>技术状态</span>
        <StatusBadge :text="backendTermLabel('technicalStatus', module.technicalStatus)" :tone="module.technicalStatus === 'MODULE_TECHNICAL_STATUS_READY' ? 'success' : 'warning'" />
      </div>
      <div>
        <span>销售状态</span>
        <StatusBadge :text="backendTermLabel('salesStatus', module.salesStatus)" :tone="module.salesStatus === 'MODULE_SALES_STATUS_SELLABLE' ? 'success' : 'neutral'" />
      </div>
    </div>
    <nav class="module-tabs" role="tablist" aria-label="模块详情分类" @keydown="navigateTabs">
      <UiButton v-for="item in tabs" :id="`module-tab-${item.id}`" :key="item.id" role="tab" :aria-selected="tab === item.id" :aria-controls="`module-panel-${item.id}`" :tabindex="tab === item.id ? 0 : -1" :class="['module-tab', { active: tab === item.id }]" @click="changeTab(item.id)">{{ item.label }}</UiButton>
    </nav>
    <section :id="`module-panel-${tab}`" role="tabpanel" :aria-labelledby="`module-tab-${tab}`" tabindex="0" class="module-tab-content">
      <template v-if="tab === 'overview'">
        <div v-if="writeUnresolved" class="module-notice warning" role="status">该模块有未确认的提交结果，已暂停继续变更。请先重新读取模块并核对原操作。</div>
        <div class="module-section-header">
          <div>
            <h3>基础配置</h3>
            <p>平台全局模块，不属于某个租户。只修改名称、分类和销售范围。</p>
          </div>
          <UiButton class="btn" :disabled="busy || writeUnresolved" @click="emit('change', 'metadata')">编辑基础配置</UiButton>
        </div>
        <dl class="module-facts">
          <div><dt>模块名称</dt><dd>{{ module.name }}</dd></div>
          <div><dt>分类</dt><dd>{{ backendTermLabel('moduleCategory', module.category) }}</dd></div>
          <div class="module-full"><dt>销售范围</dt><dd>{{ module.salesScope?.join('、') || '未配置' }}</dd></div>
        </dl>
        <div class="module-governance">
          <section>
            <h4>功能能力</h4>
            <p v-if="!module.capabilityCodes?.length">未配置</p>
            <div class="module-chips">
              <span v-for="code in module.capabilityCodes" :key="code" class="module-tag">{{ backendTermLabel('entitlementKey', code) }}</span>
            </div>
          </section>
          <section>
            <h4>模块依赖</h4>
            <p v-if="!module.dependencies?.length">无</p>
            <div class="module-chips">
              <span v-for="code in module.dependencies" :key="code" class="module-tag">{{ backendTermLabel('module', code) }}</span>
            </div>
          </section>
          <section>
            <h4>额度定义</h4>
            <p v-if="!module.quotaSchemaKeys?.length">未配置</p>
            <div class="module-chips">
              <span v-for="code in module.quotaSchemaKeys" :key="code" class="module-tag">{{ backendTermLabel('entitlementKey', code) }}</span>
            </div>
          </section>
          <section>
            <h4>字段规则定义</h4>
            <p v-if="!module.fieldPolicySchemaKeys?.length">未配置</p>
            <div class="module-chips">
              <span v-for="code in module.fieldPolicySchemaKeys" :key="code" class="module-tag">{{ backendTermLabel('entitlementKey', code) }}</span>
            </div>
          </section>
        </div>
        <div class="module-section-header">
          <div>
            <h3>技术与销售状态</h3>
            <p>状态变更分别确认。基础配置或技术状态变化会形成新目录版本并自动停售；销售状态变更不创建新目录版本。</p>
          </div>
        </div>
        <div class="module-actions">
          <UiButton class="btn" :disabled="busy || writeUnresolved" @click="emit('change', 'technical')">调整技术状态</UiButton>
          <UiButton class="btn" :disabled="busy || writeUnresolved" @click="emit('change', 'sales')">{{ module.salesStatus === 'MODULE_SALES_STATUS_SELLABLE' ? '停售销售' : '恢复销售' }}</UiButton>
        </div>
        <p class="module-note">额度定义不是租户剩余额度，字段规则不是当前成员权限。当前目录接口不提供历史审计或权限预览。</p>
      </template>
      <template v-else-if="tab === 'permissions'">
        <div class="module-section-header">
          <div>
            <h3>平台管理权限要求</h3>
            <p>下表来自操作契约，不表示当前账号已获授权。实际请求由系统逐项检查。</p>
          </div>
        </div>
        <div class="module-table-scroll" tabindex="0" role="region" aria-label="平台管理权限定义">
          <table class="module-definition-table">
            <thead><tr><th>操作</th><th>所需权限</th></tr></thead>
            <tbody>
              <tr v-for="operation in managementDefinitions" :key="operation.operationId">
                <td><code>{{ operation.operationId }}</code></td>
                <td><code v-for="permission in operation.permissions" :key="permission">{{ permission }}</code></td>
              </tr>
            </tbody>
          </table>
        </div>
        <div class="module-section-header">
          <div>
            <h3>能力与业务操作</h3>
            <p>商业映射版本 {{ moduleMappingVersion }}；仅展示当前构建中的公开业务操作定义，不授予租户能力。</p>
          </div>
        </div>
        <div v-if="definitions.catalogMismatch || definitions.unmappedCapabilities.length" class="module-notice warning" role="status">当前模块能力与此版本前端契约并非全部匹配。缺失映射不表示没有权限；请先核对部署版本。</div>
        <div v-if="definitions.operations.length" class="module-table-scroll" tabindex="0" role="region" aria-label="能力权限关联">
          <table class="module-definition-table">
            <thead><tr><th>能力标识</th><th>业务操作</th><th>IAM 权限与组合</th></tr></thead>
            <tbody>
              <tr v-for="operation in definitions.operations" :key="operation.operationId">
                <td><code v-for="capability in operation.capabilityCodes" :key="capability">{{ capability }}</code></td>
                <td><code>{{ operation.operationId }}</code></td>
                <td>
                  <code v-for="permission in operation.permissions" :key="permission">{{ permission }}</code>
                  <small>{{ operation.permissionMode === 'any' ? '任一满足' : '全部满足' }}</small>
                </td>
              </tr>
            </tbody>
          </table>
        </div>
        <p v-else class="module-empty">当前构建未找到该模块的公开业务操作映射，不能据此推断权限或能力已就绪。</p>
      </template>
      <template v-else-if="tab === 'pages'">
        <div class="module-section-header">
          <div>
            <h3>关联前端页面</h3>
            <p>通过真实 Route 的入口 Operation 关联，导航分组不等于商业模块。此视图只读。</p>
          </div>
        </div>
        <div class="module-notice">关联范围：页面入口声明。页面内按钮、组合调用、数据范围与对象状态仍需独立核验；不提供手工绑定入口。</div>
        <div v-if="definitions.pages.length" class="module-table-scroll" tabindex="0" role="region" aria-label="模块关联页面">
          <table class="module-definition-table">
            <thead><tr><th>页面</th><th>路由</th><th>关联入口操作</th><th>操作</th></tr></thead>
            <tbody>
              <tr v-for="page in definitions.pages" :key="page.path">
                <td><strong>{{ page.title }}</strong><small>{{ page.surface }} · {{ page.navigationModule }}</small></td>
                <td><code>{{ page.path }}</code></td>
                <td><code v-for="operation in page.matchedOperations" :key="operation">{{ operation }}</code></td>
                <td><UiButton class="btn" :aria-label="`查看${page.title}关联定义`" @click="routeSelection = page.path">查看关联</UiButton></td>
              </tr>
            </tbody>
          </table>
        </div>
        <p v-else class="module-empty">当前构建没有找到使用该模块业务操作的页面入口。无页面关联不等于系统没有该能力。</p>
        <section v-if="selectedPage" class="module-route-detail" aria-live="polite">
          <h4>{{ selectedPage.title }} · 页面入口定义</h4>
          <code>{{ selectedPage.path }}</code>
          <p>入口声明的全部操作：</p>
          <code v-for="operation in selectedPage.requiredOperations" :key="operation">{{ operation }}</code>
          <p>这里只检查定义，不切换租户、不模拟成员身份，也不执行业务操作。</p>
        </section>
      </template>
      <template v-else>
        <div class="module-section-header">
          <div>
            <h3>可用性核验</h3>
            <p>定义关联完整、技术就绪、当前成员可执行，是三个不同的结果。</p>
          </div>
        </div>
        <div class="module-notice warning">
          <strong>成员级授权核验尚未接入</strong>
          <p>当前接口不提供“指定租户＋指定成员＋指定对象”的完整授权解释。本页不生成模拟通过结果。</p>
        </div>
        <dl class="module-verification">
          <div><dt>模块定义</dt><dd>来自系统模块目录</dd></div>
          <div><dt>权限与页面映射</dt><dd>来自当前构建的操作契约和真实路由，只读</dd></div>
          <div><dt>当前平台账号权限</dt><dd>暂无预览；提交操作时系统逐项校验</dd></div>
          <div><dt>租户成员、数据范围与额度</dt><dd>尚未执行核验</dd></div>
          <div><dt>运行验证证据</dt><dd>当前模块接口不提供；不能由管理员自行勾选通过</dd></div>
        </dl>
        <UiButton class="btn" @click="router.push('/platform/commercial/tenant-entitlements')">打开已有租户权益页面</UiButton>
        <p class="module-note">套餐变更继续复用已有流程。打开权益页面不代表拥有租户业务操作权限。</p>
      </template>
    </section>
  </div>
</template>
