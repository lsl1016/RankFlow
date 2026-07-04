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

    <a-table
      :columns="columns"
      :data-source="rows"
      :loading="loading"
      row-key="rankId"
      :pagination="pagination"
      :expandable="{ rowExpandable }"
      @change="onTableChange"
      @expand="onExpand"
    >
      <template #expandedRowRender="{ record }">
        <div class="sub-board-panel">
          <div class="sub-board-title">
            <span>子榜</span>
            <a-button size="small" @click="loadSubBoards(record.rankId)">刷新</a-button>
          </div>
          <a-table
            :columns="subColumnsFor(record.rankId)"
            :data-source="subBoards[record.rankId] || []"
            :loading="subLoading[record.rankId]"
            row-key="typeId"
            size="small"
            :pagination="false"
          >
            <template #bodyCell="{ column, record: sub }">
              <template v-if="String(column.key).startsWith('dim:')">
                {{ sub.dimensions?.[column.dimensionField] || '-' }}
              </template>
              <template v-else-if="column.key === 'status'">
                <a-tag :color="STATUS_COLOR[sub.status]">{{ STATUS_TEXT[sub.status] }}</a-tag>
              </template>
              <template v-else-if="column.key === 'action'">
                <a @click="$router.push({ path: `/ranks/${record.rankId}`, query: { typeId: sub.typeId } })">查看</a>
                <a-divider type="vertical" />
                <a v-if="sub.status !== 1" @click="changeSubStatus(record.rankId, sub, 1)">上线</a>
                <a v-else @click="changeSubStatus(record.rankId, sub, 2)">下线</a>
              </template>
            </template>
          </a-table>
        </div>
      </template>

      <template #bodyCell="{ column, record }">
        <template v-if="column.key === 'status'">
          <a-tag :color="STATUS_COLOR[record.status]">{{ STATUS_TEXT[record.status] }}</a-tag>
        </template>
        <template v-else-if="column.key === 'timeType'">
          {{ TIME_TYPE_TEXT[record.timeTypeLabel] || '-' }}
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
const subBoards = reactive({})
const subLoading = reactive({})

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
    await Promise.all(rows.value.map((row) => loadSubBoards(row.rankId)))
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

async function onExpand(expanded, record) {
  if (expanded && !subBoards[record.rankId]) {
    await loadSubBoards(record.rankId)
  }
}

async function loadSubBoards(rankId) {
  subLoading[rankId] = true
  try {
    const data = await api.listSubBoards(rankId)
    subBoards[rankId] = data.list || []
  } finally {
    subLoading[rankId] = false
  }
}

function rowExpandable(record) {
  return (subBoards[record.rankId] || []).length > 0
}

async function changeStatus(record, status) {
  await api.setStatus(record.rankId, status)
  message.success(status === 1 ? '已上线' : '已下线')
  reload()
}

async function changeSubStatus(rankId, sub, status) {
  await api.setSubBoardStatus(rankId, { typeId: sub.typeId, status })
  message.success(status === 1 ? '子榜已上线' : '子榜已下线')
  loadSubBoards(rankId)
}

function dimensionKeysFor(rankId) {
  const keys = []
  for (const sub of subBoards[rankId] || []) {
    for (const key of Object.keys(sub.dimensions || {})) {
      if (!keys.includes(key)) keys.push(key)
    }
  }
  return keys
}

function subColumnsFor(rankId) {
  const dimensionColumns = dimensionKeysFor(rankId).map((field) => ({
    title: field,
    key: `dim:${field}`,
    dimensionField: field,
    width: 140,
  }))
  return [
    { title: 'Type ID', dataIndex: 'typeId', key: 'typeId', width: 180 },
    ...dimensionColumns,
    { title: '成员数', dataIndex: 'memberCount', key: 'memberCount', width: 100 },
    { title: '状态', key: 'status', width: 100 },
    { title: '操作', key: 'action', width: 140 },
  ]
}

onMounted(reload)
</script>

<style scoped>
.sub-board-panel {
  padding: 8px 16px 12px;
  background: #fafafa;
}

.sub-board-title {
  display: flex;
  align-items: center;
  justify-content: space-between;
  margin-bottom: 8px;
  font-weight: 600;
}
</style>
