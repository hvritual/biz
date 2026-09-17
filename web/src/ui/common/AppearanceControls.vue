<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import { UiButton } from '@/ui/base'
import { useUiTheme } from '@/ui/base/theme'
import AppIcon from './AppIcon.vue'

const { t } = useI18n()
const theme = useUiTheme()
const dark = computed(() => theme.colorMode.value === 'dark')
const compact = computed(() => theme.density.value === 'compact')
const modeLabel = computed(() => t(dark.value ? 'appearance.switchLight' : 'appearance.switchDark'))
const densityLabel = computed(() => t(compact.value ? 'appearance.switchDefaultDensity' : 'appearance.switchCompactDensity'))

function toggleMode() {
  theme.setColorMode(dark.value ? 'light' : 'dark')
}
function toggleDensity() {
  theme.setDensity(compact.value ? 'default' : 'compact')
}
</script>

<template>
  <div class="appearance-controls" data-ui-appearance-controls>
    <UiButton class="icon-button appearance-toggle" :aria-label="modeLabel" :title="modeLabel" @click="toggleMode">
      <span class="mode-glyph" aria-hidden="true">{{ dark ? '◐' : '☀' }}</span>
    </UiButton>
    <UiButton class="icon-button appearance-toggle" :aria-label="densityLabel" :title="densityLabel" @click="toggleDensity">
      <AppIcon name="layers" :size="17" />
    </UiButton>
  </div>
</template>

<style scoped>
.appearance-controls { display: flex; align-items: center; gap: 2px; padding: 2px; border: 1px solid var(--color-border); border-radius: var(--radius-sm); background: var(--color-surface-soft); }
.appearance-toggle { width: 32px; height: 32px; }
.mode-glyph { font-size: 17px; line-height: 1; color: var(--color-text-secondary); }
@media (max-width: 767px) { .appearance-toggle { width: 36px; height: 36px; } }
</style>
