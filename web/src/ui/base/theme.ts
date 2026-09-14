import { readonly, ref } from 'vue'

export type UiThemePalette = {
  primary: string
  primaryHover: string
  primarySoft: string
  onPrimary: string
  gradientEnd: string
}

export const uiThemePresets = {
  blue: {
    primary: '#2563eb',
    primaryHover: '#1d4ed8',
    primarySoft: '#eff6ff',
    onPrimary: '#ffffff',
    gradientEnd: '#93c5fd',
  },
  emerald: {
    primary: '#059669',
    primaryHover: '#047857',
    primarySoft: '#ecfdf5',
    onPrimary: '#ffffff',
    gradientEnd: '#6ee7b7',
  },
  violet: {
    primary: '#7c3aed',
    primaryHover: '#6d28d9',
    primarySoft: '#f5f3ff',
    onPrimary: '#ffffff',
    gradientEnd: '#c4b5fd',
  },
  amber: {
    primary: '#d97706',
    primaryHover: '#b45309',
    primarySoft: '#fffbeb',
    onPrimary: '#ffffff',
    gradientEnd: '#fcd34d',
  },
} as const satisfies Record<string, UiThemePalette>

export type UiThemePresetName = keyof typeof uiThemePresets
export type ActiveUiTheme = {
  name: UiThemePresetName | 'custom'
  palette: UiThemePalette
}

const storageKey = 'coffeelink.ui-theme'
const themeProperties: Record<keyof UiThemePalette, string> = {
  primary: '--color-primary',
  primaryHover: '--color-primary-hover',
  primarySoft: '--color-primary-soft',
  onPrimary: '--color-on-primary',
  gradientEnd: '--color-gradient-end',
}
const activeTheme = ref<ActiveUiTheme>({ name: 'blue', palette: { ...uiThemePresets.blue } })

function resolveRoot(root?: HTMLElement) {
  if (root) return root
  return typeof document === 'undefined' ? null : document.documentElement
}

function resolveTheme(theme: UiThemePresetName | UiThemePalette): ActiveUiTheme {
  if (typeof theme === 'string') return { name: theme, palette: { ...uiThemePresets[theme] } }
  return { name: 'custom', palette: { ...theme } }
}

function storeTheme(theme: ActiveUiTheme) {
  if (typeof window === 'undefined') return
  const stored = theme.name === 'custom' ? { name: 'custom', ...theme.palette } : { name: theme.name }
  window.localStorage.setItem(storageKey, JSON.stringify(stored))
}

function matchesPreset(value: Partial<UiThemePalette>, name: UiThemePresetName) {
  const preset = uiThemePresets[name]
  return (
    value.primary === preset.primary &&
    value.primaryHover === preset.primaryHover &&
    value.primarySoft === preset.primarySoft &&
    (value.onPrimary == null || value.onPrimary === preset.onPrimary) &&
    (value.gradientEnd == null || value.gradientEnd === preset.gradientEnd)
  )
}

function parseStoredTheme(raw: string): UiThemePresetName | UiThemePalette | null {
  try {
    const value = JSON.parse(raw) as Partial<UiThemePalette> & { name?: string }
    if (value.name && value.name in uiThemePresets) {
      const name = value.name as UiThemePresetName
      if (!value.primary || matchesPreset(value, name)) return name
    }
    if (value.primary && value.primaryHover && value.primarySoft) {
      const preset = value.name && value.name in uiThemePresets
        ? uiThemePresets[value.name as UiThemePresetName]
        : uiThemePresets.blue
      return {
        primary: value.primary,
        primaryHover: value.primaryHover,
        primarySoft: value.primarySoft,
        onPrimary: value.onPrimary ?? '#ffffff',
        gradientEnd: value.gradientEnd ?? preset.gradientEnd,
      }
    }
  } catch {
    return null
  }
  return null
}

export function applyUiTheme(theme: UiThemePresetName | UiThemePalette, root?: HTMLElement) {
  const target = resolveRoot(root)
  if (!target) return false
  const resolved = resolveTheme(theme)
  for (const key of Object.keys(themeProperties) as Array<keyof UiThemePalette>) {
    target.style.setProperty(themeProperties[key], resolved.palette[key])
  }
  target.dataset.uiTheme = resolved.name
  activeTheme.value = resolved
  return true
}

export function setUiTheme(
  theme: UiThemePresetName | UiThemePalette,
  persist = true,
  root?: HTMLElement,
) {
  const applied = applyUiTheme(theme, root)
  if (applied && persist) storeTheme(resolveTheme(theme))
  return applied
}

export function setUiThemePreset(name: UiThemePresetName, persist = true, root?: HTMLElement) {
  return setUiTheme(name, persist, root)
}

export function initializeUiTheme(root?: HTMLElement) {
  if (typeof window === 'undefined') return applyUiTheme('blue', root)
  const saved = window.localStorage.getItem(storageKey)
  if (saved) {
    const theme = parseStoredTheme(saved)
    if (theme) return applyUiTheme(theme, root)
    window.localStorage.removeItem(storageKey)
  }
  return applyUiTheme('blue', root)
}

export function resetUiTheme(root?: HTMLElement, clearPersisted = true) {
  const target = resolveRoot(root)
  if (!target) return false
  for (const property of Object.values(themeProperties)) target.style.removeProperty(property)
  delete target.dataset.uiTheme
  activeTheme.value = { name: 'blue', palette: { ...uiThemePresets.blue } }
  if (clearPersisted && typeof window !== 'undefined') window.localStorage.removeItem(storageKey)
  return true
}

export function useUiTheme() {
  return {
    activeTheme: readonly(activeTheme),
    presets: uiThemePresets,
    setTheme: setUiTheme,
    setPreset: setUiThemePreset,
    resetTheme: resetUiTheme,
  }
}
