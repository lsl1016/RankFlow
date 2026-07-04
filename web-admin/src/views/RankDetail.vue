<template>
  <a-space direction="vertical" style="width: 100%" :size="16">
    <a-card :title="`榜单详情：${config.rankName || ''}`">
      <template #extra>
        <a-space>
          <a-tag :color="STATUS_COLOR[config.status]">{{ STATUS_TEXT[config.status] }}</a-tag>
          <a-button @click="$router.push('/ranks')">返回列表</a-button>
        </a-space>
      </template>
      <a-descriptions bordered :column="3" size="small">
        <a-descriptions-item label="榜单ID">{{ config.rankId }}</a-descriptions-item>
        <a-descriptions-item label="业务线">{{ config.bizCode }}</a-descriptions-item>
        <a-descriptions-item label="榜单对象">{{ config.targetType }}</a-descriptions-item>
        <a-descriptions-item label="时间粒度">{{ TIME_TYPE_TEXT[timeType] }}</a-descriptions-item>
        <a-descriptions-item label="排序">{{ config.sortType }}</a-descriptions-item>
        <a-descriptions-item label="最大长度">{{ config.maxRankSize }}</a-descriptions-item>
        <a-descriptions-item label="当前子榜 type_id" :span="3">
          <a-tag :color="activeSubBoard ? 'blue' : 'default'">{{ activeSubBoard?.typeId || '未选择' }}</a-tag>
          <a-tag v-if="activeSubBoard" :color="STATUS_COLOR[activeSubBoard.status]">{{ STATUS_TEXT[activeSubBoard.status] }}</a-tag>
        </a-descriptions-item>
      </a-descriptions>
    </a-card>

    <a-card title="子榜选择" size="small">
      <a-form layout="inline">
        <a-form-item label="子榜">
          <a-select
            v-model:value="activeTypeID"
            style="width: 360px"
            show-search
            allow-clear
            placeholder="请选择子榜"
            :loading="subLoading"
            :options="subBoardOptions"
            @change="selectSubBoardByType"
          />
        </a-form-item>
        <a-form-item v-if="activeSubBoard" label="维度">
          <a-space wrap>
            <a-tag v-for="(value, key) in activeSubBoard.dimensions" :key="key">{{ key }}={{ value }}</a-tag>
            <span v-if="!Object.keys(activeSubBoard.dimensions || {}).length">全局</span>
          </a-space>
        </a-form-item>
        <a-form-item v-if="activeSubBoard" label="状态">
          <a-tag :color="STATUS_COLOR[activeSubBoard.status]">{{ STATUS_TEXT[activeSubBoard.status] }}</a-tag>
          <a-divider type="vertical" />
          <a v-if="activeSubBoard.status !== 1" @click="changeSubStatus(activeSubBoard, 1)">上线</a>
          <a v-else @click="changeSubStatus(activeSubBoard, 2)">下线</a>
        </a-form-item>
        <a-form-item>
          <a-button @click="loadSubBoards">刷新子榜</a-button>
        </a-form-item>
      </a-form>
      <a-empty v-if="!subLoading && !subBoards.length" description="当前榜单暂无子榜" />
    </a-card>

    <a-card title="实时概览" size="small">
      <a-empty v-if="!activeSubBoard" description="请先选择子榜" />
      <a-row v-else :gutter="16">
        <a-col :span="6"><a-statistic title="当前成员数" :value="stats.memberCount || 0" /></a-col>
        <a-col :span="6"><a-statistic title="写入QPS(均值)" :value="stats.writeQps || 0" :precision="2" /></a-col>
        <a-col :span="6"><a-statistic title="读取QPS(均值)" :value="stats.readQps || 0" :precision="2" /></a-col>
        <a-col :span="6"><a-statistic title="配置缓存命中率" :value="(stats.cacheHitRate || 0) * 100" :precision="1" suffix="%" /></a-col>
      </a-row>
    </a-card>

    <a-card title="当前子榜加分" size="small">
      <a-form layout="inline">
        <a-form-item label="成员ID">
          <a-input v-model:value="testItem" placeholder="如 user_10086" />
        </a-form-item>
        <a-form-item label="加分">
          <a-input-number v-model:value="testScore" :min="-100000" style="width: 120px" />
        </a-form-item>
        <a-form-item>
          <a-button @click="testAdd" :disabled="!canAddScore">提交加分</a-button>
          <span v-if="!canAddScore" style="color:#999;margin-left:8px">{{ addDisabledText }}</span>
        </a-form-item>
      </a-form>
    </a-card>

    <a-card title="实时排名" size="small">
      <template #extra>
        <a-space>
          <a-switch v-model:checked="autoRefresh" checked-children="自动刷新" un-checked-children="手动" />
          <a-button size="small" :disabled="!activeSubBoard" @click="refresh">刷新</a-button>
        </a-space>
      </template>
      <a-empty v-if="!activeSubBoard" description="请先选择一个子榜" />
      <a-table v-else :columns="rankColumns" :data-source="items" :loading="loading" row-key="itemId" :pagination="false" size="small" />
    </a-card>
  </a-space>
