<script setup lang="ts">
import { computed, onMounted, reactive, ref } from 'vue'
import { ElMessage, ElMessageBox, type FormInstance, type FormRules } from 'element-plus'
import { Delete, Edit, Key, Plus, Search, SwitchButton } from '@element-plus/icons-vue'
import { peopleApi, apiMessage } from '@/api'
import { buildDepartmentTree } from '@/departments'
import { auth } from '@/auth'
import type { Department, Employee, EmployeeInput, EmploymentType } from '@/types'

const items = ref<Employee[]>([])
const departments = ref<Department[]>([])
const departmentTree = computed(() => buildDepartmentTree(departments.value))
const total = ref(0)
const loading = ref(false)
const query = reactive({ q: '', page: 1, pageSize: 20 })
const dialogVisible = ref(false)
const editingID = ref('')
const editingAdmin = ref(false)
const saving = ref(false)
const formRef = ref<FormInstance>()
const canManage = computed(() => auth.can('people.employee:manage'))
const canReset = computed(() => auth.can('people.employee:reset'))
const canDisable = computed(() => auth.can('people.employee:disable'))
const emptyForm = (): EmployeeInput => ({ username: '', displayName: '', email: '', phone: '', departmentId: '', title: '', employmentType: 'full_time', hireDate: '', probationEndDate: '', workLocation: '' })
const form = reactive<EmployeeInput>(emptyForm())
const rules: FormRules = {
  username: [{ required: true, message: '请输入用户名', trigger: 'blur' }, { pattern: /^[A-Za-z][A-Za-z0-9_.-]{2,63}$/, message: '用户名格式无效', trigger: 'blur' }],
  displayName: [{ required: true, message: '请输入姓名', trigger: 'blur' }],
  departmentId: [{ validator: (_rule, value, callback) => { if (!editingAdmin.value && !value) callback(new Error('请选择部门')); else callback() }, trigger: 'change' }],
}
const employmentLabels: Record<EmploymentType, string> = { full_time: '全职', part_time: '兼职', contract: '合同制', intern: '实习' }

function employeeNo(value: number) { return String(value).padStart(6, '0') }

async function load() {
  loading.value = true
  try {
    const result = await peopleApi.employees(query)
    items.value = result.items
    total.value = result.total
  } catch (error) {
    ElMessage.error(apiMessage(error, '员工列表加载失败'))
  } finally {
    loading.value = false
  }
}

async function loadDepartments() {
  try {
    departments.value = await peopleApi.departments()
  } catch (error) {
    ElMessage.error(apiMessage(error, '部门列表加载失败'))
  }
}

function create() {
  editingID.value = ''
  editingAdmin.value = false
  Object.assign(form, emptyForm())
  dialogVisible.value = true
}

function edit(item: Employee) {
  editingID.value = item.id
  editingAdmin.value = item.role === 'admin'
  Object.assign(form, {
    username: item.username, displayName: item.displayName, email: item.email, phone: item.phone,
    departmentId: item.departmentId, title: item.title, employmentType: item.employmentType || 'full_time',
    hireDate: item.hireDate || '', probationEndDate: item.probationEndDate || '', workLocation: item.workLocation || '',
  })
  dialogVisible.value = true
}

async function save() {
  if (!(await formRef.value?.validate().catch(() => false))) return
  saving.value = true
  try {
    if (editingID.value) await peopleApi.updateEmployee(editingID.value, form)
    else await peopleApi.createEmployee(form)
    ElMessage.success(editingID.value ? '员工资料已更新' : '员工已创建，工号已自动生成')
    dialogVisible.value = false
    await load()
  } catch (error) {
    ElMessage.error(apiMessage(error, '保存失败'))
  } finally {
    saving.value = false
  }
}

async function resetPassword(item: Employee) {
  try {
    await ElMessageBox.confirm(`确认重置“${item.displayName}”的密码？该员工需要在下次登录时重新设置密码。`, '重置密码', { type: 'warning', confirmButtonText: '重置', cancelButtonText: '取消' })
    await peopleApi.resetEmployeePassword(item.id)
    ElMessage.success('密码已重置')
    await load()
  } catch (error) {
    if (error !== 'cancel' && error !== 'close') ElMessage.error(apiMessage(error, '重置失败'))
  }
}

async function toggleEnabled(item: Employee) {
  const enabled = item.status !== 'enabled'
  try {
    await ElMessageBox.confirm(`确认${enabled ? '启用' : '停用'}“${item.displayName}”？`, `${enabled ? '启用' : '停用'}账号`, { type: 'warning', confirmButtonText: '确认', cancelButtonText: '取消' })
    await peopleApi.setEmployeeEnabled(item.id, enabled)
    ElMessage.success(`账号已${enabled ? '启用' : '停用'}`)
    await load()
  } catch (error) {
    if (error !== 'cancel' && error !== 'close') ElMessage.error(apiMessage(error, '操作失败'))
  }
}

async function remove(item: Employee) {
  try {
    await ElMessageBox.confirm(`确认删除员工“${item.displayName}”？`, '删除员工', { type: 'warning', confirmButtonText: '删除', cancelButtonText: '取消' })
    await peopleApi.deleteEmployee(item.id)
    ElMessage.success('员工已删除')
    await load()
  } catch (error) {
    if (error !== 'cancel' && error !== 'close') ElMessage.error(apiMessage(error, '删除失败'))
  }
}

onMounted(() => {
  void load()
  void loadDepartments()
})
</script>

