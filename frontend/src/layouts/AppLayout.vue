<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { ElMessage, ElNotification } from 'element-plus'
import { Bell, DataAnalysis, DocumentChecked, OfficeBuilding, User, UserFilled, SwitchButton } from '@element-plus/icons-vue'
import { auth } from '@/auth'
import { apiMessage } from '@/api'
import { peopleApi } from '@/api'
import type { NotificationItem, NotificationSummary } from '@/types'

const route = useRoute()
const router = useRouter()
const displayName = computed(() => auth.state.user?.displayName || auth.state.user?.username || '')
const summary = ref<NotificationSummary>({ unread: 0, pendingTasks: 0, total: 0 })
const notifications = ref<NotificationItem[]>([])
let previousTotal: number | null = null
let timer = 0

function formatEmployeeNo(value?: number) {
  return value ? String(value).padStart(6, '0') : '-'
}

async function loadSummary() {
  try {
    const next = await peopleApi.notificationSummary()
    if (previousTotal !== null && next.total > previousTotal) {
      ElNotification({ title: '有新的待办', message: '新的离职审批或处理结果需要关注', type: 'warning', duration: 6000 })
    }
    previousTotal = next.total
    summary.value = next
  } catch {
    // A transient polling failure should not interrupt the current workflow.
  }
}

async function loadNotifications() {
  try {
    notifications.value = await peopleApi.notifications(true)
  } catch (error) {
    ElMessage.error(apiMessage(error, '通知加载失败'))
  }
}

async function readAll() {
  await peopleApi.markAllNotificationsRead()
  notifications.value = []
  await loadSummary()
}

onMounted(() => {
  void loadSummary()
  timer = window.setInterval(loadSummary, 10_000)
})
onBeforeUnmount(() => window.clearInterval(timer))

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
        <RouterLink v-if="auth.can('people.dashboard:view')" to="/dashboard" :class="{ active: route.path === '/dashboard' }">
          <el-icon><DataAnalysis /></el-icon><span>人事概览</span>
        </RouterLink>
        <RouterLink v-if="auth.can('people.employee:view')" to="/employees" :class="{ active: route.path === '/employees' }">
          <el-icon><UserFilled /></el-icon><span>员工管理</span>
        </RouterLink>
        <RouterLink v-if="auth.can('people.department:manage')" to="/departments" :class="{ active: route.path === '/departments' }">
          <el-icon><OfficeBuilding /></el-icon><span>部门管理</span>
        </RouterLink>
        <RouterLink to="/departures" :class="{ active: route.path === '/departures' }">
          <el-icon><DocumentChecked /></el-icon><span>离职审批</span>
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
          <span class="account-copy"><strong>{{ displayName }}</strong><small>{{ formatEmployeeNo(auth.state.user?.employeeNo) }}</small></span>
        </div>
        <el-popover placement="bottom-end" :width="360" trigger="click" @show="loadNotifications">
          <template #reference>
            <el-badge :value="summary.total" :hidden="summary.total === 0" :max="99" class="notification-badge">
              <el-button class="notification-button" :icon="Bell" circle aria-label="通知与待审批" />
            </el-badge>
          </template>
          <div class="notification-panel">
            <header><strong>通知与待办</strong><el-button v-if="summary.unread" link type="primary" @click="readAll">全部已读</el-button></header>
            <RouterLink v-if="summary.pendingTasks" to="/departures" class="task-notice">
              <span>{{ summary.pendingTasks }} 项离职审批待处理</span><small>查看审批列表</small>
            </RouterLink>
            <div v-for="item in notifications" :key="item.id" class="notice-item"><strong>{{ item.title }}</strong><span>{{ item.content }}</span></div>
            <el-empty v-if="!summary.total" :image-size="54" description="暂无通知" />
          </div>
        </el-popover>
        <el-tooltip content="退出登录" placement="bottom">
          <el-button class="logout-button" :icon="SwitchButton" circle aria-label="退出登录" @click="logout" />
        </el-tooltip>
      </header>
      <RouterView />
    </main>
  </div>
</template>
