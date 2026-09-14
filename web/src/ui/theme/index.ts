import { readonly, ref } from 'vue'

export interface UiThemeDefinition {
  name: string
  primary: string
  primaryHover: string
  primarySoft: string
  onPrimary?: string
}

export const uiThemePresets = {
  blue: { name: 'blue', primary: '#2563eb', primaryHover: '#1d4ed8', primarySoft: '#eff6ff', onPrimary: '#ffffff' },
  emerald: { name: 'emerald', primary: '#059669', primaryHover: '#047857', primarySoft: '#ecfdf5', onPrimary: '#ffffff' },
  violet: { name: 'violet', primary: '#7c3aed', primaryHover: '#6d28d9', primarySoft: '#f5f3ff', onPrimary: '#ffffff' },
  amber: { name: 'amber', primary: '#d97706', primaryHover: '#b45309', primarySoft: '#fffbeb', onPrimary: '#ffffff' },
} satisfies Record<string, UiThemeDefinition>

const storageKey = 'coffeelink.ui-theme'
const activeTheme = ref<UiThemeDefinition>(uiThemePresets.blue)

function applyTheme(theme: UiThemeDefinition) {
  if (typeof document === 'undefined') return
  const style = document.documentElement.style
  style.setProperty('--color-primary', theme.primary)
  style.setProperty('--color-primary-hover', theme.primaryHover)
  style.setProperty('--color-primary-soft', theme.primarySoft)
  style.setProperty('--color-on-primary', theme.onPrimary ?? '#ffffff')
  activeTheme.value = theme
}

export function setUiTheme(theme: UiThemeDefinition, persist = true) {
  applyTheme(theme)
  if (persist && typeof window !== 'undefined') window.localStorage.setItem(storageKey, JSON.stringify(theme))
}

export function setUiThemePreset(name: keyof typeof uiThemePresets) {
  setUiTheme(uiThemePresets[name])
}

export function initializeUiTheme() {
  if (typeof window === 'undefined') return
  const saved = window.localStorage.getItem(storageKey)
  if (!saved) return applyTheme(uiThemePresets.blue)
  try {
    const value = JSON.parse(saved) as UiThemeDefinition
    if (value.primary && value.primaryHover && value.primarySoft) return applyTheme(value)
  } catch {
    window.localStorage.removeItem(storageKey)
  }
  applyTheme(uiThemePresets.blue)
}

export function useUiTheme() {
  return { activeTheme: readonly(activeTheme), presets: uiThemePresets, setTheme: setUiTheme, setPreset: setUiThemePreset }
}
