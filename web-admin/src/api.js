import axios from 'axios'
import { message } from 'ant-design-vue'

const http = axios.create({ baseURL: '/api', timeout: 10000 })

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
    const msg = err.response?.data?.message || err.message || '网络错误'
    message.error(msg)
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
