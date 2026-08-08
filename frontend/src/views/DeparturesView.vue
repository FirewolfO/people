<script setup lang="ts">
import { computed, onMounted, reactive, ref } from 'vue'
import { ElMessage, ElMessageBox, type FormInstance, type FormRules } from 'element-plus'
import { Check, Close, Plus, RefreshLeft } from '@element-plus/icons-vue'
import { apiMessage, peopleApi } from '@/api'
import { auth } from '@/auth'
import type { DepartureRequest, DepartureStatus } from '@/types'

const items = ref<DepartureRequest[]>([])
const loading = ref(false)
const applicationOpen = ref(false)
const reviewOpen = ref(false)
const saving = ref(false)
const filter = ref<'all' | DepartureStatus>('all')
const applicationRef = ref<FormInstance>()
const application = reactive({ reason: '', lastWorkingDate: '' })
const review = reactive({ id: '', stage: 'manager' as 'manager' | 'hr', approved: true, comment: '', employeeName: '' })
const rules: FormRules = {
  reason: [{ required: true, message: '请输入离职原因', trigger: 'blur' }],
  lastWorkingDate: [{ required: true, message: '请选择最后工作日', trigger: 'change' }],
}
const filtered = computed(() => filter.value === 'all' ? items.value : items.value.filter((item) => item.status === filter.value))
const canApply = computed(() => auth.state.user?.role !== 'admin' && !items.value.some((item) => item.employeeId === auth.state.user?.id && ['pending_manager', 'pending_hr'].includes(item.status)))
const statusMeta: Record<DepartureStatus, { label: string; type: 'warning' | 'primary' | 'success' | 'danger' | 'info' }> = {
  pending_manager: { label: '待负责人审批', type: 'warning' },
  pending_hr: { label: '待 HR 审批', type: 'primary' },
  approved: { label: '已通过', type: 'success' },
  rejected: { label: '已驳回', type: 'danger' },
  cancelled: { label: '已撤回', type: 'info' },
}

function employeeNo(value: number) { return String(value).padStart(6, '0') }
function disablePast(date: Date) { return date.getTime() < new Date().setHours(0, 0, 0, 0) }

async function load() {
  loading.value = true
  try {
    items.value = await peopleApi.departures()
  } catch (error) {
    ElMessage.error(apiMessage(error, '离职审批列表加载失败'))
  } finally {
    loading.value = false
  }
}

function openApplication() {
  application.reason = ''
  application.lastWorkingDate = ''
  applicationOpen.value = true
}

async function submitApplication() {
  if (!(await applicationRef.value?.validate().catch(() => false))) return
  saving.value = true
  try {
    await peopleApi.createDeparture(application)
    ElMessage.success('离职申请已提交')
    applicationOpen.value = false
    await load()
  } catch (error) {
    ElMessage.error(apiMessage(error, '提交失败'))
  } finally {
    saving.value = false
  }
}

function openReview(item: DepartureRequest, stage: 'manager' | 'hr', approved: boolean) {
  Object.assign(review, { id: item.id, stage, approved, comment: '', employeeName: item.employeeName })
  reviewOpen.value = true
}

async function submitReview() {
  saving.value = true
  try {
    await peopleApi.reviewDeparture(review.id, review.stage, { approved: review.approved, comment: review.comment })
    ElMessage.success(review.approved ? '审批已通过' : '申请已驳回')
    reviewOpen.value = false
    await load()
  } catch (error) {
    ElMessage.error(apiMessage(error, '审批失败'))
  } finally {
    saving.value = false
  }
}

async function cancel(item: DepartureRequest) {
  try {
    await ElMessageBox.confirm('确认撤回这份离职申请？', '撤回申请', { type: 'warning', confirmButtonText: '撤回', cancelButtonText: '取消' })
    await peopleApi.cancelDeparture(item.id)
    ElMessage.success('申请已撤回')
    await load()
  } catch (error) {
    if (error !== 'cancel' && error !== 'close') ElMessage.error(apiMessage(error, '撤回失败'))
  }
}

onMounted(load)
</script>

