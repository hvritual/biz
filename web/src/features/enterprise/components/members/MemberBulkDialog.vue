<script setup lang="ts">
import { UiButton, UiInput, UiTextarea } from '@/ui/base'
import { computed, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { useEnterpriseStore } from '@/stores/enterprise'
import { useUiStore } from '@/stores/ui'
import { prepareMemberStatusBatch } from '@/services/memberPolicy'
import UiDialog from '@/ui/common/UiDialog.vue'
const props=defineProps<{open:boolean;action:'activate'|'suspend';targets:{id:string;version:number}[]}>(),emit=defineEmits<{close:[];saved:[]}>(),store=useEnterpriseStore(),ui=useUiStore(),{t}=useI18n(),reason=ref(''),confirmed=ref(false),error=ref('')
const label=computed(()=>t(`members.bulk.${props.action}`)); const policyError=computed(()=>{if(!props.open)return'';try{prepareMemberStatusBatch(store.members,store.roles,props.targets,props.action);return''}catch(e){return(e as Error).message}})
watch(()=>props.open,()=>{reason.value='';confirmed.value=false;error.value=''})
async function submit(){if(policyError.value)return;if(!reason.value.trim()){error.value=t('members.bulk.reasonRequired');return}if(!confirmed.value){error.value=t('members.bulk.confirmRequired');return}try{await store.changeStatuses(props.targets,props.action,reason.value.trim());ui.toast(store.previewMode?t('members.bulk.previewToast',{action:label.value,count:props.targets.length}):t('members.bulk.serverToast',{action:label.value,count:props.targets.length}));emit('saved')}catch(e){error.value=(e as Error).message}}
</script>
<template><UiDialog :open="open" :title="t('members.bulk.title',{action:label})" width="500px" @close="emit('close')"><div class="notice-box">{{ t('members.bulk.description',{action:label,count:targets.length}) }}</div><p class="selected-names">{{ targets.map(x=>store.members.find(m=>m.id===x.id)?.name).join(' · ') }}</p><form id="member-batch-form" class="page-stack" @submit.prevent="submit"><label class="field"><span class="required">{{ t('members.bulk.reason') }}</span><UiTextarea v-model="reason" class="textarea" :aria-label="t('members.bulk.reason')" maxlength="200" :placeholder="t('members.bulk.reasonPlaceholder')"/></label><label class="confirm-check"><UiInput v-model="confirmed" type="checkbox"/>{{ t('members.bulk.confirm') }}</label><p v-if="policyError||error" class="form-error" role="alert">{{ policyError||error }}</p></form><template #footer><UiButton class="btn" @click="emit('close')">{{ t('common.cancel') }}</UiButton><UiButton class="btn" :class="action==='suspend'?'btn-danger':'btn-primary'" :disabled="Boolean(policyError)" type="submit" form="member-batch-form">{{ t('members.bulk.confirmAction',{action:label}) }}</UiButton></template></UiDialog></template>
<style scoped>.selected-names{padding:16px 0;font-size:13px;color:var(--color-text-secondary)}.confirm-check{display:flex;align-items:center;gap:8px;font-size:12px}</style>
