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
export type UiColorMode = 'light' | 'dark'
export type UiDensity = 'default' | 'compact'
export type UiAppearance = Readonly<{ mode: UiColorMode; density: UiDensity }>
export type ActiveUiTheme = {
  name: UiThemePresetName | 'custom'
  palette: UiThemePalette
}
export type TenantUiTheme = Readonly<{
  preset: UiThemePresetName | 'custom'
  primary?: string
}>

const legacyBrandStorageKey = 'coffeelink.ui-theme'
const appearanceStorageKey = 'coffeelink.ui-appearance'
const themeProperties: Record<keyof UiThemePalette, string> = {
  primary: '--color-primary',
  primaryHover: '--color-primary-hover',
  primarySoft: '--color-primary-soft',
  onPrimary: '--color-on-primary',
  gradientEnd: '--color-gradient-end',
}
const activeTheme = ref<ActiveUiTheme>({ name: 'blue', palette: { ...uiThemePresets.blue } })
const colorMode = ref<UiColorMode>('light')
const density = ref<UiDensity>('default')

function resolveRoot(root?: HTMLElement) {
  if (root) return root
  return typeof document === 'undefined' ? null : document.documentElement
}

function resolveTheme(theme: UiThemePresetName | UiThemePalette): ActiveUiTheme {
  if (typeof theme === 'string') return { name: theme, palette: { ...uiThemePresets[theme] } }
  return { name: 'custom', palette: { ...theme } }
}

