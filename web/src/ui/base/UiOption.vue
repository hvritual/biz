<script setup lang="ts">
import { Check } from 'lucide-vue-next'
import { computed } from 'vue'
import { SelectItem, SelectItemIndicator, SelectItemText } from 'reka-ui'
const props = withDefaults(defineProps<{ value?: string | number; disabled?: boolean }>(), { value: '', disabled: false })
const normalized = computed(() => props.value === '' ? '__coffeelink_empty__' : String(props.value))
</script>
<template>
  <SelectItem :value="normalized" :disabled="disabled" data-slot="select-item" class="relative flex min-h-9 cursor-default select-none items-center rounded-[var(--radius-sm)] py-2 pl-8 pr-3 text-[var(--text-sm)] outline-none data-[highlighted]:bg-accent data-[highlighted]:text-accent-foreground data-[disabled]:pointer-events-none data-[disabled]:opacity-50">
    <span class="absolute left-2.5 flex size-4 items-center justify-center"><SelectItemIndicator><Check class="size-4" /></SelectItemIndicator></span>
    <SelectItemText><slot /></SelectItemText>
  </SelectItem>
</template>
