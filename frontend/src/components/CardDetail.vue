<script setup>
import { ref, computed } from 'vue'
import { priceService } from '../services/api'
import { formatPrice, formatTimeAgo, isPriceStale as checkPriceStale } from '../utils/formatters'

const props = defineProps({
  item: {
    type: Object,
    required: true
  },
  isCollectionItem: {
    type: Boolean,
    default: false
  },
  // If true, item is a grouped collection item with variants, items array, etc.
  isGrouped: {
    type: Boolean,
    default: false
  }
})

const emit = defineEmits(['close', 'add', 'update', 'remove', 'priceUpdated'])

// The card data
const card = computed(() => props.item.card || props.item)

// For non-grouped items or adding new cards
const quantity = ref(props.item.quantity || 1)
const condition = ref(props.item.condition || 'NM')
const printing = ref(props.item.printing || 'Normal')

// For editing individual items from grouped view
const editingItem = ref(null)
const editQuantity = ref(1)
const editCondition = ref('NM')
const editPrinting = ref('Normal')

// UI state
const refreshingPrice = ref(false)
const priceError = ref(null)
const priceMessage = ref(null)
const showScannedImage = ref(false)
const activeTab = ref('variants') // 'variants' | 'scans' | 'items'

const printingOptions = [
  { value: 'Normal', label: 'Normal' },
  { value: 'Foil', label: 'Foil / Holo' },
  { value: '1st Edition', label: '1st Edition' },
  { value: 'Reverse Holofoil', label: 'Reverse Holo' },
  { value: 'Unlimited', label: 'Unlimited' }
]

const conditions = [
  { value: 'M', label: 'Mint' },
  { value: 'NM', label: 'Near Mint' },
  { value: 'LP', label: 'Light Play' },
  { value: 'MP', label: 'Moderate Play' },
  { value: 'HP', label: 'Heavy Play' },
  { value: 'D', label: 'Damaged' }
]

// Computed for grouped items
const totalQuantity = computed(() => props.isGrouped ? props.item.total_quantity : props.item.quantity)
const totalValue = computed(() => props.isGrouped ? props.item.total_value : null)
const scannedCount = computed(() => props.isGrouped ? props.item.scanned_count : (props.item.scanned_image_path ? 1 : 0))
const variants = computed(() => props.isGrouped ? props.item.variants || [] : [])
const items = computed(() => props.isGrouped ? props.item.items || [] : [])

// Get all scanned items
const scannedItems = computed(() => {
  if (!props.isGrouped) {
    return props.item.scanned_image_path ? [props.item] : []
  }
  return items.value.filter(i => i.scanned_image_path)
})

// For non-grouped single items
const hasScannedImage = computed(() => !props.isGrouped && props.item.scanned_image_path)
const scannedImageUrl = computed(() => {
  if (!hasScannedImage.value) return null
  return `/images/scanned/${props.item.scanned_image_path}`
})

const isPriceStale = computed(() => checkPriceStale(card.value))
const priceAge = computed(() => formatTimeAgo(card.value.price_updated_at))
const isPokemon = computed(() => card.value.game === 'pokemon')

// Helper to get condition label
function getConditionLabel(value) {
  const c = conditions.find(c => c.value === value)
  return c ? c.label : value
}

// Helper to get printing label
function getPrintingLabel(value) {
  const p = printingOptions.find(p => p.value === value)
  return p ? p.label : value
}

// Start editing an individual item
function startEditItem(item) {
  editingItem.value = item
  editQuantity.value = item.quantity
  editCondition.value = item.condition
  editPrinting.value = item.printing
  // Switch to items tab so the edit form is visible
  activeTab.value = 'items'
}

// Cancel editing
function cancelEdit() {
  editingItem.value = null
}

// Save edited item
function saveEditItem() {
  if (!editingItem.value) return
  
  emit('update', {
    id: editingItem.value.id,
    quantity: editQuantity.value,
    condition: editCondition.value,
    printing: editPrinting.value
  })
  editingItem.value = null
}

