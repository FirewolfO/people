<script setup lang="ts">
import { onMounted, reactive, ref } from 'vue'
import { ElMessage, ElMessageBox, type FormInstance, type FormRules } from 'element-plus'
import { Delete, Edit, Plus, Search } from '@element-plus/icons-vue'
import { peopleApi, apiMessage } from '@/api'
import { auth } from '@/auth'
import type { Employee, EmployeeInput } from '@/types'

const items = ref<Employee[]>([])
const total = ref(0)
const loading = ref(false)
const query = reactive({ q: '', page: 1, pageSize: 20 })
const dialogVisible = ref(false)
const editingID = ref('')
const saving = ref(false)
const formRef = ref<FormInstance>()
const emptyForm = (): EmployeeInput => ({ employeeNo: '', username: '', displayName: '', email: '', phone: '', department: '', title: '', role: 'employee', status: 'enabled' })
const form = reactive<EmployeeInput>(emptyForm())
const rules: FormRules = {
  employeeNo: [{ required: true, message: '请输入工号', trigger: 'blur' }],
  username: [{ required: true, message: '请输入用户名', trigger: 'blur' }, { pattern: /^[A-Za-z][A-Za-z0-9_.-]{2,63}$/, message: '用户名格式无效', trigger: 'blur' }],
  displayName: [{ required: true, message: '请输入姓名', trigger: 'blur' }],
}

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

function create() {
  editingID.value = ''
  Object.assign(form, emptyForm())
  dialogVisible.value = true
}

function edit(item: Employee) {
  editingID.value = item.id
  Object.assign(form, { employeeNo: item.employeeNo, username: item.username, displayName: item.displayName, email: item.email, phone: item.phone, department: item.department, title: item.title, role: item.role, status: item.status })
  dialogVisible.value = true
}

async function save() {
  if (!(await formRef.value?.validate().catch(() => false))) return
  saving.value = true
  try {
    if (editingID.value) await peopleApi.updateEmployee(editingID.value, form)
    else await peopleApi.createEmployee(form)
    ElMessage.success(editingID.value ? '员工资料已更新' : '员工已创建')
    dialogVisible.value = false
    await load()
  } catch (error) {
    ElMessage.error(apiMessage(error, '保存失败'))
  } finally {
    saving.value = false
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

onMounted(load)
</script>

<template>
  <div class="page">
    <header class="page-header"><div><h1>员工管理</h1><p>共 {{ total }} 名员工</p></div><el-button type="primary" :icon="Plus" @click="create">新增员工</el-button></header>
    <div class="toolbar">
      <el-input v-model="query.q" clearable placeholder="搜索工号、姓名、部门" :prefix-icon="Search" @keyup.enter="query.page = 1; load()" @clear="query.page = 1; load()" />
      <el-button @click="query.page = 1; load()">查询</el-button>
    </div>
    <el-table v-loading="loading" :data="items" row-key="id">
      <el-table-column prop="employeeNo" label="工号" width="120" />
      <el-table-column prop="displayName" label="姓名" min-width="130" />
      <el-table-column prop="username" label="用户名" min-width="130" />
      <el-table-column prop="department" label="部门" min-width="140" show-overflow-tooltip />
      <el-table-column prop="title" label="职务" min-width="130" show-overflow-tooltip />
      <el-table-column label="角色" width="100"><template #default="{ row }"><el-tag :type="row.role === 'admin' ? 'danger' : 'info'" effect="plain">{{ row.role === 'admin' ? '管理员' : '员工' }}</el-tag></template></el-table-column>
      <el-table-column label="状态" width="100"><template #default="{ row }"><el-tag :type="row.status === 'enabled' ? 'success' : 'info'" effect="plain">{{ row.status === 'enabled' ? '启用' : '停用' }}</el-tag></template></el-table-column>
      <el-table-column label="密码" width="110"><template #default="{ row }"><span :class="row.mustChangePassword ? 'pending' : 'ready'">{{ row.mustChangePassword ? '待设置' : '已设置' }}</span></template></el-table-column>
      <el-table-column label="操作" width="112" fixed="right"><template #default="{ row }"><el-button link :icon="Edit" title="编辑" @click="edit(row)" /><el-button link type="danger" :icon="Delete" title="删除" :disabled="row.id === auth.state.user?.id || row.username === 'admin'" @click="remove(row)" /></template></el-table-column>
    </el-table>
    <el-pagination v-model:current-page="query.page" v-model:page-size="query.pageSize" layout="total, prev, pager, next" :total="total" @current-change="load" />

    <el-dialog v-model="dialogVisible" :title="editingID ? '编辑员工' : '新增员工'" width="min(720px, calc(100vw - 32px))" destroy-on-close>
      <el-form ref="formRef" :model="form" :rules="rules" label-position="top" class="employee-form">
        <el-form-item label="工号" prop="employeeNo"><el-input v-model="form.employeeNo" /></el-form-item>
        <el-form-item label="用户名" prop="username"><el-input v-model="form.username" :disabled="form.username === 'admin'" /></el-form-item>
        <el-form-item label="姓名" prop="displayName"><el-input v-model="form.displayName" /></el-form-item>
        <el-form-item label="邮箱"><el-input v-model="form.email" /></el-form-item>
        <el-form-item label="手机号"><el-input v-model="form.phone" /></el-form-item>
        <el-form-item label="部门"><el-input v-model="form.department" /></el-form-item>
        <el-form-item label="职务"><el-input v-model="form.title" /></el-form-item>
        <el-form-item label="角色"><el-select v-model="form.role" :disabled="form.username === 'admin'"><el-option label="员工" value="employee" /><el-option label="管理员" value="admin" /></el-select></el-form-item>
        <el-form-item label="状态"><el-switch v-model="form.status" active-value="enabled" inactive-value="disabled" active-text="启用" inactive-text="停用" :disabled="form.username === 'admin'" /></el-form-item>
      </el-form>
      <template #footer><el-button @click="dialogVisible = false">取消</el-button><el-button type="primary" :loading="saving" @click="save">保存</el-button></template>
    </el-dialog>
  </div>
</template>
