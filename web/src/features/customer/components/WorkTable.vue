<script setup lang="ts">
import type { WorkItem, SavedWorkView } from '@/types/customer'
import { useCustomerStore } from '@/stores/customer'
import { useCustomerActions } from '@/composables/customerActions'
import { workKindNames } from '@/services/customer/seed'
import StatusBadge from '@/components/ui/StatusBadge.vue'
import AvatarMark from '@/components/ui/AvatarMark.vue'
const props = withDefaults(
  defineProps<{ items: WorkItem[]; selected?: string[]; selectable?: boolean; view?: SavedWorkView }>(),
  { selected: () => [], selectable: false, view: undefined },
)
const emit = defineEmits<{ 'update:selected': [string[]] }>()
const store = useCustomerStore(),
  actions = useCustomerActions()
function select(id: string, on: boolean) {
  emit('update:selected', on ? [...new Set([...props.selected, id])] : props.selected.filter((x) => x !== id))
}
const visible = (field: string) => !props.view || props.view.columns.includes(field)
</script>
<template>
  <div class="table-scroll">
    <table class="data-table" :class="{ 'work-compact': view?.density === '紧凑' }">
      <thead>
        <tr>
          <th v-if="selectable" class="check-cell">
            <input
              type="checkbox"
              aria-label="选择当前页全部事项"
              :checked="items.length > 0 && items.every((w) => selected.includes(w.id))"
              @change="
                emit(
                  'update:selected',
                  ($event.target as HTMLInputElement).checked ? items.map((w) => w.id) : [],
                )
              "
            />
          </th>
          <th>事项</th>
          <th v-if="visible('客户')">关联客户</th>
          <th>类型 / 状态</th>
          <th v-if="visible('负责人')">负责人</th>
          <th v-if="visible('期限')">截止时间</th>
          <th v-if="visible('下一步')">下一步行动</th>
          <th>操作</th>
        </tr>
      </thead>
      <tbody>
        <tr v-for="w in items" :key="w.id" :class="{ selected: selected.includes(w.id) }">
          <td v-if="selectable" class="check-cell">
            <input
              type="checkbox"
              :aria-label="`选择 ${w.id}`"
              :checked="selected.includes(w.id)"
              @change="select(w.id, ($event.target as HTMLInputElement).checked)"
            />
          </td>
          <td>
            <RouterLink :to="`/customers/work/${w.id}`" class="btn-link mainline">{{ w.title }}</RouterLink>
            <p class="subline">
              {{ w.id }}<span v-if="w.dependencies.length"> · {{ w.dependencies.length }} 项依赖</span>
            </p>
          </td>
          <td v-if="visible('客户')">
            <RouterLink :to="`/customers/accounts/${w.customerId}`">{{
              store.customerName(w.customerId)
            }}</RouterLink>
          </td>
          <td>
            <p>{{ workKindNames[w.kind] }}</p>
            <StatusBadge
              :text="w.status === '已结束' ? `已结束 · ${w.resolution}` : w.status"
              :tone="
                w.status === '已结束'
                  ? w.resolution === '成功'
                    ? 'success'
                    : 'neutral'
                  : w.status === '待验收'
                    ? 'warning'
                    : 'primary'
              "
            />
          </td>
          <td v-if="visible('负责人')">
            <div class="row"><AvatarMark :name="w.owner" :size="24" />{{ w.owner }}</div>
          </td>
          <td v-if="visible('期限')" class="numeric">
            {{ w.deadline }}
            <p v-if="!w.writable" class="subline">只读事项</p>
          </td>
          <td v-if="visible('下一步')" class="work-next">
            {{ w.nextAction }}
            <p class="subline">{{ w.nextAt.replace('T', ' ') }}</p>
          </td>
          <td>
            <div class="table-actions">
              <RouterLink class="btn-link" :to="`/customers/work/${w.id}`">查看</RouterLink
              ><button
                class="btn-link"
                :disabled="!w.writable"
                @click="actions.open(w.status === '已结束' ? 'reopen' : 'reschedule', w.id)"
              >
                {{ w.status === '已结束' ? '重开' : '改期' }}
              </button>
            </div>
          </td>
        </tr>
      </tbody>
    </table>
  </div>
</template>
<style scoped>
.work-next {
  max-width: 260px;
  min-width: 160px;
  line-height: 1.6;
}
.work-compact td {
  height: 42px !important;
  padding-top: 6px;
  padding-bottom: 6px;
}
.data-table input[type='checkbox'] {
  accent-color: var(--color-primary);
  width: 15px;
  height: 15px;
}
.mainline {
  max-width: 240px;
  white-space: normal;
  text-align: left;
  line-height: 1.6;
  display: inline-block;
}
</style>
