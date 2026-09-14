<script setup lang="ts">
import { UiInput, UiOption, UiSelect, UiTextarea } from '@/ui/base'

import type { FormField, FormValues } from '@/types/customer'
const props = defineProps<{ fields: FormField[]; modelValue: FormValues; labels?: Record<string, string> }>()
const emit = defineEmits<{ 'update:modelValue': [FormValues] }>()
function update(key: string, value: string | boolean) {
  emit('update:modelValue', { ...props.modelValue, [key]: value })
}
</script>
<template>
  <div class="form-grid">
    <label
      v-for="field in fields"
      :key="field.key"
      :data-field="field.key"
      class="field"
      :class="{ 'full-width': field.full }"
      ><template v-if="field.type === 'checkbox'"
        ><span class="check-field"
          ><UiInput
            :checked="Boolean(modelValue[field.key])"
            type="checkbox"
            :required="field.required"
            @change="update(field.key, ($event.target as HTMLInputElement).checked)"
          />{{ field.label }}</span
        ></template
      ><template v-else
        ><span :class="{ required: field.required }">{{ field.label }}</span
        ><UiSelect
          v-if="field.type === 'select'"
          :value="String(modelValue[field.key] ?? '')"
          class="select"
          :required="field.required"
          :disabled="field.readonly"
          @change="update(field.key, ($event.target as HTMLSelectElement).value)"
        >
          <UiOption value="">请选择</UiOption>
          <UiOption v-for="option in field.options" :key="option" :value="option">
            {{ labels?.[option] || option || '不限' }}
          </UiOption></UiSelect><UiTextarea
          v-else-if="field.type === 'textarea'"
          :value="String(modelValue[field.key] ?? '')"
          class="textarea"
          :required="field.required"
          :readonly="field.readonly"
          maxlength="1000"
          @input="update(field.key, ($event.target as HTMLTextAreaElement).value)" /><UiInput
          v-else
          :value="String(modelValue[field.key] ?? '')"
          class="input"
          :type="field.type || 'text'"
          :required="field.required"
          :readonly="field.readonly"
          :min="field.type === 'number' ? 1 : undefined"
          maxlength="200"
          @input="update(field.key, ($event.target as HTMLInputElement).value)" /></template
      ><span v-if="field.help" class="form-help">{{ field.help }}</span></label
    >
  </div>
</template>
