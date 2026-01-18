<script setup>
import { ref } from 'vue'
import { useCollectionStore } from '../stores/collection'
import SearchBar from '../components/SearchBar.vue'
import CardGrid from '../components/CardGrid.vue'
import CardDetail from '../components/CardDetail.vue'
import ImageUpload from '../components/ImageUpload.vue'

const store = useCollectionStore()

const selectedCard = ref(null)
const message = ref('')
const searchMode = ref('text') // 'text' or 'image'
const selectedGame = ref('pokemon')
const scanMetadata = ref(null)

const handleSearch = async ({ query, game }) => {
  message.value = ''
  selectedGame.value = game
  scanMetadata.value = null
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

const handlePriceUpdated = (updatedCard) => {
  // Update the card in the selected card and search results
  if (selectedCard.value) {
    selectedCard.value.price_usd = updatedCard.price_usd
    selectedCard.value.price_foil_usd = updatedCard.price_foil_usd
    selectedCard.value.price_updated_at = updatedCard.price_updated_at
    selectedCard.value.price_source = updatedCard.price_source
  }
}

const handleImageResults = (result) => {
  message.value = ''
  scanMetadata.value = result.parsed || null

  // Convert cards to the format expected by the store
  if (result.cards && result.cards.length > 0) {
    store.searchResults = result.cards
    store.searchLoading = false

    // Show set identification info if available
    if (result.set_icon?.best_set_id && !result.set_icon.low_confidence) {
      message.value = `Set identified: ${result.set_icon.best_set_id}`
      setTimeout(() => { message.value = '' }, 5000)
    }
  } else {
    message.value = 'No cards found in image'
  }
}

const handleImageError = (error) => {
  message.value = error
  setTimeout(() => { message.value = '' }, 5000)
}
</script>

<template>
  <div>
    <h1 class="text-3xl font-bold text-gray-800 dark:text-white mb-6">Add Cards</h1>

    <div class="bg-white dark:bg-gray-800 rounded-lg shadow p-6 mb-6">
      <!-- Mode Toggle -->
      <div class="flex border-b border-gray-200 dark:border-gray-700 mb-4">
        <button
          @click="searchMode = 'text'"
          class="px-4 py-2 font-medium text-sm transition-colors"
          :class="{
            'text-blue-600 dark:text-blue-400 border-b-2 border-blue-600 dark:border-blue-400': searchMode === 'text',
            'text-gray-500 dark:text-gray-400 hover:text-gray-700 dark:hover:text-gray-300': searchMode !== 'text'
          }"
        >
          <svg class="w-5 h-5 inline-block mr-1" fill="none" stroke="currentColor" viewBox="0 0 24 24">
            <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M21 21l-6-6m2-5a7 7 0 11-14 0 7 7 0 0114 0z" />
          </svg>
          Text Search
        </button>
        <button
          @click="searchMode = 'image'"
          class="px-4 py-2 font-medium text-sm transition-colors"
          :class="{
            'text-blue-600 dark:text-blue-400 border-b-2 border-blue-600 dark:border-blue-400': searchMode === 'image',
            'text-gray-500 dark:text-gray-400 hover:text-gray-700 dark:hover:text-gray-300': searchMode !== 'image'
          }"
        >
          <svg class="w-5 h-5 inline-block mr-1" fill="none" stroke="currentColor" viewBox="0 0 24 24">
            <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M3 9a2 2 0 012-2h.93a2 2 0 001.664-.89l.812-1.22A2 2 0 0110.07 4h3.86a2 2 0 011.664.89l.812 1.22A2 2 0 0018.07 7H19a2 2 0 012 2v9a2 2 0 01-2 2H5a2 2 0 01-2-2V9z" />
            <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M15 13a3 3 0 11-6 0 3 3 0 016 0z" />
          </svg>
          Image Scan
        </button>
      </div>

      <!-- Text Search Mode -->
      <div v-if="searchMode === 'text'">
        <SearchBar @search="handleSearch" />
      </div>

      <!-- Image Upload Mode -->
      <div v-else>
        <!-- Game Selector -->
        <div class="mb-4">
          <label class="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-2">Game</label>
          <div class="flex space-x-2">
            <button
              @click="selectedGame = 'pokemon'"
              class="px-4 py-2 rounded-lg text-sm font-medium transition-colors"
              :class="{
                'bg-blue-600 text-white': selectedGame === 'pokemon',
                'bg-gray-200 dark:bg-gray-700 text-gray-700 dark:text-gray-300 hover:bg-gray-300 dark:hover:bg-gray-600': selectedGame !== 'pokemon'
              }"
            >
              Pokemon
            </button>
            <button
              @click="selectedGame = 'mtg'"
              class="px-4 py-2 rounded-lg text-sm font-medium transition-colors"
              :class="{
                'bg-blue-600 text-white': selectedGame === 'mtg',
                'bg-gray-200 dark:bg-gray-700 text-gray-700 dark:text-gray-300 hover:bg-gray-300 dark:hover:bg-gray-600': selectedGame !== 'mtg'
              }"
            >
              MTG
            </button>
          </div>
        </div>

        <ImageUpload
          :game="selectedGame"
          @results="handleImageResults"
          @error="handleImageError"
        />
      </div>

      <!-- Message -->
      <div
        v-if="message"
        class="mt-4 p-3 rounded-lg"
        :class="message.includes('Failed') || message.includes('No cards') ? 'bg-red-100 dark:bg-red-900 text-red-700 dark:text-red-200' : 'bg-green-100 dark:bg-green-900 text-green-700 dark:text-green-200'"
      >
        {{ message }}
      </div>

      <!-- Scan Metadata Display -->
      <div v-if="scanMetadata && searchMode === 'image'" class="mt-4 p-3 bg-gray-100 dark:bg-gray-700 rounded-lg text-sm">
        <div class="font-medium text-gray-700 dark:text-gray-300 mb-2">Detected Information:</div>
        <div class="grid grid-cols-2 gap-2 text-gray-600 dark:text-gray-400">
          <div v-if="scanMetadata.card_name">Name: <span class="font-medium">{{ scanMetadata.card_name }}</span></div>
          <div v-if="scanMetadata.card_number">Number: <span class="font-medium">{{ scanMetadata.card_number }}</span></div>
          <div v-if="scanMetadata.set_code">Set: <span class="font-medium">{{ scanMetadata.set_code }}</span></div>
          <div v-if="scanMetadata.rarity">Rarity: <span class="font-medium">{{ scanMetadata.rarity }}</span></div>
          <div v-if="scanMetadata.is_foil">Foil: <span class="font-medium text-yellow-600">Yes</span></div>
        </div>
      </div>
    </div>

    <div v-if="store.searchLoading" class="text-center py-8">
      <div class="animate-spin rounded-full h-12 w-12 border-b-2 border-blue-600 mx-auto"></div>
      <p class="mt-4 text-gray-500 dark:text-gray-400">Searching...</p>
    </div>

    <div v-else-if="store.searchResults.length > 0">
      <div class="flex justify-between items-center mb-4">
        <h2 class="text-xl font-semibold text-gray-800 dark:text-white">
          Search Results ({{ store.searchResults.length }})
        </h2>
        <button
          @click="store.clearSearch"
          class="text-gray-500 dark:text-gray-400 hover:text-gray-700 dark:hover:text-gray-200"
        >
          Clear
        </button>
      </div>

      <CardGrid
        :cards="store.searchResults"
        @select="handleSelect"
      />
    </div>

    <div v-else class="text-center py-12 bg-white dark:bg-gray-800 rounded-lg">
      <svg class="mx-auto h-12 w-12 text-gray-400" fill="none" stroke="currentColor" viewBox="0 0 24 24">
        <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M21 21l-6-6m2-5a7 7 0 11-14 0 7 7 0 0114 0z" />
      </svg>
      <p class="mt-4 text-gray-500 dark:text-gray-400">Search for cards to add to your collection</p>
    </div>

    <CardDetail
      v-if="selectedCard"
      :item="selectedCard"
      :is-collection-item="false"
      @close="selectedCard = null"
      @add="handleAdd"
      @priceUpdated="handlePriceUpdated"
    />
  </div>
</template>
