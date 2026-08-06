<script setup lang="ts">
import { computed } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { ElMessage } from 'element-plus'
import { User, UserFilled, SwitchButton } from '@element-plus/icons-vue'
import { auth } from '@/auth'
import { apiMessage } from '@/api'

const route = useRoute()
const router = useRouter()
const displayName = computed(() => auth.state.user?.displayName || auth.state.user?.username || '')

async function logout() {
  try {
    await auth.logout()
    await router.replace('/login')
  } catch (error) {
    ElMessage.error(apiMessage(error, '退出失败'))
  }
}
</script>

<template>
  <div class="app-shell">
    <aside class="sidebar">
      <div class="brand"><span class="brand-mark">P</span><span>People</span></div>
      <nav>
        <RouterLink v-if="auth.state.user?.role === 'admin'" to="/employees" :class="{ active: route.path === '/employees' }">
          <el-icon><UserFilled /></el-icon><span>员工管理</span>
        </RouterLink>
        <RouterLink to="/profile" :class="{ active: route.path === '/profile' }">
          <el-icon><User /></el-icon><span>个人资料</span>
        </RouterLink>
      </nav>
      <button class="account-row" type="button" @click="logout">
        <span class="avatar">{{ displayName.slice(0, 1).toUpperCase() }}</span>
        <span class="account-copy"><strong>{{ displayName }}</strong><small>{{ auth.state.user?.employeeNo }}</small></span>
        <el-icon><SwitchButton /></el-icon>
      </button>
    </aside>
    <main class="main"><RouterView /></main>
  </div>
</template>
