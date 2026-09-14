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
})
