<script setup lang="ts">
import { onMounted, reactive, ref } from 'vue'
import { ElMessage, ElMessageBox, type FormInstance, type FormRules } from 'element-plus'
import { Delete, Edit, Plus, Search } from '@element-plus/icons-vue'
import { apiMessage, peopleApi } from '@/api'
import type { Department, Position, PositionInput } from '@/types'

const items = ref<Position[]>([])
const departments = ref<Department[]>([])
const loading = ref(false)
const saving = ref(false)
const dialogVisible = ref(false)
const editingID = ref('')
const editingBuiltin = ref(false)
const query = ref('')
const formRef = ref<FormInstance>()
const emptyForm = (): PositionInput => ({ code: '', name: '', description: '', departmentIds: [], status: 'enabled' })
const form = reactive<PositionInput>(emptyForm())
const rules: FormRules = {
  code: [
    { required: true, message: '请输入岗位编码', trigger: 'blur' },
    { pattern: /^[a-z][a-z0-9_-]{0,31}$/, message: '编码须以小写字母开头', trigger: 'blur' },
  ],
  name: [{ required: true, message: '请输入岗位名称', trigger: 'blur' }],
}

async function load() {
  loading.value = true
  try {
    items.value = await peopleApi.positions({ q: query.value })
  } catch (error) {
    ElMessage.error(apiMessage(error, '岗位列表加载失败'))
  } finally {
    loading.value = false
  }
}

function create() {
  editingID.value = ''
  editingBuiltin.value = false
  Object.assign(form, emptyForm())
  dialogVisible.value = true
}

function edit(item: Position) {
  editingID.value = item.id
  editingBuiltin.value = item.builtin
  Object.assign(form, {
    code: item.code,
    name: item.name,
    description: item.description,
    departmentIds: [...item.departmentIds],
    status: item.status,
  })
  dialogVisible.value = true
}

async function save() {
  if (!(await formRef.value?.validate().catch(() => false))) return
  saving.value = true
  try {
    if (editingID.value) await peopleApi.updatePosition(editingID.value, form)
    else await peopleApi.createPosition(form)
    ElMessage.success(editingID.value ? '岗位已更新' : '岗位已创建')
    dialogVisible.value = false
    await load()
  } catch (error) {
    ElMessage.error(apiMessage(error, '岗位保存失败'))
  } finally {
    saving.value = false
  }
}

async function remove(item: Position) {
  try {
    await ElMessageBox.confirm(`确认删除岗位“${item.name}”？`, '删除岗位', { type: 'warning', confirmButtonText: '删除', cancelButtonText: '取消' })
    await peopleApi.deletePosition(item.id)
    ElMessage.success('岗位已删除')
    await load()
  } catch (error) {
    if (error !== 'cancel' && error !== 'close') ElMessage.error(apiMessage(error, '岗位删除失败'))
  }
}

onMounted(async () => {
  try {
    departments.value = await peopleApi.departments()
  } catch (error) {
    ElMessage.error(apiMessage(error, '部门列表加载失败'))
  }
  await load()
})
</script>

<template>
  <div class="page">
    <header class="page-header">
      <div><h1>岗位管理</h1><p>共 {{ items.length }} 个岗位</p></div>
      <el-button type="primary" :icon="Plus" @click="create">新增岗位</el-button>
    </header>
    <div class="toolbar">
      <el-input v-model="query" clearable placeholder="搜索岗位编码或名称" :prefix-icon="Search" @keyup.enter="load" @clear="load" />
      <el-button @click="load">查询</el-button>
    </div>
    <el-table v-loading="loading" :data="items" row-key="id" empty-text="暂无岗位">
      <el-table-column prop="code" label="岗位编码" min-width="150" show-overflow-tooltip />
      <el-table-column label="岗位名称" min-width="170">
        <template #default="{ row }"><div class="identity-cell"><strong>{{ row.name }}</strong><small>{{ row.description || '-' }}</small></div></template>
      </el-table-column>
      <el-table-column label="关联部门" min-width="260">
        <template #default="{ row }">
          <div v-if="row.departmentNames.length" class="tag-list"><el-tag v-for="name in row.departmentNames" :key="name" effect="plain" type="info">{{ name }}</el-tag></div>
          <span v-else class="muted">未关联</span>
        </template>
      </el-table-column>
      <el-table-column prop="employeeCount" label="员工数" width="90" align="center" />
      <el-table-column label="状态" width="90"><template #default="{ row }"><el-tag :type="row.status === 'enabled' ? 'success' : 'info'" effect="plain">{{ row.status === 'enabled' ? '启用' : '停用' }}</el-tag></template></el-table-column>
      <el-table-column label="操作" width="110" fixed="right">
        <template #default="{ row }">
          <el-tooltip content="编辑岗位"><el-button link :icon="Edit" aria-label="编辑岗位" @click="edit(row)" /></el-tooltip>
          <el-tooltip :content="row.builtin ? '内置岗位不能删除' : row.employeeCount ? '有关联员工，不能删除' : '删除岗位'"><el-button link type="danger" :icon="Delete" aria-label="删除岗位" :disabled="row.builtin || row.employeeCount > 0" @click="remove(row)" /></el-tooltip>
        </template>
      </el-table-column>
    </el-table>

    <el-dialog v-model="dialogVisible" :title="editingID ? '编辑岗位' : '新增岗位'" width="min(620px, calc(100vw - 32px))" destroy-on-close>
      <el-form ref="formRef" :model="form" :rules="rules" label-position="top" class="employee-form">
        <el-form-item label="岗位编码" prop="code"><el-input v-model="form.code" :disabled="editingBuiltin" maxlength="32" /></el-form-item>
        <el-form-item label="岗位名称" prop="name"><el-input v-model="form.name" :disabled="editingBuiltin" maxlength="100" /></el-form-item>
        <el-form-item label="关联部门" class="form-span"><el-select v-model="form.departmentIds" multiple filterable collapse-tags collapse-tags-tooltip class="full-width" placeholder="选择可使用该岗位的部门"><el-option v-for="department in departments" :key="department.id" :label="department.status === 'enabled' ? department.name : `${department.name}（已停用）`" :value="department.id" /></el-select></el-form-item>
        <el-form-item label="岗位描述" class="form-span"><el-input v-model="form.description" type="textarea" :rows="3" maxlength="500" show-word-limit /></el-form-item>
        <el-form-item label="状态"><el-select v-model="form.status" :disabled="editingBuiltin" class="full-width"><el-option label="启用" value="enabled" /><el-option label="停用" value="disabled" /></el-select></el-form-item>
      </el-form>
      <template #footer><el-button @click="dialogVisible = false">取消</el-button><el-button type="primary" :loading="saving" @click="save">保存</el-button></template>
    </el-dialog>
  </div>
</template>
