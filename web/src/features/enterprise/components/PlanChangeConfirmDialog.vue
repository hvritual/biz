<script setup lang="ts">
import UiDialog from '@/ui/common/UiDialog.vue'
import { UiButton } from '@/ui/base'
import type { SubscriptionChangePreviewDTO } from '@/services/commercial/platformCommercial'
defineProps<{ open: boolean, preview: SubscriptionChangePreviewDTO, pending: boolean }>()
const emit = defineEmits<{ close: [], confirm: [] }>()
</script>
<template><UiDialog :open="open" title="确认套餐变更" @close="emit('close')"><div data-plan-change-confirm-dialog><p>请核对本次套餐与生效时间；确认后将按预览方案处理。</p><strong>{{ preview.target?.name || '目标套餐' }}</strong></div><template #footer><UiButton class="btn" :disabled="pending" @click="emit('close')">返回修改</UiButton><UiButton class="btn primary" :disabled="pending" data-plan-change-confirm-dialog-submit @click="emit('confirm')">{{ pending ? '正在确认…' : '确认开通' }}</UiButton></template></UiDialog></template>