function normalizeHex(value: string) {
  const trimmed = value.trim().toLowerCase()
  if (!/^#[0-9a-f]{6}$/.test(trimmed)) throw new Error('Brand primary must be a six-digit hex color.')
  return trimmed
}

function rgb(hex: string) {
  const value = Number.parseInt(hex.slice(1), 16)
  return [(value >> 16) & 255, (value >> 8) & 255, value & 255] as const
}

function hex(parts: readonly number[]) {
  return `#${parts.map((value) => Math.round(Math.max(0, Math.min(255, value))).toString(16).padStart(2, '0')).join('')}`
}

function mix(source: string, target: string, ratio: number) {
  const from = rgb(source), to = rgb(target)
  return hex(from.map((value, index) => value + (to[index]! - value) * ratio))
}

function luminance(hexColor: string) {
  const channels = rgb(hexColor).map((value) => {
    const normalized = value / 255
    return normalized <= 0.03928 ? normalized / 12.92 : ((normalized + 0.055) / 1.055) ** 2.4
  })
  return 0.2126 * channels[0]! + 0.7152 * channels[1]! + 0.0722 * channels[2]!
}

function contrast(first: string, second: string) {
  const a = luminance(first), b = luminance(second)
  return (Math.max(a, b) + 0.05) / (Math.min(a, b) + 0.05)
}

function runtimePalette(base: UiThemePalette, mode: UiColorMode): UiThemePalette {
  if (mode === 'light') return { ...base }
  return {
    primary: base.primary,
    primaryHover: mix(base.primary, '#ffffff', 0.12),
    primarySoft: mix(base.primary, '#111827', 0.78),
    onPrimary: base.onPrimary,
    gradientEnd: mix(base.primary, '#0b1120', 0.54),
  }
}

function applyBrandTokens(target: HTMLElement) {
  const palette = runtimePalette(activeTheme.value.palette, colorMode.value)
  for (const key of Object.keys(themeProperties) as Array<keyof UiThemePalette>) {
    target.style.setProperty(themeProperties[key], palette[key])
  }
}

function validMode(value: unknown): value is UiColorMode {
  return value === 'light' || value === 'dark'
}

function validDensity(value: unknown): value is UiDensity {
  return value === 'default' || value === 'compact'
}

function parseAppearance(raw: string | null): UiAppearance | null {
  if (!raw) return null
  try {
    const value = JSON.parse(raw) as Partial<UiAppearance>
    if (validMode(value.mode) && validDensity(value.density)) return { mode: value.mode, density: value.density }
  } catch {
    return null
  }
  return null
}

function persistAppearance() {
  if (typeof window === 'undefined') return
  window.localStorage.setItem(appearanceStorageKey, JSON.stringify({ mode: colorMode.value, density: density.value }))
}

export function createUiThemePalette(primary: string): UiThemePalette {
  const normalized = normalizeHex(primary)
  const white = '#ffffff', dark = '#111827'
  const onPrimary = contrast(normalized, white) >= contrast(normalized, dark) ? white : dark
  if (contrast(normalized, onPrimary) < 4.5) throw new Error('Brand primary does not provide readable foreground contrast.')
  return {
    primary: normalized,
    primaryHover: mix(normalized, '#000000', 0.16),
    primarySoft: mix(normalized, white, 0.92),
    onPrimary,
    gradientEnd: mix(normalized, white, 0.58),
  }
}

export function resolveTenantUiTheme(value: TenantUiTheme): UiThemePresetName | UiThemePalette {
  if (value.preset !== 'custom') return value.preset
  if (!value.primary) throw new Error('Custom tenant branding requires a primary color.')
  return createUiThemePalette(value.primary)
}

export function applyUiTheme(theme: UiThemePresetName | UiThemePalette, root?: HTMLElement) {
  const target = resolveRoot(root)
  if (!target) return false
  activeTheme.value = resolveTheme(theme)
  target.dataset.uiTheme = activeTheme.value.name
  applyBrandTokens(target)
  return true
}

// Brand persistence is intentionally not a browser authority. #107 server branding wins on tenant lifecycle.
export function setUiTheme(theme: UiThemePresetName | UiThemePalette, _persist = false, root?: HTMLElement) {
  return applyUiTheme(theme, root)
}

export function setUiThemePreset(name: UiThemePresetName, persist = false, root?: HTMLElement) {
  return setUiTheme(name, persist, root)
}

export function applyUiAppearance(value: UiAppearance, root?: HTMLElement) {
  const target = resolveRoot(root)
  if (!target || !validMode(value.mode) || !validDensity(value.density)) return false
  colorMode.value = value.mode
  density.value = value.density
  target.dataset.uiMode = value.mode
  target.dataset.uiDensity = value.density
  target.style.colorScheme = value.mode
  applyBrandTokens(target)
  return true
}

export function setUiColorMode(mode: UiColorMode, persist = true, root?: HTMLElement) {
  const applied = applyUiAppearance({ mode, density: density.value }, root)
  if (applied && persist) persistAppearance()
  return applied
}

export function setUiDensity(nextDensity: UiDensity, persist = true, root?: HTMLElement) {
  const applied = applyUiAppearance({ mode: colorMode.value, density: nextDensity }, root)
  if (applied && persist) persistAppearance()
  return applied
}

export function initializeUiTheme(root?: HTMLElement) {
  const target = resolveRoot(root)
  if (!target) return false
  // Legacy local brand state can leak between tenants, so it is retired rather than restored.
  if (typeof window !== 'undefined') window.localStorage.removeItem(legacyBrandStorageKey)
  applyUiTheme('blue', target)
  const stored = typeof window === 'undefined' ? null : parseAppearance(window.localStorage.getItem(appearanceStorageKey))
  return applyUiAppearance(stored ?? { mode: 'light', density: 'default' }, target)
}

export function resetUiTheme(root?: HTMLElement, clearPersisted = true) {
  const target = resolveRoot(root)
  if (!target) return false
  if (clearPersisted && typeof window !== 'undefined') window.localStorage.removeItem(legacyBrandStorageKey)
  return applyUiTheme('blue', target)
}

export function resetUiAppearance(root?: HTMLElement, clearPersisted = true) {
  const target = resolveRoot(root)
  if (!target) return false
  if (clearPersisted && typeof window !== 'undefined') window.localStorage.removeItem(appearanceStorageKey)
  return applyUiAppearance({ mode: 'light', density: 'default' }, target)
}

export function useUiTheme() {
  return {
    activeTheme: readonly(activeTheme),
    colorMode: readonly(colorMode),
    density: readonly(density),
    presets: uiThemePresets,
    setTheme: setUiTheme,
    setPreset: setUiThemePreset,
    setColorMode: setUiColorMode,
    setDensity: setUiDensity,
    resetTheme: resetUiTheme,
    resetAppearance: resetUiAppearance,
  }
}
