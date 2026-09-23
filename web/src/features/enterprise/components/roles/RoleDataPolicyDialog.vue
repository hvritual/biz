<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { UiButton, UiOption, UiSelect } from '@/ui/base'
import UiDialog from '@/ui/common/UiDialog.vue'
import StatusBadge from '@/ui/common/StatusBadge.vue'
import AppIcon from '@/ui/common/AppIcon.vue'
import { useEnterpriseStore } from '@/stores/enterprise'
import { useUiStore } from '@/stores/ui'
import type { Role } from '@/types/enterprise'
import { CommercialApiError } from '@/services/commercial/platformCommercial'
import {
  getEnterpriseRole,
  listEnterpriseDataPolicies,
  readEnterpriseRoleSession,
  roleRequestId,
  sameEnterpriseRoleSession,
  setEnterpriseRoleDataPolicy,
  type EnterpriseDataPolicy,
  type EnterpriseTenantRole,
} from '@/services/enterprise/roleRuntime'

const props=defineProps<{open:boolean;role:Role|null;canManage?:boolean}>()
const emit=defineEmits<{close:[];saved:[]}>()
const store=useEnterpriseStore(),ui=useUiStore()
const current=ref<EnterpriseTenantRole|null>(null),policies=ref<EnterpriseDataPolicy[]>([]),selected=ref(''),busy=ref(false),loading=ref(false),error=ref(''),pendingVerification=ref(false),expectedPolicyId=ref(''),mutation=ref<{signature:string;key:string}|null>(null)
const referenced=computed(()=>current.value?.dataPolicy)
const selectedPolicy=computed(()=>policies.value.find(policy=>policy.id===selected.value)??null)
const selectable=(policy:EnterpriseDataPolicy)=>policy.effective&&policy.status==='active'
const invalidLabel=(policy:EnterpriseDataPolicy)=>policy.invalidReason||policy.status||'不可用'

