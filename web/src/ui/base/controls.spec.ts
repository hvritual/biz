import { mount } from '@vue/test-utils'
import { describe, expect, it, vi } from 'vitest'
import UiInput from './UiInput.vue'
import UiSelect from './UiSelect.vue'
import UiTextarea from './UiTextarea.vue'

describe('base control compatibility', () => {
  it('keeps legacy input value and input events controlled by the caller', async () => {
    const onInput = vi.fn()
    const wrapper = mount(UiInput, {
      props: { value: 'before' },
      attrs: { onInput },
    })
    const input = wrapper.get('input')

    expect((input.element as HTMLInputElement).value).toBe('before')
    await input.setValue('after')
    expect(onInput).toHaveBeenCalledTimes(1)
    expect(((onInput.mock.calls[0] as unknown[])[0] as Event).target).toBe(input.element)

    await wrapper.setProps({ value: 'reset' })
    expect((input.element as HTMLInputElement).value).toBe('reset')
  })

  it('preserves number and trim modifiers after native inputs become base components', async () => {
    const numberInput = mount(UiInput, {
      props: { modelValue: 1, modelModifiers: { number: true } },
    })
    await numberInput.get('input').setValue('42.5')
    expect(numberInput.emitted('update:modelValue')?.at(-1)).toEqual([42.5])

    const trimInput = mount(UiInput, {
      props: { modelValue: '', modelModifiers: { trim: true } },
    })
    await trimInput.get('input').setValue('  coffee  ')
    expect(trimInput.emitted('update:modelValue')?.at(-1)).toEqual(['coffee'])
  })

  it('renders prefix and suffix slots inside one external input frame without changing input events', async () => {
    const onInput = vi.fn()
    const wrapper = mount(UiInput, {
      props: { value: 'before' },
      attrs: { onInput },
      slots: {
        prefix: '<span>搜索</span>',
        suffix: '<kbd>⌘ K</kbd>',
      },
    })

    const frame = wrapper.get('[data-slot="input-wrapper"]')
    expect(frame.get('[data-slot="input-prefix"]').text()).toBe('搜索')
    expect(frame.get('[data-slot="input-suffix"]').text()).toBe('⌘ K')

    await frame.get('input').setValue('after')
    expect(onInput).toHaveBeenCalledTimes(1)
  })

  it('keeps text-input focus treatment on the wrapper instead of the inner input', () => {
    const wrapper = mount(UiInput, { props: { value: 'value' } })
    const frame = wrapper.get('[data-slot="input-wrapper"]')
    const input = wrapper.get('input')

    expect(frame.attributes('data-slot')).toBe('input-wrapper')
    expect(input.attributes('data-ui-input-inner')).toBeDefined()
    expect(input.classes()).toContain('outline-none')
  })

  it('keeps password inputs hidden by default and toggles their visibility inside the input frame', async () => {
    const wrapper = mount(UiInput, {
      props: { modelValue: 'Coffee2026' },
      attrs: { type: 'password', 'aria-label': '当前密码' },
    })
    const input = wrapper.get('input')
    const toggle = wrapper.get('[data-slot="password-visibility-toggle"]')

    expect(input.attributes('type')).toBe('password')
    expect(toggle.attributes('aria-label')).toBe('显示密码')
    await toggle.trigger('click')
    expect(input.attributes('type')).toBe('text')
    expect(toggle.attributes('aria-label')).toBe('隐藏密码')
    expect((input.element as HTMLInputElement).value).toBe('Coffee2026')
  })

  it('keeps legacy textarea value and input events controlled by the caller', async () => {
    const onInput = vi.fn()
    const wrapper = mount(UiTextarea, {
      props: { value: 'before' },
      attrs: { onInput },
    })
    const textarea = wrapper.get('textarea')

    expect((textarea.element as HTMLTextAreaElement).value).toBe('before')
    await textarea.setValue('after')
    expect(onInput).toHaveBeenCalledTimes(1)

    await wrapper.setProps({ value: 'reset' })
    expect((textarea.element as HTMLTextAreaElement).value).toBe('reset')
  })

  it('preserves textarea modifiers and lazy updates', async () => {
    const wrapper = mount(UiTextarea, {
      props: { modelValue: '', modelModifiers: { trim: true, lazy: true } },
    })
    const textarea = wrapper.get('textarea')
    ;(textarea.element as HTMLTextAreaElement).value = '  delayed  '
    await textarea.trigger('input')
    expect(wrapper.emitted('update:modelValue')).toBeUndefined()
    await textarea.trigger('change')
    expect(wrapper.emitted('update:modelValue')?.at(-1)).toEqual(['delayed'])
  })

  it('does not replace an enclosing field label with the placeholder', () => {
    const wrapper = mount(UiSelect)
    const trigger = wrapper.get('[data-slot="select-trigger"]')
    expect(trigger.attributes('aria-label')).toBeUndefined()
  })

  it('preserves an explicit accessible name for standalone selects', () => {
    const wrapper = mount(UiSelect, { attrs: { 'aria-label': '切换企业' } })
    expect(wrapper.get('[data-slot="select-trigger"]').attributes('aria-label')).toBe('切换企业')
  })

  it('projects checkbox indeterminate state and mixed accessibility semantics', async () => {
    const wrapper = mount(UiInput, {
      props: { checked: false, indeterminate: true },
      attrs: { type: 'checkbox', 'aria-label': '权限分组' },
    })
    const input = wrapper.get('input')
    expect((input.element as HTMLInputElement).indeterminate).toBe(true)
    expect(input.attributes('aria-checked')).toBe('mixed')

    await wrapper.setProps({ checked: true, indeterminate: false })
    expect((input.element as HTMLInputElement).indeterminate).toBe(false)
    expect((input.element as HTMLInputElement).checked).toBe(true)
    expect(input.attributes('aria-checked')).toBeUndefined()
  })
})