</template>

<script setup>
import { computed, reactive, ref, onMounted, onUnmounted, watch } from 'vue'
import { useRoute } from 'vue-router'
import { message } from 'ant-design-vue'
import { api, STATUS_TEXT, STATUS_COLOR, TIME_TYPE_TEXT } from '../api'

const props = defineProps({ id: { type: String, required: true } })
const route = useRoute()

const config = reactive({})
const dimensions = ref([])
const timeType = ref('none')
const stats = reactive({})
const items = ref([])
const subBoards = ref([])
const activeSubBoard = ref(null)
const activeTypeID = ref(undefined)
const loading = ref(false)
const subLoading = ref(false)
const autoRefresh = ref(false)
const testItem = ref('user_10086')
const testScore = ref(10)
let timer = null

const rankColumns = [
  { title: '排名', dataIndex: 'rank', key: 'rank', width: 80 },
  { title: '成员ID', dataIndex: 'itemId', key: 'itemId' },
  { title: '分数', dataIndex: 'score', key: 'score' },
]

const subBoardOptions = computed(() =>
  subBoards.value.map((sub) => ({
    value: sub.typeId,
    label: subBoardLabel(sub),
  })),
)

const canAddScore = computed(() => config.status === 1 && activeSubBoard.value?.status === 1)
const addDisabledText = computed(() => {
  if (!activeSubBoard.value) return '请选择子榜'
  if (config.status !== 1) return '主榜上线后才能加分'
  if (activeSubBoard.value.status !== 1) return '子榜上线后才能加分'
  return ''
})

function currentDimensions() {
  return activeSubBoard.value?.dimensions || {}
}

function dimParams() {
  const p = { timestamp: Math.floor(Date.now() / 1000) }
  for (const [k, v] of Object.entries(currentDimensions())) {
    if (v) p[`dim_${k}`] = v
  }
  return p
}

function subBoardLabel(sub) {
  const dims = Object.entries(sub.dimensions || {})
  if (!dims.length) return `${sub.typeId}（全局）`
  return `${sub.typeId}（${dims.map(([k, v]) => `${k}=${v}`).join('，')}）`
}

async function loadConfig() {
  const data = await api.getRank(props.id)
  Object.assign(config, data.config)
  dimensions.value = data.dimensions || []
  timeType.value = data.time?.timeType || 'none'
}

async function loadSubBoards() {
  subLoading.value = true
  try {
    const data = await api.listSubBoards(props.id)
    subBoards.value = data.list || []
    if (activeTypeID.value) {
      const latest = subBoards.value.find((x) => x.typeId === activeTypeID.value)
      if (latest) {
        activeSubBoard.value = latest
      } else {
        activeTypeID.value = undefined
        activeSubBoard.value = null
        clearBoardData()
      }
    }
  } finally {
    subLoading.value = false
  }
}

function selectSubBoardByType(typeID) {
  if (!typeID) {
    activeSubBoard.value = null
    clearBoardData()
    return
  }
  const sub = subBoards.value.find((x) => x.typeId === typeID)
  if (!sub) return
  activeSubBoard.value = sub
  activeTypeID.value = sub.typeId
  refresh()
}

async function changeSubStatus(sub, status) {
  await api.setSubBoardStatus(props.id, { typeId: sub.typeId, status })
  message.success(status === 1 ? '子榜已上线' : '子榜已下线')
  await loadSubBoards()
}

async function refresh() {
  if (!activeSubBoard.value) return
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

function clearBoardData() {
  items.value = []
  Object.assign(stats, {
    memberCount: 0,
    writeQps: 0,
    readQps: 0,
    cacheHitRate: 0,
    writeCount: 0,
    readCount: 0,
  })
}

async function testAdd() {
  if (!testItem.value) {
    message.warning('请填写成员ID')
    return
  }
  if (!canAddScore.value) {
    message.warning(addDisabledText.value)
    return
  }
  await api.addScore(props.id, {
    requestId: `manual_${Date.now()}`,
    itemId: testItem.value,
    score: testScore.value,
    eventTime: Math.floor(Date.now() / 1000),
    dimensions: currentDimensions(),
  })
  message.success('加分成功')
  await loadSubBoards()
  await refresh()
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
  await loadSubBoards()
  const routeTypeID = Array.isArray(route.query.typeId) ? route.query.typeId[0] : route.query.typeId
  if (routeTypeID) {
    activeTypeID.value = routeTypeID
    selectSubBoardByType(routeTypeID)
  }
})
onUnmounted(() => timer && clearInterval(timer))

watch(autoRefresh, toggleTimer)
</script>
