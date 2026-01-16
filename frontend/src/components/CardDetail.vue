<script setup>
import { ref, computed } from 'vue'
import { priceService } from '../services/api'

const props = defineProps({
  item: {
    type: Object,
    required: true
  },
  isCollectionItem: {
    type: Boolean,
    default: false
  }
})

const emit = defineEmits(['close', 'add', 'update', 'remove', 'priceUpdated'])

const card = computed(() => props.item.card || props.item)
const quantity = ref(props.item.quantity || 1)
const condition = ref(props.item.condition || 'NM')
const foil = ref(props.item.foil || false)
const refreshingPrice = ref(false)
const priceError = ref(null)

const conditions = [
  { value: 'M', label: 'Mint' },
  { value: 'NM', label: 'Near Mint' },
  { value: 'EX', label: 'Excellent' },
  { value: 'GD', label: 'Good' },
  { value: 'LP', label: 'Light Play' },
  { value: 'PL', label: 'Played' },
  { value: 'PR', label: 'Poor' }
]

const formatPrice = (price) => {
  if (!price) return '-'
  return `$${price.toFixed(2)}`
}

const formatTimeAgo = (dateString) => {
  if (!dateString) return null
  const date = new Date(dateString)
  const now = new Date()
  const seconds = Math.floor((now - date) / 1000)

  if (seconds < 60) return 'just now'
  if (seconds < 3600) return `${Math.floor(seconds / 60)}m ago`
  if (seconds < 86400) return `${Math.floor(seconds / 3600)}h ago`
  return `${Math.floor(seconds / 86400)}d ago`
}

const isPriceStale = computed(() => {
  if (!card.value.price_updated_at) return true
  const date = new Date(card.value.price_updated_at)
  const now = new Date()
  return (now - date) > 24 * 60 * 60 * 1000 // 24 hours
})

const priceAge = computed(() => formatTimeAgo(card.value.price_updated_at))

const isPokemon = computed(() => card.value.game === 'pokemon')

const refreshPrice = async () => {
  if (!isPokemon.value) return

  refreshingPrice.value = true
  priceError.value = null

  try {
    const result = await priceService.refreshCardPrice(card.value.id)
    if (result.card) {
      // Update the card's price data
      card.value.price_usd = result.card.price_usd
      card.value.price_foil_usd = result.card.price_foil_usd
      card.value.price_updated_at = result.card.price_updated_at
      card.value.price_source = result.card.price_source
      emit('priceUpdated', result.card)
    }
  } catch (err) {
    if (err.response?.status === 429) {
      priceError.value = 'Daily quota exceeded'
    } else {
      priceError.value = 'Failed to refresh price'
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
    foil: foil.value
  })
}

const handleUpdate = () => {
  emit('update', {
    id: props.item.id,
    quantity: quantity.value,
    condition: condition.value,
    foil: foil.value
  })
}

const handleRemove = () => {
  if (confirm('Are you sure you want to remove this card from your collection?')) {
    emit('remove', props.item.id)
  }
}
</script>

<template>
  <div class="fixed inset-0 bg-black bg-opacity-50 flex items-center justify-center z-50 p-4" @click.self="emit('close')">
    <div class="bg-white rounded-lg max-w-2xl w-full max-h-[90vh] overflow-y-auto">
      <div class="flex flex-col md:flex-row">
        <div class="md:w-1/2 p-4">
          <img
            :src="card.image_url_large || card.image_url"
            :alt="card.name"
            class="w-full rounded-lg shadow"
          />
        </div>
        <div class="md:w-1/2 p-6">
          <div class="flex justify-between items-start mb-4">
            <div>
              <h2 class="text-2xl font-bold text-gray-800">{{ card.name }}</h2>
              <p class="text-gray-500">{{ card.set_name }}</p>
            </div>
            <button @click="emit('close')" class="text-gray-400 hover:text-gray-600">
              <svg class="w-6 h-6" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M6 18L18 6M6 6l12 12" />
              </svg>
            </button>
          </div>

          <div class="space-y-3 mb-6">
            <div class="flex justify-between">
              <span class="text-gray-600">Set:</span>
              <span class="font-medium">{{ card.set_name }} ({{ card.set_code }})</span>
            </div>
            <div class="flex justify-between">
              <span class="text-gray-600">Number:</span>
              <span class="font-medium">{{ card.card_number }}</span>
            </div>
            <div class="flex justify-between">
              <span class="text-gray-600">Rarity:</span>
              <span class="font-medium capitalize">{{ card.rarity }}</span>
            </div>
            <div class="flex justify-between items-center">
              <span class="text-gray-600">Price:</span>
              <div class="flex items-center gap-2">
                <span class="font-medium text-green-600">{{ formatPrice(card.price_usd) }}</span>
                <span v-if="priceAge" class="text-xs" :class="isPriceStale ? 'text-orange-500' : 'text-gray-400'">
                  ({{ priceAge }})
                </span>
              </div>
            </div>
            <div v-if="card.price_foil_usd" class="flex justify-between">
              <span class="text-gray-600">Foil Price:</span>
              <span class="font-medium text-yellow-600">{{ formatPrice(card.price_foil_usd) }}</span>
            </div>
            <div v-if="isPokemon" class="flex justify-between items-center">
              <span class="text-gray-600">Price Status:</span>
              <div class="flex items-center gap-2">
                <span class="text-xs px-2 py-1 rounded" :class="{
                  'bg-green-100 text-green-700': card.price_source === 'api',
                  'bg-blue-100 text-blue-700': card.price_source === 'cached',
                  'bg-yellow-100 text-yellow-700': card.price_source === 'pending',
                  'bg-gray-100 text-gray-600': !card.price_source
                }">
                  {{ card.price_source || 'unknown' }}
                </span>
                <button
                  @click="refreshPrice"
                  :disabled="refreshingPrice"
                  class="text-blue-600 hover:text-blue-800 text-sm disabled:opacity-50"
                >
                  <span v-if="refreshingPrice">Refreshing...</span>
                  <span v-else>Refresh</span>
                </button>
              </div>
            </div>
            <div v-if="priceError" class="text-red-500 text-sm">
              {{ priceError }}
            </div>
          </div>

          <div class="border-t pt-4 space-y-4">
            <div>
              <label class="block text-sm font-medium text-gray-700 mb-1">Quantity</label>
              <input
                v-model.number="quantity"
                type="number"
                min="1"
                class="w-full border rounded-lg px-3 py-2"
              />
            </div>
            <div>
              <label class="block text-sm font-medium text-gray-700 mb-1">Condition</label>
              <select v-model="condition" class="w-full border rounded-lg px-3 py-2">
                <option v-for="c in conditions" :key="c.value" :value="c.value">
                  {{ c.label }}
                </option>
              </select>
            </div>
            <div class="flex items-center">
              <input
                v-model="foil"
                type="checkbox"
                id="foil"
                class="rounded border-gray-300 text-blue-600 mr-2"
              />
              <label for="foil" class="text-sm font-medium text-gray-700">Foil</label>
            </div>
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
</template>
