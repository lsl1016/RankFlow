<template>
  <a-space direction="vertical" style="width: 100%" :size="16">
    <a-card :title="`榜单详情：${config.rankName || ''}`">
      <template #extra>
        <a-tag :color="STATUS_COLOR[config.status]">{{ STATUS_TEXT[config.status] }}</a-tag>
      </template>
      <a-descriptions bordered :column="3" size="small">
        <a-descriptions-item label="榜单ID">{{ config.rankId }}</a-descriptions-item>
        <a-descriptions-item label="业务线">{{ config.bizCode }}</a-descriptions-item>
        <a-descriptions-item label="榜单对象">{{ config.targetType }}</a-descriptions-item>
        <a-descriptions-item label="时间粒度">{{ TIME_TYPE_TEXT[timeType] }}</a-descriptions-item>
        <a-descriptions-item label="排序">{{ config.sortType }}</a-descriptions-item>
        <a-descriptions-item label="最大长度">{{ config.maxRankSize }}</a-descriptions-item>
        <a-descriptions-item label="当前子榜 type_id" :span="3">
          <a-tag color="blue">{{ stats.typeId || '—' }}</a-tag>
        </a-descriptions-item>
      </a-descriptions>
    </a-card>

    <a-card title="实时概览" size="small">
      <a-row :gutter="16">
        <a-col :span="6"><a-statistic title="当前成员数" :value="stats.memberCount || 0" /></a-col>
        <a-col :span="6"><a-statistic title="写入QPS(均值)" :value="stats.writeQps || 0" :precision="2" /></a-col>
        <a-col :span="6"><a-statistic title="读取QPS(均值)" :value="stats.readQps || 0" :precision="2" /></a-col>
        <a-col :span="6"><a-statistic title="配置缓存命中率" :value="(stats.cacheHitRate || 0) * 100" :precision="1" suffix="%" /></a-col>
      </a-row>
    </a-card>

    <a-card title="子榜维度 / 测试加分" size="small">
      <a-form layout="inline">
        <a-form-item v-for="d in dimensions" :key="d.dimensionField" :label="d.dimensionName || d.dimensionField">
          <a-input v-model:value="dimValues[d.dimensionField]" :placeholder="d.dimensionField" allow-clear />
        </a-form-item>
        <a-form-item>
          <a-button type="primary" @click="refresh">查询榜单</a-button>
        </a-form-item>
      </a-form>
      <a-divider />
      <a-form layout="inline">
        <a-form-item label="成员ID">
          <a-input v-model:value="testItem" placeholder="如 user_10086" />
        </a-form-item>
        <a-form-item label="加分">
          <a-input-number v-model:value="testScore" :min="-100000" style="width: 120px" />
        </a-form-item>
        <a-form-item>
          <a-button @click="testAdd" :disabled="config.status !== 1">提交加分</a-button>
          <span v-if="config.status !== 1" style="color:#999;margin-left:8px">榜单需上线后才能加分</span>
        </a-form-item>
      </a-form>
    </a-card>

    <a-card title="实时排名" size="small">
      <template #extra>
        <a-space>
          <a-switch v-model:checked="autoRefresh" checked-children="自动刷新" un-checked-children="手动" />
          <a-button size="small" @click="refresh">刷新</a-button>
        </a-space>
      </template>
      <a-table :columns="rankColumns" :data-source="items" :loading="loading" row-key="itemId" :pagination="false" size="small" />
    </a-card>
  </a-space>
</template>

<script setup>
import { reactive, ref, onMounted, onUnmounted, watch } from 'vue'
import { message } from 'ant-design-vue'
import { api, STATUS_TEXT, STATUS_COLOR, TIME_TYPE_TEXT } from '../api'

const props = defineProps({ id: { type: String, required: true } })

const config = reactive({})
const dimensions = ref([])
const timeType = ref('none')
const dimValues = reactive({})
const stats = reactive({})
const items = ref([])
const loading = ref(false)
const autoRefresh = ref(false)
const testItem = ref('user_10086')
const testScore = ref(10)
let timer = null

const rankColumns = [
  { title: '排名', dataIndex: 'rank', key: 'rank', width: 80 },
  { title: '成员ID', dataIndex: 'itemId', key: 'itemId' },
  { title: '分数', dataIndex: 'score', key: 'score' },
]

function dimParams() {
  const p = { timestamp: Math.floor(Date.now() / 1000) }
  for (const [k, v] of Object.entries(dimValues)) {
    if (v) p[`dim_${k}`] = v
  }
  return p
}

async function loadConfig() {
  const data = await api.getRank(props.id)
  Object.assign(config, data.config)
  dimensions.value = data.dimensions || []
  timeType.value = data.time?.timeType || 'none'
}

async function refresh() {
  loading.value = true
  try {
    const params = dimParams()
    const [topData, statData] = await Promise.all([
      api.top(props.id, { ...params, offset: 0, limit: 100 }),
      api.stats(props.id, params),
    ])
    items.value = topData.items || []
    Object.assign(stats, statData)
  } finally {
    loading.value = false
  }
}

async function testAdd() {
  if (!testItem.value) {
    message.warning('请填写成员ID')
    return
  }
  const dims = {}
  for (const [k, v] of Object.entries(dimValues)) if (v) dims[k] = v
  await api.addScore(props.id, {
    requestId: `manual_${Date.now()}`,
    itemId: testItem.value,
    score: testScore.value,
    eventTime: Math.floor(Date.now() / 1000),
    dimensions: dims,
  })
  message.success('加分成功')
  refresh()
}

function toggleTimer() {
  if (autoRefresh.value) {
    timer = setInterval(refresh, 3000)
  } else if (timer) {
    clearInterval(timer)
    timer = null
  }
}

onMounted(async () => {
  await loadConfig()
  await refresh()
})
onUnmounted(() => timer && clearInterval(timer))

watch(autoRefresh, toggleTimer)
</script>
