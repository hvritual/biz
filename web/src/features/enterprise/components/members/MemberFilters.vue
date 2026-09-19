<script setup lang="ts">
import { UiButton, UiOption, UiSelect } from '@/ui/base'
import { computed, reactive, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { useEnterpriseStore } from '@/stores/enterprise'
import SearchField from '@/ui/common/SearchField.vue'
export interface MemberFilterValue { query:string; department:string; role:string; status:string }
const props=defineProps<{value:MemberFilterValue}>(),emit=defineEmits<{apply:[value:MemberFilterValue];reset:[]}>(),store=useEnterpriseStore(),{t}=useI18n(),draft=reactive({...props.value})
watch(()=>props.value,value=>Object.assign(draft,value),{deep:true})
const statuses=computed(()=>store.previewMode?(['active','invited','suspended','removed'] as const):(['active','invited','suspended'] as const))
</script>
<template>
  <form class="card member-filters" :aria-label="t('members.searchAria')" @submit.prevent="emit('apply',{...draft})">
    <SearchField v-model="draft.query" :label="t('members.searchLabel')" :placeholder="t('members.searchPlaceholder')"/>
    <label class="filter-item"><span>{{ t('members.department') }}</span><UiSelect v-model="draft.department" class="select" :aria-label="t('members.filterDepartment')"><UiOption value="">{{ t('members.allDepartments') }}</UiOption><UiOption v-for="item in store.departments" :key="item.id" :value="item.id">{{ item.name }}</UiOption></UiSelect></label>
    <label class="filter-item"><span>{{ t('members.role') }}</span><UiSelect v-model="draft.role" class="select" :aria-label="t('members.filterRole')"><UiOption value="">{{ t('members.allRoles') }}</UiOption><UiOption v-for="item in store.roles" :key="item.id" :value="item.id">{{ item.name }}</UiOption></UiSelect></label>
    <label class="filter-item"><span>{{ t('members.status') }}</span><UiSelect v-model="draft.status" class="select" :aria-label="t('members.filterStatus')"><UiOption value="">{{ t('members.allStatuses') }}</UiOption><UiOption v-for="key in statuses" :key="key" :value="key">{{ t(`members.statuses.${key}`) }}</UiOption></UiSelect></label>
    <div class="filter-buttons"><UiButton class="btn" type="button" @click="emit('reset')">{{ t('common.reset') }}</UiButton><UiButton class="btn btn-primary" type="submit">{{ t('common.query') }}</UiButton></div>
  </form>
</template>
<style scoped>
.member-filters{display:flex;align-items:center;gap:14px;flex-wrap:wrap;padding:15px 16px;border-radius:var(--radius-md)}.member-filters .search-field{flex:1 1 220px;min-width:190px}.filter-item{display:flex;align-items:center;gap:9px;font-size:12px}.filter-item>span{white-space:nowrap}.filter-item .select{width:120px;font-size:12px;padding-left:10px}.filter-buttons{display:flex;gap:8px}.filter-buttons .btn{min-width:62px}@container member-main (max-width:1010px){.member-filters{gap:10px;padding:14px}.member-filters .search-field{flex-basis:190px}.filter-item{gap:7px}.filter-item .select{width:112px}.filter-buttons .btn{min-width:55px;padding:0 12px}}@container member-main (max-width:790px){.member-filters .search-field{flex-basis:calc(100% - 140px)}.filter-buttons{margin-left:auto}.filter-item{flex:1}.filter-item .select{flex:1}}@media(max-width:767px){.member-filters .search-field{flex-basis:100%}.filter-item{flex:1 1 150px}}
</style>
