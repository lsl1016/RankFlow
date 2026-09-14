import axios from 'axios'
import { message } from 'ant-design-vue'

const ADMIN_TOKEN_KEY = 'rankflow.adminToken'

export function getAdminToken() {
  return sessionStorage.getItem(ADMIN_TOKEN_KEY) || ''
}

export function setAdminToken(token) {
  const value = String(token || '').trim()
  if (value) sessionStorage.setItem(ADMIN_TOKEN_KEY, value)
  else sessionStorage.removeItem(ADMIN_TOKEN_KEY)
}

const http = axios.create({ baseURL: '/api', timeout: 10000 })

http.interceptors.request.use((config) => {
  const token = getAdminToken()
  if (token) {
    config.headers = config.headers || {}
    config.headers.Authorization = `Bearer ${token}`
  }
  return config
})

http.interceptors.response.use(
  (res) => {
    const body = res.data
    if (body && body.code !== 0) {
      message.error(body.message || '请求失败')
      return Promise.reject(new Error(body.message))
    }
    return body.data
  },
  (err) => {
    const status = err.response?.status
    if (status === 401) {
      window.dispatchEvent(new CustomEvent('rankflow:auth-required'))
      message.error('未授权，请设置管理令牌')
    } else if (status === 403) {
      message.error('当前令牌权限不足，需要 Admin 权限')
    } else {
      const msg = err.response?.data?.message || err.message || '网络错误'
      message.error(msg)
    }
    return Promise.reject(err)
  },
)

export const api = {
  listRanks: (params) => http.get('/ranks', { params }),
  getRank: (id) => http.get(`/ranks/${id}`),
  createRank: (body) => http.post('/ranks', body),
  updateRank: (id, body) => http.put(`/ranks/${id}`, body),
  setStatus: (id, status) => http.post(`/ranks/${id}/status`, { status }),
  listSubBoards: (id) => http.get(`/ranks/${id}/subboards`),
  resolveSubBoard: (id, body) => http.post(`/ranks/${id}/subboards`, body),
  setSubBoardStatus: (id, body) => http.post(`/ranks/${id}/subboards/status`, body),
  addScore: (id, body) => http.post(`/ranks/${id}/score/add`, body),
  setScore: (id, body) => http.post(`/ranks/${id}/score/set`, body),
  top: (id, params) => http.get(`/ranks/${id}/top`, { params }),
  memberRank: (id, itemId, params) => http.get(`/ranks/${id}/members/${itemId}/rank`, { params }),
  stats: (id, params) => http.get(`/ranks/${id}/stats`, { params }),
}

export const STATUS_TEXT = { 0: '草稿', 1: '已上线', 2: '已下线', 3: '已归档' }
export const STATUS_COLOR = { 0: 'default', 1: 'green', 2: 'orange', 3: 'gray' }
export const TIME_TYPE_TEXT = {
  none: '无', hour: '小时', day: '日榜', week: '周榜', month: '月榜', season: '季度', custom: '自定义',
}
