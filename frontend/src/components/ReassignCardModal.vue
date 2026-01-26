<script setup>
import { ref, watch, onUnmounted } from 'vue'
import { cardService } from '../services/api'

const props = defineProps({
  item: {
    type: Object,
    required: true
  },
  currentCard: {
    type: Object,
    required: true
  }
})

const emit = defineEmits(['reassign', 'close'])

const searchQuery = ref('')
const searchResults = ref([])
const searching = ref(false)
const searchError = ref(null)
const selectedCard = ref(null)
const game = ref(props.currentCard.game || 'pokemon')

// Debounce search
let searchTimeout = null
watch(searchQuery, (newQuery) => {
  if (searchTimeout) clearTimeout(searchTimeout)
  if (!newQuery || newQuery.length < 2) {
    searchResults.value = []
    return
  }
  searchTimeout = setTimeout(() => {
    performSearch(newQuery)
  }, 300)
})

// Cleanup timeout on unmount to prevent memory leaks
onUnmounted(() => {
  if (searchTimeout) clearTimeout(searchTimeout)
})

// Switch game and clear results
function switchGame(newGame) {
  game.value = newGame
  searchResults.value = []
  selectedCard.value = null
}

async function performSearch(query) {
  searching.value = true
  searchError.value = null
  try {
    const results = await cardService.search(query, game.value)
    searchResults.value = results
  } catch {
    searchError.value = 'Search failed'
    searchResults.value = []
  } finally {
    searching.value = false
  }
}

function selectCard(card) {
  selectedCard.value = card
}

function confirmReassign() {
  if (selectedCard.value) {
    emit('reassign', selectedCard.value.id)
  }
}

function formatPrice(price) {
  if (!price) return '-'
  return `$${price.toFixed(2)}`
}
</script>

