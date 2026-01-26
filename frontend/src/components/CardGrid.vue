<script setup>
import { formatPrice, isPriceStale, getItemValue } from '../utils/formatters'

const props = defineProps({
  cards: {
    type: Array,
    required: true
  },
  showQuantity: {
    type: Boolean,
    default: false
  },
  // If true, cards are grouped collection items with total_quantity, variants, etc.
  grouped: {
    type: Boolean,
    default: false
  }
})

const emit = defineEmits(['select'])

// Helper to get quantity - handles both single items and grouped items
function getQuantity(item) {
  if (props.grouped) {
    return item.total_quantity || 1
  }
  return item.quantity || 1
}

// Helper to get value - handles both single items and grouped items
function getValue(item) {
  if (props.grouped) {
    return item.total_value || 0
  }
  return getItemValue(item)
}

// Helper to check if item has multiple variants (for grouped items)
function hasMultipleVariants(item) {
  return props.grouped && item.variants && item.variants.length > 1
}

// Helper to get scanned count (for grouped items)
function getScannedCount(item) {
  return props.grouped ? (item.scanned_count || 0) : (item.scanned_image_path ? 1 : 0)
}

// Check if any variant has a price fallback (language mismatch warning)
function hasPriceFallback(item) {
  if (props.grouped && item.variants) {
    return item.variants.some(v => v.price_fallback)
  }
  // Support flat collection view (individual items have price_fallback directly)
  return item.price_fallback || false
}

// Get the primary printing badge to show (if not Normal)
// For grouped items, show badge if any variant has special printing
function getPrintingBadge(item) {
  if (props.grouped) {
    if (!item.variants) return null
    // Find the first non-Normal printing
    for (const v of item.variants) {
      if (v.printing && v.printing !== 'Normal') {
        return v.printing
      }
    }
    return null
  }
  return item.printing && item.printing !== 'Normal' ? item.printing : null
}

function getPrintingLabel(printing) {
  switch (printing) {
    case 'Foil': return 'FOIL'
    case '1st Edition': return '1ST ED'
    case 'Reverse Holofoil': return 'REV HOLO'
    case 'Unlimited': return 'UNLTD'
    default: return printing
  }
}
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
        <!-- Quantity badge (top right) -->
        <div
          v-if="showQuantity && getQuantity(item) > 1"
          class="absolute top-2 right-2 bg-blue-600 text-white text-sm font-bold px-2 py-1 rounded-full"
        >
          x{{ getQuantity(item) }}
        </div>
        <!-- Scanned indicator (below quantity badge) -->
        <div
          v-if="showQuantity && getScannedCount(item) > 0"
          class="absolute top-10 right-2 bg-green-600 text-white text-xs px-2 py-1 rounded-full flex items-center gap-1"
        >
          <svg class="w-3 h-3" fill="none" stroke="currentColor" viewBox="0 0 24 24">
            <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M3 9a2 2 0 012-2h.93a2 2 0 001.664-.89l.812-1.22A2 2 0 0110.07 4h3.86a2 2 0 011.664.89l.812 1.22A2 2 0 0018.07 7H19a2 2 0 012 2v9a2 2 0 01-2 2H5a2 2 0 01-2-2V9z" />
            <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M15 13a3 3 0 11-6 0 3 3 0 016 0z" />
          </svg>
          {{ getScannedCount(item) }}
        </div>
        <!-- Warning indicator for price fallback (bottom left) -->
        <div
          v-if="showQuantity && hasPriceFallback(item)"
          class="absolute bottom-2 left-2 bg-amber-500 text-white text-xs px-2 py-1 rounded-full flex items-center gap-1"
          title="Price from different language market"
        >
          <svg class="w-3 h-3" fill="none" stroke="currentColor" viewBox="0 0 24 24">
            <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M12 9v2m0 4h.01m-6.938 4h13.856c1.54 0 2.502-1.667 1.732-3L13.732 4c-.77-1.333-2.694-1.333-3.464 0L3.34 16c-.77 1.333.192 3 1.732 3z" />
          </svg>
        </div>
        <!-- Printing badge (top left) -->
        <div
          v-if="getPrintingBadge(item)"
          class="absolute top-2 left-2 text-xs font-bold px-2 py-1 rounded"
          :class="{
            'bg-yellow-400 text-yellow-900': getPrintingBadge(item) === 'Foil',
            'bg-amber-600 text-white': getPrintingBadge(item) === '1st Edition',
            'bg-purple-500 text-white': getPrintingBadge(item) === 'Reverse Holofoil',
            'bg-gray-500 text-white': getPrintingBadge(item) === 'Unlimited'
          }"
        >
          {{ getPrintingLabel(getPrintingBadge(item)) }}
          <span v-if="hasMultipleVariants(item)" class="ml-1">+</span>
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
            {{ showQuantity ? formatPrice(getValue(item)) : formatPrice(item.card?.price_usd || item.price_usd) }}
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
