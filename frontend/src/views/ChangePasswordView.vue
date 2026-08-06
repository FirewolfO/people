<script setup lang="ts">
import { computed, reactive, ref } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { ElMessage, type FormInstance, type FormRules } from 'element-plus'
import { auth } from '@/auth'
import { apiMessage } from '@/api'

const route = useRoute()
const router = useRouter()
const formRef = ref<FormInstance>()
const loading = ref(false)
const firstSetup = computed(() => Boolean(auth.state.user?.mustChangePassword))
const form = reactive({ currentPassword: '', newPassword: '', confirmPassword: '' })
const rules: FormRules = {
  currentPassword: [{ validator: (_rule, value, callback) => firstSetup.value || value ? callback() : callback(new Error('请输入当前密码')), trigger: 'blur' }],
  newPassword: [{ required: true, message: '请输入新密码', trigger: 'blur' }, { min: 8, max: 72, message: '密码长度为 8 到 72 个字符', trigger: 'blur' }],
  confirmPassword: [{ validator: (_rule, value, callback) => value === form.newPassword ? callback() : callback(new Error('两次输入的密码不一致')), trigger: 'blur' }],
}

async function submit() {
  if (!(await formRef.value?.validate().catch(() => false))) return
  loading.value = true
  try {
    await auth.changePassword(form.currentPassword, form.newPassword)
    ElMessage.success('密码已更新')
    const fallback = auth.state.user?.role === 'admin' ? '/employees' : '/profile'
    await router.replace(String(route.query.redirect || fallback))
  } catch (error) {
    ElMessage.error(apiMessage(error, '密码更新失败'))
  } finally {
    loading.value = false
  }
}
</script>

<template>
  <div class="auth-page">
    <section class="auth-panel compact">
      <div class="brand auth-brand"><span class="brand-mark">P</span><span>People</span></div>
      <h1>{{ firstSetup ? '设置登录密码' : '修改密码' }}</h1>
      <el-form ref="formRef" :model="form" :rules="rules" label-position="top" @submit.prevent="submit">
        <el-form-item v-if="!firstSetup" label="当前密码" prop="currentPassword"><el-input v-model="form.currentPassword" type="password" show-password /></el-form-item>
        <el-form-item label="新密码" prop="newPassword"><el-input v-model="form.newPassword" type="password" show-password /></el-form-item>
        <el-form-item label="确认新密码" prop="confirmPassword"><el-input v-model="form.confirmPassword" type="password" show-password /></el-form-item>
        <el-button native-type="submit" type="primary" size="large" :loading="loading">确认</el-button>
      </el-form>
    </section>
  </div>
</template>
