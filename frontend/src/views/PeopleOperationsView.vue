<script setup lang="ts">
import { computed, onMounted, reactive, ref } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import { Delete, Edit, Plus } from '@element-plus/icons-vue'
import { apiMessage, peopleApi } from '@/api'
import { auth } from '@/auth'
import type { ContractInput, Employee, EmployeeContract, GoalInput, LeaveBalance, LeaveRecord, PerformanceGoal } from '@/types'

const activeTab = ref('leave')
const loading = ref(false)
const saving = ref(false)
const balance = ref<LeaveBalance | null>(null)
const leaveRecords = ref<LeaveRecord[]>([])
const contracts = ref<EmployeeContract[]>([])
const goals = ref<PerformanceGoal[]>([])
const employees = ref<Employee[]>([])
const month = ref(new Date().toISOString().slice(0, 7))
const contractOpen = ref(false)
const goalOpen = ref(false)
const editingContract = ref('')
const editingGoal = ref('')
const reviewingGoal = ref(false)
const canManageContracts = computed(() => auth.can('people.contract:manage'))
const canManagePerformance = computed(() => auth.can('people.performance:manage'))
const contractForm = reactive<ContractInput & { employeeId: string }>({ employeeId: '', type: 'fixed_term', startDate: '', endDate: '', status: 'active', documentName: '', note: '' })
const goalForm = reactive<GoalInput>({ cycle: '2026-H2', title: '', description: '', dueDate: '', weight: 100, progress: 0, status: 'active', managerComment: '' })

const leaveLabels: Record<string, string> = { annual: '年假', sick: '病假', personal: '事假', other: '其他' }
const contractTypeLabels: Record<string, string> = { fixed_term: '固定期限', open_ended: '无固定期限', internship: '实习协议', service: '服务协议' }
const contractStatusLabels: Record<string, string> = { active: '生效中', ended: '已到期', terminated: '已终止' }
const goalStatusLabels: Record<string, string> = { draft: '草稿', active: '进行中', completed: '已完成', cancelled: '已取消' }

async function loadLeave() {
  loading.value = true
  try {
    ;[balance.value, leaveRecords.value] = await Promise.all([peopleApi.leaveBalance(), peopleApi.leaveCalendar(month.value)])
  } catch (error) {
    ElMessage.error(apiMessage(error, '假期数据加载失败'))
  } finally {
    loading.value = false
  }
}

async function loadContracts() {
  loading.value = true
  try { contracts.value = await peopleApi.contracts() } catch (error) { ElMessage.error(apiMessage(error, '合同台账加载失败')) } finally { loading.value = false }
}

async function loadGoals() {
  loading.value = true
  try { goals.value = await peopleApi.goals() } catch (error) { ElMessage.error(apiMessage(error, '绩效目标加载失败')) } finally { loading.value = false }
}

function tabChanged(name: string | number) {
  if (name === 'leave') void loadLeave()
  if (name === 'contracts') void loadContracts()
  if (name === 'goals') void loadGoals()
}

function newContract() {
  editingContract.value = ''
  Object.assign(contractForm, { employeeId: '', type: 'fixed_term', startDate: '', endDate: '', status: 'active', documentName: '', note: '' })
  contractOpen.value = true
}

function editContract(item: EmployeeContract) {
  editingContract.value = item.id
  Object.assign(contractForm, { employeeId: item.employeeId, type: item.type, startDate: item.startDate, endDate: item.endDate, status: item.status, documentName: item.documentName, note: item.note })
  contractOpen.value = true
}

