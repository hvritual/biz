<script setup lang="ts">
import { computed } from 'vue'
import type { WorkItem } from '@/types/customer'
import { useCustomerStore } from '@/stores/customer'
import { useCustomerActions } from '@/composables/customerActions'
import { workSources } from '@/services/customer/selectors'
import CustomerSection from './CustomerSection.vue'
import CustomerAlert from './CustomerAlert.vue'
import SourceRecords from './SourceRecords.vue'
import StatusBadge from '@/components/ui/StatusBadge.vue'
const props = defineProps<{ work: WorkItem }>(),
  store = useCustomerStore(),
  actions = useCustomerActions()
const records = computed(() => workSources(store.snapshot, props.work).filter((s) => s.kind === 'contract'))
const original = computed(() =>
  records.value.find((s) => !s.facts.renewal && props.work.evidenceIds.includes(s.id)),
)
const renewed = computed(() =>
  records.value.find((s) => s.facts.original === original.value?.id && s.verified),
)
const dependencies = computed(() => store.snapshot.work.filter((w) => props.work.dependencies.includes(w.id)))
</script>
<template>
  <div class="page-stack">
    <CustomerSection title="续约范围与合同依据" icon="file">
      <template #action
        ><button
          class="btn-link"
          :disabled="!work.writable || work.status === '已结束' || !records.length"
          @click="actions.open('link-source', work.id, [], { sourceId: renewed?.id || original?.id || '' })"
        >
          关联生效续约合同
        </button></template
      >
      <div class="customer-mini-metrics">
        <div>
          <small>原合同</small><strong style="font-size: 20px">{{ original?.id || '未关联' }}</strong
          ><small>到期 {{ original?.facts.end || '—' }}</small>
        </div>
        <div>
          <small>本次覆盖点位</small><strong>{{ original?.facts.sites ?? '—' }} <small>个</small></strong
          ><small>范围随原合同核对</small>
        </div>
        <div>
          <small>续约结果</small><strong style="font-size: 20px">{{ work.resolution || '尚未确认' }}</strong
          ><small>不以事项关闭率替代</small>
        </div>
      </div>
      <dl class="customer-info-list">
        <dt>续约目标</dt>
        <dd>{{ work.description }}</dd>
        <dt>候选合同</dt>
        <dd>
          {{
            renewed
              ? `${renewed.id} · ${renewed.facts.start} 至 ${renewed.facts.end}`
              : '尚无关联原合同的有效续约来源'
          }}
        </dd>
        <dt>关键阻塞</dt>
        <dd>
          <div v-for="dependency in dependencies" :key="dependency.id" class="row">
            <RouterLink :to="`/customers/work/${dependency.id}`" class="btn-link"
              >{{ dependency.id }} · {{ dependency.title }}</RouterLink
            ><StatusBadge
              :text="dependency.status"
              :tone="dependency.status === '已结束' ? 'success' : 'warning'"
            />
          </div>
          <span v-if="!dependencies.length">暂无关联依赖</span>
        </dd>
        <dt>成功标准</dt>
        <dd>关联原合同的续约合同已生效，核对合同范围与有效日期。</dd>
      </dl>
    </CustomerSection>
    <CustomerSection title="谈判与后续处置" icon="checks">
      <CustomerAlert
        title="合同生效与回款是不同业务结果"
        description="续约成功依据生效合同；本合同未续约也不直接将整个客户标记为流失。"
      />
      <div class="customer-inline-actions" style="margin-top: 20px">
        <button class="btn" :disabled="!work.writable" @click="actions.open('comment', work.id)">
          记录内部谈判进展</button
        ><button
          class="btn"
          :disabled="!work.writable || work.status === '已结束'"
          @click="actions.open('nonrenewal', work.id)"
        >
          记录未续约结果
        </button>
      </div>
    </CustomerSection>
    <CustomerSection title="已关联原合同与续约记录" icon="link"
      ><SourceRecords :records="records.filter((s) => work.evidenceIds.includes(s.id))"
    /></CustomerSection>
  </div>
</template>
