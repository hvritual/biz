<script setup lang="ts">
import AppIcon from "./AppIcon.vue";
import type { IconName } from "../lib/icons";
withDefaults(
  defineProps<{
    icon?: IconName;
    variant?: "primary" | "secondary" | "quiet" | "danger";
    type?: "button" | "submit" | "reset";
    disabled?: boolean;
  }>(),
  { variant: "secondary", type: "button", disabled: false },
);
</script>
<template>
  <button :type="type" :class="['button', `button--${variant}`]" :disabled="disabled">
    <AppIcon v-if="icon" :name="icon" :size="16" /><slot />
  </button>
</template>
