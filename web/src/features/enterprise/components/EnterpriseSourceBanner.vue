<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import { UiButton, UiOption, UiSelect } from '@/ui/base'
import AppIcon from '@/ui/common/AppIcon.vue'
import { useEnterpriseStore } from '@/stores/enterprise'
import { useUiStore } from '@/stores/ui'
const store=useEnterpriseStore(),ui=useUiStore(); const {t}=useI18n(); const visible=computed(()=>store.sourceKind==='api')
async function changeTenant(event:Event){const tenantId=(event.target as HTMLSelectElement).value;if(!tenantId)return;try{await store.switchTenant(tenantId);ui.toast(t('members.source.switched'),'success')}catch(error){ui.toast(error instanceof Error?error.message:t('members.source.switchFailed'),'error')}}
async function refresh(){try{await store.refresh();ui.toast(t('members.source.refreshed'),'success')}catch(error){ui.toast(error instanceof Error?error.message:t('members.source.refreshFailed'),'error')}}
</script>
<template>
  <section v-if="visible" class="card source-banner" data-enterprise-source="api" :aria-label="t('members.source.aria')">
    <div class="source-main"><AppIcon name="shield" :size="17"/><div><strong>{{ store.authenticated?t('members.source.real'):t('members.source.loginRequired') }}</strong><span v-if="store.sourceError" class="source-error" role="alert">{{ store.sourceError }}</span><span v-else-if="store.loading">{{ t('members.source.loading') }}</span><span v-else-if="store.authenticated">{{ t('members.source.confirmed') }}</span><span v-else-if="store.ready" class="source-error" role="alert">{{ t('members.source.expired') }}</span><span v-else>{{ t('members.source.checking') }}</span></div></div>
    <template v-if="store.authenticated"><label v-if="store.tenantOptions.length" class="source-tenant"><span>{{ t('members.source.currentTenant') }}</span><UiSelect :value="store.tenantId" :disabled="store.loading" @change="changeTenant"><UiOption value="" disabled>{{ t('members.source.selectTenant') }}</UiOption><UiOption v-for="tenant in store.tenantOptions" :key="tenant.id" :value="tenant.id">{{ tenant.name }}</UiOption></UiSelect></label><UiButton class="btn" :disabled="store.loading" @click="refresh"><AppIcon name="refresh" :size="15"/>{{ t('common.refresh') }}</UiButton></template>
    <a v-else class="btn btn-primary" :href="store.loginHref()">{{ t('members.source.loginBusiness') }}</a>
  </section>
</template>
<style scoped>
.source-banner{display:flex;align-items:center;gap:16px;padding:12px 16px;border-color:var(--color-border);background:var(--color-surface-soft)}.source-main{min-width:0;flex:1;display:flex;align-items:center;gap:10px}.source-main>.icon{color:var(--color-primary);flex:0 0 auto}.source-main strong,.source-main span{display:block}.source-main strong{font-size:12px}.source-main span{margin-top:3px;color:var(--color-text-muted);font-size:11px}.source-main .source-error{color:var(--color-danger)}.source-tenant{display:flex;align-items:center;gap:8px;font-size:11px;color:var(--color-text-muted)}.source-tenant :deep(select){min-width:180px}@media(max-width:767px){.source-banner{align-items:stretch;flex-direction:column}.source-tenant{width:100%}.source-tenant :deep(select){min-width:0;flex:1}}
</style>
