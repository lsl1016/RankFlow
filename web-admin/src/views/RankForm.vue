<template>
  <a-card :title="isEdit ? `编辑榜单 #${id}` : '新建榜单'">
    <a-form :model="form" :label-col="{ span: 5 }" :wrapper-col="{ span: 14 }">
      <a-divider orientation="left">1. 基础信息</a-divider>
      <a-form-item label="榜单名称" required>
        <a-input v-model:value="form.rankName" placeholder="如：创作者月度贡献榜" />
      </a-form-item>
      <a-form-item label="业务线" required>
        <a-input v-model:value="form.bizCode" placeholder="如：content_community" />
      </a-form-item>
      <a-form-item label="榜单对象" required>
        <a-select v-model:value="form.targetType">
          <a-select-option value="user">用户</a-select-option>
          <a-select-option value="content">内容</a-select-option>
          <a-select-option value="room">房间</a-select-option>
          <a-select-option value="product">商品</a-select-option>
          <a-select-option value="org">组织</a-select-option>
        </a-select>
      </a-form-item>

      <a-divider orientation="left">2. 排序配置</a-divider>
      <a-form-item label="排序方向">
        <a-select v-model:value="form.sortType">
          <a-select-option value="score_desc">分数越高越靠前</a-select-option>
          <a-select-option value="score_asc">分数越低越靠前</a-select-option>
        </a-select>
      </a-form-item>
      <a-form-item label="同分排序">
        <a-select v-model:value="form.sameScorePolicy">
          <a-select-option value="early_first">先达到分数者优先</a-select-option>
          <a-select-option value="late_first">后达到分数者优先</a-select-option>
          <a-select-option value="sub_score">业务二级排序</a-select-option>
        </a-select>
      </a-form-item>
      <a-form-item label="榜单最大长度">
        <a-input-number v-model:value="form.maxRankSize" :min="1" style="width: 200px" />
      </a-form-item>
      <a-form-item label="缓存TTL(秒)">
        <a-input-number v-model:value="form.cacheTtlSeconds" :min="1" style="width: 200px" />
      </a-form-item>

      <a-divider orientation="left">3. 时间维度</a-divider>
      <a-form-item label="时间粒度">
        <a-select v-model:value="form.timeType">
          <a-select-option value="none">无</a-select-option>
          <a-select-option value="hour">小时</a-select-option>
          <a-select-option value="day">自然日</a-select-option>
          <a-select-option value="week">自然周</a-select-option>
          <a-select-option value="month">自然月</a-select-option>
          <a-select-option value="season">季度</a-select-option>
        </a-select>
      </a-form-item>
      <a-form-item label="时间锚点">
        <a-select v-model:value="form.anchorType">
          <a-select-option value="event_time">行为发生时间</a-select-option>
          <a-select-option value="request_time">请求时间</a-select-option>
        </a-select>
      </a-form-item>

      <a-divider orientation="left">4. 横向维度（拆分子榜）</a-divider>
      <a-form-item label="维度字段" :wrapper-col="{ span: 18 }">
        <div v-for="(d, i) in form.dimensions" :key="i" style="margin-bottom: 8px; display: flex; gap: 8px">
          <a-input v-model:value="d.dimensionName" placeholder="名称，如 业务线" style="width: 160px" />
          <a-input v-model:value="d.dimensionField" placeholder="字段，如 business_id" style="width: 200px" />
          <a-checkbox v-model:checked="d.required">必填</a-checkbox>
          <a-button danger @click="form.dimensions.splice(i, 1)">删除</a-button>
        </div>
        <a-button type="dashed" @click="addDimension">+ 添加维度</a-button>
        <div style="color: #999; margin-top: 6px">不配置维度则为全站单榜；维度按顺序拼接生成 type_id。</div>
      </a-form-item>

      <a-form-item :wrapper-col="{ offset: 5 }">
        <a-checkbox v-model:checked="form.online">保存后立即上线</a-checkbox>
      </a-form-item>
      <a-form-item :wrapper-col="{ offset: 5 }">
        <a-button type="primary" :loading="saving" @click="submit">保存</a-button>
        <a-button style="margin-left: 8px" @click="$router.back()">取消</a-button>
      </a-form-item>
    </a-form>
  </a-card>
</template>

<script setup>
import { reactive, ref, onMounted, computed } from 'vue'
import { useRouter } from 'vue-router'
import { message } from 'ant-design-vue'
import { api } from '../api'

const props = defineProps({ id: { type: String, default: '' } })
const router = useRouter()
const isEdit = computed(() => !!props.id)
const saving = ref(false)

const form = reactive({
  rankName: '',
  bizCode: '',
  targetType: 'user',
  sortType: 'score_desc',
  sameScorePolicy: 'early_first',
  maxRankSize: 10000,
  cacheTtlSeconds: 3600,
  timeType: 'none',
  anchorType: 'event_time',
  dimensions: [],
  online: false,
})

function addDimension() {
  form.dimensions.push({ dimensionName: '', dimensionField: '', required: true })
}

async function load() {
  if (!isEdit.value) return
  const data = await api.getRank(props.id)
  const c = data.config
  Object.assign(form, {
    rankName: c.rankName,
    bizCode: c.bizCode,
    targetType: c.targetType,
    sortType: c.sortType,
    sameScorePolicy: c.sameScorePolicy,
    maxRankSize: c.maxRankSize,
    cacheTtlSeconds: c.cacheTtlSeconds,
    timeType: data.time?.timeType || 'none',
    anchorType: data.time?.anchorType || 'event_time',
    dimensions: (data.dimensions || []).map((d) => ({
      dimensionName: d.dimensionName,
      dimensionField: d.dimensionField,
      required: d.required === true || d.required === 1,
    })),
    online: c.status === 1,
  })
}

async function submit() {
  if (!form.rankName || !form.bizCode) {
    message.warning('请填写榜单名称和业务线')
    return
  }
  saving.value = true
  try {
    if (isEdit.value) {
      await api.updateRank(props.id, form)
      message.success('已保存')
    } else {
      const res = await api.createRank(form)
      message.success(`创建成功，榜单ID ${res.rankId}`)
    }
    router.push('/ranks')
  } finally {
    saving.value = false
  }
}

onMounted(load)
</script>
