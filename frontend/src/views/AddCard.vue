<script setup>
import { ref } from 'vue'
import { useCollectionStore } from '../stores/collection'
import SearchBar from '../components/SearchBar.vue'
import CardGrid from '../components/CardGrid.vue'
import CardDetail from '../components/CardDetail.vue'

const store = useCollectionStore()

const selectedCard = ref(null)
const message = ref('')

const handleSearch = async ({ query, game }) => {
  message.value = ''
  await store.searchCards(query, game)
}

const handleSelect = (card) => {
  selectedCard.value = card
}

const handleAdd = async ({ cardId, quantity, condition, foil }) => {
  try {
    await store.addToCollection(cardId, { quantity, condition, foil })
    message.value = 'Card added to collection!'
    selectedCard.value = null
    setTimeout(() => { message.value = '' }, 3000)
  } catch (err) {
    message.value = 'Failed to add card: ' + err.message
  }
}
</script>

<template>
  <div>
    <h1 class="text-3xl font-bold text-gray-800 mb-6">Add Cards</h1>

    <div class="bg-white rounded-lg shadow p-6 mb-6">
      <SearchBar @search="handleSearch" />

      <div
        v-if="message"
        class="mt-4 p-3 rounded-lg"
        :class="message.includes('Failed') ? 'bg-red-100 text-red-700' : 'bg-green-100 text-green-700'"
      >
        {{ message }}
      </div>
    </div>

    <div v-if="store.searchLoading" class="text-center py-8">
      <div class="animate-spin rounded-full h-12 w-12 border-b-2 border-blue-600 mx-auto"></div>
      <p class="mt-4 text-gray-500">Searching...</p>
    </div>

    <div v-else-if="store.searchResults.length > 0">
      <div class="flex justify-between items-center mb-4">
        <h2 class="text-xl font-semibold text-gray-800">
          Search Results ({{ store.searchResults.length }})
        </h2>
        <button
          @click="store.clearSearch"
          class="text-gray-500 hover:text-gray-700"
        >
          Clear
        </button>
      </div>

      <CardGrid
        :cards="store.searchResults"
        @select="handleSelect"
      />
    </div>

    <div v-else class="text-center py-12 bg-white rounded-lg">
      <svg class="mx-auto h-12 w-12 text-gray-400" fill="none" stroke="currentColor" viewBox="0 0 24 24">
        <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M21 21l-6-6m2-5a7 7 0 11-14 0 7 7 0 0114 0z" />
      </svg>
      <p class="mt-4 text-gray-500">Search for cards to add to your collection</p>
    </div>

    <CardDetail
      v-if="selectedCard"
      :item="selectedCard"
      :is-collection-item="false"
      @close="selectedCard = null"
      @add="handleAdd"
    />
  </div>
</template>
