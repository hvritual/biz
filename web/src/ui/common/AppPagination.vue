<script setup lang="ts">
import { UiButton, UiInput, UiOption, UiSelect } from '@/ui/base'
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import { formatNumber } from '@/i18n'
import AppIcon from './AppIcon.vue'
const props = defineProps<{ total: number; page: number; pageSize: number }>()
const emit = defineEmits<{ 'update:page': [value: number]; 'update:pageSize': [value: number] }>()
const { t } = useI18n()
const pages = computed(() => Math.max(1, Math.ceil(props.total / props.pageSize)))
const visible = computed(() => { const start=Math.max(1,Math.min(props.page-2,pages.value-4)); return Array.from({length:Math.min(5,pages.value)},(_,i)=>start+i) })
function change(value:number){emit('update:page',Math.max(1,Math.min(pages.value,value)))}
</script>
<template>
  <footer class="pagination">
    <span class="muted">{{ t('common.totalItems',{count:formatNumber(total)}) }}</span>
    <div class="pagination-controls">
      <UiButton class="icon-button" :aria-label="t('common.previousPage')" :disabled="page<=1" @click="change(page-1)"><AppIcon name="left" :size="16"/></UiButton>
      <UiButton v-for="n in visible" :key="n" :class="['page-number',{active:page===n}]" :aria-label="t('common.pageNumber',{page:n})" :aria-current="page===n?'page':undefined" @click="change(n)">{{ n }}</UiButton>
      <span v-if="visible.at(-1)!==pages" class="muted">…</span><UiButton v-if="visible.at(-1)!==pages" class="page-number" @click="change(pages)">{{ pages }}</UiButton>
      <UiButton class="icon-button" :aria-label="t('common.nextPage')" :disabled="page>=pages" @click="change(page+1)"><AppIcon name="right" :size="16"/></UiButton>
      <UiSelect class="select page-size" :value="pageSize" :aria-label="t('common.pageSize',{count:pageSize})" @change="($event:Event)=>{emit('update:pageSize',Number(($event.target as HTMLSelectElement).value));change(1)}">
        <UiOption :value="10">{{ t('common.pageSize',{count:10}) }}</UiOption><UiOption :value="20">{{ t('common.pageSize',{count:20}) }}</UiOption><UiOption :value="50">{{ t('common.pageSize',{count:50}) }}</UiOption>
      </UiSelect>
      <span class="page-jump muted">{{ t('common.goTo') }}<UiInput class="input" type="number" :value="page" min="1" :max="pages" :aria-label="t('common.pageNumber',{page})" @change="change(Number(($event.target as HTMLInputElement).value)||1)"/>{{ t('common.page') }}</span>
    </div>
  </footer>
</template>
<style scoped>
.pagination{display:flex;align-items:center;justify-content:space-between;gap:14px;padding:22px 2px 4px;font-size:var(--text-sm);flex-wrap:wrap}.pagination-controls{display:flex;gap:7px;align-items:center}.page-number{width:32px;height:32px;border-radius:6px;font-size:var(--text-sm)}.page-number.active{color:var(--color-primary);background:var(--color-primary-soft);font-weight:600}.page-number:hover{background:var(--color-surface-soft)}.page-size{width:118px;margin-left:14px}.page-jump{display:flex;gap:9px;align-items:center}.page-jump .input{width:52px;text-align:center;padding:0 3px}@media(max-width:900px){.page-jump{display:none}}@media(max-width:600px){.pagination-controls{gap:2px;flex-wrap:wrap;max-width:100%}.page-size{margin-left:3px;width:110px}.page-number{width:28px}.pagination>span{width:100%}}
</style>
