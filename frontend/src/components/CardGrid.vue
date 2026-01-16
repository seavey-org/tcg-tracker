<script setup>
import { computed } from 'vue'

const props = defineProps({
  cards: {
    type: Array,
    required: true
  },
  showQuantity: {
    type: Boolean,
    default: false
  }
})

const emit = defineEmits(['select'])

const formatPrice = (price) => {
  if (!price) return '-'
  return `$${price.toFixed(2)}`
}

const getItemValue = (item) => {
  if (item.foil && item.card.price_foil_usd) {
    return item.card.price_foil_usd * item.quantity
  }
  return (item.card.price_usd || 0) * item.quantity
}

const isPriceStale = (item) => {
  const card = item.card || item
  if (!card.price_updated_at) return true
  const date = new Date(card.price_updated_at)
  const now = new Date()
  return (now - date) > 24 * 60 * 60 * 1000 // 24 hours
}
</script>

<template>
  <div class="grid grid-cols-2 sm:grid-cols-3 md:grid-cols-4 lg:grid-cols-5 xl:grid-cols-6 gap-4">
    <div
      v-for="item in cards"
      :key="item.id || item.card?.id"
      class="bg-white rounded-lg shadow-md overflow-hidden cursor-pointer hover:shadow-lg transition-shadow"
      @click="emit('select', item)"
    >
      <div class="relative">
        <img
          :src="item.card?.image_url || item.image_url"
          :alt="item.card?.name || item.name"
          class="w-full aspect-[2.5/3.5] object-cover"
          loading="lazy"
        />
        <div
          v-if="showQuantity && item.quantity > 1"
          class="absolute top-2 right-2 bg-blue-600 text-white text-sm font-bold px-2 py-1 rounded-full"
        >
          x{{ item.quantity }}
        </div>
        <div
          v-if="item.foil"
          class="absolute top-2 left-2 bg-yellow-400 text-yellow-900 text-xs font-bold px-2 py-1 rounded"
        >
          FOIL
        </div>
      </div>
      <div class="p-3">
        <h3 class="font-semibold text-sm truncate text-gray-800">
          {{ item.card?.name || item.name }}
        </h3>
        <p class="text-xs text-gray-500 truncate">
          {{ item.card?.set_name || item.set_name }}
        </p>
        <div class="mt-2 flex justify-between items-center">
          <span class="text-sm font-medium" :class="isPriceStale(item) ? 'text-orange-500' : 'text-green-600'">
            {{ showQuantity ? formatPrice(getItemValue(item)) : formatPrice(item.card?.price_usd || item.price_usd) }}
            <span v-if="isPriceStale(item)" class="text-xs">*</span>
          </span>
          <span
            class="text-xs px-2 py-0.5 rounded"
            :class="{
              'bg-purple-100 text-purple-800': (item.card?.game || item.game) === 'mtg',
              'bg-yellow-100 text-yellow-800': (item.card?.game || item.game) === 'pokemon'
            }"
          >
            {{ (item.card?.game || item.game) === 'mtg' ? 'MTG' : 'Pokemon' }}
          </span>
        </div>
      </div>
    </div>
  </div>
</template>
