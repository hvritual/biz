<script setup lang="ts">
import { computed } from 'vue'
import { UiButton, UiOption, UiSelect } from '@/ui/base'
import AppIcon from '@/ui/common/AppIcon.vue'
import { useEnterpriseStore } from '@/stores/enterprise'
import { useUiStore } from '@/stores/ui'

const store = useEnterpriseStore()
const ui = useUiStore()
const visible = computed(() => store.sourceKind === 'api')

async function changeTenant(event: Event) {
  const tenantId = (event.target as HTMLSelectElement).value
  if (!tenantId) return
  try {
    await store.switchTenant(tenantId)
    ui.toast('已切换企业，并从服务端重新读取当前租户数据。', 'success')
  } catch (error) {
    ui.toast(error instanceof Error ? error.message : '切换企业失败。', 'error')
  }
}

async function refresh() {
  try {
    await store.refresh()
    ui.toast('服务端数据已刷新。', 'success')
  } catch (error) {
    ui.toast(error instanceof Error ? error.message : '刷新失败。', 'error')
  }
}
</script>

<template>
  <section v-if="visible" class="card source-banner" data-enterprise-source="api" aria-label="企业数据源状态">
    <div class="source-main">
      <AppIcon name="shield" :size="17" />
      <div>
        <strong>{{ store.authenticated ? '真实服务数据' : '需要登录业务账号' }}</strong>
        <span v-if="store.sourceError" class="source-error">{{ store.sourceError }}</span>
        <span v-else-if="store.loading">正在读取服务端数据…</span>
        <span v-else-if="store.authenticated">当前页面保持统一产品界面，数据与写操作由服务端确认。</span>
        <span v-else>API 模式不会回退到本地示例数据。</span>
      </div>
    </div>

    <template v-if="store.authenticated">
      <label v-if="store.tenantOptions.length" class="source-tenant">
        <span>当前企业</span>
        <UiSelect :value="store.tenantId" :disabled="store.loading" @change="changeTenant">
          <UiOption value="" disabled>请选择企业</UiOption>
          <UiOption v-for="tenant in store.tenantOptions" :key="tenant.id" :value="tenant.id">
            {{ tenant.name }}
          </UiOption>
        </UiSelect>
      </label>
      <UiButton class="btn" :disabled="store.loading" @click="refresh">
        <AppIcon name="refresh" :size="15" />刷新
      </UiButton>
    </template>
    <a v-else class="btn btn-primary" :href="store.loginHref()">登录业务账号</a>
  </section>
</template>

<style scoped>
.source-banner {
  display: flex;
  align-items: center;
  gap: 16px;
  padding: 12px 16px;
  border-color: var(--color-border);
  background: var(--color-surface-soft);
}
.source-main {
  min-width: 0;
  flex: 1;
  display: flex;
  align-items: center;
  gap: 10px;
}
.source-main > .icon {
  color: var(--color-primary);
  flex: 0 0 auto;
}
.source-main strong,
.source-main span {
  display: block;
}
.source-main strong {
  font-size: 12px;
}
.source-main span {
  margin-top: 3px;
  color: var(--color-text-muted);
  font-size: 11px;
}
.source-main .source-error {
  color: var(--color-danger);
}
.source-tenant {
  display: flex;
  align-items: center;
  gap: 8px;
  font-size: 11px;
  color: var(--color-text-muted);
}
.source-tenant :deep(select) {
  min-width: 180px;
}
@media (max-width: 767px) {
  .source-banner {
    align-items: stretch;
    flex-direction: column;
  }
  .source-tenant {
    width: 100%;
  }
  .source-tenant :deep(select) {
    min-width: 0;
    flex: 1;
  }
}
</style>
