import pluginVue from "eslint-plugin-vue";
import { withVueTs, vueTsConfigs } from "@vue/eslint-config-typescript";
import skipFormatting from "@vue/eslint-config-prettier/skip-formatting";
export default withVueTs(
  {
    ignores: ["dist/**", "node_modules/**", "playwright-report/**", "test-results/**"],
  },
  pluginVue.configs["flat/essential"],
  vueTsConfigs.recommended,
  skipFormatting,
  {
    files: ["src/**/*.{ts,vue}"],
    rules: {
      "vue/multi-word-component-names": "error",
      "@typescript-eslint/no-explicit-any": "error",
    },
  },
);
