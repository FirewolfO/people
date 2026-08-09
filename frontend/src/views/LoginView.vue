<script setup lang="ts">
import { reactive, ref } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { ElMessage, type FormInstance, type FormRules } from 'element-plus'
import { Lock, User } from '@element-plus/icons-vue'
import { auth } from '@/auth'
import { apiMessage } from '@/api'

const route = useRoute()
const router = useRouter()
const formRef = ref<FormInstance>()
const loading = ref(false)
const form = reactive({ username: '', password: '' })
const rules: FormRules = { username: [{ required: true, message: '请输入用户名', trigger: 'blur' }] }

async function submit() {
  if (!(await formRef.value?.validate().catch(() => false))) return
  loading.value = true
  try {
    await auth.login(form.username, form.password)
    const fallback = auth.can('people.dashboard:view') ? '/dashboard' : '/approvals'
    await router.replace(auth.state.user?.mustChangePassword ? { name: 'change-password', query: route.query } : String(route.query.redirect || fallback))
  } catch (error) {
    ElMessage.error(apiMessage(error, '用户名或密码错误'))
  } finally {
    loading.value = false
  }
}
</script>

<template>
  <div class="auth-page">
    <section class="auth-panel">
      <div class="brand auth-brand"><span class="brand-mark">P</span><span>People</span></div>
      <h1>企业员工中心</h1>
      <el-form ref="formRef" :model="form" :rules="rules" label-position="top" @submit.prevent="submit">
        <el-form-item label="用户名" prop="username"><el-input v-model="form.username" size="large" autocomplete="username" :prefix-icon="User" /></el-form-item>
        <el-form-item label="密码"><el-input v-model="form.password" size="large" type="password" autocomplete="current-password" show-password :prefix-icon="Lock" /></el-form-item>
        <el-button native-type="submit" type="primary" size="large" :loading="loading">登录</el-button>
      </el-form>
    </section>
  </div>
</template>
