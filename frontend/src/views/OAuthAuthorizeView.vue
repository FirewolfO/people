<script setup lang="ts">
import { onMounted, ref } from 'vue'
import { useRoute } from 'vue-router'
import { ElMessage } from 'element-plus'
import { peopleApi, apiMessage } from '@/api'

const route = useRoute()
const failed = ref(false)

onMounted(async () => {
  try {
    const result = await peopleApi.authorize(String(route.query.client_id || ''), String(route.query.redirect_uri || ''), String(route.query.state || ''))
    window.location.assign(result.redirectUrl)
  } catch (error) {
    failed.value = true
    ElMessage.error(apiMessage(error, '授权请求无效'))
  }
})
</script>

<template><div class="status-page"><el-result :icon="failed ? 'error' : 'info'" :title="failed ? '授权失败' : '正在授权'" /></div></template>
