import { createRouter, createWebHistory } from 'vue-router'
import Dashboard from './views/Dashboard.vue'
import Collection from './views/Collection.vue'
import AddCard from './views/AddCard.vue'

const routes = [
  {
    path: '/',
    name: 'Dashboard',
    component: Dashboard
  },
  {
    path: '/collection',
    name: 'Collection',
    component: Collection
  },
  {
    path: '/add',
    name: 'AddCard',
    component: AddCard
  }
]

const router = createRouter({
  history: createWebHistory(),
  routes
})

export default router
