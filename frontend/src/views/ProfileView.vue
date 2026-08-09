<script setup lang="ts">
import { onMounted, reactive, ref } from 'vue'
import { useRouter } from 'vue-router'
import { ElMessage } from 'element-plus'
import { Edit } from '@element-plus/icons-vue'
import { auth } from '@/auth'
import { apiMessage, peopleApi } from '@/api'
import type { EmploymentEvent } from '@/types'

const router = useRouter()
const editing = ref(false)
const saving = ref(false)
const events = ref<EmploymentEvent[]>([])
const form = reactive({ email: '', phone: '', emergencyContactName: '', emergencyContactPhone: '', emergencyContactRelation: '' })
const eventLabels: Record<string, string> = { hire: '入职', transfer: '部门异动', promotion: '职务调整', departure: '离职', enable: '账号启用', disable: '账号停用' }

function openEdit() {
  const user = auth.state.user
  Object.assign(form, { email: user?.email || '', phone: user?.phone || '', emergencyContactName: user?.emergencyContactName || '', emergencyContactPhone: user?.emergencyContactPhone || '', emergencyContactRelation: user?.emergencyContactRelation || '' })
  editing.value = true
}

async function save() {
  saving.value = true
  try {
    auth.state.user = await peopleApi.updateMyProfile(form)
    ElMessage.success('联系方式已更新')
    editing.value = false
  } catch (error) { ElMessage.error(apiMessage(error, '资料更新失败')) } finally { saving.value = false }
}

onMounted(async () => {
  if (!auth.state.user) return
  try { events.value = await peopleApi.employmentEvents(auth.state.user.id) } catch { events.value = [] }
})
</script>

<template>
  <div class="page profile-page">
    <header class="page-header"><div><h1>个人资料</h1><p>{{ auth.state.user?.username }}</p></div><div class="header-actions"><el-button :icon="Edit" @click="openEdit">编辑联系方式</el-button><el-button @click="router.push('/change-password')">修改密码</el-button></div></header>
    <dl class="profile-grid">
      <div><dt>姓名</dt><dd>{{ auth.state.user?.displayName || '-' }}</dd></div>
      <div><dt>工号</dt><dd>{{ auth.state.user?.employeeNo ? String(auth.state.user.employeeNo).padStart(6, '0') : '-' }}</dd></div>
      <div><dt>部门</dt><dd>{{ auth.state.user?.department || '-' }}</dd></div>
      <div><dt>职务</dt><dd>{{ auth.state.user?.title || '-' }}</dd></div>
      <div><dt>邮箱</dt><dd>{{ auth.state.user?.email || '-' }}</dd></div>
      <div><dt>手机号</dt><dd>{{ auth.state.user?.phone || '-' }}</dd></div>
      <div><dt>紧急联系人</dt><dd>{{ auth.state.user?.emergencyContactName || '-' }}<small v-if="auth.state.user?.emergencyContactRelation"> · {{ auth.state.user.emergencyContactRelation }}</small></dd></div>
      <div><dt>紧急联系电话</dt><dd>{{ auth.state.user?.emergencyContactPhone || '-' }}</dd></div>
      <div><dt>用工类型</dt><dd>{{ ({ full_time: '全职', part_time: '兼职', contract: '合同制', intern: '实习' } as Record<string, string>)[auth.state.user?.employmentType || ''] || '-' }}</dd></div>
      <div><dt>入职日期</dt><dd>{{ auth.state.user?.hireDate || '-' }}</dd></div>
      <div><dt>试用期结束</dt><dd>{{ auth.state.user?.probationEndDate || '-' }}</dd></div>
      <div><dt>工作地点</dt><dd>{{ auth.state.user?.workLocation || '-' }}</dd></div>
    </dl>
    <section class="profile-history">
      <div class="section-toolbar"><div><h2>任职履历</h2><p>入职、异动与账号状态变更记录</p></div></div>
      <el-timeline v-if="events.length"><el-timeline-item v-for="event in events" :key="event.id" :timestamp="event.effectiveDate" placement="top"><strong>{{ eventLabels[event.type] || event.type }}</strong><p>{{ event.note || [event.fromDepartment, event.toDepartment].filter(Boolean).join(' → ') }}</p><small v-if="event.fromTitle || event.toTitle">{{ [event.fromTitle, event.toTitle].filter(Boolean).join(' → ') }}</small></el-timeline-item></el-timeline>
      <el-empty v-else :image-size="64" description="暂无履历记录" />
    </section>

    <el-dialog v-model="editing" title="编辑联系方式" width="min(600px, calc(100vw - 32px))" destroy-on-close>
      <el-form label-position="top" class="employee-form">
        <el-form-item label="邮箱"><el-input v-model="form.email" /></el-form-item><el-form-item label="手机号"><el-input v-model="form.phone" /></el-form-item>
        <el-form-item label="紧急联系人"><el-input v-model="form.emergencyContactName" /></el-form-item><el-form-item label="与本人关系"><el-input v-model="form.emergencyContactRelation" /></el-form-item>
        <el-form-item label="紧急联系电话"><el-input v-model="form.emergencyContactPhone" /></el-form-item>
      </el-form>
      <template #footer><el-button @click="editing = false">取消</el-button><el-button type="primary" :loading="saving" @click="save">保存</el-button></template>
    </el-dialog>
  </div>
</template>
