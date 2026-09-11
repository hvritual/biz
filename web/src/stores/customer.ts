import { executeRentalCommand, type RentalCommand } from '@/services/siteRental/commands'
import { computed, ref, watch } from 'vue'
import { defineStore } from 'pinia'
import { useEnterpriseStore } from './enterprise'
import { createCustomerSeed } from '@/services/customer/seed'
import { loadCustomerSnapshot, persistCustomerSnapshot } from '@/services/customer/repository'
import { executeCustomerCommand } from '@/services/customer/commands'
import type { Command } from '@/services/customer/commands'
import { applyCustomerImport, type ImportRow } from '@/services/customer/importer'
import type { FormValues } from '@/types/customer'
export const useCustomerStore = defineStore('customer-preview', () => {
  const enterprise = useEnterpriseStore()
  const snapshot = ref(createCustomerSeed(enterprise.tenantId)),
    loadError = ref(''),
    lastReceipt = ref('')
  function load() {
    loadError.value = ''
    lastReceipt.value = ''
    try {
      snapshot.value = loadCustomerSnapshot(enterprise.tenantId, localStorage)
    } catch (e) {
      loadError.value = (e as Error).message
      snapshot.value = createCustomerSeed(enterprise.tenantId)
    }
  }
  watch(() => enterprise.tenantId, load, { immediate: true })
  function run(command: Command) {
    if (!enterprise.previewMode) throw new Error('真实客户经营接口尚未接入，不允许降级为本地保存')
    if (loadError.value) throw new Error(loadError.value)
    const persisted = loadCustomerSnapshot(enterprise.tenantId, localStorage)
    if (persisted.revision > snapshot.value.revision) snapshot.value = persisted
    const result = executeCustomerCommand(snapshot.value, command)
    persistCustomerSnapshot(result.snapshot, localStorage)
    // Publish only after durable local write; caller retains form on any failure.
    snapshot.value = result.snapshot
    lastReceipt.value = result.result.detail
    return result.result
  }
  function refreshRental() {
    const current = loadCustomerSnapshot(enterprise.tenantId, localStorage)
    if (current.revision > snapshot.value.revision) snapshot.value = current
  }
  function runRental(command: RentalCommand) {
    if (!enterprise.previewMode || loadError.value) throw new Error('当前数据模式不可进行本地预览写入')
    const current = loadCustomerSnapshot(enterprise.tenantId, localStorage)
    const result = executeRentalCommand(current, command)
    persistCustomerSnapshot(result.snapshot, localStorage)
    snapshot.value = result.snapshot
    lastReceipt.value = result.result.detail
    return result.result
  }
  function saveDraft(key: string, values: FormValues) {
    if (!enterprise.previewMode || loadError.value) throw new Error('当前数据模式不允许保存草稿')
    const persisted = loadCustomerSnapshot(enterprise.tenantId, localStorage)
    const latest = persisted.revision > snapshot.value.revision ? persisted : snapshot.value
    const next = JSON.parse(JSON.stringify(latest))
    next.revision++
    next.drafts[key] = values
    persistCustomerSnapshot(next, localStorage)
    snapshot.value = next
  }
  function importRows(rows: ImportRow[], batch: string) {
    if (!enterprise.previewMode || loadError.value) throw new Error('当前数据模式不允许导入')
    const current = loadCustomerSnapshot(enterprise.tenantId, localStorage)
    const result = applyCustomerImport(current, rows, batch)
    persistCustomerSnapshot(result.snapshot, localStorage)
    snapshot.value = result.snapshot
    lastReceipt.value = `已导入 ${result.results.filter((r) => r.result === '已导入').length} 条；重复和错误行未覆盖已有客户`
    return result.results
  }
  function resetPreview() {
    const next = createCustomerSeed(enterprise.tenantId)
    persistCustomerSnapshot(next, localStorage)
    snapshot.value = next
    loadError.value = ''
    lastReceipt.value = '当前租户的预览数据已重置'
  }
  const customerName = (id: string) => snapshot.value.customers.find((c) => c.id === id)?.name ?? '未知客户'
  const openWork = computed(() => snapshot.value.work.filter((w) => w.status !== '已结束'))
  return {
    snapshot,
    runRental,
    refreshRental,
    loadError,
    lastReceipt,
    run,
    saveDraft,
    resetPreview,
    importRows,
    customerName,
    openWork,
  }
})
