<script setup lang="ts">
import { computed } from 'vue'
import { backendTermLabel } from '@/i18n/backend-terms'
import { UiButton } from '@/ui/base'
import type { SubscriptionChangeReceiptDTO } from '@/services/commercial/platformCommercial'
const props = defineProps<{ receipt: SubscriptionChangeReceiptDTO, refreshing: boolean }>()
const emit = defineEmits<{ refresh: [], reset: [] }>()
const refreshing = computed(() => ['SCHEDULED','PROVISIONING'].includes(props.receipt.status))
</script>
<template><section data-plan-change-receipt><h3>{{ backendTermLabel('receiptStatus', receipt.status) }}</h3><p>{{ receipt.changeId }}</p><p>现有租户数据将被保留，本次操作不会删除资源。</p><UiButton v-if="refreshing" class="btn" :disabled="props.refreshing" data-plan-change-receipt-refresh @click="emit('refresh')">刷新处理结果</UiButton><UiButton class="btn" @click="emit('reset')">发起其他变更</UiButton></section></template>
