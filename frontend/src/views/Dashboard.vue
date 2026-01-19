<script setup>
import { ref, onMounted, computed } from 'vue'
import { useCollectionStore } from '../stores/collection'
import { priceService } from '../services/api'
import StatsPanel from '../components/StatsPanel.vue'
import CardGrid from '../components/CardGrid.vue'
import CardDetail from '../components/CardDetail.vue'

const store = useCollectionStore()
const priceStatus = ref(null)
const priceStatusError = ref(null)
const selectedItem = ref(null)

const recentCards = computed(() => {
  return store.items.slice(0, 12)
})

const quotaRemaining = computed(() => {
  if (!priceStatus.value) return 0
  return priceStatus.value.remaining ?? 0
})

const quotaDailyLimit = computed(() => {
  if (!priceStatus.value) return 0
  return priceStatus.value.daily_limit ?? 0
})

const quotaPercentage = computed(() => {
  if (!priceStatus.value) return 0
  if (!quotaDailyLimit.value) return 0
  return Math.round((quotaRemaining.value / quotaDailyLimit.value) * 100)
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
      priceStatusError.value = null
    }).catch(err => {
      console.error('Failed to fetch price status:', err)
      priceStatusError.value = err.message
    })
  ])
})

const handleSelect = (item) => {
  selectedItem.value = item
}

const handleUpdate = async (data) => {
  await store.updateItem(data.id, data)
  selectedItem.value = null
}

const handleRemove = async (id) => {
  await store.removeItem(id)
  selectedItem.value = null
}

const handlePriceUpdated = (updatedCard) => {
  // Update the card in the store
  const item = store.items.find(i => i.card?.id === updatedCard.id || i.card_id === updatedCard.id)
  if (item && item.card) {
    item.card.price_usd = updatedCard.price_usd
    item.card.price_foil_usd = updatedCard.price_foil_usd
    item.card.price_updated_at = updatedCard.price_updated_at
    item.card.price_source = updatedCard.price_source
  }
}
</script>

<template>
  <div class="space-y-8">
    <div>
      <h1 class="text-3xl font-bold text-gray-800 dark:text-white mb-6">Dashboard</h1>
      <StatsPanel v-if="store.stats" :stats="store.stats" />

      <div v-if="priceStatus" class="mt-4 bg-white dark:bg-gray-800 rounded-lg shadow p-4">
        <h3 class="text-sm font-medium text-gray-700 dark:text-gray-300 mb-2">Pokemon Price API Quota</h3>
        <div class="flex items-center gap-4">
          <div class="flex-1">
            <div class="h-2 bg-gray-200 dark:bg-gray-700 rounded-full overflow-hidden">
              <div
                :class="quotaColor"
                :style="{ width: quotaPercentage + '%' }"
                class="h-full transition-all duration-300"
              ></div>
            </div>
          </div>
          <div class="text-sm text-gray-600 dark:text-gray-400">
            {{ quotaRemaining }} / {{ quotaDailyLimit }} remaining
          </div>
        </div>
        <p class="text-xs text-gray-500 dark:text-gray-500 mt-1">
          Resets at {{ formatResetTime(priceStatus.resets_at) }}
        </p>
      </div>
    </div>

    <div>
      <div class="flex justify-between items-center mb-4">
        <h2 class="text-xl font-semibold text-gray-800 dark:text-white">Recent Additions</h2>
        <router-link to="/collection" class="text-blue-600 dark:text-blue-400 hover:text-blue-800 dark:hover:text-blue-300">
          View All &rarr;
        </router-link>
      </div>

      <div v-if="store.loading" class="text-center py-8">
        <div class="animate-spin rounded-full h-12 w-12 border-b-2 border-blue-600 mx-auto"></div>
      </div>

      <div v-else-if="recentCards.length === 0" class="text-center py-12 bg-white dark:bg-gray-800 rounded-lg">
        <p class="text-gray-500 dark:text-gray-400 mb-4">Your collection is empty</p>
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
        @select="handleSelect"
      />
    </div>

    <CardDetail
      v-if="selectedItem"
      :item="selectedItem"
      :is-collection-item="true"
      @close="selectedItem = null"
      @update="handleUpdate"
      @remove="handleRemove"
      @priceUpdated="handlePriceUpdated"
    />
  </div>
</template>
