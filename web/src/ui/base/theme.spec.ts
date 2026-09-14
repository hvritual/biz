import { beforeEach, describe, expect, it } from 'vitest'
import { applyUiTheme, resetUiTheme, type UiThemePalette } from './theme'

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

  it('removes runtime overrides and falls back to the stylesheet theme', () => {
    applyUiTheme('violet', root())
    expect(resetUiTheme(root())).toBe(true)
    expect(root().style.getPropertyValue('--color-primary')).toBe('')
    expect(root().dataset.uiTheme).toBeUndefined()
  })
})
