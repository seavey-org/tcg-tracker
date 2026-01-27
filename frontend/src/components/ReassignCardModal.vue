<script setup>
import { ref, watch, onUnmounted, computed } from 'vue'
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

// UI state
const phase = ref('search') // 'search', 'set-list', 'card-list'
const searchQuery = ref('')
const searching = ref(false)
const searchError = ref(null)
const selectedCard = ref(null)
const game = ref(props.currentCard.game || 'pokemon')
const sortOrder = ref('release_date') // 'release_date', 'release_date_asc', 'name', 'cards'

// Data
const setGroups = ref([]) // Grouped search results
const selectedSet = ref(null) // Currently expanded set
const setCards = ref([]) // Cards in selected set

// Debounce search
let searchTimeout = null
watch(searchQuery, (newQuery) => {
  if (searchTimeout) clearTimeout(searchTimeout)
  if (!newQuery || newQuery.length < 2) {
    setGroups.value = []
    phase.value = 'search'
    return
  }
  searchTimeout = setTimeout(() => {
    performGroupedSearch(newQuery)
  }, 300)
})

// Cleanup timeout on unmount
onUnmounted(() => {
  if (searchTimeout) clearTimeout(searchTimeout)
})

// Switch game and clear results
function switchGame(newGame) {
  game.value = newGame
  setGroups.value = []
  selectedSet.value = null
  setCards.value = []
  selectedCard.value = null
  phase.value = 'search'
}

// Search for cards grouped by set
async function performGroupedSearch(query) {
  searching.value = true
  searchError.value = null
  try {
    const result = await cardService.searchGrouped(query, game.value, sortOrder.value)
    setGroups.value = result.set_groups || []
    phase.value = setGroups.value.length > 0 ? 'set-list' : 'search'
  } catch {
    searchError.value = 'Search failed'
    setGroups.value = []
    phase.value = 'search'
  } finally {
    searching.value = false
  }
}

// Re-sort when sort order changes
function changeSortOrder(newOrder) {
  sortOrder.value = newOrder
  if (searchQuery.value.length >= 2) {
    performGroupedSearch(searchQuery.value)
  }
}

// Select a set to view its cards
function selectSet(setGroup) {
  selectedSet.value = setGroup
  setCards.value = setGroup.cards || []
  phase.value = 'card-list'
}

// Go back to set list
function backToSetList() {
  selectedSet.value = null
  setCards.value = []
  phase.value = 'set-list'
}

// Select a card for reassignment
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

// Extract year from release date (YYYY-MM-DD or YYYY/MM/DD)
function getReleaseYear(releaseDate) {
  if (!releaseDate) return ''
  return releaseDate.substring(0, 4)
}

// Computed: total cards across all sets
const totalCards = computed(() => {
  return setGroups.value.reduce((sum, group) => sum + group.card_count, 0)
})
</script>

