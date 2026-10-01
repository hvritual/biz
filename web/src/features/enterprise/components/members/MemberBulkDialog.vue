<script setup lang="ts">
import { UiButton, UiInput, UiTextarea } from '@/ui/base'
import { computed, onBeforeUnmount, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { useEnterpriseStore } from '@/stores/enterprise'
import { useUiStore } from '@/stores/ui'
import { prepareMemberStatusBatch } from '@/services/memberPolicy'
import UiDialog from '@/ui/common/UiDialog.vue'
const props=defineProps<{open:boolean;action:'activate'|'suspend';targets:{id:string;version:number}[]}>()
const emit=defineEmits<{close:[];saved:[];inspect:[]}>(),store=useEnterpriseStore(),ui=useUiStore(),{t}=useI18n()
const reason=ref(''),confirmed=ref(false),error=ref(''),busy=ref(false),needsInspection=ref(false),names=ref<string[]>([])
let generation=0
const label=computed(()=>t(`members.bulk.${props.action}`))
const policyError=computed(()=>{if(!props.open||busy.value||needsInspection.value)return'';try{prepareMemberStatusBatch(store.members,store.roles,props.targets,props.action);return''}catch(e){return(e as Error).message}})
watch(()=>props.open,()=>{generation++;reason.value='';confirmed.value=false;error.value='';busy.value=false;needsInspection.value=false;names.value=props.targets.map(x=>store.members.find(m=>m.id===x.id)?.name||x.id)})
onBeforeUnmount(()=>{generation++})
function close(){if(!busy.value)emit('close')}
async function submit(){
  if(busy.value||needsInspection.value||policyError.value)return
  if(!reason.value.trim()){error.value=t('members.bulk.reasonRequired');return}
  if(!confirmed.value){error.value=t('members.bulk.confirmRequired');return}
  const token=++generation,tenant=store.tenantId,count=props.targets.length,actionLabel=label.value
  const current=()=>token===generation&&props.open&&store.tenantId===tenant
  busy.value=true;error.value=''
  try{
    await store.changeStatuses(props.targets.map(x=>({...x})),props.action,reason.value.trim())
    if(!current())return
    ui.toast(store.previewMode?t('members.bulk.previewToast',{action:actionLabel,count}):t('members.bulk.serverToast',{action:actionLabel,count}))
    emit('saved')
  }catch(e){
    if(!current())return
    error.value=e instanceof Error?e.message:t('members.task.batchFailure')
    needsInspection.value=true
  }finally{if(current())busy.value=false}
}
</script>
<template>
  <UiDialog :open="open" :title="t('members.bulk.title',{action:label})" width="500px" @close="close">
    <div class="notice-box">{{ t('members.bulk.description',{action:label,count:targets.length}) }}</div>
    <p class="selected-names">{{ names.join(' · ') }}</p>
    <p v-if="!store.previewMode" class="batch-guidance">{{ t('members.task.batchEffect') }}</p>
    <form id="member-batch-form" class="page-stack" @submit.prevent="submit">
      <label class="field"><span class="required">{{ t('members.bulk.reason') }}</span><UiTextarea v-model="reason" class="textarea" :disabled="busy" :aria-label="t('members.bulk.reason')" maxlength="200" :placeholder="t('members.bulk.reasonPlaceholder')"/></label>
      <label class="confirm-check"><UiInput v-model="confirmed" type="checkbox" :disabled="busy"/>{{ t('members.bulk.confirm') }}</label>
      <p v-if="busy" role="status" class="batch-guidance">{{ t('members.task.batchPending') }}</p>
      <div v-if="policyError||error" class="form-error" role="alert"><p>{{ policyError||error }}</p><p v-if="needsInspection" class="batch-guidance">{{ t('members.task.batchRecovery') }}</p></div>
    </form>
    <template #footer>
      <UiButton v-if="needsInspection" class="btn" @click="emit('inspect')">{{ t('members.task.inspect') }}</UiButton>
      <UiButton v-else class="btn" :disabled="busy" @click="close">{{ t('common.cancel') }}</UiButton>
      <UiButton class="btn" :class="action==='suspend'?'btn-danger':'btn-primary'" :disabled="busy||needsInspection||Boolean(policyError)" type="submit" form="member-batch-form">{{ busy?t('common.processing'):t('members.bulk.confirmAction',{action:label}) }}</UiButton>
    </template>
  </UiDialog>
</template>
<style scoped>
.selected-names{padding:16px 0;font-size:13px;color:var(--color-text-secondary);overflow-wrap:anywhere}.confirm-check{display:flex;align-items:center;gap:8px;font-size:13px}.batch-guidance{font-size:13px;line-height:1.6;margin-bottom:12px;overflow-wrap:anywhere}
</style>
