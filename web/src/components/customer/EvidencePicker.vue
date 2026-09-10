<script setup lang="ts">
import { computed } from 'vue'
import type { SourceRecord } from '@/types/customer'
import StatusBadge from '@/components/ui/StatusBadge.vue'
const props = defineProps<{ sources: SourceRecord[]; modelValue: string }>()
const emit = defineEmits<{ 'update:modelValue': [string] }>()
const selected = computed(() => props.modelValue.split(',').filter(Boolean))
function toggle(id: string, on: boolean) {
  emit(
    'update:modelValue',
    (on ? [...new Set([...selected.value, id])] : selected.value.filter((x) => x !== id)).join(','),
  )
}
</script>
<template>
  <div>
    <p class="field-title">关联已核验业务来源 <span class="text-danger">*</span></p>
    <div class="evidence-list">
      <label v-for="source in sources" :key="source.id" class="evidence-option"
        ><input
          type="checkbox"
          :checked="selected.includes(source.id)"
          :disabled="!source.verified"
          :aria-label="source.id"
          @change="toggle(source.id, ($event.target as HTMLInputElement).checked)" />
        <div class="flex-1">
          <strong>{{ source.id }} · {{ source.title }}</strong
          ><small>{{ source.verified ? '业务模块的本地示例投影' : '尚未核验，不可作为完成依据' }}</small>
        </div>
        <StatusBadge :text="source.state" :tone="source.verified ? 'primary' : 'neutral'"
      /></label>
    </div>
  </div>
</template>
