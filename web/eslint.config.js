import pluginVue from 'eslint-plugin-vue'
import { defineConfigWithVueTs, vueTsConfigs } from '@vue/eslint-config-typescript'
import skipFormatting from '@vue/eslint-config-prettier/skip-formatting'
export default defineConfigWithVueTs(
  { ignores: ['dist/**', 'node_modules/**', 'playwright-report/**', 'test-results/**', 'scripts/**'] },
  ...pluginVue.configs['flat/recommended'],
  vueTsConfigs.recommended,
  skipFormatting,
  {
    rules: {
      'vue/multi-word-component-names': 'error',
      '@typescript-eslint/no-explicit-any': 'error',
      'vue/no-mutating-props': 'error',
    },
  },
)
