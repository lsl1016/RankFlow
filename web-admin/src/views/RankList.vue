<template>
  <a-card title="榜单管理">
    <template #extra>
      <a-button type="primary" @click="$router.push('/ranks/new')">+ 新建榜单</a-button>
    </template>

    <a-form layout="inline" style="margin-bottom: 16px">
      <a-form-item label="榜单名称">
        <a-input v-model:value="query.name" placeholder="按名称搜索" allow-clear />
      </a-form-item>
      <a-form-item label="业务线">
        <a-input v-model:value="query.bizCode" placeholder="业务线编码" allow-clear />
      </a-form-item>
      <a-form-item label="状态">
        <a-select v-model:value="query.status" style="width: 120px" allow-clear placeholder="全部">
          <a-select-option :value="0">草稿</a-select-option>
          <a-select-option :value="1">已上线</a-select-option>
          <a-select-option :value="2">已下线</a-select-option>
          <a-select-option :value="3">已归档</a-select-option>
        </a-select>
      </a-form-item>
      <a-form-item>
        <a-button type="primary" @click="reload">查询</a-button>
        <a-button style="margin-left: 8px" @click="reset">重置</a-button>
      </a-form-item>
    </a-form>

    <a-table :columns="columns" :data-source="rows" :loading="loading" row-key="rankId"
      :pagination="pagination" @change="onTableChange">
      <template #bodyCell="{ column, record }">
        <template v-if="column.key === 'status'">
          <a-tag :color="STATUS_COLOR[record.status]">{{ STATUS_TEXT[record.status] }}</a-tag>
        </template>
        <template v-else-if="column.key === 'timeType'">
          {{ TIME_TYPE_TEXT[record.timeTypeLabel] || '—' }}
        </template>
        <template v-else-if="column.key === 'action'">
          <a @click="$router.push(`/ranks/${record.rankId}`)">详情</a>
          <a-divider type="vertical" />
          <a @click="$router.push(`/ranks/${record.rankId}/edit`)">编辑</a>
          <a-divider type="vertical" />
          <a v-if="record.status !== 1" @click="changeStatus(record, 1)">上线</a>
          <a v-else @click="changeStatus(record, 2)">下线</a>
        </template>
      </template>
    </a-table>
  </a-card>
</template>

<script setup>
import { reactive, ref, onMounted } from 'vue'
import { message } from 'ant-design-vue'
import { api, STATUS_TEXT, STATUS_COLOR, TIME_TYPE_TEXT } from '../api'

const columns = [
  { title: '榜单ID', dataIndex: 'rankId', key: 'rankId', width: 100 },
  { title: '榜单名称', dataIndex: 'rankName', key: 'rankName' },
  { title: '业务线', dataIndex: 'bizCode', key: 'bizCode' },
  { title: '榜单对象', dataIndex: 'targetType', key: 'targetType' },
  { title: '排序', dataIndex: 'sortType', key: 'sortType' },
  { title: '状态', key: 'status', width: 100 },
  { title: '操作', key: 'action', width: 220 },
]

const query = reactive({ name: '', bizCode: '', status: undefined })
const rows = ref([])
const loading = ref(false)
const pagination = reactive({ current: 1, pageSize: 20, total: 0 })

async function reload() {
  loading.value = true
  try {
    const data = await api.listRanks({
      name: query.name || undefined,
      bizCode: query.bizCode || undefined,
      status: query.status,
      page: pagination.current,
      size: pagination.pageSize,
    })
    rows.value = data.list || []
    pagination.total = data.total || 0
  } finally {
    loading.value = false
  }
}

function reset() {
  query.name = ''
  query.bizCode = ''
  query.status = undefined
  pagination.current = 1
  reload()
}

function onTableChange(p) {
  pagination.current = p.current
  pagination.pageSize = p.pageSize
  reload()
}

async function changeStatus(record, status) {
  await api.setStatus(record.rankId, status)
  message.success(status === 1 ? '已上线' : '已下线')
  reload()
}

onMounted(reload)
</script>
