<script setup lang="ts">
import BaseDialog from "@/shared/ui/BaseDialog.vue";
import BaseButton from "@/shared/ui/BaseButton.vue";
import StatusBadge from "@/shared/ui/StatusBadge.vue";
import { useToast } from "@/shared/ui/useToast";
import type { AuditRecord } from "@/services/audit";
defineProps<{ record?: AuditRecord }>();
const emit = defineEmits<{ close: [] }>();
const toast = useToast();
async function copy(value: string) {
  try {
    await navigator.clipboard.writeText(value);
    toast.show("请求 ID 已复制。");
  } catch {
    toast.show("当前环境不允许访问剪贴板，请手动复制请求 ID。");
  }
}
</script>
<template>
  <BaseDialog :open="!!record" title="操作日志详情" wide @close="emit('close')"
    ><template v-if="record"
      ><div class="person-strip">
        <div>
          <h3>{{ record.action }}</h3>
          <p>{{ record.time }}</p>
        </div>
        <StatusBadge
          :tone="
            record.risk === '高风险'
              ? 'red'
              : record.risk === '中风险'
                ? 'orange'
                : 'blue'
          "
          >{{ record.risk }}</StatusBadge
        >
      </div>
      <dl class="description-grid">
        <div>
          <dt>操作人</dt>
          <dd>{{ record.actor }}</dd>
        </div>
        <div>
          <dt>操作模块</dt>
          <dd>{{ record.module }}</dd>
        </div>
        <div>
          <dt>操作对象</dt>
          <dd>{{ record.target }}</dd>
        </div>
        <div>
          <dt>执行结果</dt>
          <dd>
            <StatusBadge tone="green" dot>{{ record.result }}</StatusBadge>
          </dd>
        </div>
        <div>
          <dt>操作来源</dt>
          <dd>Web 界面演示</dd>
        </div>
        <div>
          <dt>数据范围</dt>
          <dd>当前演示企业</dd>
        </div>
      </dl>
      <h3 class="section-title separated">变更内容</h3>
      <div class="audit-detail-grid">
        <div class="audit-diff">
          <h3>变更前</h3>
          <p>{{ record.before }}</p>
        </div>
        <div class="audit-diff after">
          <h3>变更后</h3>
          <p>{{ record.after }}</p>
        </div>
      </div>
      <div class="audit-request-id">请求 ID：{{ record.requestId }}</div>
      <p class="permission-note">
        不记录密码、重置令牌或其他认证秘密。当前记录为本地 UI 操作，不是服务端审计凭证。
      </p></template
    ><template #footer
      ><BaseButton v-if="record" icon="Copy" @click="copy(record.requestId)"
        >复制请求 ID</BaseButton
      ><BaseButton variant="primary" @click="emit('close')">关闭</BaseButton></template
    ></BaseDialog
  >
</template>
