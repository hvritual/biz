<script setup lang="ts">
import AppIcon from "./AppIcon.vue";
import type { IconName } from "../lib/icons";
withDefaults(
  defineProps<{
    title: string;
    value: string | number;
    icon: IconName;
    tone?: string;
    change?: string;
    note?: string;
    unit?: string;
  }>(),
  { tone: "blue", change: "", note: "", unit: "" },
);
</script>
<template>
  <article
    :class="['metric-card', { 'metric-card--long-value': String(value).length > 7 }]"
  >
    <span :class="['metric-icon', `tone-${tone}`]"
      ><AppIcon :name="icon" :size="27"
    /></span>
    <div class="metric-copy">
      <div class="metric-label">{{ title }}</div>
      <div class="metric-value-line">
        <strong>{{ value }}</strong
        ><span v-if="unit" class="metric-unit">{{ unit }}</span
        ><span v-if="change" class="metric-change"
          ><AppIcon name="ArrowUp" :size="13" />{{ change }}</span
        >
      </div>
      <div v-if="note" class="metric-note">{{ note }}</div>
    </div>
  </article>
</template>
