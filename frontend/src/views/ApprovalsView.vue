<script setup lang="ts">
import { computed, onMounted, reactive, ref } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import { Check, Close, Plus, RefreshLeft, View } from '@element-plus/icons-vue'
import { apiMessage, peopleApi } from '@/api'
import { auth } from '@/auth'
import type { ApprovalRequest, ApprovalStatus, ApprovalType, ApprovalTypeDefinition, Department } from '@/types'

const items = ref<ApprovalRequest[]>([])
const types = ref<ApprovalTypeDefinition[]>([])
const departments = ref<Department[]>([])
const loading = ref(false)
const saving = ref(false)
const applicationOpen = ref(false)
const reviewOpen = ref(false)
const detailOpen = ref(false)
const selected = ref<ApprovalRequest | null>(null)
const scope = ref<'mine' | 'pending' | 'all'>('mine')
const typeFilter = ref<ApprovalType | ''>('')
const statusFilter = ref<ApprovalStatus | ''>('')
const canViewAll = computed(() => auth.can('people.approval:view'))
const form = reactive({ type: 'leave' as ApprovalType, leaveType: 'annual', startDate: '', endDate: '', reason: '', lastWorkingDate: '', targetDepartmentId: '', targetTitle: '', effectiveDate: '' })
const review = reactive({ id: '', approved: true, comment: '', title: '' })

const typeMeta: Record<ApprovalType, { label: string; tone: 'success' | 'warning' | 'primary' }> = {
  leave: { label: '请假', tone: 'success' },
  transfer: { label: '岗位异动', tone: 'primary' },
  departure: { label: '离职', tone: 'warning' },
}
const statusMeta: Record<ApprovalStatus, { label: string; tone: 'warning' | 'success' | 'danger' | 'info' }> = {
  pending: { label: '审批中', tone: 'warning' }, approved: { label: '已通过', tone: 'success' },
  rejected: { label: '已驳回', tone: 'danger' }, cancelled: { label: '已撤回', tone: 'info' },
}
const leaveLabels: Record<string, string> = { annual: '年假', sick: '病假', personal: '事假', other: '其他' }

function employeeNo(value: number) { return String(value).padStart(6, '0') }
function disablePast(date: Date) { return date.getTime() < new Date().setHours(0, 0, 0, 0) }

async function load() {
  loading.value = true
  try {
    items.value = await peopleApi.approvals({ scope: scope.value, type: typeFilter.value, status: statusFilter.value })
  } catch (error) {
    ElMessage.error(apiMessage(error, '审批列表加载失败'))
  } finally {
    loading.value = false
  }
}

function openApplication() {
  Object.assign(form, { type: 'leave', leaveType: 'annual', startDate: '', endDate: '', reason: '', lastWorkingDate: '', targetDepartmentId: '', targetTitle: '', effectiveDate: '' })
  applicationOpen.value = true
}

async function submitApplication() {
  const data: Record<string, string> = { reason: form.reason }
  if (form.type === 'leave') Object.assign(data, { leaveType: form.leaveType, startDate: form.startDate, endDate: form.endDate })
  if (form.type === 'departure') data.lastWorkingDate = form.lastWorkingDate
  if (form.type === 'transfer') Object.assign(data, { targetDepartmentId: form.targetDepartmentId, targetTitle: form.targetTitle, effectiveDate: form.effectiveDate })
  if ((form.type === 'leave' && (!form.startDate || !form.endDate)) || (form.type === 'departure' && (!form.lastWorkingDate || !form.reason)) || (form.type === 'transfer' && (!form.targetDepartmentId || !form.targetTitle || !form.effectiveDate))) {
    ElMessage.warning('请完整填写申请信息')
    return
  }
  saving.value = true
  try {
    await peopleApi.createApproval({ type: form.type, data })
    ElMessage.success('申请已提交')
    applicationOpen.value = false
    scope.value = 'mine'
    await load()
  } catch (error) {
    ElMessage.error(apiMessage(error, '申请提交失败'))
  } finally {
    saving.value = false
  }
}

function openReview(item: ApprovalRequest, approved: boolean) {
  Object.assign(review, { id: item.id, approved, comment: '', title: item.title })
  reviewOpen.value = true
}

async function submitReview() {
  saving.value = true
  try {
    await peopleApi.reviewApproval(review.id, { approved: review.approved, comment: review.comment })
    ElMessage.success(review.approved ? '审批已通过' : '申请已驳回')
    reviewOpen.value = false
    await load()
  } catch (error) {
    ElMessage.error(apiMessage(error, '审批失败'))
  } finally {
    saving.value = false
  }
}