<template>
  <div class="page departures-page">
    <header class="page-header">
      <div><h1>离职审批</h1><p>申请记录与两级审批进度</p></div>
      <el-button v-if="canApply" type="primary" :icon="Plus" @click="openApplication">申请离职</el-button>
    </header>
    <div class="toolbar approval-toolbar">
      <el-segmented v-model="filter" :options="[
        { label: '全部', value: 'all' }, { label: '待负责人', value: 'pending_manager' },
        { label: '待 HR', value: 'pending_hr' }, { label: '已完成', value: 'approved' }, { label: '已驳回', value: 'rejected' },
      ]" />
    </div>
    <el-table v-loading="loading" :data="filtered" row-key="id">
      <el-table-column label="员工" min-width="150"><template #default="{ row }"><div class="identity-cell"><strong>{{ row.employeeName }}</strong><small>{{ employeeNo(row.employeeNo) }}</small></div></template></el-table-column>
      <el-table-column prop="departmentName" label="部门" min-width="130" />
      <el-table-column prop="lastWorkingDate" label="最后工作日" width="130" />
      <el-table-column prop="reason" label="离职原因" min-width="220" show-overflow-tooltip />
      <el-table-column label="审批状态" width="150"><template #default="{ row }"><el-tag :type="statusMeta[row.status as DepartureStatus].type" effect="plain">{{ statusMeta[row.status as DepartureStatus].label }}</el-tag></template></el-table-column>
      <el-table-column label="审批人" min-width="170"><template #default="{ row }"><span v-if="row.hrReviewerName">HR · {{ row.hrReviewerName }}</span><span v-else-if="row.managerReviewerName">负责人 · {{ row.managerReviewerName }}</span><span v-else class="muted">尚未审批</span></template></el-table-column>
      <el-table-column label="操作" width="178" fixed="right">
        <template #default="{ row }">
          <template v-if="row.canManagerReview"><el-button link type="success" :icon="Check" @click="openReview(row, 'manager', true)">通过</el-button><el-button link type="danger" :icon="Close" @click="openReview(row, 'manager', false)">驳回</el-button></template>
          <template v-else-if="row.canHrReview"><el-button link type="success" :icon="Check" @click="openReview(row, 'hr', true)">通过</el-button><el-button link type="danger" :icon="Close" @click="openReview(row, 'hr', false)">驳回</el-button></template>
          <el-button v-else-if="row.canCancel" link :icon="RefreshLeft" @click="cancel(row)">撤回</el-button>
          <span v-else class="muted">-</span>
        </template>
      </el-table-column>
    </el-table>

    <el-dialog v-model="applicationOpen" title="申请离职" width="min(560px, calc(100vw - 32px))" destroy-on-close>
      <el-form ref="applicationRef" :model="application" :rules="rules" label-position="top">
        <el-form-item label="最后工作日" prop="lastWorkingDate"><el-date-picker v-model="application.lastWorkingDate" type="date" value-format="YYYY-MM-DD" :disabled-date="disablePast" class="full-width" /></el-form-item>
        <el-form-item label="离职原因" prop="reason"><el-input v-model="application.reason" type="textarea" :rows="5" maxlength="1000" show-word-limit /></el-form-item>
      </el-form>
      <template #footer><el-button @click="applicationOpen = false">取消</el-button><el-button type="primary" :loading="saving" @click="submitApplication">提交申请</el-button></template>
    </el-dialog>

    <el-dialog v-model="reviewOpen" :title="review.approved ? '通过离职申请' : '驳回离职申请'" width="min(520px, calc(100vw - 32px))" destroy-on-close>
      <p class="review-target">{{ review.employeeName }} · {{ review.stage === 'manager' ? '部门负责人审批' : 'HR 终审' }}</p>
      <el-form label-position="top"><el-form-item label="审批意见"><el-input v-model="review.comment" type="textarea" :rows="4" maxlength="500" show-word-limit /></el-form-item></el-form>
      <template #footer><el-button @click="reviewOpen = false">取消</el-button><el-button :type="review.approved ? 'success' : 'danger'" :loading="saving" @click="submitReview">确认{{ review.approved ? '通过' : '驳回' }}</el-button></template>
    </el-dialog>
  </div>
</template>
