<script setup lang="ts">
import { UiButton, UiTextarea } from '@/ui/base'
import { computed, nextTick, onBeforeUnmount, onMounted, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { useRoute, useRouter } from 'vue-router'
import { useEnterpriseStore, type MemberBatchInspection, type MemberMutationTargetOutcome } from '@/stores/enterprise'
import { useUiStore } from '@/stores/ui'
import type { Member, MemberAction, MemberStatus } from '@/types/enterprise'
import { downloadCsv } from '@/utils/format'
import PageHeading from '@/ui/common/PageHeading.vue'
import AppIcon from '@/ui/common/AppIcon.vue'
import AppPagination from '@/ui/common/AppPagination.vue'
import EmptyState from '@/ui/common/EmptyState.vue'
import UiDialog from '@/ui/common/UiDialog.vue'
import MemberOverview from '@/features/enterprise/components/members/MemberOverview.vue'
import MemberFilters,{type MemberFilterValue} from '@/features/enterprise/components/members/MemberFilters.vue'
import MemberTable,{type MemberDetailTrigger,type MemberSortKey} from '@/features/enterprise/components/members/MemberTable.vue'
import MemberActionDialog from '@/features/enterprise/components/members/MemberActionDialog.vue'
import MemberDetailDrawer from '@/features/enterprise/components/members/MemberDetailDrawer.vue'
import MemberBulkDialog from '@/features/enterprise/components/members/MemberBulkDialog.vue'
import type { EnterpriseDomain } from '@/services/enterprise/dataSource'
import { currentAuthorizationAllows } from '@/services/runtime/authorization'
const store=useEnterpriseStore(),ui=useUiStore(),route=useRoute(),router=useRouter(),{t,locale}=useI18n()
const defaultFilters=():MemberFilterValue=>({query:'',role:'',department:'',status:''}); const filters=ref(defaultFilters()),page=ref(1),pageSize=ref(10),selected=ref<string[]>([]),action=ref<MemberAction>('create'),actionOpen=ref(false),target=ref<Member|null>(null),detailId=ref<string|null>(null),more=ref<Member|null>(null),sortKey=ref<MemberSortKey>(),sortDirection=ref<'asc'|'desc'>('asc'),batchOpen=ref(false),batchAction=ref<'activate'|'suspend'>('suspend'),batchTargets=ref<{id:string;version:number}[]>([]),mounted=ref(false)
type BatchResolutionState=MemberBatchInspection['state']|'rejected'|'not_started'
type BatchResolutionItem=Omit<MemberBatchInspection,'state'>&{state:BatchResolutionState;name:string}
type BatchInspectionView={action:'activate'|'suspend';reason:string;error:string;checking:boolean;refreshRequired:boolean;items:BatchResolutionItem[]}
const helpOpen=ref(false),batchResult=ref(''),setupError=ref(''),batchInspection=ref<BatchInspectionView|null>(null),batchInspectionPanel=ref<HTMLElement|null>(null),detailRestore=ref<{id:string;trigger:MemberDetailTrigger}|null>(null)
const memberPending=computed(()=>!mounted.value||store.memberQueryLoading),memberError=computed(()=>setupError.value||store.memberQueryError)
const batchInspectionPending=computed(()=>Boolean(batchInspection.value&&(batchInspection.value.checking||batchInspection.value.refreshRequired||batchInspection.value.items.some(item=>item.state==='unknown'))))
const contextKey=computed(()=>`${store.tenantId}:${store.session?.user_id??''}:${store.session?.context_version??''}`)
const detail=computed(()=>store.members.find(m=>m.id===detailId.value)??null)
const recycleOpen=ref(false),restoreTarget=ref<Member|null>(null),restoreReason=ref(''),restoreBusy=ref(false)
const apiAllows=(code:string)=>store.previewMode||currentAuthorizationAllows(code)
const apiAllowsAll=(codes:string[])=>store.previewMode||codes.every(currentAuthorizationAllows)
// Creation and edits use atomic Operations. These existing forms also need
// the role directory; a write grant does not imply its independent read grant.
const canCreateMember=computed(()=>apiAllowsAll(['tenant.member.create','tenant.role.list']))
const canInviteMember=computed(()=>apiAllowsAll(['tenant.member.create','tenant.role.list']))
const canEditMember=computed(()=>apiAllowsAll(['tenant.member.update','tenant.role.list']))
const canChangeRoles=computed(()=>apiAllowsAll(['tenant.member.update','tenant.role.list']))
const canReadBusinessScope=computed(()=>apiAllows('tenant.member.business_scope.get'))
const canEditBusinessScope=computed(()=>apiAllowsAll(['tenant.member.business_scope.get','tenant.member.business_scope.set','tenant.member.scope_candidates']))
const canActivateMember=computed(()=>apiAllows('tenant.member.activate'))
const canSuspendMember=computed(()=>apiAllows('tenant.member.suspend'))
const canRemoveMember=computed(()=>apiAllows('tenant.member.remove'))
const canListRemovedMembers=computed(()=>apiAllows('tenant.member.list_removed'))
const canRestoreMember=computed(()=>apiAllowsAll(['tenant.member.restore','tenant.role.assign_member']))
const canResetPassword=computed(()=>store.previewMode)
const canExportMembers=computed(()=>store.previewMode)
const canMoreMemberActions=computed(()=>canEditMember.value||canChangeRoles.value||canActivateMember.value||canSuspendMember.value||canRemoveMember.value||canResetPassword.value)
const filtered=computed(()=>{if(!store.previewMode)return store.members;const {query,role,department,status}=filters.value;const result=store.members.filter(m=>(!query||`${m.name} ${m.email} ${m.phone} ${m.employeeId}`.toLowerCase().includes(query.trim().toLowerCase()))&&(!role||m.roleIds.includes(role))&&(!department||m.departmentId===department)&&(!status||m.status===status));if(!sortKey.value)return result;const key=sortKey.value;return[...result].sort((a,b)=>{const value=(m:Member)=>key==='departmentId'?store.departmentName(m.departmentId):String(m[key]??'');return value(a).localeCompare(value(b),locale.value,{numeric:true})*(sortDirection.value==='asc'?1:-1)})})
const total=computed(()=>store.previewMode?filtered.value.length:store.memberTotal)
const paged=computed(()=>store.previewMode?filtered.value.slice((page.value-1)*pageSize.value,page.value*pageSize.value):store.members),selection=computed(()=>store.members.filter(m=>selected.value.includes(m.id))),canActivate=computed(()=>canActivateMember.value&&selection.value.length>0&&selection.value.every(m=>m.status==='suspended')),canSuspend=computed(()=>canSuspendMember.value&&selection.value.length>0&&selection.value.every(m=>m.status==='active'))
watch([page,pageSize],()=>{selected.value=[];detailId.value=null})
watch(()=>total.value,()=>{const next=Math.min(page.value,Math.max(1,Math.ceil(total.value/pageSize.value)));if(next!==page.value)page.value=next})
watch(()=>ui.module,module=>{if(module)detailId.value=null})
watch(contextKey,()=>{if(!mounted.value)return;filters.value=defaultFilters();page.value=1;selected.value=[];sortKey.value=undefined;sortDirection.value='asc';detailId.value=null;detailRestore.value=null;more.value=null;actionOpen.value=false;target.value=null;batchOpen.value=false;batchTargets.value=[];batchResult.value='';batchInspection.value=null;recycleOpen.value=false;restoreTarget.value=null;restoreReason.value='';restoreBusy.value=false;helpOpen.value=false;setupError.value=''},{flush:'sync'})
// One post-flush read for a filter/page/context change; old requests cannot win.
watch([mounted,page,pageSize,filters,contextKey],()=>{if(mounted.value&&!store.previewMode&&store.tenantId)void loadMembers()},{flush:'post'})
watch(()=>route.query,q=>{if(typeof q.q==='string'||typeof q.department==='string')applyFilters({...defaultFilters(),query:typeof q.q==='string'?q.q:'',department:typeof q.department==='string'?q.department:''});if(q.action==='create'||q.action==='invite'){if(q.action==='create'?canCreateMember.value:canInviteMember.value)openAction(q.action);void router.replace({path:route.path,query:{}})}},{immediate:true})
function scopeLabel(scope:Member['scope']){return t(`members.scopes.${scope==='department_tree'?'department':scope==='self'?'own':scope}`)}
function statusLabel(status:Member['status']){return t(`members.statuses.${status}`)}
function loadMembers(){return store.queryMembers({query:filters.value.query,roleId:filters.value.role,departmentId:filters.value.department,status:filters.value.status as MemberStatus|'',page:page.value,pageSize:pageSize.value}).catch(()=>false)}
async function refreshMembers(){selected.value=[];detailId.value=null;const context=contextKey.value;try{if(setupError.value){await store.ensureDomains(readableMemberDomains());if(context!==contextKey.value||!mounted.value)return false;setupError.value=''}return await loadMembers()}catch(error){if(context===contextKey.value&&mounted.value)setupError.value=error instanceof Error?error.message:t('members.action.failure');return false}}
async function verifyBatchInspection(){
  const inspection=batchInspection.value
  if(!inspection||inspection.checking)return
  const context=contextKey.value
  inspection.checking=true
  try{
    const unresolved=inspection.items.filter(item=>item.state==='unknown')
    if(unresolved.length){
      const results=await store.inspectMemberStatusBatch(unresolved.map(item=>({id:item.id,version:item.baselineVersion})),inspection.action)
      if(context!==contextKey.value||!mounted.value||batchInspection.value!==inspection)return
      const updates=new Map(results.map(result=>[result.id,result]))
      inspection.items=inspection.items.map(item=>{
        const result=updates.get(item.id)
        return result?{...result,name:item.name}:item
      })
    }
    if(context!==contextKey.value||!mounted.value||batchInspection.value!==inspection)return
    const refreshed=await loadMembers()
    inspection.refreshRequired=!refreshed
    if(refreshed&&!inspection.items.some(item=>item.state==='unknown'||item.state==='rejected'))inspection.error=''
  }catch(error){
    if(context!==contextKey.value||!mounted.value||batchInspection.value!==inspection)return
    const message=error instanceof Error?error.message:t('members.task.batchFailure')
    inspection.items=inspection.items.map(item=>item.state==='unknown'?{...item,message}:item)
    inspection.refreshRequired=true
  }finally{if(context===contextKey.value&&mounted.value&&batchInspection.value===inspection)inspection.checking=false}
}
function targetResolutionState(target:MemberMutationTargetOutcome):BatchResolutionState{
  return target.outcome==='write_confirmed'?'confirmed':target.outcome==='rejected'?'rejected':target.outcome==='not_started'?'not_started':'unknown'
}
async function inspectBatch(details:{reason:string;error:string;targets?:MemberMutationTargetOutcome[];refreshRequired?:boolean}){
  const names=new Map(batchTargets.value.map(target=>[target.id,store.members.find(member=>member.id===target.id)?.name??target.id]))
  const items:BatchResolutionItem[]=details.targets?.length
    ? details.targets.map(target=>({
        id:target.id,
        baselineVersion:target.baselineVersion,
        state:targetResolutionState(target),
        status:null,
        currentVersion:null,
        message:target.message,
        name:names.get(target.id)??target.id,
      }))
    : batchTargets.value.map(target=>({id:target.id,baselineVersion:target.version,state:'unknown',status:null,currentVersion:null,message:'',name:names.get(target.id)??target.id}))
  batchInspection.value={action:batchAction.value,reason:details.reason,error:details.error,checking:false,refreshRequired:details.refreshRequired??false,items}
  batchOpen.value=false
  selected.value=[]
  await nextTick()
  batchInspectionPanel.value?.focus()
  if(items.some(item=>item.state==='unknown'))void verifyBatchInspection()
}
function completeBatchInspection(){if(!batchInspection.value||batchInspectionPending.value)return;batchInspection.value=null;batchTargets.value=[]}
function inspectAction(){actionOpen.value=false;void refreshMembers()}
onBeforeUnmount(()=>{mounted.value=false})
function applyFilters(value:MemberFilterValue){filters.value={...value};page.value=1;selected.value=[];detailId.value=null;batchResult.value=''}
function clear(){applyFilters(defaultFilters())}
function sort(key:MemberSortKey){if(!store.previewMode)return;sortDirection.value=sortKey.value===key&&sortDirection.value==='asc'?'desc':'asc';sortKey.value=key;page.value=1;selected.value=[]}
function memberActionAllowed(kind:MemberAction){if(store.previewMode)return true;return kind==='create'?canCreateMember.value:kind==='invite'?canInviteMember.value:kind==='edit'?canEditMember.value:kind==='role'?canChangeRoles.value:kind==='reset'?false:kind==='activate'?canActivateMember.value:kind==='suspend'?canSuspendMember.value:kind==='remove'?canRemoveMember.value:false}
function openDetail(member:Member,trigger:MemberDetailTrigger){detailRestore.value={id:member.id,trigger};detailId.value=member.id}
async function closeDetail(){
  const restore=detailRestore.value
  detailRestore.value=null
  detailId.value=null
  await nextTick()
  if(!restore||detailId.value||!mounted.value)return
  const row=[...document.querySelectorAll<HTMLElement>('[data-member-id]')].find(element=>element.dataset.memberId===restore.id)
  row?.querySelector<HTMLElement>(`[data-member-detail-trigger="${restore.trigger}"]`)?.focus({preventScroll:true})
}
function openAction(kind:MemberAction,member:Member|null=null){if(!memberActionAllowed(kind))return;action.value=kind;target.value=member?(JSON.parse(JSON.stringify(member)) as Member):null;more.value=null;detailId.value=null;detailRestore.value=null;actionOpen.value=true}
function showMore(member:Member){detailId.value=null;detailRestore.value=null;more.value=member}
function openDetailFromMore(){
  const member=more.value
  if(!member)return
  detailRestore.value={id:member.id,trigger:'more'}
  detailId.value=member.id
  more.value=null
}
function openBatch(kind:'activate'|'suspend'){if(memberPending.value||memberError.value||batchInspection.value)return;batchResult.value='';batchAction.value=kind;batchTargets.value=selection.value.map(m=>({id:m.id,version:m.version}));batchOpen.value=true}
function finishBatch(){const count=batchTargets.value.length;batchResult.value=t(store.previewMode?'members.task.batchPreviewResult':'members.task.batchResult',{count});batchOpen.value=false;selected.value=[];batchTargets.value=[];batchInspection.value=null}
async function openRecycle(){if(!canListRemovedMembers.value)return;const context=contextKey.value;recycleOpen.value=true;restoreTarget.value=null;restoreReason.value='';try{await store.queryRemovedMembers()}catch(error){if(mounted.value&&context===contextKey.value&&recycleOpen.value)ui.toast(error instanceof Error?error.message:t('members.recycle.loadFailed'),'error')}}
function closeRecycle(){if(restoreBusy.value)return;recycleOpen.value=false;restoreTarget.value=null;restoreReason.value=''}
function chooseRestore(member:Member){restoreTarget.value=member;restoreReason.value=''}
async function confirmRestore(){if(restoreBusy.value||!restoreTarget.value||!restoreReason.value.trim())return;const context=contextKey.value;const current=()=>mounted.value&&context===contextKey.value;restoreBusy.value=true;try{await store.restoreMember(restoreTarget.value.id,restoreTarget.value.version,restoreReason.value);if(!current())return;ui.toast(t('members.recycle.success'),'success');restoreTarget.value=null;restoreReason.value=''}catch(error){if(current())ui.toast(error instanceof Error?error.message:t('members.recycle.failed'),'error')}finally{if(current())restoreBusy.value=false}}
function exportMembers(){try{const output=selected.value.length?filtered.value.filter(m=>selected.value.includes(m.id)):filtered.value;downloadCsv(t('members.exportFile'),[[t('members.columns.name'),t('members.columns.contact'),t('members.department'),t('members.role'),t('members.columns.scope'),t('members.columns.status'),t('members.columns.lastLogin'),t('members.columns.joinedAt')],...output.map(m=>[m.name,`${m.phone} / ${m.email}`,store.departmentName(m.departmentId),m.roleIds.map(store.roleName).join(', '),scopeLabel(m.scope),statusLabel(m.status),m.lastLogin||'',m.joinedAt])]);store.audit(t('members.auditModule'),t('members.auditExport'),t('members.auditCount',{count:output.length}));ui.toast(t('members.exportSuccess',{count:output.length}))}catch(e){ui.toast((e as Error).message,'error')}}
function readableMemberDomains(){
  const domains:EnterpriseDomain[]=store.previewMode?['members']:[]
  if(apiAllows('tenant.role.list'))domains.push('roles')
  if(apiAllows('tenant.department.list'))domains.push('departments')
  return domains
}
onMounted(async()=>{try{await store.ensureDomains(readableMemberDomains())}catch(error){setupError.value=error instanceof Error?error.message:t('members.action.failure')}finally{mounted.value=true}})
</script>
<template>
  <div class="members-view" data-enterprise-page="members" data-ui-template="ListPage" :class="{'has-detail':Boolean(detail)}"><div class="member-main"><div class="page-stack members-stack"><PageHeading :title="t('members.title')" :description="t('members.description')" banner compact/><MemberOverview/><div class="task-help-toggle"><UiButton class="btn-link" :aria-expanded="helpOpen" aria-controls="member-task-help" @click="helpOpen=!helpOpen">{{ t('members.task.help') }}</UiButton></div><section v-if="helpOpen" id="member-task-help" class="card member-task-help" :aria-label="t('members.task.help')"><p>{{ t('members.task.find') }}</p><p>{{ t('members.task.view') }}</p><p>{{ t('members.task.batch') }}</p><p v-if="!store.previewMode">{{ t('members.task.serverOrder') }}</p></section><MemberFilters data-ui-region="query" :value="filters" @apply="applyFilters" @reset="clear"/><section data-ui-region="data" class="card member-data-panel" :aria-label="t('members.listAria')"><div class="row-between member-toolbar" data-ui-region="toolbar"><div class="row wrap member-tools"><UiButton v-if="canCreateMember" class="btn btn-primary" @click="openAction('create')"><AppIcon name="plus" :size="16"/>{{ t('members.add') }}</UiButton><UiButton v-if="canInviteMember" class="btn invite-button" @click="openAction('invite')"><AppIcon name="invite" :size="16"/>{{ t('members.invite') }}</UiButton><UiButton v-if="canActivateMember" class="btn" :disabled="!canActivate||memberPending||Boolean(memberError)||Boolean(batchInspection)" @click="openBatch('activate')"><AppIcon name="checks" :size="15"/>{{ t('members.batchActivate') }}</UiButton><UiButton v-if="canSuspendMember" class="btn" :disabled="!canSuspend||memberPending||Boolean(memberError)||Boolean(batchInspection)" @click="openBatch('suspend')"><AppIcon name="lock" :size="14"/>{{ t('members.batchSuspend') }}</UiButton><UiButton v-if="canListRemovedMembers" class="btn" @click="openRecycle">{{ t('members.recycleBin') }}</UiButton><UiButton v-if="canExportMembers" class="btn" :aria-label="selected.length?t('members.exportSelected'):t('members.exportList')" @click="exportMembers"><AppIcon name="download" :size="15"/>{{ t('common.export') }}</UiButton></div><span class="selection-label">{{ t('common.selected',{count:selected.length}) }}<span v-if="selected.length"> · {{ t('common.currentPage') }}</span></span></div><section v-if="batchInspection" ref="batchInspectionPanel" class="batch-inspection" tabindex="-1" :aria-label="t('members.task.inspectionTitle')"><div class="row-between batch-inspection-heading"><div><strong>{{ t('members.task.inspectionTitle') }}</strong><p>{{ t('members.task.inspectionSummary') }}</p></div><span v-if="batchInspection.checking" aria-live="polite">{{ t('members.task.inspectionChecking') }}</span></div><p v-if="batchInspection.error" class="batch-inspection-error">{{ batchInspection.error }}</p><ul class="batch-inspection-list"><li v-for="item in batchInspection.items" :key="item.id" :data-batch-inspection-id="item.id"><span>{{ item.name }}</span><strong :class="`state-${item.state}`">{{ t(`members.task.${item.state==='confirmed'?'inspectionConfirmed':item.state==='not_applied'?'inspectionNotApplied':item.state==='rejected'?'inspectionRejected':item.state==='not_started'?'inspectionNotStarted':'inspectionUnknown'}`) }}</strong><small v-if="item.message">{{ item.message }}</small></li></ul><div class="batch-inspection-actions"><UiButton v-if="batchInspectionPending" class="btn" :disabled="batchInspection.checking" @click="verifyBatchInspection">{{ t('members.task.inspectionRetry') }}</UiButton><UiButton v-else class="btn" @click="completeBatchInspection">{{ t('members.task.inspectionComplete') }}</UiButton></div></section><p v-if="batchResult" class="member-task-result" aria-live="polite">{{ batchResult }}</p><p v-if="memberPending" class="member-task-state" aria-live="polite">{{ t('members.task.loading') }}</p><div v-else-if="memberError" class="member-task-state" role="alert"><p>{{ memberError }}</p><p>{{ t('members.task.readRecovery') }}</p><UiButton class="btn" @click="refreshMembers">{{ t('members.task.refresh') }}</UiButton></div><MemberTable v-else-if="paged.length" v-model:selected="selected" :members="paged" :detail-id="detailId" :can-edit="canEditMember" :can-more="canMoreMemberActions" :sortable="store.previewMode" :sort-key="sortKey" :sort-direction="sortDirection" @sort="sort" @detail="openDetail" @edit="openAction('edit',$event)" @more="showMore"/><EmptyState v-else><UiButton class="btn" @click="clear">{{ t('members.clearFilters') }}</UiButton></EmptyState><AppPagination v-show="!memberPending&&!memberError" data-ui-region="pagination" v-model:page="page" v-model:page-size="pageSize" :total="total"/></section></div></div><MemberActionDialog :key="`action-${contextKey}`" :open="actionOpen" :action="action" :member="target" @close="actionOpen=false" @inspect="inspectAction"/><MemberDetailDrawer :key="`detail-${contextKey}`" :member="detail" :can-edit="canEditMember" :can-change-roles="canChangeRoles" :can-read-scope="canReadBusinessScope" :can-edit-scope="canEditBusinessScope" @scope-saved="loadMembers().catch(()=>undefined)" :can-more="canMoreMemberActions" @close="closeDetail" @action="openAction" @more="showMore"/><MemberBulkDialog :key="`bulk-${contextKey}`" :open="batchOpen" :action="batchAction" :targets="batchTargets" @close="batchOpen=false" @saved="finishBatch" @inspect="inspectBatch"/><UiDialog :open="recycleOpen" :title="t('members.recycle.title')" width="640px" @close="closeRecycle"><div class="recycle-stack"><p class="recycle-hint">{{ t('members.recycle.description') }}</p><div v-if="store.removedMembers.length" class="recycle-list"><article v-for="member in store.removedMembers" :key="member.id" class="recycle-row"><div><strong>{{ member.name }}</strong><span>{{ member.email || member.phone || member.username }}</span></div><UiButton v-if="canRestoreMember" class="btn" @click="chooseRestore(member)">{{ t('members.recycle.restore') }}</UiButton></article></div><EmptyState v-else>{{ t('members.recycle.empty') }}</EmptyState></div></UiDialog><UiDialog :open="Boolean(restoreTarget)" :title="t('members.recycle.restoreTitle',{name:restoreTarget?.name??''})" width="520px" @close="()=>{if(!restoreBusy){restoreTarget=null;restoreReason=''}}"><div class="restore-stack"><p>{{ t('members.recycle.restoreDescription') }}</p><label><span>{{ t('members.recycle.reason') }}</span><UiTextarea v-model="restoreReason" class="restore-reason" maxlength="500" :placeholder="t('members.recycle.reasonPlaceholder')"/></label></div><template #footer><UiButton class="btn" :disabled="restoreBusy" @click="()=>{restoreTarget=null;restoreReason=''}">{{ t('common.cancel') }}</UiButton><UiButton class="btn btn-primary" :disabled="restoreBusy||!restoreReason.trim()" @click="confirmRestore">{{ restoreBusy?t('common.processing'):t('members.recycle.confirm') }}</UiButton></template></UiDialog><UiDialog :open="Boolean(more)" :title="t('members.operations',{name:more?.name??''})" width="420px" @close="more=null"><div v-if="more" class="member-operation-list"><UiButton @click="openDetailFromMore"><AppIcon name="eye"/>{{ t('members.viewFull') }}</UiButton><UiButton v-if="canEditMember" @click="openAction('edit',more)"><AppIcon name="edit"/>{{ t('members.editInfo') }}</UiButton><UiButton v-if="canChangeRoles" @click="openAction('role',more)"><AppIcon name="shield"/>{{ t('members.roleChange') }}</UiButton><UiButton v-if="canResetPassword" @click="openAction('reset',more)"><AppIcon name="key"/>{{ t('members.resetPassword') }}</UiButton><UiButton v-if="canSuspendMember&&more.status==='active'" class="text-danger" @click="openAction('suspend',more)"><AppIcon name="lock"/>{{ t('members.disableAccess') }}</UiButton><UiButton v-if="canActivateMember&&more.status==='suspended'" @click="openAction('activate',more)"><AppIcon name="success"/>{{ t('members.reactivate') }}</UiButton><UiButton v-if="canRemoveMember&&more.status!=='removed'" class="text-danger" @click="openAction('remove',more)"><AppIcon name="logout"/>{{ t('members.task.remove') }}</UiButton></div></UiDialog></div>
</template>
<style scoped>
.task-help-toggle{display:flex;justify-content:flex-end}.task-help-toggle .btn-link{font-size:13px}.member-task-help{padding:14px 16px;display:grid;gap:8px;font-size:13px;line-height:1.6;overflow-wrap:anywhere}.member-task-state{padding:18px 4px;display:grid;gap:12px;font-size:13px;line-height:1.6}.member-task-state .btn{justify-self:start}.member-task-result{padding:10px 4px;font-size:13px;line-height:1.6;color:var(--color-text-secondary)}
.batch-inspection{margin:4px 0 12px;padding:14px;border:1px solid var(--color-border);border-radius:var(--radius-sm);background:var(--color-surface-soft);display:grid;gap:10px}.batch-inspection-heading{align-items:flex-start;gap:12px}.batch-inspection-heading p{margin-top:4px;color:var(--color-text-secondary);font-size:12px;line-height:1.55}.batch-inspection-heading>span{font-size:12px;color:var(--color-text-secondary)}.batch-inspection-error{font-size:12px;color:var(--color-danger)}.batch-inspection-list{display:grid;gap:6px}.batch-inspection-list li{display:grid;grid-template-columns:minmax(0,1fr) auto;gap:3px 12px;padding:8px 10px;border-radius:var(--radius-sm);background:var(--color-surface)}.batch-inspection-list span{overflow-wrap:anywhere}.batch-inspection-list strong{font-size:12px}.batch-inspection-list small{grid-column:1/-1;color:var(--color-text-muted);overflow-wrap:anywhere}.batch-inspection-list .state-confirmed{color:var(--color-success)}.batch-inspection-list .state-not_applied,.batch-inspection-list .state-not_started{color:var(--color-text-secondary)}.batch-inspection-list .state-rejected{color:var(--color-danger)}.batch-inspection-list .state-unknown{color:var(--color-warning)}.batch-inspection-actions{display:flex;justify-content:flex-end}
.member-main{min-width:0;max-width:100%;container-type:inline-size;container-name:member-main}.members-view{min-width:0;max-width:100%;overflow-x:clip}.members-stack{gap:16px}.member-data-panel{padding:0 12px 12px;border-radius:var(--radius-md)}.member-toolbar{min-height:68px;padding:14px 0;flex-wrap:wrap;gap:10px}.member-tools{gap:8px}.member-tools .btn{border-color:var(--color-border)}.member-tools .btn-primary{border-color:var(--color-primary)}.member-tools .invite-button{color:var(--color-primary)}.selection-label{color:var(--color-text-muted);font-size:11px;white-space:normal;margin-left:auto}.member-data-panel :deep(.pagination){padding:20px 2px 5px;font-size:12px}.member-data-panel :deep(.pagination-controls){gap:5px}.member-operation-list{display:flex;flex-direction:column;gap:5px}.member-operation-list button{display:flex;align-items:center;gap:12px;padding:13px;border-radius:7px;text-align:left}.member-operation-list button:hover{background:var(--color-surface-soft)}.member-operation-list .icon{color:var(--color-text-secondary)}.member-operation-list .text-danger .icon{color:var(--color-danger)}@media(min-width:1280px){.members-view{transition:margin-right .24s cubic-bezier(.2,.8,.2,1)}.members-view.has-detail{margin-right:var(--member-detail-width)}}@container member-main (max-width:790px){.member-tools{gap:6px}.member-tools .btn{padding:0 10px;font-size:12px}.member-data-panel :deep(.page-jump){display:none}}@media(prefers-reduced-motion:reduce){.members-view{transition:none}}
.recycle-stack,.restore-stack{display:flex;flex-direction:column;gap:14px}.recycle-hint,.restore-stack>p{color:var(--color-text-secondary);font-size:12px;line-height:1.65}.recycle-list{display:flex;flex-direction:column;gap:8px}.recycle-row{display:flex;align-items:center;justify-content:space-between;gap:14px;padding:12px;border:1px solid var(--color-border);border-radius:var(--radius-sm)}.recycle-row strong,.recycle-row span{display:block}.recycle-row span{margin-top:4px;color:var(--color-text-muted);font-size:11px}.restore-stack label>span{display:block;margin-bottom:7px;font-size:12px;font-weight:600}.restore-reason{width:100%;min-height:110px;padding:10px;resize:vertical}@media(max-width:640px){.recycle-row{align-items:stretch;flex-direction:column}}
</style>