<template>
  <div
    class="fixed inset-0 bg-black bg-opacity-50 flex items-center justify-center z-[60] p-4"
    @click.self="emit('close')"
  >
    <div class="bg-white dark:bg-gray-800 rounded-lg max-w-4xl w-full max-h-[90vh] overflow-hidden flex flex-col">
      <!-- Header -->
      <div class="p-4 border-b dark:border-gray-700 flex justify-between items-center">
        <div class="flex items-center gap-2">
          <h2 class="text-xl font-bold text-gray-800 dark:text-white">Reassign Card</h2>
          <!-- Breadcrumb navigation -->
          <span v-if="phase === 'set-list'" class="text-sm text-gray-500 dark:text-gray-400">
            / {{ setGroups.length }} sets
          </span>
          <span v-if="phase === 'card-list' && selectedSet" class="text-sm text-gray-500 dark:text-gray-400">
            / {{ selectedSet.set_name }}
          </span>
        </div>
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
          <!-- Left: Scanned image (if available) or current card -->
          <div class="md:w-1/3">
            <!-- Scanned Image - shown prominently if available -->
            <template v-if="item.scanned_image_path">
              <h3 class="text-sm font-medium text-gray-600 dark:text-gray-400 mb-2">Scanned Card</h3>
              <div class="bg-gray-50 dark:bg-gray-700 rounded-lg p-3 mb-3">
                <img
                  :src="`/images/scanned/${item.scanned_image_path.split('/').pop()}`"
                  alt="Scanned card"
                  class="w-full rounded shadow-lg"
                />
              </div>
              <!-- Current assignment info below scan -->
              <div class="bg-amber-50 dark:bg-amber-900/20 border border-amber-200 dark:border-amber-700 rounded-lg p-3">
                <p class="text-xs text-amber-600 dark:text-amber-400 font-medium mb-1">Currently assigned to:</p>
                <p class="font-medium text-gray-800 dark:text-white text-sm">{{ currentCard.name }}</p>
                <p class="text-xs text-gray-500 dark:text-gray-400">{{ currentCard.set_name }}</p>
                <p class="text-xs text-gray-400 dark:text-gray-500 font-mono mt-1">{{ currentCard.id }}</p>
              </div>
            </template>
            <!-- No scan - just show current card -->
            <template v-else>
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
            </template>
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
              <div class="animate-spin rounded-full h-8 w-8 border-b-2 border-blue-600 mx-auto mb-2"></div>
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

            <!-- Phase 1: Set List -->
            <div v-if="phase === 'set-list' && !selectedCard" class="space-y-2 max-h-80 overflow-y-auto">
              <div class="flex items-center justify-between mb-2">
                <p class="text-sm text-gray-500 dark:text-gray-400">
                  Found {{ totalCards }} cards in {{ setGroups.length }} sets. Select a set:
                </p>
                <!-- Sort dropdown -->
                <select
                  :value="sortOrder"
                  @change="changeSortOrder($event.target.value)"
                  class="text-xs border dark:border-gray-600 rounded px-2 py-1 bg-white dark:bg-gray-700 text-gray-700 dark:text-gray-300"
                >
                  <option value="release_date">Newest First</option>
                  <option value="release_date_asc">Oldest First</option>
                  <option value="name">Alphabetical</option>
                  <option value="cards">Most Cards</option>
                </select>
              </div>
              <div
                v-for="setGroup in setGroups"
                :key="setGroup.set_code"
                @click="selectSet(setGroup)"
                class="flex items-center gap-3 p-3 rounded-lg border dark:border-gray-600 hover:bg-gray-50 dark:hover:bg-gray-700 cursor-pointer transition"
              >
                <!-- Set symbol -->
                <div class="w-8 h-8 flex-shrink-0 flex items-center justify-center">
                  <img
                    v-if="setGroup.symbol_url"
                    :src="setGroup.symbol_url"
                    :alt="setGroup.set_name"
                    class="w-6 h-6 object-contain"
                    @error="$event.target.style.display = 'none'"
                  />
                  <span v-else class="text-gray-400 text-xs">{{ setGroup.set_code.toUpperCase().slice(0, 3) }}</span>
                </div>
                <!-- Set info -->
                <div class="flex-1 min-w-0">
                  <p class="font-medium text-gray-800 dark:text-white truncate">{{ setGroup.set_name }}</p>
                  <p class="text-xs text-gray-500 dark:text-gray-400">
                    {{ setGroup.series || '' }}
                    <span v-if="setGroup.release_date" class="ml-1">({{ getReleaseYear(setGroup.release_date) }})</span>
                  </p>
                </div>
                <!-- Card count -->
                <div class="text-right">
                  <span class="text-sm font-medium text-blue-600 dark:text-blue-400">
                    {{ setGroup.card_count }} {{ setGroup.card_count === 1 ? 'card' : 'cards' }}
                  </span>
                </div>
                <!-- Arrow -->
                <svg class="w-5 h-5 text-gray-400" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                  <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M9 5l7 7-7 7" />
                </svg>
              </div>
            </div>

            <!-- Phase 2: Card List -->
            <div v-if="phase === 'card-list' && selectedSet && !selectedCard">
              <!-- Back button -->
              <button
                @click="backToSetList"
                class="flex items-center gap-1 text-blue-600 dark:text-blue-400 hover:underline mb-3"
              >
                <svg class="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                  <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M15 19l-7-7 7-7" />
                </svg>
                Back to sets
              </button>

              <!-- Set header -->
              <div class="flex items-center gap-2 mb-3 pb-2 border-b dark:border-gray-600">
                <img
                  v-if="selectedSet.symbol_url"
                  :src="selectedSet.symbol_url"
                  :alt="selectedSet.set_name"
                  class="w-6 h-6 object-contain"
                />
                <span class="font-medium text-gray-800 dark:text-white">{{ selectedSet.set_name }}</span>
                <span class="text-sm text-gray-500">{{ setCards.length }} cards</span>
              </div>

              <!-- Card grid -->
              <div class="grid grid-cols-3 sm:grid-cols-4 gap-3 max-h-64 overflow-y-auto">
                <div
                  v-for="card in setCards"
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
                  <p class="text-xs text-gray-400 dark:text-gray-500">#{{ card.card_number }}</p>
                </div>
              </div>
            </div>

            <!-- Empty state: Initial search prompt -->
            <div v-if="phase === 'search' && !searching && searchQuery.length < 2 && !selectedCard" class="text-center py-8 text-gray-500 dark:text-gray-400">
              Enter at least 2 characters to search
            </div>

            <!-- Empty state: No results -->
            <div v-if="phase === 'search' && !searching && searchQuery.length >= 2 && setGroups.length === 0 && !selectedCard" class="text-center py-8 text-gray-500 dark:text-gray-400">
              No cards found
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
