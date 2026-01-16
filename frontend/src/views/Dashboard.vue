<script setup>
import { ref, onMounted, computed } from 'vue'
import { useCollectionStore } from '../stores/collection'
import { priceService } from '../services/api'
import StatsPanel from '../components/StatsPanel.vue'
import CardGrid from '../components/CardGrid.vue'

const store = useCollectionStore()
const priceStatus = ref(null)

const recentCards = computed(() => {
  return store.items.slice(0, 12)
})

const quotaPercentage = computed(() => {
  if (!priceStatus.value) return 0
  return Math.round((priceStatus.value.remaining / priceStatus.value.daily_limit) * 100)
})

const quotaColor = computed(() => {
  const pct = quotaPercentage.value
  if (pct > 50) return 'bg-green-500'
  if (pct > 20) return 'bg-yellow-500'
  return 'bg-red-500'
})

const formatResetTime = (dateString) => {
  if (!dateString) return ''
  const date = new Date(dateString)
  return date.toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' })
}

onMounted(async () => {
  await Promise.all([
    store.fetchCollection(),
    store.fetchStats(),
    priceService.getStatus().then(status => {
      priceStatus.value = status
    }).catch(() => {})
  ])
})
</script>

<template>
  <div class="space-y-8">
    <div>
      <h1 class="text-3xl font-bold text-gray-800 mb-6">Dashboard</h1>
      <StatsPanel v-if="store.stats" :stats="store.stats" />

      <div v-if="priceStatus" class="mt-4 bg-white rounded-lg shadow p-4">
        <h3 class="text-sm font-medium text-gray-700 mb-2">Pokemon Price API Quota</h3>
        <div class="flex items-center gap-4">
          <div class="flex-1">
            <div class="h-2 bg-gray-200 rounded-full overflow-hidden">
              <div
                :class="quotaColor"
                :style="{ width: quotaPercentage + '%' }"
                class="h-full transition-all duration-300"
              ></div>
            </div>
          </div>
          <div class="text-sm text-gray-600">
            {{ priceStatus.remaining }} / {{ priceStatus.daily_limit }} remaining
          </div>
        </div>
        <p class="text-xs text-gray-500 mt-1">
          Resets at {{ formatResetTime(priceStatus.resets_at) }}
        </p>
      </div>
    </div>

    <div>
      <div class="flex justify-between items-center mb-4">
        <h2 class="text-xl font-semibold text-gray-800">Recent Additions</h2>
        <router-link to="/collection" class="text-blue-600 hover:text-blue-800">
          View All &rarr;
        </router-link>
      </div>

      <div v-if="store.loading" class="text-center py-8">
        <div class="animate-spin rounded-full h-12 w-12 border-b-2 border-blue-600 mx-auto"></div>
      </div>

      <div v-else-if="recentCards.length === 0" class="text-center py-12 bg-white rounded-lg">
        <p class="text-gray-500 mb-4">Your collection is empty</p>
        <router-link
          to="/add"
          class="inline-block bg-blue-600 text-white px-6 py-2 rounded-lg hover:bg-blue-700"
        >
          Add Your First Card
        </router-link>
      </div>

      <CardGrid
        v-else
        :cards="recentCards"
        :show-quantity="true"
      />
    </div>
  </div>
</template>
