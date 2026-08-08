<script setup lang="ts">
import { computed, reactive, ref } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { ElMessage, type FormInstance, type FormRules } from 'element-plus'
import { Check, Lock, Switch, User } from '@element-plus/icons-vue'
import { peopleApi, apiMessage } from '@/api'
import { auth } from '@/auth'

const route = useRoute()
const router = useRouter()
const formRef = ref<FormInstance>()
const loading = ref(false)
const switching = ref(false)
const form = reactive({ username: '', password: '' })
const rules: FormRules = {
  username: [{ required: true, message: '请输入用户名', trigger: 'blur' }],
  password: [{ required: true, message: '请输入密码', trigger: 'blur' }],
}

const clientID = computed(() => String(route.query.client_id || ''))
const redirectURI = computed(() => String(route.query.redirect_uri || ''))
const state = computed(() => String(route.query.state || ''))
const invalidRequest = computed(() => !clientID.value || !redirectURI.value || !state.value)
const applicationName = computed(() => ({
  'permission-ui': '权限中心',
  'gateway-admin-ui': 'Gateway 管理系统',
  'blog-ui': '内部博客',
}[clientID.value] || clientID.value))
const currentName = computed(() => auth.state.user?.displayName || auth.state.user?.username || '')

function switchAccount() {
  form.username = ''
  form.password = ''
  switching.value = true
}

function useCurrentAccount() {
  form.password = ''
  switching.value = false
}

async function authorize() {
  if (invalidRequest.value) return
  if (switching.value && !(await formRef.value?.validate().catch(() => false))) return
  loading.value = true
  try {
    const account = switching.value ? { username: form.username, password: form.password } : undefined
    const result = await peopleApi.authorize(clientID.value, redirectURI.value, state.value, account)
    window.location.assign(result.redirectUrl)
  } catch (error) {
    form.password = ''
    ElMessage.error(apiMessage(error, switching.value ? '账号或密码错误' : '授权请求无效'))
    loading.value = false
  }
}

function cancel() {
  void router.replace('/profile')
}
</script>

<template>
  <div class="auth-page">
    <section class="auth-panel oauth-panel">
      <div class="brand auth-brand"><span class="brand-mark">P</span><span>People</span></div>
      <el-result v-if="invalidRequest" icon="error" title="授权请求无效">
        <template #extra><el-button @click="cancel">返回 People</el-button></template>
      </el-result>
      <template v-else>
        <div class="oauth-heading">
          <p>应用授权</p>
          <h1>{{ applicationName }} 请求访问</h1>
          <span>读取你的姓名、用户名及员工基本资料</span>
        </div>

        <template v-if="!switching">
          <div class="oauth-account">
            <span class="avatar">{{ currentName.slice(0, 1).toUpperCase() }}</span>
            <span class="oauth-account-copy">
              <strong>{{ currentName }}</strong>
              <small>{{ auth.state.user?.username }} · {{ auth.state.user?.employeeNo }}</small>
            </span>
            <el-tag type="success" effect="plain" size="small">当前账号</el-tag>
          </div>
          <div class="oauth-actions">
            <el-button size="large" @click="cancel">取消</el-button>
            <el-button type="primary" size="large" :icon="Check" :loading="loading" @click="authorize">授权并继续</el-button>
          </div>
          <el-button class="oauth-switch-button" link :icon="Switch" @click="switchAccount">切换账号</el-button>
        </template>

        <el-form v-else ref="formRef" class="oauth-form" :model="form" :rules="rules" label-position="top" @submit.prevent="authorize">
          <div class="oauth-switch-heading">
            <strong>使用其他账号授权</strong>
            <el-button link @click="useCurrentAccount">返回当前账号</el-button>
          </div>
          <el-form-item label="用户名" prop="username">
            <el-input v-model="form.username" size="large" autocomplete="username" :prefix-icon="User" />
          </el-form-item>
          <el-form-item label="密码" prop="password">
            <el-input v-model="form.password" size="large" type="password" autocomplete="current-password" show-password :prefix-icon="Lock" />
          </el-form-item>
          <p class="oauth-note">本次授权不会退出或替换当前登录的 People 账号 {{ currentName }}</p>
          <div class="oauth-actions">
            <el-button size="large" @click="cancel">取消</el-button>
            <el-button native-type="submit" type="primary" size="large" :icon="Check" :loading="loading">使用此账号授权</el-button>
          </div>
        </el-form>
      </template>
    </section>
  </div>
</template>
