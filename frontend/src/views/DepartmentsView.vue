<script setup lang="ts">
import { computed, onMounted, reactive, ref } from 'vue'
import { ElMessage, ElMessageBox, type FormInstance, type FormRules } from 'element-plus'
import { Delete, Edit, Plus, Search } from '@element-plus/icons-vue'
import { apiMessage, peopleApi } from '@/api'
import { buildDepartmentTree, departmentDescendants } from '@/departments'
import type { Department, DepartmentInput, Employee } from '@/types'

const items = ref<Department[]>([])
const employees = ref<Employee[]>([])
const loading = ref(false)
const saving = ref(false)
const query = ref('')
const dialogVisible = ref(false)
const editingID = ref('')
const formRef = ref<FormInstance>()
const emptyForm = (): DepartmentInput => ({ parentId: '', code: '', name: '', description: '', leaderId: '', status: 'enabled' })
const form = reactive<DepartmentInput>(emptyForm())
const rules: FormRules = {
  code: [
    { required: true, message: '请输入部门编码', trigger: 'blur' },
    { pattern: /^[a-z][a-z0-9_-]{0,31}$/, message: '以小写字母开头，可包含数字、下划线和连字符', trigger: 'blur' },
  ],
  name: [{ required: true, message: '请输入部门名称', trigger: 'blur' }],
}
const treeItems = computed(() => buildDepartmentTree(items.value))
const parentOptions = computed(() => buildDepartmentTree(items.value, editingID.value ? departmentDescendants(items.value, editingID.value) : new Set()))
const leaderOptions = computed(() => employees.value.filter((employee) => employee.departmentId === editingID.value && employee.status === 'enabled'))

async function load() {
  loading.value = true
  try {
    items.value = await peopleApi.departments({ q: query.value })
  } catch (error) {
    ElMessage.error(apiMessage(error, '部门列表加载失败'))
  } finally {
    loading.value = false
  }
}

async function loadEmployees() {
  try {
    employees.value = (await peopleApi.employees({ page: 1, pageSize: 100 })).items
  } catch (error) {
    ElMessage.error(apiMessage(error, '负责人候选人加载失败'))
  }
}

function create(parentId = '') {
  editingID.value = ''
  Object.assign(form, emptyForm())
  form.parentId = parentId
  dialogVisible.value = true
}

function edit(item: Department) {
  editingID.value = item.id
  Object.assign(form, { parentId: item.parentId, code: item.code, name: item.name, description: item.description, leaderId: item.leaderId, status: item.status })
  dialogVisible.value = true
}

async function save() {
  if (!(await formRef.value?.validate().catch(() => false))) return
  saving.value = true
  try {
    if (editingID.value) await peopleApi.updateDepartment(editingID.value, form)
    else await peopleApi.createDepartment(form)
    ElMessage.success(editingID.value ? '部门已更新' : '部门已创建')
    dialogVisible.value = false
    await load()
  } catch (error) {
    ElMessage.error(apiMessage(error, '保存失败'))
  } finally {
    saving.value = false
  }
}

async function remove(item: Department) {
  try {
    await ElMessageBox.confirm(`确认删除部门“${item.name}”？`, '删除部门', { type: 'warning', confirmButtonText: '删除', cancelButtonText: '取消' })
    await peopleApi.deleteDepartment(item.id)
    ElMessage.success('部门已删除')
    await load()
  } catch (error) {
    if (error !== 'cancel' && error !== 'close') ElMessage.error(apiMessage(error, '删除失败'))
  }
}

onMounted(() => { void load(); void loadEmployees() })
</script>

<template>
  <div class="page">
    <header class="page-header">
      <div><h1>部门管理</h1><p>维护员工所属的组织部门</p></div>
      <el-button type="primary" :icon="Plus" @click="create">新增部门</el-button>
    </header>
    <div class="toolbar">
      <el-input v-model="query" clearable placeholder="搜索部门编码或名称" :prefix-icon="Search" @keyup.enter="load" @clear="load" />
      <el-button @click="load">查询</el-button>
    </div>
    <el-table v-loading="loading" :data="treeItems" row-key="id" default-expand-all :tree-props="{ children: 'children' }">
      <el-table-column prop="code" label="部门编码" min-width="150" />
      <el-table-column prop="name" label="部门名称" min-width="160" />
      <el-table-column prop="description" label="描述" min-width="220" show-overflow-tooltip />
      <el-table-column prop="leaderName" label="部门负责人" min-width="130"><template #default="{ row }">{{ row.leaderName || '-' }}</template></el-table-column>
      <el-table-column prop="employeeCount" label="员工数" width="100" align="right" />
      <el-table-column label="状态" width="100">
        <template #default="{ row }"><el-tag :type="row.status === 'enabled' ? 'success' : 'info'" effect="plain">{{ row.status === 'enabled' ? '启用' : '停用' }}</el-tag></template>
      </el-table-column>
      <el-table-column label="操作" width="144" fixed="right">
        <template #default="{ row }">
          <el-button link :icon="Plus" title="新增子部门" @click="create(row.id)" />
          <el-button link :icon="Edit" title="编辑" @click="edit(row)" />
          <el-button link type="danger" :icon="Delete" title="删除" :disabled="row.employeeCount > 0 || row.children?.length > 0" @click="remove(row)" />
        </template>
      </el-table-column>
    </el-table>

    <el-dialog v-model="dialogVisible" :title="editingID ? '编辑部门' : '新增部门'" width="min(560px, calc(100vw - 32px))" destroy-on-close>
      <el-form ref="formRef" :model="form" :rules="rules" label-position="top">
        <el-form-item label="上级部门"><el-tree-select v-model="form.parentId" :data="parentOptions" node-key="id" :props="{ label: 'name', children: 'children' }" check-strictly clearable class="full-width" placeholder="顶级部门" /></el-form-item>
        <el-form-item label="部门编码" prop="code"><el-input v-model="form.code" maxlength="32" /></el-form-item>
        <el-form-item label="部门名称" prop="name"><el-input v-model="form.name" maxlength="100" /></el-form-item>
        <el-form-item label="部门负责人">
          <el-select v-model="form.leaderId" clearable filterable :disabled="!editingID" placeholder="选择本部门员工" class="full-width">
            <el-option v-for="employee in leaderOptions" :key="employee.id" :label="employee.displayName" :value="employee.id"><span>{{ employee.displayName }} · {{ String(employee.employeeNo).padStart(6, '0') }}</span></el-option>
          </el-select>
        </el-form-item>
        <el-form-item label="描述"><el-input v-model="form.description" type="textarea" :rows="3" maxlength="500" show-word-limit /></el-form-item>
        <el-form-item label="状态"><el-switch v-model="form.status" active-value="enabled" inactive-value="disabled" active-text="启用" inactive-text="停用" /></el-form-item>
      </el-form>
      <template #footer><el-button @click="dialogVisible = false">取消</el-button><el-button type="primary" :loading="saving" @click="save">保存</el-button></template>
    </el-dialog>
  </div>
</template>
