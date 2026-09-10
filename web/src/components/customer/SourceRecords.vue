<script setup lang="ts">
import { ref, computed } from 'vue'
import type { SourceRecord } from '@/types/customer'
import AppIcon from '@/components/ui/AppIcon.vue'
import StatusBadge from '@/components/ui/StatusBadge.vue'
import UiDialog from '@/components/ui/UiDialog.vue'
import CustomerAlert from './CustomerAlert.vue'
const props = defineProps<{ records: SourceRecord[] }>()
const current = ref(''),
  record = computed(() => props.records.find((r) => r.id === current.value))
const labels: Record<string, string> = {
  start: '开始日期',
  end: '结束 / 终止日期',
  amount: '合同金额',
  sites: '点位数量',
  renewal: '是否续约',
  original: '原业务记录',
  workComplete: '工单是否已完成',
  recovered: '运行恢复验证',
  confirmed: '客户是否确认',
  workId: '关联事项',
  passed: '已通过点位',
  total: '全部点位',
  allPassed: '是否全部通过',
  scope: '作用范围',
  active: '有效投放',
  due: '应收金额',
  paid: '已核销金额',
  balance: '未核销余额',
  currency: '币种',
  returned: '是否已回收',
  count: '设备数量',
  settled: '是否结清',
  terminated: '投放是否终止',
  history: '历史期间数据保留',
  onlineMinutes: '连续在线（分钟）',
  validCups: '验证出杯（杯）',
  grantId: '授权编号',
}
</script>
<template>
  <div>
    <div v-for="r in records" :key="r.id" class="customer-record">
      <div class="row">
        <AppIcon name="file" :size="20" />
        <div>
          <strong>{{ r.id }}</strong>
          <p>{{ r.title }}</p>
        </div>
      </div>
      <div class="row">
        <StatusBadge :text="r.state" :tone="r.verified ? 'primary' : 'neutral'" /><button
          class="btn-link"
          :aria-label="`查看来源 ${r.id}`"
          @click="current = r.id"
        >
          查看
        </button>
      </div>
    </div>
    <p v-if="!records.length" class="customer-help">尚未关联业务证据，不能据此判断工作成功。</p>
    <UiDialog
      :open="Boolean(current)"
      :title="`业务来源 · ${record?.id || ''}`"
      width="620px"
      @close="current = ''"
      ><div v-if="record" class="page-stack customer-dialog">
        <CustomerAlert
          title="专业业务模块的示例投影"
          description="这里提供只读详情；真实状态需由合同、运维或财务模块查询。修改客户事项不会修改来源记录。"
        />
        <h3>{{ record.title }}</h3>
        <StatusBadge :text="record.state" :tone="record.verified ? 'success' : 'warning'" />
        <dl class="customer-info-list">
          <template v-for="(value, key) in record.facts" :key="key"
            ><dt>{{ labels[key] || key }}</dt>
            <dd>{{ value === true ? '是' : value === false ? '否' : value }}</dd></template
          >
        </dl>
      </div>
      <template #footer><button class="btn" @click="current = ''">关闭详情</button></template></UiDialog
    >
  </div>
</template>
