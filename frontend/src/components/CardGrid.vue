<script setup>
import { formatPrice, isPriceStale, getItemValue } from '../utils/formatters'

defineProps({
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
</script>

<template>
  <div class="grid grid-cols-2 sm:grid-cols-3 md:grid-cols-4 lg:grid-cols-5 xl:grid-cols-6 gap-4">
    <div
      v-for="(item, index) in cards"
      :key="item.id || item.card?.id || `card-${index}`"
      class="bg-white dark:bg-gray-800 rounded-lg shadow-md overflow-hidden cursor-pointer hover:shadow-lg transition-shadow"
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
        <div
          v-if="item.first_edition"
          class="absolute top-2 left-2 bg-amber-600 text-white text-xs font-bold px-2 py-1 rounded"
          :class="{ 'top-10': item.foil }"
        >
          1ST ED
        </div>
      </div>
      <div class="p-3">
        <h3 class="font-semibold text-sm truncate text-gray-800 dark:text-white">
          {{ item.card?.name || item.name }}
        </h3>
        <p class="text-xs text-gray-500 dark:text-gray-400 truncate">
          {{ item.card?.set_name || item.set_name }}
        </p>
        <div class="mt-2 flex justify-between items-center">
          <span class="text-sm font-medium" :class="isPriceStale(item) ? 'text-orange-500' : 'text-green-600 dark:text-green-400'">
            {{ showQuantity ? formatPrice(getItemValue(item)) : formatPrice(item.card?.price_usd || item.price_usd) }}
            <span v-if="isPriceStale(item)" class="text-xs">*</span>
          </span>
          <span
            class="text-xs px-2 py-0.5 rounded"
            :class="{
              'bg-purple-100 dark:bg-purple-900 text-purple-800 dark:text-purple-200': (item.card?.game || item.game) === 'mtg',
              'bg-yellow-100 dark:bg-yellow-900 text-yellow-800 dark:text-yellow-200': (item.card?.game || item.game) === 'pokemon'
            }"
          >
            {{ (item.card?.game || item.game) === 'mtg' ? 'MTG' : 'Pokemon' }}
          </span>
        </div>
      </div>
    </div>
  </div>
</template>
