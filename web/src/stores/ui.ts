import { defineStore } from 'pinia'
import { ref } from 'vue'
export const useUiStore = defineStore('ui', () => {
  const collapsed = ref(false),
    module = ref<string | null>(null),
    mobileOpen = ref(false)
  const notice = ref(''),
    noticeTone = ref<'success' | 'error' | 'info'>('success')
  let timer: ReturnType<typeof setTimeout> | undefined
  function toast(message: string, tone: 'success' | 'error' | 'info' = 'success') {
    notice.value = message
    noticeTone.value = tone
    clearTimeout(timer)
    timer = setTimeout(() => (notice.value = ''), 4500)
  }
  function closeMenu() {
    module.value = null
    mobileOpen.value = false
  }
  function toggleModule(id: string) {
    module.value = module.value === id ? null : id
  }
  return { collapsed, module, mobileOpen, notice, noticeTone, toast, closeMenu, toggleModule }
})
