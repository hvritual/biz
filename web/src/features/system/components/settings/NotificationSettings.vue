<script setup lang="ts">
import { computed, onBeforeUnmount, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { UiButton, UiInput, UiOption, UiSelect } from '@/ui/base'
import AppPagination from '@/ui/common/AppPagination.vue'
import EmptyState from '@/ui/common/EmptyState.vue'
import UiDialog from '@/ui/common/UiDialog.vue'
import AppIcon from '@/ui/common/AppIcon.vue'
import { useEnterpriseStore } from '@/stores/enterprise'
import { currentAuthorizationAllows, currentAuthorizationState } from '@/services/runtime/authorization'
import { sessionContext } from '@/services/runtime/api'
import { subscribeSessionContextChange } from '@/services/runtime/sessionCoordinator'
import { messageLevels } from '@/services/enterprise/notificationConfigurationRuntime'
import { useNotificationConfiguration } from '../../composables/useNotificationConfiguration'
import NotificationDirectoryPicker from './NotificationDirectoryPicker.vue'
import NotificationConfigurationEditor from './NotificationConfigurationEditor.vue'
const { t } = useI18n(), enterprise = useEnterpriseStore()
const flow = useNotificationConfiguration(() => enterprise.session, currentAuthorizationAllows)
const { rows, types, channels, query, typeQuery, draft, selected, pending, lastReceipt, busy, listBusy, typeBusy,
  loaded, error, listError, typeError, mustReload, outcome, editorOpen, deleteOpen, recovery, canStart } = flow
const tenantName = computed(() => currentAuthorizationState.snapshot?.tenant_name || enterprise.session?.tenants?.find(item => item.id === enterprise.tenantId)?.name || enterprise.tenantId)
const canCreate = computed(() => currentAuthorizationAllows('notification.configuration.create'))
const canUpdate = computed(() => currentAuthorizationAllows('notification.configuration.update'))
const canDelete = computed(() => currentAuthorizationAllows('notification.configuration.delete'))
function channelNames(codes: readonly string[]) { return codes.map(code => channels.value.find(channel => channel.code === code)?.name ?? t('notificationConfiguration.unavailableChannel')).join(' / ') }
function applyList() { query.page = 1; void flow.loadList() }
function applyTypes() { typeQuery.page = 1; void flow.loadTypes() }
function retry() { if (error.value === 'signIn' || error.value === 'sessionChanged') window.location.reload(); else void flow.load() }
watch(() => [enterprise.sourceKind, enterprise.session ? sessionContext(enterprise.session) : ''], () => {
  flow.invalidate()
  if (enterprise.sourceKind === 'api' && enterprise.session?.authenticated) void flow.load()
}, { immediate: true, flush: 'sync' })
const unsubscribe = subscribeSessionContextChange(() => flow.invalidate('sessionChanged'))
function invalidatePage() { flow.invalidate() }
window.addEventListener('pagehide', invalidatePage)
onBeforeUnmount(() => { unsubscribe(); window.removeEventListener('pagehide', invalidatePage); flow.invalidate() })
</script>
<template>
  <div class="notification-settings" data-enterprise-page="notification-settings" :aria-busy="busy || listBusy || typeBusy">
    <div class="notification-heading"><div><h2>{{ t('notificationConfiguration.title') }}</h2><p>{{ t('notificationConfiguration.description') }}</p></div>
      <UiButton v-if="canCreate" :disabled="!canStart" @click="flow.startCreate"><AppIcon name="plus" :size="16" />{{ t('notificationConfiguration.create') }}</UiButton>
    </div>
    <p class="tenant-scope">{{ t('notificationConfiguration.scope', { tenant: tenantName }) }}</p>
    <p v-if="enterprise.sourceKind !== 'api'" class="notice-box" role="status">{{ t('notificationConfiguration.preview') }}</p>
    <template v-else>
      <p v-if="!loaded && busy" role="status">{{ t('notificationConfiguration.loading') }}</p>
      <div v-if="error" class="notice-box danger" role="alert">{{ t(`notificationConfiguration.errors.${error}`) }} <UiButton v-if="!pending" variant="outline" :disabled="busy" @click="retry">{{ t('notificationConfiguration.retry') }}</UiButton></div>
      <div v-if="outcome" class="notice-box" role="status" data-notification-outcome><p>{{ t(`notificationConfiguration.${outcome}`) }}</p><small v-if="lastReceipt">{{ t('notificationConfiguration.receipt', { id: lastReceipt.receiptId }) }}</small></div>
      <div v-if="recovery" class="notice-box warning" role="status"><p>{{ t(recovery === 'read' ? 'notificationConfiguration.readPending' : 'notificationConfiguration.uncertain') }}</p><UiButton :disabled="busy" @click="flow.recover">{{ t('notificationConfiguration.recover') }}</UiButton></div>
      <section class="catalog-panel" :aria-label="t('notificationConfiguration.catalog')">
        <header><h3>{{ t('notificationConfiguration.catalog') }}</h3><span v-if="types">{{ t('notificationConfiguration.total', { total: types.total }) }}</span></header>
        <div class="type-filters">
          <UiInput v-model="typeQuery.code" :aria-label="t('notificationConfiguration.typeCode')" :placeholder="t('notificationConfiguration.typeCode')" maxlength="64" @keydown.enter.prevent="applyTypes" />
          <UiInput v-model="typeQuery.name" :aria-label="t('notificationConfiguration.typeName')" :placeholder="t('notificationConfiguration.typeName')" maxlength="200" @keydown.enter.prevent="applyTypes" />
          <UiSelect v-model="typeQuery.level" :aria-label="t('notificationConfiguration.level')"><UiOption value="">{{ t('notificationConfiguration.all') }}</UiOption><UiOption v-for="level in messageLevels" :key="level" :value="level">{{ t(`notificationConfiguration.${level}`) }}</UiOption></UiSelect>
          <UiButton variant="outline" :disabled="typeBusy || !loaded" @click="applyTypes">{{ t('notificationConfiguration.search') }}</UiButton>
        </div>
        <p v-if="typeError" role="alert">{{ t(`notificationConfiguration.errors.${typeError}`) }}</p>
        <template v-else-if="types">
          <div class="level-totals"><span v-for="group in types.groups" :key="group.level" :class="`level-${group.level}`">{{ t('notificationConfiguration.groupTotal', { level: t(`notificationConfiguration.${group.level}`), total: group.total }) }}</span></div>
          <div class="table-scroll"><table v-if="types.items.length"><thead><tr><th>{{ t('notificationConfiguration.typeCode') }}</th><th>{{ t('notificationConfiguration.typeName') }}</th><th>{{ t('notificationConfiguration.level') }}</th></tr></thead><tbody><tr v-for="row in types.items" :key="row.code"><td>{{ row.code }}</td><td>{{ row.name }}</td><td>{{ t(`notificationConfiguration.${row.level}`) }}</td></tr></tbody></table></div>
          <p v-if="!types.items.length">{{ t('notificationConfiguration.emptyTypes') }}</p>
          <AppPagination :page="typeQuery.page" :page-size="typeQuery.pageSize" :total="types.total" @update:page="typeQuery.page = $event; flow.loadTypes()" @update:page-size="typeQuery.pageSize = $event; applyTypes()" />
        </template>
      </section>
      <section class="configuration-panel" :aria-label="t('notificationConfiguration.configurations')">
        <header><h3>{{ t('notificationConfiguration.configurations') }}</h3><span v-if="rows">{{ t('notificationConfiguration.total', { total: rows.total }) }}</span></header>
        <div class="configuration-filters">
          <NotificationDirectoryPicker kind="groups" :session="enterprise.session" :label="t('notificationConfiguration.group')" :model-value="query.groupId ? [query.groupId] : []" :disabled="Boolean(pending)" @update:model-value="query.groupId = $event[0] ?? ''" />
          <NotificationDirectoryPicker kind="recipients" :session="enterprise.session" :label="t('notificationConfiguration.recipientFilter')" :model-value="query.recipientId ? [query.recipientId] : []" :disabled="Boolean(pending)" @update:model-value="query.recipientId = $event[0] ?? ''" />
        </div>
        <div class="rule-filter-actions"><UiSelect v-model="query.level" :aria-label="t('notificationConfiguration.level')"><UiOption value="">{{ t('notificationConfiguration.all') }}</UiOption><UiOption v-for="level in messageLevels" :key="level" :value="level">{{ t(`notificationConfiguration.${level}`) }}</UiOption></UiSelect><UiButton variant="outline" :disabled="listBusy || Boolean(pending) || !loaded" @click="applyList">{{ t('notificationConfiguration.search') }}</UiButton><UiButton variant="ghost" :disabled="Boolean(pending)" @click="query.groupId = ''; query.recipientId = ''; query.level = ''; applyList()">{{ t('notificationConfiguration.reset') }}</UiButton></div>
        <p v-if="listError" class="notice-box danger" role="alert">{{ t(`notificationConfiguration.errors.${listError}`) }} <UiButton variant="outline" :disabled="listBusy" @click="flow.loadList">{{ t('notificationConfiguration.retry') }}</UiButton></p>
        <p v-else-if="listBusy" role="status">{{ t('notificationConfiguration.loading') }}</p>
        <template v-else-if="rows">
          <div v-if="rows.items.length" class="table-scroll"><table class="configuration-table"><thead><tr><th>{{ t('notificationConfiguration.group') }}</th><th>{{ t('notificationConfiguration.level') }}</th><th>{{ t('notificationConfiguration.channelLabel') }}</th><th>{{ t('notificationConfiguration.primary') }}</th><th>{{ t('notificationConfiguration.version') }}</th><th>{{ t('notificationConfiguration.action') }}</th></tr></thead><tbody>
            <tr v-for="row in rows.items" :key="row.id"><td>{{ row.groupName || row.groupId }}</td><td>{{ t(`notificationConfiguration.${row.level}`) }}</td><td>{{ channelNames(row.channels) }}</td><td>{{ row.primaryUserId }}</td><td>{{ row.version }}</td><td class="row-actions"><UiButton v-if="canUpdate" size="sm" variant="outline" :disabled="!canStart" @click="flow.startEdit(row)">{{ t('notificationConfiguration.edit') }}</UiButton><UiButton v-if="canDelete" size="sm" variant="outline" :disabled="!canStart" @click="flow.startEdit(row, 'delete')">{{ t('notificationConfiguration.delete') }}</UiButton></td></tr>
          </tbody></table></div>
          <EmptyState v-else :title="t('notificationConfiguration.emptyTitle')" :description="t('notificationConfiguration.emptyDescription')" />
          <AppPagination :page="query.page" :page-size="query.pageSize" :total="rows.total" @update:page="query.page = $event; flow.loadList()" @update:page-size="query.pageSize = $event; applyList()" />
        </template>
      </section>
      <section class="channel-panel"><h3>{{ t('notificationConfiguration.channels') }}</h3><div v-for="channel in channels" :key="channel.code" class="channel-status"><strong>{{ channel.name }}</strong><span>{{ channel.configurable ? t('notificationConfiguration.available') : channel.unavailableReason }}</span></div><p>{{ t('notificationConfiguration.policyNote') }}</p></section>
    </template>
  </div>
  <NotificationConfigurationEditor :open="editorOpen" :session="enterprise.session" :draft="draft" :selected="selected" :channels="channels" :busy="busy" :recovery="recovery" :error="error" :must-reload="mustReload" @change="Object.assign(draft, $event)" @close="flow.close" @confirm="flow.confirm(selected ? 'update' : 'create')" @recover="flow.recover" @reload="flow.load" />
  <UiDialog :open="deleteOpen" :title="t('notificationConfiguration.deleteTitle')" width="540px" @close="flow.close">
    <p>{{ t('notificationConfiguration.deleteNote', { group: selected?.groupName || selected?.groupId, level: selected ? t(`notificationConfiguration.${selected.level}`) : '' }) }}</p>
    <p v-if="error" class="notice-box danger" role="alert">{{ t(`notificationConfiguration.errors.${error}`) }}</p>
    <p v-if="recovery" class="notice-box warning" role="status">{{ t(recovery === 'read' ? 'notificationConfiguration.readPending' : 'notificationConfiguration.uncertain') }}</p>
    <template #footer><UiButton variant="outline" :disabled="busy" @click="flow.close">{{ t('notificationConfiguration.cancel') }}</UiButton><UiButton v-if="recovery" :disabled="busy" @click="flow.recover">{{ t('notificationConfiguration.recover') }}</UiButton><UiButton v-else-if="mustReload" :disabled="busy" @click="flow.load">{{ t('notificationConfiguration.retry') }}</UiButton><UiButton v-else :disabled="busy" @click="flow.confirm('delete')">{{ t('notificationConfiguration.confirmDelete') }}</UiButton></template>
  </UiDialog>
</template>
<style scoped>
.notification-settings { display:flex; flex-direction:column; gap:20px; min-width:0; }
.notification-heading,header,.channel-status { display:flex; align-items:flex-start; justify-content:space-between; gap:16px; }
h2 { font-size:20px; } h3 { font-size:16px; } p { font-size:var(--text-sm); color:var(--color-text-secondary); line-height:1.6; }
.notification-heading p { margin-top:8px; max-width:680px; }.notification-heading button { flex-shrink:0; }
.tenant-scope { overflow-wrap:anywhere; }
.catalog-panel,.configuration-panel,.channel-panel { border:1px solid var(--color-border); border-radius:var(--radius-md); padding:18px; min-width:0; }
header>span { font-size:var(--text-sm); color:var(--color-text-muted); }
.type-filters { display:grid; grid-template-columns:minmax(0,1fr) minmax(0,1fr) 130px auto; gap:10px; margin:16px 0; }
.configuration-filters { display:grid; grid-template-columns:repeat(2,minmax(0,1fr)); gap:20px; margin:16px 0; }
.rule-filter-actions { display:flex; gap:8px; flex-wrap:wrap; margin-bottom:16px; }.rule-filter-actions>button:first-child { max-width:180px; }
.level-totals { display:flex; flex-wrap:wrap; gap:8px; margin:14px 0; font-size:var(--text-sm); }.level-totals span { padding:5px 9px; border-radius:var(--radius-sm); background:var(--color-surface-soft); }
.level-urgent { color:var(--color-danger); }.level-important { color:var(--color-warning); }
.table-scroll { max-width:100%; overflow:auto; } table { width:100%; border-collapse:collapse; font-size:var(--text-sm); } th,td { padding:12px 10px; text-align:left; border-bottom:1px solid var(--color-border); } th { color:var(--color-text-secondary); font-weight:500; background:var(--color-surface-soft); }
.configuration-table { min-width:680px; } td { overflow-wrap:anywhere; max-width:240px; }.row-actions { white-space:nowrap; }.row-actions button+button { margin-left:6px; }
.channel-status { padding:12px 0; font-size:var(--text-sm); border-bottom:1px solid var(--color-border); }.channel-status span { color:var(--color-text-secondary); }.channel-panel>p { margin-top:14px; }
.notice-box { display:block; overflow-wrap:anywhere; }.notice-box button { margin-top:8px; }.notice-box small { display:block; font-size:var(--text-xs); }
@media(max-width:1050px) { .type-filters { grid-template-columns:repeat(2,minmax(0,1fr)); } }
@media(max-width:640px) { .notification-heading,header { flex-direction:column; } .type-filters,.configuration-filters { grid-template-columns:1fr; } .catalog-panel,.configuration-panel,.channel-panel { padding:12px; }.table-scroll { border:1px solid var(--color-border); } }
</style>