// Remove an individual item
function removeItem(item) {
  const msg = item.quantity > 1
    ? `Remove all ${item.quantity} cards from this stack?`
    : 'Remove this card from your collection?'
  if (confirm(msg)) {
    emit('remove', item.id)
  }
}

const refreshPrice = async () => {
  if (!isPokemon.value) return

  refreshingPrice.value = true
  priceError.value = null
  priceMessage.value = null

  try {
    const result = await priceService.refreshCardPrice(card.value.id)
    if (result.queue_position) {
      priceMessage.value = `Queued for refresh (position ${result.queue_position})`
    } else if (result.card) {
      emit('priceUpdated', result.card)
    }
  } catch (err) {
    if (err.response?.status === 429) {
      priceError.value = 'Daily quota exceeded'
    } else {
      priceError.value = 'Failed to queue refresh'
    }
  } finally {
    refreshingPrice.value = false
  }
}

const handleAdd = () => {
  emit('add', {
    cardId: card.value.id,
    quantity: quantity.value,
    condition: condition.value,
    printing: printing.value
  })
}

const handleUpdate = () => {
  emit('update', {
    id: props.item.id,
    quantity: quantity.value,
    condition: condition.value,
    printing: printing.value
  })
}

const handleRemove = () => {
  if (confirm('Are you sure you want to remove this card from your collection?')) {
    emit('remove', props.item.id)
  }
}
</script>

