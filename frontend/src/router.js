import { createRouter, createWebHistory } from 'vue-router'
import Dashboard from './views/Dashboard.vue'
import Collection from './views/Collection.vue'
import AddCard from './views/AddCard.vue'
import NotFound from './views/NotFound.vue'

const routes = [
  {
    path: '/',
    name: 'Dashboard',
    component: Dashboard,
    meta: { title: 'Dashboard' }
  },
  {
    path: '/collection',
    name: 'Collection',
    component: Collection,
    meta: { title: 'Collection' }
  },
  {
    path: '/add',
    name: 'AddCard',
    component: AddCard,
    meta: { title: 'Add Card' }
  },
  {
    path: '/:pathMatch(.*)*',
    name: 'NotFound',
    component: NotFound,
    meta: { title: 'Page Not Found' }
  }
]

const router = createRouter({
  history: createWebHistory(),
  routes
})

// Update document title on route change
router.beforeEach((to, from, next) => {
  document.title = to.meta.title ? `${to.meta.title} - TCG Tracker` : 'TCG Tracker'
  next()
})

export default router
