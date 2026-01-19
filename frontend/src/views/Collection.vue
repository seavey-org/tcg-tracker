<script setup>
import { ref, onMounted, computed } from 'vue'
import { useCollectionStore } from '../stores/collection'
import CardGrid from '../components/CardGrid.vue'
import CardDetail from '../components/CardDetail.vue'

const store = useCollectionStore()

const selectedItem = ref(null)
const filterGame = ref('all')
const sortBy = ref('added_at')

// Use grouped items for display
const filteredItems = computed(() => {
  let items = [...store.groupedItems]

  if (filterGame.value !== 'all') {
    items = items.filter(item => item.card.game === filterGame.value)
  }

  items.sort((a, b) => {
    switch (sortBy.value) {
      case 'name':
        return a.card.name.localeCompare(b.card.name)
      case 'value':
        return (b.total_value || 0) - (a.total_value || 0)
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

const handleSelect = (groupedItem) => {
  selectedItem.value = groupedItem
}

const handleUpdate = async ({ id, quantity, condition, printing }) => {
  const result = await store.updateItem(id, { quantity, condition, printing })
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
  await store.refreshPrices()
}

const handlePriceUpdated = () => {
  // Refresh the collection to get updated data
  store.fetchGroupedCollection()
}

const handleClose = () => {
  selectedItem.value = null
}

onMounted(() => {
  store.fetchGroupedCollection()
  store.fetchStats()
})
</script>

<template>
  <div>
    <div class="flex flex-col sm:flex-row justify-between items-start sm:items-center gap-4 mb-6">
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
        </select>

        <button
          @click="handleRefreshPrices"
          class="bg-green-600 text-white px-4 py-2 rounded-lg hover:bg-green-700 transition"
        >
          Refresh Prices
        </button>
      </div>
    </div>

    <div v-if="store.loading" class="text-center py-8">
      <div class="animate-spin rounded-full h-12 w-12 border-b-2 border-blue-600 mx-auto"></div>
    </div>

    <div v-else-if="filteredItems.length === 0" class="text-center py-12 bg-white dark:bg-gray-800 rounded-lg">
      <p class="text-gray-500 dark:text-gray-400 mb-4">No cards found</p>
      <router-link
        to="/add"
        class="inline-block bg-blue-600 text-white px-6 py-2 rounded-lg hover:bg-blue-700"
      >
        Add Cards
      </router-link>
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
