<script setup lang="ts">
import { ref, watch } from 'vue'
import { useCustomerStore } from '@/stores/customer'
import type { RentalSite } from '@/types/siteRental'
import { rentalState } from '@/services/siteRental/model'
import { operators } from '@/services/customer/seed'
import UiDialog from '@/components/ui/UiDialog.vue'
import CustomerAlert from '@/components/customer/CustomerAlert.vue'
const props = defineProps<{ open: boolean; siteId?: string }>(),
  emit = defineEmits<{ close: []; saved: [id: string] }>()
const store = useCustomerStore(),
  form = ref<RentalSite>(),
  error = ref(''),
  discard = ref(false),
  revision = ref(0),
  tenant = ref(''),
  initial = ref(''),
  key = ref('')
watch(
  () => props.open,
  (open) => {
    if (!open) return
    store.refreshRental()
    const site = rentalState(store.snapshot).profiles.find((x) => x.id === props.siteId)
    form.value = site
      ? JSON.parse(JSON.stringify(site))
      : {
          id: '',
          customerId: '',
          name: '',
          parent: '',
          kind: 'site',
          scene: '企业办公',
          address: '',
          contact: '',
          phone: '',
          owner: '张敏',
          hours: '工作日 09:00—18:00',
          access: '',
          water: '待勘察',
          power: '待勘察',
          network: '待勘察',
          cleaning: '待确认',
          supplies: '待确认',
          phase: '待勘察',
          operation: '正常运营',
          service: '待核实',
          nextAction: '安排现场条件勘察',
          nextAt: '',
          version: 1,
        }
    revision.value = store.snapshot.revision
    tenant.value = store.snapshot.tenant
    key.value = crypto.randomUUID()
    initial.value = JSON.stringify(form.value)
    error.value = ''
    discard.value = false
  },
  { immediate: true },
)
function close() {
  if (JSON.stringify(form.value) !== initial.value) discard.value = true
  else emit('close')
}
function save() {
  try {
    if (!form.value) return
    const result = store.runRental({
      tenant: tenant.value,
      expectedRevision: revision.value,
      key: key.value,
      action: { type: 'save-site', site: form.value },
    })
    emit('saved', result.target)
    emit('close')
  } catch (e) {
    error.value = (e as Error).message
  }
}
</script>
<template>
  <UiDialog
    :open="open"
    :title="siteId ? '编辑点位资料' : '新建点位档案'"
    width="740px"
    drawer
    @close="close"
  >
    <form v-if="form" id="rental-site-form" class="rental-form page-stack" @submit.prevent="save">
      <CustomerAlert
        title="点位是履约与数据单位，设备是可替换资产"
        description="建档不等于投放或起租；合同规则、设备投放和数据授权分别确认。"
      />
      <h3>身份与位置</h3>
      <div class="rental-field-grid">
        <label
          >所属客户 <b>*</b
          ><select v-model="form.customerId" required :disabled="!!siteId">
            <option disabled value="">请选择客户</option>
            <option
              v-for="c in store.snapshot.customers.filter((x) => !x.archived)"
              :key="c.id"
              :value="c.id"
            >
              {{ c.name }}
            </option>
          </select></label
        >
        <label
          >节点类型 <b>*</b
          ><select v-model="form.kind" :disabled="!!siteId">
            <option value="site">实际服务点位</option>
            <option value="group">组织分组（不计费、不投放）</option>
          </select></label
        >
        <label>点位名称 <b>*</b><input v-model="form.name" required maxlength="80" /></label
        ><label
          >上级位置 / 分组 <b>*</b><input v-model="form.parent" required placeholder="如 上海总部 / 3 楼"
        /></label>
        <label class="rental-full"
          >详细地址 <b>*</b><input v-model="form.address" required placeholder="城市、楼宇、楼层与具体位置"
        /></label>
        <label
          >使用场景<select v-model="form.scene">
            <option>企业办公</option>
            <option>酒店公共空间</option>
            <option>商业场馆</option>
            <option>其他</option>
          </select></label
        ><label>开放时间<input v-model="form.hours" /></label>
      </div>
      <h3>责任人与现场约束</h3>
      <div class="rental-field-grid">
        <label
          >出租方点位负责人 <b>*</b
          ><select v-model="form.owner" required>
            <option v-for="owner in operators" :key="owner">{{ owner }}</option>
          </select></label
        ><label>客户现场联系人 <b>*</b><input v-model="form.contact" required /></label>
        <label>联系方式<input v-model="form.phone" placeholder="允许展示脱敏联系方式" /></label
        ><label>清洁责任<input v-model="form.cleaning" /></label>
        <label
          >补货责任<select v-model="form.supplies">
            <option>待确认</option>
            <option>出租方补货</option>
            <option>客户自行采购</option>
          </select></label
        ><label>下次行动<input v-model="form.nextAt" type="datetime-local" /></label>
        <label class="rental-full"
          >进场与服务约束<textarea
            v-model="form.access"
            rows="2"
            placeholder="进场预约、货梯、工作日与安全要求"
          />
        </label>
        <label>供水条件<input v-model="form.water" /></label
        ><label>电源条件<input v-model="form.power" /></label
        ><label>网络条件<input v-model="form.network" /></label
        ><label>下一步行动<input v-model="form.nextAction" /></label>
      </div>
      <p v-if="error" role="alert" class="rental-error">{{ error }}</p>
      <CustomerAlert v-if="discard" title="存在未保存修改" tone="warning"
        ><button type="button" class="btn" @click="discard = false">继续编辑</button>
        <button type="button" class="btn btn-danger" @click="emit('close')">放弃修改</button></CustomerAlert
      >
    </form>
    <template #footer
      ><button class="btn" @click="close">取消</button
      ><button class="btn btn-primary" type="submit" form="rental-site-form">
        保存点位（预览）
      </button></template
    >
  </UiDialog>
</template>
