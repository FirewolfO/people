<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { ElMessage } from 'element-plus'
import { Briefcase, Calendar, CircleCheck, Clock, Document, OfficeBuilding, TrendCharts, UserFilled, Warning } from '@element-plus/icons-vue'
import { apiMessage, peopleApi } from '@/api'
import type { HRDashboard, MetricBucket } from '@/types'

const emptyData = (): HRDashboard => ({ totalEmployees: 0, enabledEmployees: 0, disabledEmployees: 0, departments: 0, pendingDepartures: 0, pendingApprovals: 0, probationEmployees: 0, recentHires: 0, employeesOnLeave: 0, contractsExpiring: 0, activeGoals: 0, overdueGoals: 0, departmentDistribution: [], employmentTypeDistribution: [], approvalDistribution: [] })
const loading = ref(false)
const data = ref<HRDashboard>(emptyData())
const activeRate = computed(() => data.value.totalEmployees ? Math.round(data.value.enabledEmployees / data.value.totalEmployees * 100) : 0)
const metrics = computed(() => [
  { label: '在职员工', value: data.value.enabledEmployees, icon: UserFilled, tone: 'green' },
  { label: '组织部门', value: data.value.departments, icon: OfficeBuilding, tone: 'blue' },
  { label: '待审批流程', value: data.value.pendingApprovals, icon: Clock, tone: 'gold' },
  { label: '合同临期', value: data.value.contractsExpiring, icon: Document, tone: 'red' },
  { label: '今日休假', value: data.value.employeesOnLeave, icon: Calendar, tone: 'blue' },
  { label: '近 30 天入职', value: data.value.recentHires, icon: CircleCheck, tone: 'green' },
  { label: '试用期员工', value: data.value.probationEmployees, icon: Briefcase, tone: 'gold' },
  { label: '逾期目标', value: data.value.overdueGoals, icon: Warning, tone: 'red' },
])
const employmentLabels: Record<string, string> = { full_time: '全职', part_time: '兼职', contract: '合同制', intern: '实习' }
const approvalLabels: Record<string, string> = { leave: '请假', transfer: '岗位异动', departure: '离职' }

function percent(item: MetricBucket, items: MetricBucket[]) {
  const total = items.reduce((sum, current) => sum + current.count, 0)
  return total ? Math.round(item.count / total * 100) : 0
}

async function load() {
  loading.value = true
  try { data.value = await peopleApi.dashboard() } catch (error) { ElMessage.error(apiMessage(error, '人事概览加载失败')) } finally { loading.value = false }
}

onMounted(load)
</script>

<template>
  <div v-loading="loading" class="page dashboard-page">
    <header class="page-header"><div><h1>人事概览</h1><p>统一人员主数据、流程与人才风险视图</p></div></header>
    <section class="metric-grid dashboard-metrics"><div v-for="metric in metrics" :key="metric.label" class="metric-item"><span class="metric-icon" :class="metric.tone"><el-icon><component :is="metric.icon" /></el-icon></span><div><small>{{ metric.label }}</small><strong>{{ metric.value }}</strong></div></div></section>
    <section class="dashboard-details dashboard-columns">
      <div class="detail-block"><header><div><h2>人员状态</h2><span>在职率 {{ activeRate }}%</span></div><el-icon><Briefcase /></el-icon></header><el-progress :percentage="activeRate" :stroke-width="10" color="#23856d" /><div class="detail-stats"><span><b>{{ data.enabledEmployees }}</b> 在职</span><span><b>{{ data.disabledEmployees }}</b> 停用</span><span><b>{{ data.totalEmployees }}</b> 总计</span></div></div>
      <div class="detail-block"><header><div><h2>审批结构</h2><span>当前待处理流程按类型分布</span></div><el-icon><Clock /></el-icon></header><div v-if="data.approvalDistribution.length" class="distribution-list"><div v-for="item in data.approvalDistribution" :key="item.name"><span>{{ approvalLabels[item.name] || item.name }}<b>{{ item.count }}</b></span><el-progress :percentage="percent(item, data.approvalDistribution)" :show-text="false" color="#ad6300" /></div></div><el-empty v-else :image-size="54" description="暂无待审批流程" /></div>
      <div class="detail-block"><header><div><h2>部门分布</h2><span>在职员工组织结构</span></div><el-icon><OfficeBuilding /></el-icon></header><div class="distribution-list"><div v-for="item in data.departmentDistribution.slice(0, 6)" :key="item.name"><span>{{ item.name }}<b>{{ item.count }}</b></span><el-progress :percentage="percent(item, data.departmentDistribution)" :show-text="false" color="#23856d" /></div></div></div>
      <div class="detail-block"><header><div><h2>用工结构</h2><span>员工类型与人才目标</span></div><el-icon><TrendCharts /></el-icon></header><div class="distribution-list"><div v-for="item in data.employmentTypeDistribution" :key="item.name"><span>{{ employmentLabels[item.name] || item.name }}<b>{{ item.count }}</b></span><el-progress :percentage="percent(item, data.employmentTypeDistribution)" :show-text="false" color="#235a8d" /></div></div><div class="detail-stats"><span><b>{{ data.activeGoals }}</b> 进行中目标</span><span><b>{{ data.overdueGoals }}</b> 已逾期</span></div></div>
    </section>
  </div>
</template>
