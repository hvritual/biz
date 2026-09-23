<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { UiButton, UiInput } from '@/ui/base'
import UiDialog from '@/ui/common/UiDialog.vue'
import AppIcon from '@/ui/common/AppIcon.vue'
import { useEnterpriseStore } from '@/stores/enterprise'
import { useUiStore } from '@/stores/ui'
import type { Member } from '@/types/enterprise'
import { CommercialApiError } from '@/services/commercial/platformCommercial'
import {
  getEnterpriseMemberBusinessScope,
  listEnterpriseMemberScopeCandidates,
  memberRequestId,
  readEnterpriseMemberSession,
  sameTrustedSession,
  setEnterpriseMemberBusinessScope,
  type EnterpriseMemberBusinessScope,
  type EnterpriseMemberScopeCandidate,
} from '@/services/enterprise/memberRuntime'

const props=defineProps<{member:Member;canRead?:boolean;canEdit?:boolean}>()
const store=useEnterpriseStore(),ui=useUiStore()
const scope=ref<EnterpriseMemberBusinessScope|null>(null),candidates=ref<EnterpriseMemberScopeCandidate[]>([]),selected=ref<string[]>([]),loading=ref(false),editorOpen=ref(false),busy=ref(false),error=ref(''),candidateError=ref(''),pendingVerification=ref(false),expectedIds=ref<string[]>([]),mutation=ref<{signature:string;key:string}|null>(null)
const candidateIds=computed(()=>new Set(candidates.value.map(candidate=>candidate.id)))
const withdrawnIds=computed(()=>selected.value.filter(id=>!candidateIds.value.has(id)))
const unavailableSelected=computed(()=>candidates.value.filter(candidate=>!candidate.assignable&&selected.value.includes(candidate.id)).map(candidate=>candidate.id))
const saveBlocked=computed(()=>withdrawnIds.value.length>0||unavailableSelected.value.length>0)
const sorted=(values:string[])=>[...new Set(values)].sort()
const sameIds=(a:string[],b:string[])=>JSON.stringify(sorted(a))===JSON.stringify(sorted(b))

