<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { ElMessage } from 'element-plus'
import { Briefcase, Calendar, CircleCheck, Clock, OfficeBuilding, UserFilled, Warning } from '@element-plus/icons-vue'
import { apiMessage, peopleApi } from '@/api'
import type { HRDashboard } from '@/types'

const loading = ref(false)
const data = ref<HRDashboard>({ totalEmployees: 0, enabledEmployees: 0, disabledEmployees: 0, departments: 0, pendingDepartures: 0, probationEmployees: 0, recentHires: 0 })
const activeRate = computed(() => data.value.totalEmployees ? Math.round(data.value.enabledEmployees / data.value.totalEmployees * 100) : 0)
const metrics = computed(() => [
  { label: '员工总数', value: data.value.totalEmployees, icon: UserFilled, tone: 'green' },
  { label: '在职员工', value: data.value.enabledEmployees, icon: CircleCheck, tone: 'blue' },
  { label: '组织部门', value: data.value.departments, icon: OfficeBuilding, tone: 'gold' },
  { label: '待办离职', value: data.value.pendingDepartures, icon: Clock, tone: 'red' },
])

async function load() {
  loading.value = true
  try {
    data.value = await peopleApi.dashboard()
  } catch (error) {
    ElMessage.error(apiMessage(error, '人事概览加载失败'))
  } finally {
    loading.value = false
  }
}

onMounted(load)
</script>

<template>
  <div v-loading="loading" class="page dashboard-page">
    <header class="page-header"><div><h1>人事概览</h1><p>人员结构与近期人事动态</p></div></header>
    <section class="metric-grid">
      <div v-for="metric in metrics" :key="metric.label" class="metric-item">
        <span class="metric-icon" :class="metric.tone"><el-icon><component :is="metric.icon" /></el-icon></span>
        <div><small>{{ metric.label }}</small><strong>{{ metric.value }}</strong></div>
      </div>
    </section>
    <section class="dashboard-details">
      <div class="detail-block">
        <header><div><h2>人员状态</h2><span>在职率 {{ activeRate }}%</span></div><el-icon><Briefcase /></el-icon></header>
        <el-progress :percentage="activeRate" :stroke-width="10" color="#23856d" />
        <div class="detail-stats"><span><b>{{ data.enabledEmployees }}</b> 在职</span><span><b>{{ data.disabledEmployees }}</b> 停用</span></div>
      </div>
      <div class="detail-block">
        <header><div><h2>近期动态</h2><span>需要持续关注的人员节点</span></div><el-icon><Calendar /></el-icon></header>
        <div class="activity-row"><span><el-icon><UserFilled /></el-icon>近 30 天入职</span><strong>{{ data.recentHires }}</strong></div>
        <div class="activity-row"><span><el-icon><Warning /></el-icon>试用期员工</span><strong>{{ data.probationEmployees }}</strong></div>
      </div>
    </section>
  </div>
</template>
