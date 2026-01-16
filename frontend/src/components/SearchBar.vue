<script setup>
import { ref } from 'vue'

const emit = defineEmits(['search'])

const query = ref('')
const game = ref('mtg')

const handleSearch = () => {
  if (query.value.trim()) {
    emit('search', { query: query.value.trim(), game: game.value })
  }
}
</script>

<template>
  <div class="flex flex-col sm:flex-row gap-3">
    <div class="flex-1">
      <input
        v-model="query"
        type="text"
        placeholder="Search for cards..."
        class="w-full border rounded-lg px-4 py-2 focus:ring-2 focus:ring-blue-500 focus:border-transparent"
        @keyup.enter="handleSearch"
      />
    </div>
    <div class="flex gap-2">
      <select
        v-model="game"
        class="border rounded-lg px-4 py-2 bg-white"
      >
        <option value="mtg">Magic: The Gathering</option>
        <option value="pokemon">Pokemon</option>
      </select>
      <button
        @click="handleSearch"
        class="bg-blue-600 text-white px-6 py-2 rounded-lg hover:bg-blue-700 transition"
      >
        Search
      </button>
    </div>
  </div>
</template>
