<script setup lang="ts">
import { computed, ref, watch } from "vue";
import { useRoute, useRouter } from "vue-router";
import PageHeader from "@/shared/ui/PageHeader.vue";
import BaseButton from "@/shared/ui/BaseButton.vue";
import StatusBadge from "@/shared/ui/StatusBadge.vue";
import AppIcon from "@/shared/ui/AppIcon.vue";
import EmptyState from "@/shared/ui/EmptyState.vue";
import MemberEditor from "../components/MemberEditor.vue";
import MemberOperationDialog, {
  type OperationKind,
} from "../components/MemberOperationDialog.vue";
import { useMemberStore } from "../model/memberStore";
import { useAuditStore } from "@/services/audit";
import { statusLabels } from "../model/types";
const route = useRoute();
const router = useRouter();
const store = useMemberStore();
const audit = useAuditStore();
const member = computed(() => store.members.find((m) => m.id === route.params.id));
const tab = ref("基本信息");
const editor = ref(false);
const op = ref(false);
const kind = ref<OperationKind>("role");
const records = computed(() =>
  audit.records.filter((r) => r.target === member.value?.name),
);
watch(
  () => route.params.id,
  () => {
    editor.value = false;
    op.value = false;
  },
);
function operate(value: OperationKind) {
  kind.value = value;
  op.value = true;
}
</script>
<template>
  <PageHeader title="成员详情" description="成员信息、角色权限、访问安全与生命周期记录"
    ><BaseButton icon="ChevronLeft" @click="router.push('/enterprise/members')"
      >返回成员列表</BaseButton
    ></PageHeader
  ><template v-if="member"
    ><section class="panel member-profile">
      <span class="person-avatar person-avatar--large">{{
        member.name.slice(0, 1)
      }}</span>
      <div class="member-profile-copy">
        <h2>
          {{ member.name }}
          <StatusBadge tone="blue">{{ member.role }}</StatusBadge>
        </h2>
        <p>{{ member.department }} <span>｜</span> 工号 {{ member.employeeNo }}</p>
        <p>
          <AppIcon name="Mail" :size="15" /> {{ member.email }} <span>｜</span>
          {{ member.phone }}
        </p>
        <StatusBadge :tone="member.status === 'active' ? 'green' : 'red'" dot>{{
          statusLabels[member.status]
        }}</StatusBadge>
      </div>
      <div class="profile-actions">
        <BaseButton icon="Pencil" @click="editor = true">编辑资料</BaseButton
        ><BaseButton icon="KeyRound" @click="operate('reset')">重置密码</BaseButton
        ><BaseButton icon="UserCog" @click="operate('role')">调整角色</BaseButton
        ><BaseButton
          v-if="member.status === 'active'"
          icon="Power"
          variant="danger"
          @click="operate('suspend')"
          >禁用账号</BaseButton
        ><BaseButton
          v-if="member.status === 'suspended'"
          icon="Power"
          variant="primary"
          @click="operate('enable')"
          >重新启用</BaseButton
        >
      </div>
    </section>
    <div class="detail-grid">
      <section class="panel">
        <div class="tabs">
          <button
            v-for="name in [
              '基本信息',
              '角色权限',
              '数据权限',
              '登录安全',
              '生命周期记录',
            ]"
            :key="name"
            :class="{ active: tab === name }"
            @click="tab = name"
          >
            {{ name }}
          </button>
        </div>
        <div class="panel-content">
          <template v-if="tab === '基本信息'"
            ><h3 class="section-title">个人信息</h3>
            <dl class="description-grid">
              <div>
                <dt>成员姓名</dt>
                <dd>{{ member.name }}</dd>
              </div>
              <div>
                <dt>邮箱</dt>
                <dd>{{ member.email }}</dd>
              </div>
              <div>
                <dt>手机号码</dt>
                <dd>{{ member.phone }}</dd>
              </div>
              <div>
                <dt>员工编号</dt>
                <dd>{{ member.employeeNo }}</dd>
              </div>
            </dl>
            <h3 class="section-title separated">组织信息</h3>
            <dl class="description-grid">
              <div>
                <dt>所属部门</dt>
                <dd>{{ member.department }}</dd>
              </div>
              <div>
                <dt>成员角色</dt>
                <dd>{{ member.role }}</dd>
              </div>
              <div>
                <dt>加入时间</dt>
                <dd>{{ member.joined }}</dd>
              </div>
              <div>
                <dt>直属上级</dt>
                <dd>张三</dd>
              </div>
            </dl>
            <h3 class="section-title separated">账号信息</h3>
            <dl class="description-grid">
              <div>
                <dt>账号状态</dt>
                <dd>{{ statusLabels[member.status] }}</dd>
              </div>
              <div>
                <dt>最后登录</dt>
                <dd>{{ member.lastLogin }}</dd>
              </div>
              <div>
                <dt>记录版本</dt>
                <dd>v{{ member.version }}</dd>
              </div>
              <div>
                <dt>数据来源</dt>
                <dd>本地界面演示</dd>
              </div>
            </dl></template
          ><template v-else-if="tab === '角色权限' || tab === '数据权限'"
            ><div class="permission-summary">
              <span class="metric-icon tone-blue"
                ><AppIcon name="ShieldCheck" :size="30"
              /></span>
              <div>
                <h2>
                  {{ tab === "角色权限" ? member.role : member.dataScope }}
                </h2>
                <p>功能权限与数据范围分别管理，正式授权以服务端判定为准。</p>
              </div>
            </div>
            <BaseButton icon="UserCog" @click="operate('role')"
              >调整角色与数据范围</BaseButton
            ></template
          ><template v-else-if="tab === '登录安全'"
            ><h3 class="section-title">密码与会话</h3>
            <div class="security-callout">
              <AppIcon name="LockKeyhole" :size="30" />
              <div>
                <h3>通过验证邮箱重置密码</h3>
                <p>不展示现有密码，不把重置令牌写入日志。</p>
              </div>
              <BaseButton @click="operate('reset')">重置密码</BaseButton>
            </div>
            <div class="info-banner">
              演示环境未连接认证系统；MFA、活跃会话与风险状态待真实接口接入后展示。
            </div></template
          ><template v-else
            ><div v-for="record in records" :key="record.id" class="timeline-item">
              <span />
              <div>
                <strong>{{ record.action }}</strong>
                <p>{{ record.time }} · {{ record.actor }}</p>
                <small>{{ record.after }}</small>
              </div>
            </div>
            <EmptyState
              v-if="!records.length"
              title="暂无生命周期操作记录"
              description="对该成员执行演示操作后，记录会显示在这里。"
          /></template>
        </div>
      </section>
      <aside>
        <section class="panel">
          <h3 class="panel-heading">成员访问摘要</h3>
          <div class="panel-content">
            <div class="summary-line">
              <span>所属部门</span><b>{{ member.department }}</b>
            </div>
            <div class="summary-line">
              <span>功能角色</span><b>{{ member.role }}</b>
            </div>
            <div class="summary-line">
              <span>数据范围</span><b>{{ member.dataScope }}</b>
            </div>
            <div class="summary-line">
              <span>账号状态</span><b>{{ statusLabels[member.status] }}</b>
            </div>
          </div>
        </section>
        <section class="panel note-panel">
          <AppIcon name="ShieldCheck" :size="28" />
          <h3>每一次变更，都有迹可循</h3>
          <p>信息修改、角色调整、启用禁用与密码重置均生成演示操作回执。</p>
          <RouterLink to="/enterprise/audit" class="text-button"
            >查看操作日志 <AppIcon name="ArrowRight" :size="14"
          /></RouterLink>
        </section>
      </aside>
    </div>
    <MemberEditor
      :open="editor"
      :member="member"
      @close="editor = false" /><MemberOperationDialog
      :open="op"
      :member="member"
      :kind="kind"
      @close="op = false" /></template
  ><EmptyState v-else title="成员不存在或不属于当前企业" />
</template>
