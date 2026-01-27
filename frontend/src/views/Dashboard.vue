<script setup>
import { ref, onMounted, computed } from 'vue'
import { useCollectionStore } from '../stores/collection'
import { priceService } from '../services/api'
import StatsPanel from '../components/StatsPanel.vue'
import ValueChart from '../components/ValueChart.vue'
import CardGrid from '../components/CardGrid.vue'
import CardDetail from '../components/CardDetail.vue'

const store = useCollectionStore()
const priceStatus = ref(null)
const priceStatusError = ref(null)
const selectedItem = ref(null)

// Show most valuable scanned cards, sorted by actual condition-based price (high to low)
// Uses item_value from backend which accounts for condition, printing, and language
const topValueScans = computed(() => {
  return [...store.items]
    .filter(item => item.scanned_image_path) // Only show scanned items
    .sort((a, b) => (b.item_value || 0) - (a.item_value || 0))
    .slice(0, 12)
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

const unmatchedCards = computed(() => {
  return priceStatus.value?.unmatched_cards || []
})

const showUnmatchedWarning = computed(() => {
  return unmatchedCards.value.length > 0
})

const formatResetTime = (dateString) => {
  if (!dateString) return ''
  const date = new Date(dateString)
  return date.toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' })
}

const formatNextUpdate = computed(() => {
  if (!priceStatus.value?.next_update_time) return 'Unknown'
  const next = new Date(priceStatus.value.next_update_time)
  const now = new Date()
  const diffMs = next - now

  if (diffMs <= 0) return 'Any moment now'
  if (diffMs < 60000) return 'Less than a minute'

  const diffMins = Math.round(diffMs / 60000)
  if (diffMins === 1) return '1 minute'
  return `${diffMins} minutes`
})

const lastUpdateTime = computed(() => {
  if (!priceStatus.value?.last_update_time) return null
  const last = new Date(priceStatus.value.last_update_time)
  // Check if it's a zero time (Go's zero value)
  if (last.getFullYear() < 2000) return null
  return last.toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' })
})

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

      <div class="mt-4">
        <ValueChart />
      </div>

      <div v-if="priceStatus" class="mt-4 bg-white dark:bg-gray-800 rounded-lg shadow p-4">
        <h3 class="text-sm font-medium text-gray-700 dark:text-gray-300 mb-3">Price Update Status</h3>

        <!-- Next Update Countdown -->
        <div class="flex items-center justify-between mb-3 p-2 bg-blue-50 dark:bg-blue-900/20 rounded">
          <span class="text-sm text-gray-600 dark:text-gray-400">Next price update</span>
          <span class="text-sm font-medium text-blue-600 dark:text-blue-400">{{ formatNextUpdate }}</span>
        </div>

        <!-- Update Stats Grid -->
        <div class="grid grid-cols-3 gap-3 mb-3">
          <div class="text-center p-2 bg-gray-50 dark:bg-gray-700/50 rounded">
            <div class="text-lg font-semibold text-gray-800 dark:text-white">{{ priceStatus.cards_updated_today || 0 }}</div>
            <div class="text-xs text-gray-500 dark:text-gray-400">Updated today</div>
          </div>
          <div class="text-center p-2 bg-gray-50 dark:bg-gray-700/50 rounded">
            <div class="text-lg font-semibold text-gray-800 dark:text-white">{{ priceStatus.queue_size || 0 }}</div>
            <div class="text-xs text-gray-500 dark:text-gray-400">In queue</div>
          </div>
          <div class="text-center p-2 bg-gray-50 dark:bg-gray-700/50 rounded">
            <div class="text-lg font-semibold text-gray-800 dark:text-white">{{ priceStatus.batch_size || 20 }}</div>
            <div class="text-xs text-gray-500 dark:text-gray-400">Per batch</div>
          </div>
        </div>

        <!-- API Quota Bar -->
        <div class="mb-2">
          <div class="flex items-center justify-between text-xs text-gray-500 dark:text-gray-400 mb-1">
            <span>JustTCG API Quota</span>
            <span>{{ quotaRemaining }} / {{ quotaDailyLimit }} remaining</span>
          </div>
          <div class="h-2 bg-gray-200 dark:bg-gray-700 rounded-full overflow-hidden">
            <div
              :class="quotaColor"
              :style="{ width: quotaPercentage + '%' }"
              class="h-full transition-all duration-300"
            ></div>
          </div>
        </div>

        <!-- Footer Info -->
        <div class="flex items-center justify-between text-xs text-gray-500 dark:text-gray-500">
          <span v-if="lastUpdateTime">Last update: {{ lastUpdateTime }}</span>
          <span v-else>No updates yet</span>
          <span>Quota resets at {{ formatResetTime(priceStatus.resets_at) }}</span>
        </div>
      </div>

      <!-- Unmatched Cards Warning -->
      <div v-if="showUnmatchedWarning" class="mt-4 bg-amber-50 dark:bg-amber-900/20 border border-amber-200 dark:border-amber-700 rounded-lg p-4">
        <div class="flex items-start gap-3">
          <svg class="w-5 h-5 text-amber-500 mt-0.5 flex-shrink-0" fill="currentColor" viewBox="0 0 20 20">
            <path fill-rule="evenodd" d="M8.257 3.099c.765-1.36 2.722-1.36 3.486 0l5.58 9.92c.75 1.334-.213 2.98-1.742 2.98H4.42c-1.53 0-2.493-1.646-1.743-2.98l5.58-9.92zM11 13a1 1 0 11-2 0 1 1 0 012 0zm-1-8a1 1 0 00-1 1v3a1 1 0 002 0V6a1 1 0 00-1-1z" clip-rule="evenodd" />
          </svg>
          <div class="flex-1">
            <h4 class="text-sm font-medium text-amber-800 dark:text-amber-200">
              {{ unmatchedCards.length }} card{{ unmatchedCards.length === 1 ? '' : 's' }} cannot receive price updates
            </h4>
            <p class="mt-1 text-xs text-amber-700 dark:text-amber-300">
              These cards couldn't be matched in the pricing database. Their prices will not be updated automatically.
            </p>
            <details class="mt-2">
              <summary class="text-xs text-amber-600 dark:text-amber-400 cursor-pointer hover:text-amber-800 dark:hover:text-amber-200">
                Show affected cards
              </summary>
              <ul class="mt-2 space-y-1 text-xs text-amber-700 dark:text-amber-300">
                <li v-for="card in unmatchedCards" :key="card.card_id" class="flex justify-between items-start gap-2 py-1 border-b border-amber-200/50 dark:border-amber-700/50 last:border-0">
                  <span class="font-medium">{{ card.name }}</span>
                  <span class="text-amber-600 dark:text-amber-400 text-right">
                    {{ card.set_name }}{{ card.card_number ? ` #${card.card_number}` : '' }}
                  </span>
                </li>
              </ul>
            </details>
          </div>
        </div>
      </div>
    </div>

    <div>
      <div class="flex justify-between items-center mb-4">
        <h2 class="text-xl font-semibold text-gray-800 dark:text-white">Most Valuable Scans</h2>
        <router-link to="/collection" class="text-blue-600 dark:text-blue-400 hover:text-blue-800 dark:hover:text-blue-300">
          View All &rarr;
        </router-link>
      </div>

      <div v-if="store.loading" class="text-center py-8">
        <div class="animate-spin rounded-full h-12 w-12 border-b-2 border-blue-600 mx-auto"></div>
      </div>

      <div v-else-if="topValueScans.length === 0" class="text-center py-12 bg-white dark:bg-gray-800 rounded-lg">
        <p class="text-gray-500 dark:text-gray-400 mb-4">No scanned cards yet</p>
        <p class="text-sm text-gray-400 dark:text-gray-500">Scan cards with the mobile app to see them here</p>
      </div>

      <CardGrid
        v-else
        :cards="topValueScans"
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