<template>
  <div class="page">
    <header class="page-header"><div><h1>员工管理</h1><p>共 {{ total }} 名员工</p></div><el-button v-if="canManage" type="primary" :icon="Plus" @click="create">新增员工</el-button></header>
    <div class="toolbar">
      <el-input v-model="query.q" clearable placeholder="搜索工号、姓名、部门" :prefix-icon="Search" @keyup.enter="query.page = 1; load()" @clear="query.page = 1; load()" />
      <el-button @click="query.page = 1; load()">查询</el-button>
    </div>
    <el-table v-loading="loading" :data="items" row-key="id">
      <el-table-column label="员工" min-width="170"><template #default="{ row }"><div class="identity-cell"><strong>{{ row.displayName }}</strong><small>{{ employeeNo(row.employeeNo) }} · {{ row.username }}</small></div></template></el-table-column>
      <el-table-column prop="department" label="部门" min-width="130" show-overflow-tooltip />
      <el-table-column prop="title" label="职务" min-width="120" show-overflow-tooltip />
      <el-table-column label="用工类型" width="100"><template #default="{ row }">{{ employmentLabels[row.employmentType as EmploymentType] || '全职' }}</template></el-table-column>
      <el-table-column prop="hireDate" label="入职日期" width="120"><template #default="{ row }">{{ row.hireDate || '-' }}</template></el-table-column>
      <el-table-column prop="workLocation" label="工作地点" min-width="120"><template #default="{ row }">{{ row.workLocation || '-' }}</template></el-table-column>
      <el-table-column label="状态" width="90"><template #default="{ row }"><el-tag :type="row.status === 'enabled' ? 'success' : 'info'" effect="plain">{{ row.status === 'enabled' ? '在职' : '停用' }}</el-tag></template></el-table-column>
      <el-table-column label="密码" width="90"><template #default="{ row }"><span :class="row.mustChangePassword ? 'pending' : 'ready'">{{ row.mustChangePassword ? '待设置' : '正常' }}</span></template></el-table-column>
      <el-table-column v-if="canManage || canReset || canDisable" label="操作" width="176" fixed="right">
        <template #default="{ row }">
          <el-tooltip v-if="canManage" content="编辑资料"><el-button link :icon="Edit" aria-label="编辑资料" @click="edit(row)" /></el-tooltip>
          <el-tooltip v-if="canReset" content="重置密码"><el-button link :icon="Key" aria-label="重置密码" :disabled="row.username === 'admin'" @click="resetPassword(row)" /></el-tooltip>
          <el-tooltip v-if="canDisable" :content="row.status === 'enabled' ? '停用账号' : '启用账号'"><el-button link :type="row.status === 'enabled' ? 'warning' : 'success'" :icon="SwitchButton" :aria-label="row.status === 'enabled' ? '停用账号' : '启用账号'" :disabled="row.username === 'admin' || row.id === auth.state.user?.id" @click="toggleEnabled(row)" /></el-tooltip>
          <el-tooltip v-if="canManage" content="删除员工"><el-button link type="danger" :icon="Delete" aria-label="删除员工" :disabled="row.id === auth.state.user?.id || row.username === 'admin'" @click="remove(row)" /></el-tooltip>
        </template>
      </el-table-column>
    </el-table>
    <el-pagination v-model:current-page="query.page" v-model:page-size="query.pageSize" layout="total, prev, pager, next" :total="total" @current-change="load" />

    <el-dialog v-model="dialogVisible" :title="editingID ? '编辑员工' : '新增员工'" width="min(760px, calc(100vw - 32px))" destroy-on-close>
      <el-form ref="formRef" :model="form" :rules="rules" label-position="top" class="employee-form">
        <el-form-item label="用户名" prop="username"><el-input v-model="form.username" :disabled="form.username === 'admin'" /></el-form-item>
        <el-form-item label="姓名" prop="displayName"><el-input v-model="form.displayName" /></el-form-item>
        <el-form-item label="邮箱"><el-input v-model="form.email" /></el-form-item>
        <el-form-item label="手机号"><el-input v-model="form.phone" /></el-form-item>
        <el-form-item :label="editingAdmin ? '部门（可选）' : '部门'" prop="departmentId"><el-tree-select v-model="form.departmentId" :data="departmentTree" node-key="id" :props="{ label: 'name', children: 'children', disabled: (node: Department) => node.status === 'disabled' }" filterable check-strictly clearable placeholder="选择部门" /></el-form-item>
        <el-form-item label="职务"><el-input v-model="form.title" /></el-form-item>
        <el-form-item label="用工类型"><el-select v-model="form.employmentType"><el-option label="全职" value="full_time" /><el-option label="兼职" value="part_time" /><el-option label="合同制" value="contract" /><el-option label="实习" value="intern" /></el-select></el-form-item>
        <el-form-item label="工作地点"><el-input v-model="form.workLocation" /></el-form-item>
        <el-form-item label="入职日期"><el-date-picker v-model="form.hireDate" type="date" value-format="YYYY-MM-DD" class="full-width" /></el-form-item>
        <el-form-item label="试用期结束"><el-date-picker v-model="form.probationEndDate" type="date" value-format="YYYY-MM-DD" class="full-width" /></el-form-item>
      </el-form>
      <template #footer><el-button @click="dialogVisible = false">取消</el-button><el-button type="primary" :loading="saving" @click="save">保存</el-button></template>
    </el-dialog>
  </div>
</template>