<template>
  <div
    class="fixed inset-0 bg-black bg-opacity-50 flex items-center justify-center z-[60] p-4"
    @click.self="emit('close')"
  >
    <div class="bg-white dark:bg-gray-800 rounded-lg max-w-4xl w-full max-h-[90vh] overflow-hidden flex flex-col">
      <!-- Header -->
      <div class="p-4 border-b dark:border-gray-700 flex justify-between items-center">
        <h2 class="text-xl font-bold text-gray-800 dark:text-white">Reassign Card</h2>
        <button
          @click="emit('close')"
          class="text-gray-400 hover:text-gray-600 dark:hover:text-gray-200"
        >
          <svg class="w-6 h-6" fill="none" stroke="currentColor" viewBox="0 0 24 24">
            <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M6 18L18 6M6 6l12 12" />
          </svg>
        </button>
      </div>

      <!-- Content -->
      <div class="flex-1 overflow-y-auto p-4">
        <div class="flex flex-col md:flex-row gap-6">
          <!-- Left: Current card -->
          <div class="md:w-1/3">
            <h3 class="text-sm font-medium text-gray-600 dark:text-gray-400 mb-2">Current Card</h3>
            <div class="bg-gray-50 dark:bg-gray-700 rounded-lg p-3">
              <img
                :src="currentCard.image_url"
                :alt="currentCard.name"
                class="w-full rounded mb-2"
              />
              <p class="font-medium text-gray-800 dark:text-white text-sm">{{ currentCard.name }}</p>
              <p class="text-xs text-gray-500 dark:text-gray-400">{{ currentCard.set_name }}</p>
              <p class="text-xs text-gray-400 dark:text-gray-500 font-mono mt-1">{{ currentCard.id }}</p>
            </div>
          </div>

          <!-- Right: Search and selection -->
          <div class="md:w-2/3">
            <!-- Game toggle -->
            <div class="flex gap-2 mb-3">
              <button
                @click="switchGame('pokemon')"
                class="px-4 py-2 text-sm rounded-lg transition"
                :class="game === 'pokemon' ? 'bg-blue-600 text-white' : 'bg-gray-200 dark:bg-gray-700 text-gray-700 dark:text-gray-300'"
              >
                Pokemon
              </button>
              <button
                @click="switchGame('mtg')"
                class="px-4 py-2 text-sm rounded-lg transition"
                :class="game === 'mtg' ? 'bg-blue-600 text-white' : 'bg-gray-200 dark:bg-gray-700 text-gray-700 dark:text-gray-300'"
              >
                Magic
              </button>
            </div>

            <!-- Search input -->
            <div class="relative mb-4">
              <input
                v-model="searchQuery"
                type="text"
                placeholder="Search for card by name..."
                class="w-full border dark:border-gray-600 rounded-lg px-4 py-2 pl-10 bg-white dark:bg-gray-700 text-gray-900 dark:text-white"
              />
              <svg class="w-5 h-5 text-gray-400 absolute left-3 top-2.5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M21 21l-6-6m2-5a7 7 0 11-14 0 7 7 0 0114 0z" />
              </svg>
            </div>

            <!-- Loading -->
            <div v-if="searching" class="text-center py-8 text-gray-500 dark:text-gray-400">
              Searching...
            </div>

            <!-- Error -->
            <div v-else-if="searchError" class="text-center py-4 text-red-500">
              {{ searchError }}
            </div>

            <!-- Selected card preview -->
            <div v-if="selectedCard" class="mb-4 p-3 bg-green-50 dark:bg-green-900/30 border border-green-200 dark:border-green-700 rounded-lg">
              <div class="flex items-center gap-3">
                <img
                  :src="selectedCard.image_url"
                  :alt="selectedCard.name"
                  class="w-16 h-22 object-cover rounded"
                />
                <div class="flex-1">
                  <p class="font-medium text-gray-800 dark:text-white">{{ selectedCard.name }}</p>
                  <p class="text-sm text-gray-500 dark:text-gray-400">{{ selectedCard.set_name }}</p>
                  <p class="text-xs text-gray-400 dark:text-gray-500 font-mono">{{ selectedCard.id }}</p>
                  <p class="text-sm text-green-600 dark:text-green-400 mt-1">{{ formatPrice(selectedCard.price_usd) }}</p>
                </div>
                <button
                  @click="selectedCard = null"
                  class="text-gray-400 hover:text-gray-600"
                >
                  <svg class="w-5 h-5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                    <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M6 18L18 6M6 6l12 12" />
                  </svg>
                </button>
              </div>
            </div>

            <!-- Search results -->
            <div v-if="searchResults.length > 0 && !selectedCard" class="grid grid-cols-3 sm:grid-cols-4 gap-3 max-h-80 overflow-y-auto">
              <div
                v-for="card in searchResults"
                :key="card.id"
                @click="selectCard(card)"
                class="cursor-pointer group"
              >
                <div class="relative">
                  <img
                    :src="card.image_url"
                    :alt="card.name"
                    class="w-full rounded shadow group-hover:ring-2 group-hover:ring-blue-500 transition"
                  />
                </div>
                <p class="text-xs text-gray-600 dark:text-gray-400 mt-1 truncate">{{ card.name }}</p>
                <p class="text-xs text-gray-400 dark:text-gray-500 truncate">{{ card.set_name }}</p>
              </div>
            </div>

            <!-- Empty state -->
            <div v-else-if="!searching && searchQuery.length >= 2 && !selectedCard" class="text-center py-8 text-gray-500 dark:text-gray-400">
              No cards found
            </div>
            <div v-else-if="!selectedCard" class="text-center py-8 text-gray-500 dark:text-gray-400">
              Enter at least 2 characters to search
            </div>
          </div>
        </div>
      </div>

      <!-- Footer -->
      <div class="p-4 border-t dark:border-gray-700 flex justify-end gap-3">
        <button
          @click="emit('close')"
          class="px-4 py-2 rounded-lg bg-gray-200 dark:bg-gray-700 text-gray-700 dark:text-gray-300 hover:bg-gray-300 dark:hover:bg-gray-600 transition"
        >
          Cancel
        </button>
        <button
          @click="confirmReassign"
          :disabled="!selectedCard"
          class="px-4 py-2 rounded-lg bg-blue-600 text-white hover:bg-blue-700 transition disabled:opacity-50 disabled:cursor-not-allowed"
        >
          Reassign to Selected Card
        </button>
      </div>
    </div>
  </div>
</template>
