<script setup>
import { ref, onMounted, computed, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { useCollectionStore } from '../stores/collection'
import CardGrid from '../components/CardGrid.vue'
import CardDetail from '../components/CardDetail.vue'
import CollectionFilters from '../components/CollectionFilters.vue'

const store = useCollectionStore()
const route = useRoute()
const router = useRouter()

const selectedItem = ref(null)
const filterGame = ref('all')
const sortBy = ref('value') // Default to value (price) sorting
const searchQuery = ref('')
const refreshMessage = ref(null)
const refreshing = ref(false)

// Advanced filter state
const filters = ref({
  printings: [],
  sets: [],
  conditions: [],
  rarities: [],
  languages: []
})

// Extract available filter options from collection data
const availablePrintings = computed(() => {
  const printings = new Set()
  store.groupedItems.forEach(group => {
    group.items?.forEach(item => {
      if (item.printing) printings.add(item.printing)
    })
  })
  // Sort with a sensible order
  const order = ['Normal', 'Foil', '1st Edition', 'Unlimited', 'Reverse Holofoil']
  return [...printings].sort((a, b) => {
    const aIdx = order.indexOf(a)
    const bIdx = order.indexOf(b)
    if (aIdx === -1 && bIdx === -1) return a.localeCompare(b)
    if (aIdx === -1) return 1
    if (bIdx === -1) return -1
    return aIdx - bIdx
  })
})

const availableSets = computed(() => {
  const sets = new Map()
  store.groupedItems.forEach(group => {
    const card = group.card
    if (card?.set_code && card?.set_name) {
      sets.set(card.set_code, { code: card.set_code, name: card.set_name })
    }
  })
  return [...sets.values()].sort((a, b) => a.name.localeCompare(b.name))
})

const availableConditions = computed(() => {
  const conditions = new Set()
  store.groupedItems.forEach(group => {
    group.items?.forEach(item => {
      if (item.condition) conditions.add(item.condition)
    })
  })
  // Sort by condition quality
  const order = ['M', 'NM', 'EX', 'GD', 'LP', 'PL', 'PR']
  return [...conditions].sort((a, b) => {
    const aIdx = order.indexOf(a)
    const bIdx = order.indexOf(b)
    if (aIdx === -1 && bIdx === -1) return a.localeCompare(b)
    if (aIdx === -1) return 1
    if (bIdx === -1) return -1
    return aIdx - bIdx
  })
})

const availableRarities = computed(() => {
  const rarities = new Set()
  store.groupedItems.forEach(group => {
    if (group.card?.rarity) {
      rarities.add(group.card.rarity)
    }
  })
  return [...rarities].sort()
})

const availableLanguages = computed(() => {
  const languages = new Set()
  store.groupedItems.forEach(group => {
    group.items?.forEach(item => {
      if (item.language) languages.add(item.language)
    })
  })
  // Sort with English first, then alphabetically
  return [...languages].sort((a, b) => {
    if (a === 'English') return -1
    if (b === 'English') return 1
    return a.localeCompare(b)
  })
})

// Use grouped items for display with search filtering
const filteredItems = computed(() => {
  let items = [...store.groupedItems]

  // Game filter
  if (filterGame.value !== 'all') {
    items = items.filter(item => item.card.game === filterGame.value)
  }

  // Search filter (name, set)
  if (searchQuery.value.trim()) {
    const query = searchQuery.value.toLowerCase().trim()
    items = items.filter(item => {
      const card = item.card
      return (
        card.name?.toLowerCase().includes(query) ||
        card.set_name?.toLowerCase().includes(query) ||
        card.set_code?.toLowerCase().includes(query)
      )
    })
  }

  // Printing filter - match if ANY variant has a selected printing
  if (filters.value.printings.length > 0) {
    items = items.filter(group =>
      group.items?.some(item => filters.value.printings.includes(item.printing))
    )
  }

  // Set filter - match by card's set_code
  if (filters.value.sets.length > 0) {
    items = items.filter(group =>
      filters.value.sets.includes(group.card?.set_code)
    )
  }

  // Condition filter - match if ANY variant has a selected condition
  if (filters.value.conditions.length > 0) {
    items = items.filter(group =>
      group.items?.some(item => filters.value.conditions.includes(item.condition))
    )
  }

  // Rarity filter - match by card's rarity
  if (filters.value.rarities.length > 0) {
    items = items.filter(group =>
      filters.value.rarities.includes(group.card?.rarity)
    )
  }

  // Language filter - match if ANY variant has a selected language
  if (filters.value.languages.length > 0) {
    items = items.filter(group =>
      group.items?.some(item => filters.value.languages.includes(item.language))
    )
  }

  // Sorting
  items.sort((a, b) => {
    switch (sortBy.value) {
      case 'name':
        return a.card.name.localeCompare(b.card.name)
      case 'value':
        return (b.total_value || 0) - (a.total_value || 0)
      case 'price_updated': {
        // Sort by most recently updated price
        const priceA = a.card.price_updated_at ? new Date(a.card.price_updated_at) : new Date(0)
        const priceB = b.card.price_updated_at ? new Date(b.card.price_updated_at) : new Date(0)
        return priceB - priceA
      }
      case 'added_at':
      default: {
        // For grouped items, use the most recent item's added_at
        const latestA = a.items?.[0]?.added_at || ''
        const latestB = b.items?.[0]?.added_at || ''
        return new Date(latestB) - new Date(latestA)
      }
    }
  })

  return items
})

// Count of active filters for display
const activeFilterCount = computed(() => {
  return filters.value.printings.length +
         filters.value.sets.length +
         filters.value.conditions.length +
         filters.value.rarities.length +
         filters.value.languages.length
})

const handleSelect = (groupedItem) => {
  selectedItem.value = groupedItem
}

const handleUpdate = async ({ id, quantity, condition, printing, language, card_id }) => {
  const result = await store.updateItem(id, { quantity, condition, printing, language, card_id })
  // Show feedback about the operation
  if (result.message) {
    // Could show a toast here
    console.log('Update operation:', result.operation, result.message)
  }
  selectedItem.value = null
}

const handleRemove = async (id) => {
  await store.removeItem(id)
  selectedItem.value = null
}

const handleRefreshPrices = async () => {
  refreshing.value = true
  refreshMessage.value = null
  try {
    const result = await store.refreshPrices()
    if (result.updated > 0) {
      refreshMessage.value = {
        type: 'success',
        text: `Updated ${result.updated} card prices.`
      }
    } else {
      refreshMessage.value = {
        type: 'info',
        text: result.daily_remaining === 0
          ? 'API quota exhausted. Resets at midnight.'
          : 'No cards needed price updates.'
      }
    }
    // Auto-hide message after 5 seconds
    setTimeout(() => { refreshMessage.value = null }, 5000)
  } catch (err) {
    refreshMessage.value = {
      type: 'error',
      text: err.message || 'Failed to update prices'
    }
  } finally {
    refreshing.value = false
  }
}

const handlePriceUpdated = () => {
  // Refresh the collection to get updated data
  store.fetchGroupedCollection()
}

const handleClose = () => {
  selectedItem.value = null
}

// Parse URL query params to filter arrays
const parseQueryArray = (param) => {
  if (!param) return []
  if (Array.isArray(param)) return param
  return param.split(',').filter(v => v.trim())
}

// Sync filters to URL query params (without polluting browser history)
const syncFiltersToUrl = () => {
  const query = {}

  if (filters.value.printings.length > 0) {
    query.printings = filters.value.printings.join(',')
  }
  if (filters.value.sets.length > 0) {
    query.sets = filters.value.sets.join(',')
  }
  if (filters.value.conditions.length > 0) {
    query.conditions = filters.value.conditions.join(',')
  }
  if (filters.value.rarities.length > 0) {
    query.rarities = filters.value.rarities.join(',')
  }
  if (filters.value.languages.length > 0) {
    query.languages = filters.value.languages.join(',')
  }
  if (filterGame.value !== 'all') {
    query.game = filterGame.value
  }
  if (searchQuery.value.trim()) {
    query.q = searchQuery.value.trim()
  }
  // Only add sort to URL if it's not the default (value)
  if (sortBy.value !== 'value') {
    query.sort = sortBy.value
  }

  router.replace({ query })
}

// Watch filters and sync to URL
watch(filters, syncFiltersToUrl, { deep: true })
watch(filterGame, syncFiltersToUrl)
watch(searchQuery, syncFiltersToUrl)
watch(sortBy, syncFiltersToUrl)

// Initialize filters from URL on mount
const initFiltersFromUrl = () => {
  const q = route.query

  filters.value = {
    printings: parseQueryArray(q.printings),
    sets: parseQueryArray(q.sets),
    conditions: parseQueryArray(q.conditions),
    rarities: parseQueryArray(q.rarities),
    languages: parseQueryArray(q.languages)
  }

  if (q.game && ['mtg', 'pokemon'].includes(q.game)) {
    filterGame.value = q.game
  }

  if (q.q) {
    searchQuery.value = q.q
  }

  // Read sort from URL, default to 'value' if not specified
  if (q.sort && ['added_at', 'name', 'value', 'price_updated'].includes(q.sort)) {
    sortBy.value = q.sort
  }
}

onMounted(() => {
  initFiltersFromUrl()
  store.fetchGroupedCollection()
  store.fetchStats()
})
</script>

<template>
  <div>
    <div class="flex flex-col gap-4 mb-6">
      <div class="flex flex-col sm:flex-row justify-between items-start sm:items-center gap-4">
        <h1 class="text-3xl font-bold text-gray-800 dark:text-white">My Collection</h1>

        <div class="flex flex-wrap gap-3">
          <select
            v-model="filterGame"
            class="border dark:border-gray-600 rounded-lg px-3 py-2 bg-white dark:bg-gray-800 text-gray-900 dark:text-white"
          >
            <option value="all">All Games</option>
            <option value="mtg">Magic: The Gathering</option>
            <option value="pokemon">Pokemon</option>
          </select>

          <select
            v-model="sortBy"
            class="border dark:border-gray-600 rounded-lg px-3 py-2 bg-white dark:bg-gray-800 text-gray-900 dark:text-white"
          >
            <option value="added_at">Recently Added</option>
            <option value="name">Name</option>
            <option value="value">Value</option>
            <option value="price_updated">Price Updated</option>
          </select>

          <button
            @click="handleRefreshPrices"
            :disabled="refreshing"
            class="bg-green-600 text-white px-4 py-2 rounded-lg hover:bg-green-700 transition disabled:opacity-50 disabled:cursor-not-allowed flex items-center gap-2"
          >
            <span v-if="refreshing" class="animate-spin h-4 w-4 border-2 border-white border-t-transparent rounded-full"></span>
            {{ refreshing ? 'Updating...' : 'Update Prices Now' }}
          </button>
        </div>
      </div>

      <!-- Search Bar and Filters -->
      <div class="flex flex-col gap-3">
        <div class="flex gap-3">
          <input
            v-model="searchQuery"
            type="text"
            placeholder="Search by card name or set..."
            class="flex-1 border dark:border-gray-600 rounded-lg px-4 py-2 bg-white dark:bg-gray-800 text-gray-900 dark:text-white placeholder-gray-400"
          />
          <button
            v-if="searchQuery"
            @click="searchQuery = ''"
            class="px-3 py-2 text-gray-500 hover:text-gray-700 dark:hover:text-gray-300"
          >
            Clear
          </button>
        </div>

        <!-- Advanced Filters -->
        <CollectionFilters
          v-model="filters"
          :available-printings="availablePrintings"
          :available-sets="availableSets"
          :available-conditions="availableConditions"
          :available-rarities="availableRarities"
          :available-languages="availableLanguages"
        />
      </div>

      <!-- Refresh Message Toast -->
      <div
        v-if="refreshMessage"
        :class="{
          'bg-green-100 dark:bg-green-900/30 text-green-800 dark:text-green-300 border-green-300 dark:border-green-700': refreshMessage.type === 'success',
          'bg-blue-100 dark:bg-blue-900/30 text-blue-800 dark:text-blue-300 border-blue-300 dark:border-blue-700': refreshMessage.type === 'info',
          'bg-red-100 dark:bg-red-900/30 text-red-800 dark:text-red-300 border-red-300 dark:border-red-700': refreshMessage.type === 'error'
        }"
        class="p-3 rounded-lg border text-sm"
      >
        {{ refreshMessage.text }}
      </div>
    </div>

    <div v-if="store.loading" class="text-center py-8">
      <div class="animate-spin rounded-full h-12 w-12 border-b-2 border-blue-600 mx-auto"></div>
    </div>

    <div v-else-if="filteredItems.length === 0" class="text-center py-12 bg-white dark:bg-gray-800 rounded-lg">
      <template v-if="activeFilterCount > 0 || searchQuery.trim()">
        <p class="text-gray-500 dark:text-gray-400 mb-4">No cards match your filters</p>
        <button
          @click="filters = { printings: [], sets: [], conditions: [], rarities: [], languages: [] }; searchQuery = ''"
          class="inline-block bg-gray-600 text-white px-6 py-2 rounded-lg hover:bg-gray-700"
        >
          Clear Filters
        </button>
      </template>
      <template v-else>
        <p class="text-gray-500 dark:text-gray-400 mb-4">No cards found</p>
        <router-link
          to="/add"
          class="inline-block bg-blue-600 text-white px-6 py-2 rounded-lg hover:bg-blue-700"
        >
          Add Cards
        </router-link>
      </template>
    </div>

    <CardGrid
      v-else
      :cards="filteredItems"
      :show-quantity="true"
      :grouped="true"
      @select="handleSelect"
    />

    <CardDetail
      v-if="selectedItem"
      :item="selectedItem"
      :is-collection-item="true"
      :is-grouped="true"
      @close="handleClose"
      @update="handleUpdate"
      @remove="handleRemove"
      @priceUpdated="handlePriceUpdated"
    />
  </div>
</template>
