<template>
  <a-layout style="min-height: 100vh">
    <a-layout-header class="header">
      <div class="logo" @click="$router.push('/ranks')">RankFlow · 通用榜单服务</div>
    </a-layout-header>
    <a-layout-content class="content">
      <router-view />
    </a-layout-content>
    <a-layout-footer class="footer">RankFlow MVP · Go + Gin + Redis + MySQL</a-layout-footer>
  </a-layout>
</template>

<script setup></script>

<style>
body { margin: 0; }
.header { display: flex; align-items: center; }
.logo { color: #fff; font-size: 18px; font-weight: 600; cursor: pointer; }
.content { padding: 24px; background: #f0f2f5; }
.footer { text-align: center; color: #999; }
</style>