async function saveContract() {
  if (!contractForm.employeeId || !contractForm.startDate || (contractForm.type !== 'open_ended' && !contractForm.endDate)) { ElMessage.warning('请完整填写合同信息'); return }
  saving.value = true
  try {
    const input: ContractInput = { type: contractForm.type, startDate: contractForm.startDate, endDate: contractForm.endDate, status: contractForm.status, documentName: contractForm.documentName, note: contractForm.note }
    if (editingContract.value) await peopleApi.updateContract(editingContract.value, input)
    else await peopleApi.createContract(contractForm.employeeId, input)
    ElMessage.success('合同已保存')
    contractOpen.value = false
    await loadContracts()
  } catch (error) { ElMessage.error(apiMessage(error, '合同保存失败')) } finally { saving.value = false }
}

async function deleteContract(item: EmployeeContract) {
  try {
    await ElMessageBox.confirm(`确认删除 ${item.employeeName} 的合同记录？`, '删除合同', { type: 'warning', confirmButtonText: '删除', cancelButtonText: '取消' })
    await peopleApi.deleteContract(item.id)
    ElMessage.success('合同已删除')
    await loadContracts()
  } catch (error) { if (error !== 'cancel' && error !== 'close') ElMessage.error(apiMessage(error, '删除失败')) }
}

function newGoal() {
  editingGoal.value = ''
  reviewingGoal.value = false
  Object.assign(goalForm, { cycle: '2026-H2', title: '', description: '', dueDate: '', weight: 100, progress: 0, status: 'active', managerComment: '' })
  goalOpen.value = true
}

function editGoal(item: PerformanceGoal, reviewOnly = false) {
  editingGoal.value = item.id
  reviewingGoal.value = reviewOnly
  Object.assign(goalForm, { cycle: item.cycle, title: item.title, description: item.description, dueDate: item.dueDate, weight: item.weight, progress: item.progress, status: item.status, managerComment: item.managerComment })
  goalOpen.value = true
}

async function saveGoal() {
  if (!goalForm.cycle || !goalForm.title || !goalForm.dueDate) { ElMessage.warning('请完整填写目标信息'); return }
  saving.value = true
  try {
    if (editingGoal.value) await peopleApi.updateGoal(editingGoal.value, goalForm)
    else await peopleApi.createGoal(goalForm)
    ElMessage.success(reviewingGoal.value ? '评语已保存' : '目标已保存')
    goalOpen.value = false
    await loadGoals()
  } catch (error) { ElMessage.error(apiMessage(error, '目标保存失败')) } finally { saving.value = false }
}

onMounted(async () => {
  if (canManageContracts.value) {
    try { employees.value = (await peopleApi.employees({ page: 1, pageSize: 100 })).items } catch { employees.value = [] }
  }
  await loadLeave()
})
</script>

