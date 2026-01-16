<script setup>
defineProps({
  stats: {
    type: Object,
    required: true
  }
})

const formatPrice = (price) => {
  if (!price) return '$0.00'
  return `$${price.toFixed(2)}`
}
</script>

<template>
  <div class="grid grid-cols-2 md:grid-cols-4 gap-4">
    <div class="bg-white rounded-lg shadow p-4">
      <div class="text-sm text-gray-500">Total Cards</div>
      <div class="text-2xl font-bold text-gray-800">{{ stats.total_cards || 0 }}</div>
    </div>
    <div class="bg-white rounded-lg shadow p-4">
      <div class="text-sm text-gray-500">Unique Cards</div>
      <div class="text-2xl font-bold text-gray-800">{{ stats.unique_cards || 0 }}</div>
    </div>
    <div class="bg-white rounded-lg shadow p-4">
      <div class="text-sm text-gray-500">Collection Value</div>
      <div class="text-2xl font-bold text-green-600">{{ formatPrice(stats.total_value) }}</div>
    </div>
    <div class="bg-white rounded-lg shadow p-4">
      <div class="text-sm text-gray-500">Breakdown</div>
      <div class="text-sm space-y-1 mt-1">
        <div class="flex justify-between">
          <span class="text-purple-600">MTG:</span>
          <span>{{ stats.mtg_cards || 0 }} ({{ formatPrice(stats.mtg_value) }})</span>
        </div>
        <div class="flex justify-between">
          <span class="text-yellow-600">Pokemon:</span>
          <span>{{ stats.pokemon_cards || 0 }} ({{ formatPrice(stats.pokemon_value) }})</span>
        </div>
      </div>
    </div>
  </div>
</template>
