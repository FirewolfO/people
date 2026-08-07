<script setup lang="ts">
import { computed } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { ElMessage } from 'element-plus'
import { OfficeBuilding, User, UserFilled, SwitchButton } from '@element-plus/icons-vue'
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
        <RouterLink v-if="auth.state.user?.role === 'admin'" to="/departments" :class="{ active: route.path === '/departments' }">
          <el-icon><OfficeBuilding /></el-icon><span>部门管理</span>
        </RouterLink>
        <RouterLink to="/profile" :class="{ active: route.path === '/profile' }">
          <el-icon><User /></el-icon><span>个人资料</span>
        </RouterLink>
      </nav>
    </aside>
    <main class="main">
      <header class="app-header">
        <div class="account-summary">
          <span class="avatar">{{ displayName.slice(0, 1).toUpperCase() }}</span>
          <span class="account-copy"><strong>{{ displayName }}</strong><small>{{ auth.state.user?.employeeNo }}</small></span>
        </div>
        <el-tooltip content="退出登录" placement="bottom">
          <el-button class="logout-button" :icon="SwitchButton" circle aria-label="退出登录" @click="logout" />
        </el-tooltip>
      </header>
      <RouterView />
    </main>
  </div>
</template>
