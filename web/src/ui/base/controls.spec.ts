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