async function cancel(item: ApprovalRequest) {
  try {
    await ElMessageBox.confirm(`确认撤回“${item.title}”？`, '撤回申请', { type: 'warning', confirmButtonText: '撤回', cancelButtonText: '取消' })
    await peopleApi.cancelApproval(item.id)
    ElMessage.success('申请已撤回')
    await load()
  } catch (error) {
    if (error !== 'cancel' && error !== 'close') ElMessage.error(apiMessage(error, '撤回失败'))
  }
}

function showDetail(item: ApprovalRequest) {
  selected.value = item
  detailOpen.value = true
}

function detailRows(item: ApprovalRequest) {
  if (item.type === 'leave') return [
    ['请假类型', leaveLabels[String(item.data.leaveType)] || item.data.leaveType], ['开始日期', item.data.startDate], ['结束日期', item.data.endDate], ['工作日', `${item.data.days} 天`], ['事由', item.data.reason || '-'],
  ]
  if (item.type === 'transfer') return [
    ['目标部门', item.data.targetDepartmentName], ['目标职务', item.data.targetTitle], ['生效日期', item.data.effectiveDate], ['异动原因', item.data.reason || '-'],
  ]
  return [['最后工作日', item.data.lastWorkingDate], ['离职原因', item.data.reason]]
}

onMounted(async () => {
  try {
    [types.value, departments.value] = await Promise.all([peopleApi.approvalTypes(), peopleApi.departments()])
  } catch (error) {
    ElMessage.error(apiMessage(error, '审批基础数据加载失败'))
  }
  await load()
})
</script>

