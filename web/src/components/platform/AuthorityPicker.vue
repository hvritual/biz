<script setup lang="ts">
import { ref } from 'vue'
import { request } from '@/services/commercial/platformCommercial'
import { createRuntimeApi } from '@/services/runtime/api'
const props = defineProps<{ kind: 'plans' | 'tenants' }>()
const emit = defineEmits<{ select: [id: string] }>()
const rows = ref<Array<{ id: string; name: string }>>([]),
  cursor = ref(''),
  busy = ref(false),
  error = ref(''),
  loaded = ref(false)
async function load(more = false) {
  busy.value = true
  error.value = ''
  try {
    if (props.kind === 'plans') {
      const result = await request<{
        plans?: Array<{ planCode: string; name: string }>
        nextAfterPlanCode?: string
      }>(`/v1/platform/plans?page_size=20&after_plan_code=${encodeURIComponent(more ? cursor.value : '')}`)
      const next = (result.plans ?? []).map((p) => ({ id: p.planCode, name: p.name }))
      rows.value = more ? [...rows.value, ...next] : next
      cursor.value = result.nextAfterPlanCode ?? ''
    } else {
      rows.value = (await createRuntimeApi('read').platformApi.listTenants()).map((t) => ({
        id: t.id,
        name: t.name,
      }))
      cursor.value = ''
    }
    loaded.value = true
  } catch (e) {
    error.value = e instanceof Error ? e.message : '目录读取失败'
  } finally {
    busy.value = false
  }
}
</script>
<template>
  <section class="card panel-pad page-stack">
    <div>
      <button class="btn" :disabled="busy" @click="load()">
        {{ kind === 'plans' ? '读取套餐目录' : '读取租户目录' }}
      </button>
    </div>
    <p v-if="error" role="alert">{{ error }}</p>
    <p v-if="loaded && !rows.length">目录中暂无可见记录。</p>
    <label v-if="rows.length"
      >{{ kind === 'plans' ? '选择套餐' : '选择租户'
      }}<select
        class="input"
        :aria-label="kind === 'plans' ? '选择套餐' : '选择租户'"
        :disabled="busy"
        value=""
        @change="emit('select', ($event.target as HTMLSelectElement).value)"
      >
        <option value="" disabled>请选择</option>
        <option v-for="row in rows" :key="row.id" :value="row.id">{{ row.name }}（{{ row.id }}）</option>
      </select></label
    >
    <button v-if="cursor" class="btn" :disabled="busy" @click="load(true)">加载更多套餐</button>
  </section>
</template>
