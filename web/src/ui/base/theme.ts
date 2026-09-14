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

const themeProperties: Record<keyof UiThemePalette, string> = {
  primary: '--color-primary',
  primaryHover: '--color-primary-hover',
  primarySoft: '--color-primary-soft',
  onPrimary: '--color-on-primary',
  gradientEnd: '--color-gradient-end',
}

function resolveRoot(root?: HTMLElement) {
  if (root) return root
  return typeof document === 'undefined' ? null : document.documentElement
}

export function applyUiTheme(theme: UiThemePresetName | UiThemePalette, root?: HTMLElement) {
  const target = resolveRoot(root)
  if (!target) return false
  const palette = typeof theme === 'string' ? uiThemePresets[theme] : theme
  for (const key of Object.keys(themeProperties) as Array<keyof UiThemePalette>) {
    target.style.setProperty(themeProperties[key], palette[key])
  }
  target.dataset.uiTheme = typeof theme === 'string' ? theme : 'custom'
  return true
}

export function resetUiTheme(root?: HTMLElement) {
  const target = resolveRoot(root)
  if (!target) return false
  for (const property of Object.values(themeProperties)) target.style.removeProperty(property)
  delete target.dataset.uiTheme
  return true
}
