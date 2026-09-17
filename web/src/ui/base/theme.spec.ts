import { beforeEach, describe, expect, it } from 'vitest'
import {
  applyUiAppearance,
  applyUiTheme,
  createUiThemePalette,
  initializeUiTheme,
  resetUiAppearance,
  resetUiTheme,
  resolveTenantUiTheme,
  setUiColorMode,
  setUiDensity,
  type UiThemePalette,
  useUiTheme,
} from './theme'

const root = () => document.documentElement

beforeEach(() => {
  window.localStorage.clear()
  resetUiAppearance(root())
  resetUiTheme(root())
})

describe('global UI theme', () => {
  it('applies a named tenant palette through root design tokens', () => {
    expect(applyUiTheme('emerald', root())).toBe(true)
    expect(root().style.getPropertyValue('--color-primary')).toBe('#059669')
    expect(root().dataset.uiTheme).toBe('emerald')
  })

  it('accepts a complete custom brand palette without touching page components', () => {
    const palette: UiThemePalette = {
      primary: '#123456', primaryHover: '#102f4d', primarySoft: '#eef4f8', onPrimary: '#ffffff', gradientEnd: '#9fb8cc',
    }
    expect(applyUiTheme(palette, root())).toBe(true)
    expect(root().style.getPropertyValue('--color-primary')).toBe(palette.primary)
    expect(root().dataset.uiTheme).toBe('custom')
  })

  it('derives custom theme tokens and rejects invalid tenant brand colors', () => {
    const palette = createUiThemePalette('#125A75')
    expect(palette.primary).toBe('#125a75')
    expect(['#ffffff', '#111827']).toContain(palette.onPrimary)
    expect(resolveTenantUiTheme({ preset: 'custom', primary: '#125a75' })).toEqual(palette)
    expect(resolveTenantUiTheme({ preset: 'violet' })).toBe('violet')
    expect(() => createUiThemePalette('#fff')).toThrow(/six-digit hex/)
    expect(() => resolveTenantUiTheme({ preset: 'custom' })).toThrow(/primary color/)
  })

  it('persists local appearance independently from server-authoritative brand identity', () => {
    applyUiTheme('violet', root())
    expect(setUiColorMode('dark', true, root())).toBe(true)
    expect(setUiDensity('compact', true, root())).toBe(true)
    expect(JSON.parse(window.localStorage.getItem('coffeelink.ui-appearance')!)).toEqual({ mode: 'dark', density: 'compact' })
    expect(useUiTheme().activeTheme.value.name).toBe('violet')
    expect(root().dataset.uiMode).toBe('dark')
    expect(root().dataset.uiDensity).toBe('compact')
  })

  it('dark mode re-derives brand soft tokens without changing tenant brand identity', () => {
    applyUiTheme('violet', root())
    const lightSoft = root().style.getPropertyValue('--color-primary-soft')
    setUiColorMode('dark', false, root())
    expect(useUiTheme().activeTheme.value.name).toBe('violet')
    expect(root().style.getPropertyValue('--color-primary-soft')).not.toBe(lightSoft)
    expect(root().style.colorScheme).toBe('dark')
  })

  it('initialization retires legacy local brand state but restores display preferences', () => {
    window.localStorage.setItem('coffeelink.ui-theme', JSON.stringify({ name: 'amber' }))
    window.localStorage.setItem('coffeelink.ui-appearance', JSON.stringify({ mode: 'dark', density: 'compact' }))
    initializeUiTheme(root())
    expect(window.localStorage.getItem('coffeelink.ui-theme')).toBeNull()
    expect(useUiTheme().activeTheme.value.name).toBe('blue')
    expect(root().dataset.uiMode).toBe('dark')
    expect(root().dataset.uiDensity).toBe('compact')
  })

  it('resetting appearance does not erase the active tenant brand', () => {
    applyUiTheme('emerald', root())
    applyUiAppearance({ mode: 'dark', density: 'compact' }, root())
    resetUiAppearance(root())
    expect(useUiTheme().activeTheme.value.name).toBe('emerald')
    expect(root().dataset.uiMode).toBe('light')
    expect(root().dataset.uiDensity).toBe('default')
  })
})
