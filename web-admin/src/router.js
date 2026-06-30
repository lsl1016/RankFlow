import { createRouter, createWebHashHistory } from 'vue-router'
import RankList from './views/RankList.vue'
import RankForm from './views/RankForm.vue'
import RankDetail from './views/RankDetail.vue'

const routes = [
  { path: '/', redirect: '/ranks' },
  { path: '/ranks', component: RankList },
  { path: '/ranks/new', component: RankForm },
  { path: '/ranks/:id/edit', component: RankForm, props: true },
  { path: '/ranks/:id', component: RankDetail, props: true },
]

export default createRouter({
  history: createWebHashHistory(),
  routes,
})