async function trustedSession(){
  const expected=store.session
  if(!expected?.authenticated||!expected.active_tenant_id)throw new Error('请先登录并选择可访问企业。')
  const current=await readEnterpriseMemberSession()
  if(!sameTrustedSession(expected,current))throw new Error('会话或当前企业已变化，请关闭后重新打开。')
  return current
}
async function loadScope(){
  if(store.previewMode||!props.canRead)return
  loading.value=true;error.value=''
  try{
    const session=await trustedSession()
    const value=await getEnterpriseMemberBusinessScope(session,props.member.id)
    if(value.tenantId!==store.tenantId)throw new Error('服务端返回的成员范围不属于当前企业。')
    scope.value=value
    if(!editorOpen.value)selected.value=[...value.siteIds]
  }catch(cause){scope.value=null;error.value=cause instanceof Error?cause.message:'成员显式范围读取失败。'}finally{loading.value=false}
}
async function loadCandidates(){
  candidateError.value=''
  try{
    const session=await trustedSession()
    const result=await listEnterpriseMemberScopeCandidates(session,{page:1,pageSize:100})
    candidates.value=result.candidates
  }catch(cause){candidates.value=[];candidateError.value=cause instanceof Error?cause.message:'可分配点位读取失败，请重试。'}
}
async function openEditor(){
  if(!props.canEdit||store.previewMode)return
  editorOpen.value=true;pendingVerification.value=false;expectedIds.value=[];mutation.value=null;error.value=''
  await loadScope()
  selected.value=[...(scope.value?.siteIds??[])]
  await loadCandidates()
}
function toggle(id:string,checked:boolean){
  selected.value=checked?sorted([...selected.value,id]):selected.value.filter(value=>value!==id)
  mutation.value=null;pendingVerification.value=false;error.value=''
}
function removeWithdrawn(id:string){selected.value=selected.value.filter(value=>value!==id);mutation.value=null}
async function confirmReadback(expected:string[]){
  const session=await trustedSession()
  const verified=await getEnterpriseMemberBusinessScope(session,props.member.id)
  if(!sameIds(verified.siteIds,expected))throw new Error('服务端回读与提交范围不一致，请刷新确认最终状态。')
  scope.value=verified;selected.value=[...verified.siteIds];pendingVerification.value=false;expectedIds.value=[];mutation.value=null
  await store.refresh(['members'])
  ui.toast('成员数据权限已保存并确认。','success')
  editorOpen.value=false
}
async function retryReadback(){
  if(!pendingVerification.value)return
  busy.value=true;error.value=''
  try{await confirmReadback(expectedIds.value)}catch(cause){error.value=cause instanceof Error?cause.message:'仍无法确认服务端状态，请稍后重试。'}finally{busy.value=false}
}
async function save(){
  if(!scope.value||saveBlocked.value)return
  if(pendingVerification.value){await retryReadback();return}
  const expected=sorted(selected.value)
  if(sameIds(scope.value.siteIds,expected)){editorOpen.value=false;return}
  const signature=[props.member.id,String(scope.value.version),expected.join(',')].join(':')
  if(!mutation.value||mutation.value.signature!==signature)mutation.value={signature,key:memberRequestId('scope')}
  busy.value=true;error.value=''
  try{
    const session=await trustedSession()
    await setEnterpriseMemberBusinessScope(session,scope.value,expected,mutation.value.key)
    expectedIds.value=expected
    try{await confirmReadback(expected)}
    catch(readbackError){pendingVerification.value=true;error.value='范围变更已提交，但暂时无法确认最终状态。请重新读取服务端状态。';console.warn(readbackError)}
  }catch(cause){
    if(cause instanceof CommercialApiError&&cause.code==='conflict'){
      await loadScope();selected.value=[...(scope.value?.siteIds??[])];error.value='成员版本已发生变化，已刷新权威范围，请重新选择后保存。';mutation.value=null
    }else error.value=cause instanceof Error?cause.message:'成员数据权限保存失败。'
  }finally{busy.value=false}
}
watch(()=>[props.member.id,props.member.runtimeVersion,store.tenantId] as const,()=>void loadScope(),{immediate:true})
</script>
<template>
  <div class="scope-panel" data-member-business-scope>
    <template v-if="store.previewMode">
      <div class="scope-summary"><AppIcon name="database" :size="24"/><div><strong>预览数据范围</strong><p>当前仅为演示数据，不代表服务端授权事实。</p></div></div>
    </template>
    <template v-else-if="!canRead">
      <div class="notice-box">当前账号无权读取成员显式数据范围。</div>
    </template>
    <template v-else>
      <div class="row-between"><div><h3>成员显式点位范围</h3><p class="secondary">来源：Access 成员显式范围；角色策略与权限范围会在查询时继续取交集。</p></div><UiButton v-if="canEdit" class="btn-link" @click="openEditor">调整范围</UiButton></div>
      <div class="scope-summary"><AppIcon name="database" :size="24"/><div><strong>{{ scope?.siteIds.length ? `已授权 ${scope.siteIds.length} 个点位` : '未分配点位' }}</strong><p>成员范围版本 v{{ scope?.version??'-' }}</p></div></div>
      <div v-if="scope?.siteIds.length" class="scope-tags"><span v-for="id in scope.siteIds" :key="id">{{ candidates.find(candidate=>candidate.id===id)?.name||id }}</span></div>
      <p class="secondary">角色授权摘要：{{ member.scope }}（展示摘要，不作为授权事实）。</p>
      <p v-if="loading" class="muted">正在读取权威范围…</p>
      <p v-if="error&&!editorOpen" class="form-error" role="alert">{{ error }}</p>
    </template>
    <UiDialog :open="editorOpen" :title="`${member.name} · 数据权限`" width="680px" @close="()=>{if(!busy)editorOpen=false}">
      <div class="scope-editor-stack">
        <div class="notice-box">只能选择当前企业仍可分配的点位。空范围表示不允许访问点位数据；部门关系不会自动扩大范围。</div>
        <div v-if="withdrawnIds.length" class="warning-box">
          <strong>当前范围包含已不可分配对象</strong>
          <div v-for="id in withdrawnIds" :key="id" class="withdrawn-row"><span>{{ id }}</span><UiButton class="btn-link" @click="removeWithdrawn(id)">移除</UiButton></div>
        </div>
        <p v-if="candidateError" class="form-error" role="alert">{{ candidateError }} <UiButton class="btn-link" @click="loadCandidates">重新读取</UiButton></p>
        <div class="candidate-list" aria-label="可分配点位">
          <label v-for="candidate in candidates" :key="candidate.id" class="candidate-row" :class="{unavailable:!candidate.assignable}">
            <UiInput type="checkbox" :checked="selected.includes(candidate.id)" :disabled="!candidate.assignable||busy||pendingVerification" @change="toggle(candidate.id,($event.target as HTMLInputElement).checked)"/>
            <span><strong>{{ candidate.name }}</strong><small>{{ candidate.id }} · {{ candidate.assignable?'可分配':candidate.unavailableReason||'不可分配' }}</small></span>
          </label>
          <p v-if="!candidates.length&&!candidateError" class="muted">当前没有可分配点位。</p>
        </div>
        <p v-if="saveBlocked" class="form-error" role="alert">请先移除已不可分配的对象，再保存范围。</p>
        <p v-if="error" class="form-error" role="alert">{{ error }}</p>
      </div>
      <template #footer>
        <UiButton class="btn" :disabled="busy" @click="editorOpen=false">取消</UiButton>
        <UiButton v-if="pendingVerification" class="btn btn-primary" :disabled="busy" @click="retryReadback">{{ busy?'正在读取…':'重新读取服务端状态' }}</UiButton>
        <UiButton v-else class="btn btn-primary" :disabled="busy||saveBlocked||Boolean(candidateError)" @click="save">{{ busy?'正在保存…':'保存数据权限' }}</UiButton>
      </template>
    </UiDialog>
  </div>
</template>
<style scoped>
.scope-panel,.scope-editor-stack{display:grid;gap:14px}.scope-summary{padding:18px 14px;background:var(--color-success-soft);color:var(--color-success);border-radius:var(--radius-md);display:flex;gap:12px;align-items:center}.scope-summary p{margin-top:3px;font-size:11px}.scope-tags{display:flex;flex-wrap:wrap;gap:6px}.scope-tags span{max-width:100%;overflow:hidden;padding:5px 8px;border:1px solid var(--color-border);border-radius:999px;color:var(--color-text-secondary);font-size:11px;text-overflow:ellipsis;white-space:nowrap}.candidate-list{display:grid;gap:8px;max-height:360px;overflow:auto}.candidate-row{display:flex;align-items:flex-start;gap:10px;padding:11px;border:1px solid var(--color-border);border-radius:var(--radius-sm)}.candidate-row>span{min-width:0}.candidate-row strong,.candidate-row small{display:block}.candidate-row small{margin-top:3px;color:var(--color-text-muted);font-size:10px}.candidate-row.unavailable{opacity:.62}.warning-box{padding:12px;border:1px solid var(--color-warning);border-radius:var(--radius-sm);background:var(--color-warning-soft)}.withdrawn-row{display:flex;align-items:center;justify-content:space-between;gap:10px;margin-top:6px;font-size:11px}
</style>
