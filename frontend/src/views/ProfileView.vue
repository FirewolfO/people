<script setup lang="ts">
import { useRouter } from 'vue-router'
import { auth } from '@/auth'

const router = useRouter()
</script>

<template>
  <div class="page">
    <header class="page-header"><div><h1>个人资料</h1><p>{{ auth.state.user?.username }}</p></div><el-button @click="router.push('/change-password')">修改密码</el-button></header>
    <dl class="profile-grid">
      <div><dt>姓名</dt><dd>{{ auth.state.user?.displayName || '-' }}</dd></div>
      <div><dt>工号</dt><dd>{{ auth.state.user?.employeeNo || '-' }}</dd></div>
      <div><dt>部门</dt><dd>{{ auth.state.user?.department || '-' }}</dd></div>
      <div><dt>职务</dt><dd>{{ auth.state.user?.title || '-' }}</dd></div>
      <div><dt>邮箱</dt><dd>{{ auth.state.user?.email || '-' }}</dd></div>
      <div><dt>手机号</dt><dd>{{ auth.state.user?.phone || '-' }}</dd></div>
    </dl>
  </div>
</template>
