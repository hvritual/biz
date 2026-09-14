import { beforeEach, describe, expect, it } from 'vitest'
import {
  applyUiTheme,
  initializeUiTheme,
  resetUiTheme,
  setUiTheme,
  setUiThemePreset,
  type UiThemePalette,
  useUiTheme,
} from './theme'

const root = () => document.documentElement

beforeEach(() => {
  resetUiTheme(root())
})

describe('global UI theme', () => {
  it('applies a named palette through root design tokens', () => {
    expect(applyUiTheme('emerald', root())).toBe(true)
    expect(root().style.getPropertyValue('--color-primary')).toBe('#059669')
    expect(root().style.getPropertyValue('--color-primary-hover')).toBe('#047857')
    expect(root().style.getPropertyValue('--color-primary-soft')).toBe('#ecfdf5')
    expect(root().dataset.uiTheme).toBe('emerald')
  })

  it('accepts a complete custom brand palette without touching page components', () => {
    const palette: UiThemePalette = {
      primary: '#123456',
      primaryHover: '#102f4d',
      primarySoft: '#eef4f8',
      onPrimary: '#ffffff',
      gradientEnd: '#9fb8cc',
    }
    expect(applyUiTheme(palette, root())).toBe(true)
    expect(root().style.getPropertyValue('--color-primary')).toBe(palette.primary)
    expect(root().style.getPropertyValue('--color-gradient-end')).toBe(palette.gradientEnd)
    expect(root().dataset.uiTheme).toBe('custom')
  })

  it('persists named presets and restores the preset identity on application startup', () => {
    expect(setUiThemePreset('violet', true, root())).toBe(true)
    expect(window.localStorage.getItem('coffeelink.ui-theme')).toBe(JSON.stringify({ name: 'violet' }))
    expect(resetUiTheme(root(), false)).toBe(true)
    expect(root().dataset.uiTheme).toBeUndefined()
    expect(initializeUiTheme(root())).toBe(true)
    expect(root().dataset.uiTheme).toBe('violet')
    expect(root().style.getPropertyValue('--color-primary')).toBe('#7c3aed')
  })

  it('restores legacy stored preset objects without degrading them to custom themes', () => {
    window.localStorage.setItem(
      'coffeelink.ui-theme',
      JSON.stringify({
        name: 'emerald',
        primary: '#059669',
        primaryHover: '#047857',
        primarySoft: '#ecfdf5',
        onPrimary: '#ffffff',
      }),
    )
    expect(initializeUiTheme(root())).toBe(true)
    expect(root().dataset.uiTheme).toBe('emerald')
  })

  it('persists and restores complete custom palettes', () => {
    const palette: UiThemePalette = {
      primary: '#111827',
      primaryHover: '#0f172a',
      primarySoft: '#f1f5f9',
      onPrimary: '#ffffff',
      gradientEnd: '#94a3b8',
    }
    expect(setUiTheme(palette, true, root())).toBe(true)
    expect(resetUiTheme(root(), false)).toBe(true)
    expect(initializeUiTheme(root())).toBe(true)
    expect(root().dataset.uiTheme).toBe('custom')
    expect(root().style.getPropertyValue('--color-gradient-end')).toBe(palette.gradientEnd)
    expect(useUiTheme().activeTheme.value.name).toBe('custom')
  })

  it('removes runtime overrides and persisted state and falls back to stylesheet tokens', () => {
    setUiThemePreset('amber', true, root())
    expect(resetUiTheme(root())).toBe(true)
    expect(root().style.getPropertyValue('--color-primary')).toBe('')
    expect(root().dataset.uiTheme).toBeUndefined()
    expect(window.localStorage.getItem('coffeelink.ui-theme')).toBeNull()
  })
})
