<script setup lang="ts">
import { computed, ref } from 'vue'
import { useCustomerStore } from '@/stores/customer'
import { parseCustomerCsv, validateImport, type ImportRow } from '@/services/customer/importer'
import { downloadCsv } from '@/utils/format'
import { operators } from '@/services/customer/seed'
import PageHeading from '@/components/ui/PageHeading.vue'
import MetricCard from '@/components/ui/MetricCard.vue'
import StatusBadge from '@/components/ui/StatusBadge.vue'
import UiDialog from '@/components/ui/UiDialog.vue'
import CustomerSection from '@/components/customer/CustomerSection.vue'
import CustomerAlert from '@/components/customer/CustomerAlert.vue'
const store = useCustomerStore(),
  stage = ref(0),
  raw = ref(''),
  fileName = ref(''),
  data = ref<string[][]>([]),
  rows = ref<ImportRow[]>([]),
  error = ref(''),
  batch = ref(''),
  mapping = ref({ name: 0, category: 1, owner: 2 }),
  edit = ref<ImportRow>()
const sample = [
  ['客户名称', '客户类型', '客户负责人'],
  ['松屿商务中心', '写字楼 / 园区', '李川'],
  ['星悦酒店集团', '连锁酒店', '张敏'],
  ['青禾连锁公寓', '公寓', '张敏'],
  ['溪桥产业园', '产业园', '无效成员'],
  ['晨光创意园', '产业园', '陈晓'],
  ['云间商务酒店', '连锁酒店', '李川'],
  ['江湾办公中心', '企业办公', '王宁'],
  ['青桐书店', '连锁零售', '张敏'],
]
const selected = computed(() => rows.value.filter((r) => r.selected && r.result === '通过'))
function read() {
  error.value = ''
  try {
    data.value = parseCustomerCsv(raw.value)
    stage.value = 1
    batch.value = crypto.randomUUID()
  } catch (e) {
    error.value = (e as Error).message
  }
}
async function loadFile(event: Event) {
  const file = (event.target as HTMLInputElement).files?.[0]
  if (!file) return
  if (file.size > 1024 * 1024) {
    error.value = 'CSV 文件不能超过 1 MB'
    return
  }
  raw.value = await file.text()
  fileName.value = file.name
  read()
}
function loadSample() {
  raw.value = sample.map((r) => r.join(',')).join('\n')
  fileName.value = '客户导入示例.csv'
  read()
  validate()
}
function validate() {
  error.value = ''
  try {
    rows.value = validateImport(data.value, mapping.value, store.snapshot)
    stage.value = 2
  } catch (e) {
    error.value = (e as Error).message
  }
}
function commit() {
  error.value = ''
  try {
    rows.value = store.importRows(rows.value, batch.value)
    stage.value = 4
  } catch (e) {
    error.value = (e as Error).message
  }
}
function fix() {
  if (!edit.value) return
  const r = edit.value
  data.value[r.line - 1]![mapping.value.name] = r.name
  data.value[r.line - 1]![mapping.value.owner] = r.owner
  data.value[r.line - 1]![mapping.value.category] = r.category
  edit.value = undefined
  validate()
}
</script>
<template>
  <div class="page-stack">
    <PageHeading
      title="客户导入 · 逐行校验"
      breadcrumb="客户经营"
      description="先校验重复与归属，再逐行确认导入结果"
      ><div class="customer-heading-actions">
        <button class="btn" @click="downloadCsv('客户导入模板.csv', [sample[0]!])">下载 CSV 模板</button
        ><RouterLink class="btn" to="/customers">返回客户总览</RouterLink>
      </div></PageHeading
    >
    <div class="card customer-flow">
      <div
        v-for="(text, index) in ['选择文件', '字段映射', '逐行校验', '确认导入', '结果回读']"
        :key="text"
        class="customer-flow-step"
        :class="{ done: index < stage, current: index === stage }"
      >
        <span>{{ index + 1 }}</span
        >{{ text }}
      </div>
    </div>
    <CustomerAlert v-if="error" title="导入未执行" :description="error" tone="danger" />
    <CustomerSection v-if="stage === 0" title="选择导入文件" icon="upload"
      ><div class="customer-empty">
        <h3>上传 UTF-8 CSV 客户资料</h3>
        <p>支持客户名称、客户类型、客户负责人三项字段。最多 500 行，文件不超过 1 MB。</p>
        <input type="file" accept=".csv,text/csv" aria-label="选择客户 CSV 文件" @change="loadFile" /><button
          class="btn"
          @click="loadSample"
        >
          载入示例文件并校验
        </button>
      </div></CustomerSection
    >
    <CustomerSection v-if="stage === 1" title="字段映射" icon="organization"
      ><p class="customer-help" style="margin-bottom: 20px">
        {{ fileName }} · 共 {{ Math.max(0, data.length - 1) }} 行记录
      </p>
      <div class="form-grid">
        <label
          v-for="(label, key) in { name: '客户名称', category: '客户类型', owner: '客户负责人' }"
          :key="key"
          class="field"
          ><span>{{ label }}</span
          ><select v-model.number="mapping[key]" class="select">
            <option v-for="(head, index) in data[0]" :key="index" :value="index">{{ head }}</option>
          </select></label
        >
      </div>
      <button class="btn btn-primary" style="margin-top: 20px" @click="validate">
        逐行校验
      </button></CustomerSection
    >
    <template v-if="stage >= 2"
      ><div class="metric-grid">
        <MetricCard
          label="待导入记录"
          :value="rows.length"
          unit="条"
          icon="file"
          caption="当前文件与租户"
        /><MetricCard
          label="可以创建"
          :value="rows.filter((r) => r.result === '通过').length"
          unit="条"
          icon="checks"
          tone="green"
          caption="必填字段与负责人校验通过"
        /><MetricCard
          label="重复待确认"
          :value="rows.filter((r) => r.result === '疑似重复').length"
          unit="条"
          icon="copy"
          tone="orange"
          caption="不自动覆盖已有客户"
        /><MetricCard
          :label="stage === 4 ? '已导入' : '字段错误'"
          :value="rows.filter((r) => r.result === (stage === 4 ? '已导入' : '字段错误')).length"
          unit="条"
          icon="warning"
          tone="purple"
          :caption="stage === 4 ? '返回实际创建编号' : '修正后重新校验'"
        />
      </div>
      <CustomerSection title="逐行校验结果" icon="checks"
        ><div class="table-scroll">
          <table class="data-table">
            <thead>
              <tr>
                <th class="check-cell">选择</th>
                <th>行号</th>
                <th>客户名称</th>
                <th>客户负责人</th>
                <th>校验结果</th>
                <th>处理建议 / 回执</th>
                <th>操作</th>
              </tr>
            </thead>
            <tbody>
              <tr v-for="row in rows" :key="row.line">
                <td>
                  <input
                    v-model="row.selected"
                    type="checkbox"
                    :disabled="row.result !== '通过'"
                    :aria-label="`选择导入第 ${row.line} 行`"
                  />
                </td>
                <td>{{ row.line }}</td>
                <td>{{ row.name }}</td>
                <td>{{ row.owner }}</td>
                <td>
                  <StatusBadge
                    :text="row.result"
                    :tone="
                      row.result === '通过' || row.result === '已导入'
                        ? 'success'
                        : row.result === '疑似重复'
                          ? 'warning'
                          : 'danger'
                    "
                  />
                </td>
                <td>{{ row.reason }}</td>
                <td>
                  <RouterLink v-if="row.existing" :to="`/customers/accounts/${row.existing}`" class="btn-link"
                    >查看已有客户</RouterLink
                  ><button v-else class="btn-link" @click="edit = { ...row }">修改字段</button>
                </td>
              </tr>
            </tbody>
          </table>
        </div></CustomerSection
      ><CustomerAlert
        :title="stage === 4 ? '导入结果已回读' : '本轮只导入已通过且明确勾选的记录'"
        description="重复与错误行保留，不自动覆盖。结果返回创建编号与失败原因，重复提交不重复创建。"
        :tone="stage === 4 ? 'success' : 'primary'"
      />
      <div class="customer-inline-actions" style="justify-content: flex-end">
        <button class="btn" @click="stage = 1">返回字段映射</button
        ><button class="btn" @click="validate">修正后重新校验</button
        ><button v-if="stage !== 4" class="btn btn-primary" :disabled="!selected.length" @click="stage = 3">
          仅导入通过的 {{ selected.length }} 条</button
        ><RouterLink v-else to="/customers" class="btn btn-primary">查看客户列表</RouterLink>
      </div></template
    >
    <UiDialog :open="stage === 3" title="确认导入客户" width="540px" @close="stage = 2"
      ><CustomerAlert
        :title="`将创建 ${selected.length} 条客户资料`"
        description="只导入已勾选且通过的行，不覆盖同名记录。保存时会再次校验重复。"
      /><template #footer
        ><button class="btn" @click="stage = 2">取消</button
        ><button class="btn btn-primary" @click="commit">确认导入</button></template
      ></UiDialog
    >
    <UiDialog :open="Boolean(edit)" title="修正导入字段" width="560px" @close="edit = undefined"
      ><form v-if="edit" id="import-edit" class="page-stack" @submit.prevent="fix">
        <label class="field"><span>客户名称</span><input v-model="edit.name" class="input" required /></label
        ><label class="field"
          ><span>客户类型</span><input v-model="edit.category" class="input" required /></label
        ><label class="field"
          ><span>有效负责人</span
          ><select v-model="edit.owner" class="select" required>
            <option value="">请选择</option>
            <option v-for="owner in operators" :key="owner">{{ owner }}</option>
          </select></label
        >
      </form>
      <template #footer
        ><button class="btn" @click="edit = undefined">取消</button
        ><button class="btn btn-primary" type="submit" form="import-edit">保存并重新校验</button></template
      ></UiDialog
    >
  </div>
</template>