<template>
  <div class="page approvals-page">
    <header class="page-header">
      <div><h1>审批中心</h1><p>请假、岗位异动与离职统一流转</p></div>
      <el-button v-if="auth.state.user?.role !== 'admin'" type="primary" :icon="Plus" @click="openApplication">发起申请</el-button>
    </header>
    <div class="toolbar approval-filters">
      <el-segmented v-model="scope" :options="[
        { label: '我的申请', value: 'mine' }, { label: '待我审批', value: 'pending' }, ...(canViewAll ? [{ label: '全部流程', value: 'all' }] : []),
      ]" @change="load" />
      <el-select v-model="typeFilter" clearable placeholder="全部类型" @change="load"><el-option v-for="item in types" :key="item.code" :label="item.name" :value="item.code" /></el-select>
      <el-select v-model="statusFilter" clearable placeholder="全部状态" @change="load"><el-option label="审批中" value="pending" /><el-option label="已通过" value="approved" /><el-option label="已驳回" value="rejected" /><el-option label="已撤回" value="cancelled" /></el-select>
    </div>
    <el-table v-loading="loading" :data="items" row-key="id" empty-text="暂无审批记录">
      <el-table-column label="申请" min-width="210"><template #default="{ row }"><div class="identity-cell"><strong>{{ row.title }}</strong><small>{{ row.summary }}</small></div></template></el-table-column>
      <el-table-column label="申请人" min-width="145"><template #default="{ row }"><div class="identity-cell"><strong>{{ row.applicantName }}</strong><small>{{ employeeNo(row.applicantNo) }} · {{ row.departmentName }}</small></div></template></el-table-column>
      <el-table-column label="类型" width="95"><template #default="{ row }"><el-tag :type="typeMeta[row.type as ApprovalType].tone" effect="plain">{{ typeMeta[row.type as ApprovalType].label }}</el-tag></template></el-table-column>
      <el-table-column label="进度" min-width="145"><template #default="{ row }"><span v-if="row.status === 'pending'">{{ row.currentStepName }} · {{ row.currentStep }}/{{ row.totalSteps }}</span><span v-else class="muted">流程已结束</span></template></el-table-column>
      <el-table-column label="状态" width="100"><template #default="{ row }"><el-tag :type="statusMeta[row.status as ApprovalStatus].tone" effect="plain">{{ statusMeta[row.status as ApprovalStatus].label }}</el-tag></template></el-table-column>
      <el-table-column prop="submittedAt" label="提交时间" width="170"><template #default="{ row }">{{ new Date(row.submittedAt).toLocaleString() }}</template></el-table-column>
      <el-table-column label="操作" width="210" fixed="right"><template #default="{ row }">
        <el-button link :icon="View" @click="showDetail(row)">详情</el-button>
        <template v-if="row.canReview"><el-button link type="success" :icon="Check" @click="openReview(row, true)">通过</el-button><el-button link type="danger" :icon="Close" @click="openReview(row, false)">驳回</el-button></template>
        <el-button v-else-if="row.canCancel" link :icon="RefreshLeft" @click="cancel(row)">撤回</el-button>
      </template></el-table-column>
    </el-table>

    <el-dialog v-model="applicationOpen" title="发起申请" width="min(640px, calc(100vw - 32px))" destroy-on-close>
      <el-form label-position="top" class="approval-form">
        <el-form-item label="审批类型"><el-select v-model="form.type" class="full-width"><el-option v-for="item in types" :key="item.code" :label="item.name" :value="item.code"><div class="type-option"><span>{{ item.name }}</span><small>{{ item.description }}</small></div></el-option></el-select></el-form-item>
        <template v-if="form.type === 'leave'">
          <el-form-item label="请假类型"><el-select v-model="form.leaveType" class="full-width"><el-option label="年假" value="annual" /><el-option label="病假" value="sick" /><el-option label="事假" value="personal" /><el-option label="其他" value="other" /></el-select></el-form-item>
          <el-form-item label="开始日期"><el-date-picker v-model="form.startDate" type="date" value-format="YYYY-MM-DD" :disabled-date="disablePast" class="full-width" /></el-form-item>
          <el-form-item label="结束日期"><el-date-picker v-model="form.endDate" type="date" value-format="YYYY-MM-DD" :disabled-date="disablePast" class="full-width" /></el-form-item>
        </template>
        <template v-else-if="form.type === 'transfer'">
          <el-form-item label="目标部门"><el-select v-model="form.targetDepartmentId" filterable class="full-width"><el-option v-for="department in departments.filter((item) => item.status === 'enabled')" :key="department.id" :label="department.name" :value="department.id" /></el-select></el-form-item>
          <el-form-item label="目标职务"><el-input v-model="form.targetTitle" maxlength="100" /></el-form-item>
          <el-form-item label="生效日期"><el-date-picker v-model="form.effectiveDate" type="date" value-format="YYYY-MM-DD" :disabled-date="disablePast" class="full-width" /></el-form-item>
        </template>
        <el-form-item v-else label="最后工作日"><el-date-picker v-model="form.lastWorkingDate" type="date" value-format="YYYY-MM-DD" :disabled-date="disablePast" class="full-width" /></el-form-item>
        <el-form-item :label="form.type === 'leave' ? '请假事由' : form.type === 'transfer' ? '异动原因' : '离职原因'"><el-input v-model="form.reason" type="textarea" :rows="4" maxlength="1000" show-word-limit /></el-form-item>
      </el-form>
      <template #footer><el-button @click="applicationOpen = false">取消</el-button><el-button type="primary" :loading="saving" @click="submitApplication">提交申请</el-button></template>
    </el-dialog>

    <el-dialog v-model="reviewOpen" :title="review.approved ? '通过申请' : '驳回申请'" width="min(520px, calc(100vw - 32px))" destroy-on-close>
      <p class="review-target">{{ review.title }}</p>
      <el-form label-position="top"><el-form-item label="审批意见"><el-input v-model="review.comment" type="textarea" :rows="4" maxlength="500" show-word-limit /></el-form-item></el-form>
      <template #footer><el-button @click="reviewOpen = false">取消</el-button><el-button :type="review.approved ? 'success' : 'danger'" :loading="saving" @click="submitReview">确认{{ review.approved ? '通过' : '驳回' }}</el-button></template>
    </el-dialog>

    <el-drawer v-model="detailOpen" title="审批详情" size="min(620px, 100vw)">
      <template v-if="selected">
        <div class="approval-detail-heading"><el-tag :type="typeMeta[selected.type].tone" effect="plain">{{ typeMeta[selected.type].label }}</el-tag><h2>{{ selected.title }}</h2><p>{{ selected.summary }}</p></div>
        <dl class="approval-data"><div v-for="row in detailRows(selected)" :key="String(row[0])"><dt>{{ row[0] }}</dt><dd>{{ row[1] }}</dd></div></dl>
        <h3 class="section-heading">审批进度</h3>
        <el-timeline><el-timeline-item v-for="step in selected.steps" :key="step.id" :type="step.status === 'approved' ? 'success' : step.status === 'rejected' ? 'danger' : step.status === 'pending' ? 'warning' : 'info'" :timestamp="step.reviewedAt ? new Date(step.reviewedAt).toLocaleString() : ''"><strong>{{ step.name }}</strong><p>{{ step.reviewerName || (step.status === 'pending' ? '等待处理' : '尚未开始') }}<template v-if="step.comment"> · {{ step.comment }}</template></p></el-timeline-item></el-timeline>
      </template>
    </el-drawer>
  </div>
</template>