<template>
  <div
    class="fixed inset-0 bg-black bg-opacity-50 flex items-center justify-center z-50 p-4"
    @click.self="emit('close')"
    role="dialog"
    aria-modal="true"
    :aria-labelledby="'card-title-' + card.id"
  >
    <div class="bg-white dark:bg-gray-800 rounded-lg max-w-4xl w-full max-h-[90vh] overflow-y-auto">
      <div class="flex flex-col md:flex-row">
        <!-- Left side: Card image -->
        <div class="md:w-2/5 p-4">
          <!-- Image toggle for single scanned images (non-grouped) -->
          <div v-if="hasScannedImage" class="flex gap-2 mb-3">
            <button
              @click="showScannedImage = false"
              class="flex-1 py-2 px-3 text-sm rounded-lg transition"
              :class="!showScannedImage ? 'bg-blue-600 text-white' : 'bg-gray-200 dark:bg-gray-700 text-gray-700 dark:text-gray-300'"
            >
              Official
            </button>
            <button
              @click="showScannedImage = true"
              class="flex-1 py-2 px-3 text-sm rounded-lg transition"
              :class="showScannedImage ? 'bg-blue-600 text-white' : 'bg-gray-200 dark:bg-gray-700 text-gray-700 dark:text-gray-300'"
            >
              My Scan
            </button>
          </div>
          <img
            :src="showScannedImage && scannedImageUrl ? scannedImageUrl : (card.image_url_large || card.image_url)"
            :alt="card.name + ' card image'"
            class="w-full rounded-lg shadow"
          />
        </div>

        <!-- Right side: Details -->
        <div class="md:w-3/5 p-6">
          <!-- Header -->
          <div class="flex justify-between items-start mb-4">
            <div>
              <h2 :id="'card-title-' + card.id" class="text-2xl font-bold text-gray-800 dark:text-white">{{ card.name }}</h2>
              <p class="text-gray-500 dark:text-gray-400">{{ card.set_name }}</p>
              <!-- Summary for grouped items -->
              <div v-if="isGrouped && isCollectionItem" class="flex gap-4 mt-2 text-sm">
                <span class="text-blue-600 dark:text-blue-400 font-medium">{{ totalQuantity }} cards</span>
                <span class="text-green-600 dark:text-green-400 font-medium">{{ formatPrice(totalValue) }}</span>
                <span v-if="scannedCount > 0" class="text-purple-600 dark:text-purple-400">{{ scannedCount }} scans</span>
              </div>
            </div>
            <button
              @click="emit('close')"
              class="text-gray-400 hover:text-gray-600 dark:hover:text-gray-200"
              aria-label="Close card details"
            >
              <svg class="w-6 h-6" fill="none" stroke="currentColor" viewBox="0 0 24 24" aria-hidden="true">
                <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M6 18L18 6M6 6l12 12" />
              </svg>
            </button>
          </div>

          <!-- Card info (always shown) -->
          <div class="space-y-2 mb-4 text-sm">
            <div class="flex justify-between">
              <span class="text-gray-600 dark:text-gray-400">Set:</span>
              <span class="font-medium text-gray-800 dark:text-white">{{ card.set_name }} ({{ card.set_code }})</span>
            </div>
            <div class="flex justify-between">
              <span class="text-gray-600 dark:text-gray-400">Number:</span>
              <span class="font-medium text-gray-800 dark:text-white">{{ card.card_number }}</span>
            </div>
            <div class="flex justify-between items-center">
              <span class="text-gray-600 dark:text-gray-400">Price (NM):</span>
              <div class="flex items-center gap-2">
                <span class="font-medium text-green-600 dark:text-green-400">{{ formatPrice(card.price_usd) }}</span>
                <span v-if="card.price_foil_usd" class="text-yellow-600 dark:text-yellow-400">/ {{ formatPrice(card.price_foil_usd) }} foil</span>
                <span v-if="priceAge" class="text-xs" :class="isPriceStale ? 'text-orange-500' : 'text-gray-400'">
                  ({{ priceAge }})
                </span>
              </div>
            </div>
            <div v-if="isPokemon" class="flex justify-between items-center">
              <span class="text-gray-600 dark:text-gray-400">Price Status:</span>
              <div class="flex items-center gap-2">
                <span class="text-xs px-2 py-1 rounded" :class="{
                  'bg-green-100 dark:bg-green-900 text-green-700 dark:text-green-200': card.price_source === 'api',
                  'bg-blue-100 dark:bg-blue-900 text-blue-700 dark:text-blue-200': card.price_source === 'cached',
                  'bg-yellow-100 dark:bg-yellow-900 text-yellow-700 dark:text-yellow-200': card.price_source === 'pending',
                  'bg-gray-100 dark:bg-gray-700 text-gray-600 dark:text-gray-300': !card.price_source
                }">
                  {{ card.price_source || 'unknown' }}
                </span>
                <button
                  @click="refreshPrice"
                  :disabled="refreshingPrice"
                  class="text-blue-600 dark:text-blue-400 hover:text-blue-800 dark:hover:text-blue-300 text-sm disabled:opacity-50"
                >
                  {{ refreshingPrice ? 'Refreshing...' : 'Refresh' }}
                </button>
              </div>
            </div>
            <div v-if="priceMessage" class="text-blue-500 text-sm">{{ priceMessage }}</div>
            <div v-if="priceError" class="text-red-500 text-sm">{{ priceError }}</div>
            <!-- TCGPlayer link for additional pricing data -->
            <div v-if="card.tcgplayer_id" class="flex justify-between items-center pt-2 border-t dark:border-gray-700 mt-2">
              <span class="text-gray-600 dark:text-gray-400">More pricing:</span>
              <a
                :href="`https://www.tcgplayer.com/product/${card.tcgplayer_id}`"
                target="_blank"
                rel="noopener noreferrer"
                class="text-blue-600 dark:text-blue-400 hover:text-blue-800 dark:hover:text-blue-300 text-sm flex items-center gap-1"
              >
                View on TCGPlayer
                <svg class="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                  <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M10 6H6a2 2 0 00-2 2v10a2 2 0 002 2h10a2 2 0 002-2v-4M14 4h6m0 0v6m0-6L10 14" />
                </svg>
              </a>
            </div>
          </div>

          <!-- GROUPED VIEW: Tabs for variants/scans/items -->
          <div v-if="isGrouped && isCollectionItem" class="border-t dark:border-gray-700 pt-4">
            <!-- Tab buttons -->
            <div class="flex gap-2 mb-4">
              <button
                @click="activeTab = 'variants'"
                class="px-4 py-2 text-sm rounded-lg transition"
                :class="activeTab === 'variants' ? 'bg-blue-600 text-white' : 'bg-gray-200 dark:bg-gray-700 text-gray-700 dark:text-gray-300'"
              >
                Variants ({{ variants.length }})
              </button>
              <button
                v-if="scannedCount > 0"
                @click="activeTab = 'scans'"
                class="px-4 py-2 text-sm rounded-lg transition"
                :class="activeTab === 'scans' ? 'bg-blue-600 text-white' : 'bg-gray-200 dark:bg-gray-700 text-gray-700 dark:text-gray-300'"
              >
                Scans ({{ scannedCount }})
              </button>
              <button
                @click="activeTab = 'items'"
                class="px-4 py-2 text-sm rounded-lg transition"
                :class="activeTab === 'items' ? 'bg-blue-600 text-white' : 'bg-gray-200 dark:bg-gray-700 text-gray-700 dark:text-gray-300'"
              >
                Edit Items ({{ items.length }})
              </button>
            </div>

            <!-- Variants tab -->
            <div v-if="activeTab === 'variants'" class="space-y-2">
              <div
                v-for="variant in variants"
                :key="`${variant.printing}-${variant.condition}`"
                class="flex justify-between items-center p-3 bg-gray-50 dark:bg-gray-700 rounded-lg"
              >
                <div class="flex items-center gap-3">
                  <span
                    v-if="variant.printing !== 'Normal'"
                    class="text-xs font-bold px-2 py-1 rounded"
                    :class="{
                      'bg-yellow-400 text-yellow-900': variant.printing === 'Foil',
                      'bg-amber-600 text-white': variant.printing === '1st Edition',
                      'bg-purple-500 text-white': variant.printing === 'Reverse Holofoil',
                      'bg-gray-500 text-white': variant.printing === 'Unlimited'
                    }"
                  >
                    {{ getPrintingLabel(variant.printing) }}
                  </span>
                  <span class="text-gray-800 dark:text-white">{{ getConditionLabel(variant.condition) }}</span>
                  <span class="text-gray-500 dark:text-gray-400">x{{ variant.quantity }}</span>
                  <span v-if="variant.has_scans" class="text-purple-500 text-xs">{{ variant.scanned_qty }} scans</span>
                </div>
                <span class="font-medium text-green-600 dark:text-green-400">{{ formatPrice(variant.value) }}</span>
              </div>
            </div>

            <!-- Scans tab -->
            <div v-if="activeTab === 'scans'" class="grid grid-cols-3 gap-3">
              <div
                v-for="(scanItem, idx) in scannedItems"
                :key="scanItem.id"
                class="relative cursor-pointer group"
                @click="startEditItem(scanItem)"
              >
                <img
                  :src="`/images/scanned/${scanItem.scanned_image_path}`"
                  :alt="`Scan ${idx + 1}`"
                  class="w-full aspect-[2.5/3.5] object-cover rounded-lg shadow group-hover:ring-2 group-hover:ring-blue-500"
                />
                <div class="absolute bottom-0 left-0 right-0 bg-black bg-opacity-60 text-white text-xs p-1 rounded-b-lg">
                  {{ scanItem.printing }} / {{ scanItem.condition }}
                </div>
              </div>
            </div>

            <!-- Items tab (edit individual items) -->
            <div v-if="activeTab === 'items'" class="space-y-2">
              <div
                v-for="collectionItem in items"
                :key="collectionItem.id"
                class="p-3 bg-gray-50 dark:bg-gray-700 rounded-lg"
              >
                <!-- Editing this item -->
                <div v-if="editingItem?.id === collectionItem.id" class="space-y-3">
                  <div class="flex gap-3">
                    <!-- Show scanned image thumbnail when editing a scanned card -->
                    <div v-if="collectionItem.scanned_image_path" class="flex-shrink-0">
                      <img
                        :src="`/images/scanned/${collectionItem.scanned_image_path}`"
                        alt="Your scanned card"
                        class="w-16 h-22 object-cover rounded shadow ring-2 ring-blue-500"
                      />
                    </div>
                    <div class="flex-1">
                      <div class="text-sm font-medium text-blue-600 dark:text-blue-400 mb-2">
                        Editing {{ collectionItem.scanned_image_path ? 'scanned card' : `stack of ${collectionItem.quantity}` }}
                        <span v-if="!collectionItem.scanned_image_path && collectionItem.quantity > 1" class="text-orange-500 block mt-1">
                          Note: Changing condition/printing will split 1 card from this stack
                        </span>
                      </div>
                      <div class="grid grid-cols-3 gap-2">
                        <div>
                          <label class="text-xs text-gray-500">Quantity</label>
                          <input
                            v-model.number="editQuantity"
                            type="number"
                            min="1"
                            class="w-full border dark:border-gray-600 rounded px-2 py-1 text-sm bg-white dark:bg-gray-800 text-gray-900 dark:text-white"
                          />
                        </div>
                        <div>
                          <label class="text-xs text-gray-500">Condition</label>
                          <select
                            v-model="editCondition"
                            class="w-full border dark:border-gray-600 rounded px-2 py-1 text-sm bg-white dark:bg-gray-800 text-gray-900 dark:text-white"
                          >
                            <option v-for="c in conditions" :key="c.value" :value="c.value">{{ c.label }}</option>
                          </select>
                        </div>
                        <div>
                          <label class="text-xs text-gray-500">Printing</label>
                          <select
                            v-model="editPrinting"
                            class="w-full border dark:border-gray-600 rounded px-2 py-1 text-sm bg-white dark:bg-gray-800 text-gray-900 dark:text-white"
                          >
                            <option v-for="p in printingOptions" :key="p.value" :value="p.value">{{ p.label }}</option>
                          </select>
                        </div>
                      </div>
                      <div class="flex gap-2 mt-3">
                        <button
                          @click="saveEditItem"
                          class="flex-1 bg-blue-600 text-white py-1 px-3 rounded text-sm hover:bg-blue-700"
                        >
                          Save
                        </button>
                        <button
                          @click="cancelEdit"
                          class="px-3 py-1 rounded text-sm bg-gray-200 dark:bg-gray-600 text-gray-700 dark:text-gray-300 hover:bg-gray-300 dark:hover:bg-gray-500"
                        >
                          Cancel
                        </button>
                      </div>
                    </div>
                  </div>
                </div>

                <!-- Display mode -->
                <div v-else class="flex items-center gap-3">
                  <!-- Scanned card: show thumbnail -->
                  <div v-if="collectionItem.scanned_image_path" class="flex-shrink-0">
                    <img
                      :src="`/images/scanned/${collectionItem.scanned_image_path}`"
                      alt="Your scanned card"
                      class="w-12 h-16 object-cover rounded shadow cursor-pointer hover:ring-2 hover:ring-blue-500 transition"
                      @click="startEditItem(collectionItem)"
                    />
                  </div>
                  <!-- Non-scanned stack: show stack icon -->
                  <div v-else class="flex-shrink-0 w-12 h-16 bg-gray-200 dark:bg-gray-600 rounded flex items-center justify-center">
                    <svg class="w-6 h-6 text-gray-400 dark:text-gray-500" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                      <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M19 11H5m14 0a2 2 0 012 2v6a2 2 0 01-2 2H5a2 2 0 01-2-2v-6a2 2 0 012-2m14 0V9a2 2 0 00-2-2M5 11V9a2 2 0 012-2m0 0V5a2 2 0 012-2h6a2 2 0 012 2v2M7 7h10" />
                    </svg>
                  </div>
                  <div class="flex-1 min-w-0">
                    <div class="flex items-center gap-2 flex-wrap">
                      <span
                        v-if="collectionItem.printing !== 'Normal'"
                        class="text-xs font-bold px-2 py-0.5 rounded"
                        :class="{
                          'bg-yellow-400 text-yellow-900': collectionItem.printing === 'Foil',
                          'bg-amber-600 text-white': collectionItem.printing === '1st Edition',
                          'bg-purple-500 text-white': collectionItem.printing === 'Reverse Holofoil',
                          'bg-gray-500 text-white': collectionItem.printing === 'Unlimited'
                        }"
                      >
                        {{ getPrintingLabel(collectionItem.printing) }}
                      </span>
                      <span class="text-gray-800 dark:text-white">{{ getConditionLabel(collectionItem.condition) }}</span>
                      <span class="text-gray-500 dark:text-gray-400">x{{ collectionItem.quantity }}</span>
                    </div>
                    <div v-if="collectionItem.scanned_image_path" class="text-xs text-purple-500 mt-1">
                      Scanned card
                    </div>
                  </div>
                  <div class="flex gap-2 flex-shrink-0">
                    <button
                      @click="startEditItem(collectionItem)"
                      class="text-blue-600 dark:text-blue-400 hover:text-blue-800 text-sm"
                    >
                      Edit
                    </button>
                    <button
                      @click="removeItem(collectionItem)"
                      class="text-red-600 dark:text-red-400 hover:text-red-800 text-sm"
                    >
                      Remove
                    </button>
                  </div>
                </div>
              </div>
            </div>
          </div>

          <!-- NON-GROUPED VIEW: Simple edit form (for search results or single items) -->
          <div v-else class="border-t dark:border-gray-700 pt-4 space-y-4">
            <div>
              <label :for="'quantity-' + card.id" class="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1">Quantity</label>
              <input
                :id="'quantity-' + card.id"
                v-model.number="quantity"
                type="number"
                min="1"
                class="w-full border dark:border-gray-600 rounded-lg px-3 py-2 bg-white dark:bg-gray-700 text-gray-900 dark:text-white"
              />
            </div>
            <div>
              <label :for="'condition-' + card.id" class="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1">Condition</label>
              <select :id="'condition-' + card.id" v-model="condition" class="w-full border dark:border-gray-600 rounded-lg px-3 py-2 bg-white dark:bg-gray-700 text-gray-900 dark:text-white">
                <option v-for="c in conditions" :key="c.value" :value="c.value">{{ c.label }}</option>
              </select>
            </div>
            <div>
              <label :for="'printing-' + card.id" class="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1">Printing</label>
              <select :id="'printing-' + card.id" v-model="printing" class="w-full border dark:border-gray-600 rounded-lg px-3 py-2 bg-white dark:bg-gray-700 text-gray-900 dark:text-white">
                <option v-for="p in printingOptions" :key="p.value" :value="p.value">{{ p.label }}</option>
              </select>
            </div>

            <div class="mt-6 flex gap-3">
              <button
                v-if="!isCollectionItem"
                @click="handleAdd"
                class="flex-1 bg-blue-600 text-white py-2 px-4 rounded-lg hover:bg-blue-700 transition"
              >
                Add to Collection
              </button>
              <template v-else>
                <button
                  @click="handleUpdate"
                  class="flex-1 bg-blue-600 text-white py-2 px-4 rounded-lg hover:bg-blue-700 transition"
                >
                  Update
                </button>
                <button
                  @click="handleRemove"
                  class="bg-red-600 text-white py-2 px-4 rounded-lg hover:bg-red-700 transition"
                >
                  Remove
                </button>
              </template>
            </div>
          </div>
        </div>
      </div>
    </div>
  </div>
</template>