<template>
  <div class="page operations-page">
    <header class="page-header"><div><h1>人事运营</h1><p>假期、合同与绩效目标统一管理</p></div></header>
    <el-tabs v-model="activeTab" class="operations-tabs" @tab-change="tabChanged">
      <el-tab-pane label="假期与休假" name="leave">
        <section v-loading="loading">
          <div v-if="balance" class="balance-strip">
            <div><small>{{ balance.year }} 年年假额度</small><strong>{{ balance.annualEntitlement }} 天</strong></div>
            <div><small>已使用</small><strong>{{ balance.annualUsed }} 天</strong></div>
            <div><small>审批中</small><strong>{{ balance.annualPending }} 天</strong></div>
            <div class="balance-primary"><small>可用余额</small><strong>{{ balance.annualRemaining }} 天</strong></div>
          </div>
          <div class="section-toolbar"><div><h2>休假日历</h2><p>本人及可见团队的休假安排</p></div><el-date-picker v-model="month" type="month" value-format="YYYY-MM" @change="loadLeave" /></div>
          <el-table :data="leaveRecords" row-key="id" empty-text="本月暂无休假记录">
            <el-table-column prop="employeeName" label="员工" min-width="130" /><el-table-column prop="departmentName" label="部门" min-width="120" />
            <el-table-column label="类型" width="90"><template #default="{ row }">{{ leaveLabels[row.leaveType] || row.leaveType }}</template></el-table-column>
            <el-table-column label="日期" min-width="200"><template #default="{ row }">{{ row.startDate }} 至 {{ row.endDate }}</template></el-table-column>
            <el-table-column prop="days" label="工作日" width="90"><template #default="{ row }">{{ row.days }} 天</template></el-table-column>
            <el-table-column label="状态" width="100"><template #default="{ row }"><el-tag :type="row.status === 'approved' ? 'success' : row.status === 'pending' ? 'warning' : 'info'" effect="plain">{{ row.status === 'approved' ? '已批准' : row.status === 'pending' ? '审批中' : '已结束' }}</el-tag></template></el-table-column>
          </el-table>
        </section>
      </el-tab-pane>
      <el-tab-pane label="合同台账" name="contracts">
        <section v-loading="loading">
          <div class="section-toolbar"><div><h2>劳动合同</h2><p>生效、到期与终止状态跟踪</p></div><el-button v-if="canManageContracts" type="primary" :icon="Plus" @click="newContract">新增合同</el-button></div>
          <el-table :data="contracts" row-key="id" empty-text="暂无合同记录">
            <el-table-column prop="employeeName" label="员工" min-width="130" /><el-table-column label="合同类型" min-width="120"><template #default="{ row }">{{ contractTypeLabels[row.type] }}</template></el-table-column>
            <el-table-column label="期限" min-width="210"><template #default="{ row }">{{ row.startDate }} 至 {{ row.endDate || '长期' }}</template></el-table-column>
            <el-table-column prop="documentName" label="文档编号/名称" min-width="150"><template #default="{ row }">{{ row.documentName || '-' }}</template></el-table-column>
            <el-table-column label="状态" width="100"><template #default="{ row }"><el-tag :type="row.status === 'active' ? 'success' : 'info'" effect="plain">{{ contractStatusLabels[row.status] }}</el-tag></template></el-table-column>
            <el-table-column v-if="canManageContracts" label="操作" width="100"><template #default="{ row }"><el-tooltip content="编辑合同"><el-button link :icon="Edit" @click="editContract(row)" /></el-tooltip><el-tooltip content="删除合同"><el-button link type="danger" :icon="Delete" @click="deleteContract(row)" /></el-tooltip></template></el-table-column>
          </el-table>
        </section>
      </el-tab-pane>
      <el-tab-pane label="绩效目标" name="goals">
        <section v-loading="loading">
          <div class="section-toolbar"><div><h2>目标与进度</h2><p>员工自助维护，负责人持续反馈</p></div><el-button type="primary" :icon="Plus" @click="newGoal">新增目标</el-button></div>
          <el-table :data="goals" row-key="id" empty-text="暂无绩效目标">
            <el-table-column label="目标" min-width="220"><template #default="{ row }"><div class="identity-cell"><strong>{{ row.title }}</strong><small>{{ row.employeeName }} · {{ row.cycle }}</small></div></template></el-table-column>
            <el-table-column prop="dueDate" label="截止日期" width="120" /><el-table-column prop="weight" label="权重" width="80"><template #default="{ row }">{{ row.weight }}%</template></el-table-column>
            <el-table-column label="进度" min-width="170"><template #default="{ row }"><el-progress :percentage="row.progress" :status="row.progress === 100 ? 'success' : undefined" /></template></el-table-column>
            <el-table-column label="状态" width="100"><template #default="{ row }">{{ goalStatusLabels[row.status] }}</template></el-table-column>
            <el-table-column label="操作" width="150"><template #default="{ row }"><el-button v-if="row.canEdit" link :icon="Edit" @click="editGoal(row)">维护</el-button><el-button v-if="row.canReview" link type="primary" @click="editGoal(row, true)">反馈</el-button></template></el-table-column>
          </el-table>
        </section>
      </el-tab-pane>
    </el-tabs>

    <el-dialog v-model="contractOpen" :title="editingContract ? '编辑合同' : '新增合同'" width="min(640px, calc(100vw - 32px))" destroy-on-close>
      <el-form label-position="top" class="employee-form">
        <el-form-item label="员工"><el-select v-model="contractForm.employeeId" filterable :disabled="Boolean(editingContract)" class="full-width"><el-option v-for="item in employees" :key="item.id" :label="`${item.displayName} · ${String(item.employeeNo).padStart(6, '0')}`" :value="item.id" /></el-select></el-form-item>
        <el-form-item label="合同类型"><el-select v-model="contractForm.type" class="full-width"><el-option label="固定期限" value="fixed_term" /><el-option label="无固定期限" value="open_ended" /><el-option label="实习协议" value="internship" /><el-option label="服务协议" value="service" /></el-select></el-form-item>
        <el-form-item label="开始日期"><el-date-picker v-model="contractForm.startDate" type="date" value-format="YYYY-MM-DD" class="full-width" /></el-form-item>
        <el-form-item label="结束日期"><el-date-picker v-model="contractForm.endDate" type="date" value-format="YYYY-MM-DD" :disabled="contractForm.type === 'open_ended'" class="full-width" /></el-form-item>
        <el-form-item label="状态"><el-select v-model="contractForm.status" class="full-width"><el-option label="生效中" value="active" /><el-option label="已到期" value="ended" /><el-option label="已终止" value="terminated" /></el-select></el-form-item>
        <el-form-item label="文档编号/名称"><el-input v-model="contractForm.documentName" /></el-form-item>
        <el-form-item label="备注" class="form-span"><el-input v-model="contractForm.note" type="textarea" :rows="3" maxlength="1000" show-word-limit /></el-form-item>
      </el-form>
      <template #footer><el-button @click="contractOpen = false">取消</el-button><el-button type="primary" :loading="saving" @click="saveContract">保存</el-button></template>
    </el-dialog>

    <el-dialog v-model="goalOpen" :title="reviewingGoal ? '负责人反馈' : editingGoal ? '维护目标' : '新增目标'" width="min(640px, calc(100vw - 32px))" destroy-on-close>
      <el-form label-position="top" class="employee-form">
        <el-form-item label="周期"><el-input v-model="goalForm.cycle" :disabled="reviewingGoal" /></el-form-item><el-form-item label="目标名称"><el-input v-model="goalForm.title" :disabled="reviewingGoal" maxlength="160" /></el-form-item>
        <el-form-item label="截止日期"><el-date-picker v-model="goalForm.dueDate" type="date" value-format="YYYY-MM-DD" :disabled="reviewingGoal" class="full-width" /></el-form-item><el-form-item label="权重"><el-input-number v-model="goalForm.weight" :min="1" :max="100" :disabled="reviewingGoal" /></el-form-item>
        <el-form-item label="完成进度"><el-slider v-model="goalForm.progress" :disabled="reviewingGoal" show-input /></el-form-item><el-form-item label="状态"><el-select v-model="goalForm.status" :disabled="reviewingGoal" class="full-width"><el-option label="草稿" value="draft" /><el-option label="进行中" value="active" /><el-option label="已完成" value="completed" /><el-option label="已取消" value="cancelled" /></el-select></el-form-item>
        <el-form-item label="目标说明" class="form-span"><el-input v-model="goalForm.description" type="textarea" :rows="3" :disabled="reviewingGoal" maxlength="1000" show-word-limit /></el-form-item>
        <el-form-item label="负责人反馈" class="form-span"><el-input v-model="goalForm.managerComment" type="textarea" :rows="3" :disabled="!reviewingGoal && !canManagePerformance" maxlength="1000" show-word-limit /></el-form-item>
      </el-form>
      <template #footer><el-button @click="goalOpen = false">取消</el-button><el-button type="primary" :loading="saving" @click="saveGoal">保存</el-button></template>
    </el-dialog>
  </div>
</template>
