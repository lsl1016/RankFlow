<template>
  <a-layout style="min-height: 100vh">
    <a-layout-header class="header">
      <div class="logo" @click="$router.push('/ranks')">RankFlow · 通用榜单服务</div>
      <div class="header-actions">
        <a-tag :color="tokenConfigured ? 'green' : 'default'">
          {{ tokenConfigured ? '管理令牌已设置' : '管理令牌未设置' }}
        </a-tag>
        <a-button ghost size="small" @click="openTokenModal">管理令牌</a-button>
      </div>
    </a-layout-header>
    <a-layout-content class="content">
      <router-view />
    </a-layout-content>
    <a-layout-footer class="footer">RankFlow · Go + Gin + Redis + MySQL</a-layout-footer>

    <a-modal
      v-model:open="tokenOpen"
      title="设置管理令牌"
      ok-text="保存并刷新"
      cancel-text="取消"
      @ok="saveToken"
    >
      <a-alert
        type="info"
        show-icon
        message="令牌仅保存在当前浏览器会话的 sessionStorage 中"
        description="管理页面需要 Admin Bearer Token；Writer Token 只能调用写分接口。"
        style="margin-bottom: 16px"
      />
      <a-input-password
        v-model:value="tokenDraft"
        placeholder="请输入 RANKFLOW_ADMIN_TOKEN"
        autocomplete="off"
        @pressEnter="saveToken"
      />
    </a-modal>
  </a-layout>
</template>

<script setup>
import { onMounted, onUnmounted, ref } from 'vue'
import { message } from 'ant-design-vue'
import { getAdminToken, setAdminToken } from './api'

const tokenOpen = ref(false)
const tokenDraft = ref('')
const tokenConfigured = ref(Boolean(getAdminToken()))

function openTokenModal() {
  tokenDraft.value = ''
  tokenOpen.value = true
}

function handleAuthRequired() {
  openTokenModal()
}

function saveToken() {
  const token = tokenDraft.value.trim()
  if (!token) {
    message.warning('请输入管理令牌')
    return
  }
  setAdminToken(token)
  tokenConfigured.value = true
  tokenOpen.value = false
  message.success('管理令牌已更新')
  window.location.reload()
}

onMounted(() => window.addEventListener('rankflow:auth-required', handleAuthRequired))
onUnmounted(() => window.removeEventListener('rankflow:auth-required', handleAuthRequired))
</script>

<style>
body { margin: 0; }
.header { display: flex; align-items: center; justify-content: space-between; }
.logo { color: #fff; font-size: 18px; font-weight: 600; cursor: pointer; }
.header-actions { display: flex; align-items: center; gap: 8px; }
.content { padding: 24px; background: #f0f2f5; }
.footer { text-align: center; color: #999; }
</style>