async function trustedSession(){
  const expected=store.session
  if(!expected?.authenticated||!expected.active_tenant_id)throw new Error('请先登录并选择可访问企业。')
  const fresh=await readEnterpriseRoleSession()
  if(!sameEnterpriseRoleSession(expected,fresh))throw new Error('会话或当前企业已变化，请关闭后重新打开。')
  return fresh
}
async function load(){
  if(!props.open||!props.role||store.previewMode)return
  loading.value=true;error.value=''
  try{
    const session=await trustedSession()
    const [role,catalog]=await Promise.all([getEnterpriseRole(session,props.role.id),listEnterpriseDataPolicies(session)])
    current.value=role;policies.value=catalog;selected.value=role.dataPolicy?.policyId??'';pendingVerification.value=false;expectedPolicyId.value='';mutation.value=null
  }catch(cause){error.value=cause instanceof Error?cause.message:'数据策略读取失败，请重试。'}finally{loading.value=false}
}
function close(){if(!busy.value)emit('close')}
function selectPolicy(value:string){selected.value=value;mutation.value=null;pendingVerification.value=false;expectedPolicyId.value='';error.value=''}
async function confirmReadback(expected:string){
  const session=await trustedSession()
  const verified=await getEnterpriseRole(session,props.role!.id)
  const actual=verified.dataPolicy?.policyId??''
  if(actual!==expected)throw new Error('服务端回读与提交目标不一致，请刷新确认最终状态。')
  current.value=verified;selected.value=actual;pendingVerification.value=false;expectedPolicyId.value='';mutation.value=null
  await store.queryRoles({query:'',status:''})
  ui.toast('角色数据策略已保存并确认。','success')
  emit('saved');emit('close')
}
async function retryReadback(){
  if(!pendingVerification.value)return
  busy.value=true;error.value=''
  try{await confirmReadback(expectedPolicyId.value)}catch(cause){error.value=cause instanceof Error?cause.message:'仍无法确认服务端状态，请稍后重试。'}finally{busy.value=false}
}
async function save(){
  if(!props.canManage||!current.value||!props.role)return
  if(pendingVerification.value){await retryReadback();return}
  const policy=selectedPolicy.value
  if(selected.value&&(!policy||!selectable(policy))){error.value='所选策略当前不可用，请重新选择。';return}
  const expected=selected.value
  if((current.value.dataPolicy?.policyId??'')===expected){emit('close');return}
  const signature=[props.role.id,String(current.value.version),expected,String(policy?.version??0)].join(':')
  if(!mutation.value||mutation.value.signature!==signature)mutation.value={signature,key:roleRequestId('data-policy')}
  busy.value=true;error.value=''
  try{
    const session=await trustedSession()
    await setEnterpriseRoleDataPolicy(session,current.value,policy,mutation.value.key)
    expectedPolicyId.value=expected
    try{await confirmReadback(expected)}
    catch(readbackError){pendingVerification.value=true;error.value='策略绑定已提交，但暂时无法确认最终状态。请重新读取服务端状态。';console.warn(readbackError)}
  }catch(cause){
    if(cause instanceof CommercialApiError&&cause.code==='conflict'){
      try{await load();error.value='角色或策略版本已发生变化，已刷新服务端状态，请重新选择后保存。'}catch{error.value='角色或策略版本已发生变化，请刷新后重试。'}
    }else error.value=cause instanceof Error?cause.message:'数据策略保存失败。'
  }finally{busy.value=false}
}
watch(()=>[props.open,props.role?.id,store.tenantId] as const,()=>{if(props.open)void load()},{immediate:true})
</script>
<template>
  <UiDialog :open="open" :title="role ? `${role.name} · 数据策略` : '数据策略'" width="620px" @close="close">
    <div class="policy-dialog-stack" data-role-data-policy-dialog>
      <div class="notice-box"><AppIcon name="database" :size="16"/>数据策略是角色数据范围之外的附加约束，不会新增功能权限，也不会把 ALL / SELF / 指定点位放大。</div>
      <div v-if="referenced" class="policy-current">
        <div class="row-between"><strong>当前引用</strong><StatusBadge :text="referenced.effective?'有效':'已失效'" :tone="referenced.effective?'success':'danger'"/></div>
        <b>{{ referenced.policyName||referenced.policyId }}</b>
        <small>{{ referenced.policyId }} · 当前版本 v{{ referenced.policyVersion }} · 绑定版本 v{{ referenced.acceptedVersion }}</small>
        <p v-if="!referenced.effective" class="form-error" role="alert">当前引用不可用：{{ referenced.invalidReason||'策略状态无效' }}。运行时会按拒绝处理。</p>
      </div>
      <div v-else class="policy-current"><strong>当前引用</strong><p class="muted">未绑定 Data Policy。</p></div>
      <label class="field">
        <span>Data Policy 约束</span>
        <UiSelect :model-value="selected" :disabled="loading||busy||!canManage||pendingVerification" aria-label="Data Policy 约束" @update:model-value="selectPolicy(String($event))">
          <UiOption value="">不绑定附加策略</UiOption>
          <UiOption v-for="policy in policies" :key="policy.id" :value="policy.id" :disabled="!selectable(policy)">
            {{ policy.name }} · v{{ policy.version }}{{ selectable(policy)?'':` · ${invalidLabel(policy)}` }}
          </UiOption>
        </UiSelect>
      </label>
      <div v-if="selectedPolicy" class="policy-preview">
        <strong>{{ selectedPolicy.name }}</strong>
        <span>{{ selectedPolicy.id }} · v{{ selectedPolicy.version }}</span>
        <span>点位 {{ selectedPolicy.siteIds.length }} 个</span>
        <span v-if="selectedPolicy.expiresAt">有效期至 {{ selectedPolicy.expiresAt }}</span>
      </div>
      <p v-if="loading" class="muted">正在读取当前企业策略…</p>
      <p v-if="error" class="form-error" role="alert">{{ error }}</p>
    </div>
    <template #footer>
      <UiButton class="btn" :disabled="busy" @click="close">关闭</UiButton>
      <UiButton v-if="pendingVerification" class="btn btn-primary" :disabled="busy" @click="retryReadback">{{ busy?'正在读取…':'重新读取服务端状态' }}</UiButton>
      <UiButton v-else-if="canManage" class="btn btn-primary" :disabled="busy||loading" @click="save">{{ busy?'正在保存…':'保存策略引用' }}</UiButton>
    </template>
  </UiDialog>
</template>
<style scoped>
.policy-dialog-stack{display:grid;gap:16px}.policy-current,.policy-preview{display:grid;gap:6px;padding:14px;border:1px solid var(--color-border);border-radius:var(--radius-md);background:var(--color-surface-soft)}.policy-current b{font-size:13px}.policy-current small,.policy-preview span{color:var(--color-text-muted);font-size:11px}.field{display:grid;gap:7px}.field>span{font-size:12px;font-weight:600}.policy-preview{grid-template-columns:minmax(0,1fr) auto}.policy-preview strong{grid-column:1/-1}@media(max-width:640px){.policy-preview{grid-template-columns:1fr}}
</style>
