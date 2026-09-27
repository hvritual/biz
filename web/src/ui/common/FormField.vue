<script setup lang="ts">
import { computed, useSlots } from 'vue'
import { useId } from 'vue'

const props = withDefaults(
  defineProps<{
    label?: string
    error?: string
    help?: string
    required?: boolean
    htmlFor?: string
  }>(),
  { label: '', error: '', help: '', required: false, htmlFor: '' },
)
const slots = useSlots()
const autoId = useId()
const fieldId = computed(() => `field-${autoId}`)
const errorId = computed(() => `${fieldId.value}-error`)
const helpId = computed(() => `${fieldId.value}-help`)
const hasLabel = computed(() => Boolean(slots.label || props.label))
</script>
<template>
  <div class="form-field" :data-error="Boolean(error) || undefined">
    <label
      v-if="hasLabel"
      class="form-field-label"
      :for="htmlFor || undefined"
    >
      <slot name="label">{{ label }}</slot>
      <span v-if="required" class="form-field-required" aria-hidden="true">*</span>
    </label>
    <div class="form-field-control">
      <slot :field-id="fieldId" :error-id="errorId" :help-id="helpId" />
    </div>
    <p v-if="error" :id="errorId" class="form-field-error" role="alert">{{ error }}</p>
    <p v-else-if="help" :id="helpId" class="form-field-help">{{ help }}</p>
  </div>
</template>
<style scoped>
.form-field {
  display: flex;
  flex-direction: column;
  gap: var(--space-2);
  min-width: 0;
}
.form-field-label {
  display: inline-flex;
  align-items: center;
  gap: var(--space-1);
  font-size: var(--text-sm);
  font-weight: 500;
  color: var(--color-text);
}
.form-field-required {
  color: var(--color-danger);
}
.form-field-control {
  min-width: 0;
}
.form-field-error {
  font-size: var(--text-xs);
  color: var(--color-danger);
  line-height: 1.5;
}
.form-field-help {
  font-size: var(--text-xs);
  color: var(--color-text-muted);
  line-height: 1.5;
}
</style>
