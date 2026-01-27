<script setup>
import { ref, computed } from 'vue'

const props = defineProps({
  modelValue: {
    type: Object,
    required: true
  },
  availablePrintings: {
    type: Array,
    default: () => []
  },
  availableSets: {
    type: Array,
    default: () => []
  },
  availableConditions: {
    type: Array,
    default: () => []
  },
  availableRarities: {
    type: Array,
    default: () => []
  },
  availableLanguages: {
    type: Array,
    default: () => []
  }
})

const emit = defineEmits(['update:modelValue'])

const expanded = ref(false)
const setSearch = ref('')

// Count of active filters across all categories
const activeCount = computed(() => {
  const f = props.modelValue
  return (f.printings?.length || 0) +
         (f.sets?.length || 0) +
         (f.conditions?.length || 0) +
         (f.rarities?.length || 0) +
         (f.languages?.length || 0) +
         (f.hasPriceWarning ? 1 : 0)
})

// Filter sets by search query
const filteredSets = computed(() => {
  if (!setSearch.value.trim()) {
    return props.availableSets
  }
  const query = setSearch.value.toLowerCase().trim()
  return props.availableSets.filter(s =>
    s.name.toLowerCase().includes(query) ||
    s.code.toLowerCase().includes(query)
  )
})

// Check if a value is selected in a category
const isSelected = (category, value) => {
  return props.modelValue[category]?.includes(value) || false
}

// Toggle a filter value on/off
const toggle = (category, value) => {
  const current = props.modelValue[category] || []
  const newFilters = { ...props.modelValue }

  if (current.includes(value)) {
    newFilters[category] = current.filter(v => v !== value)
  } else {
    newFilters[category] = [...current, value]
  }

  emit('update:modelValue', newFilters)
}

// Clear all filters
const clearAll = () => {
  emit('update:modelValue', {
    printings: [],
    sets: [],
    conditions: [],
    rarities: [],
    languages: [],
    hasPriceWarning: false
  })
  setSearch.value = ''
}

// Toggle the price warning filter
const togglePriceWarning = () => {
  emit('update:modelValue', {
    ...props.modelValue,
    hasPriceWarning: !props.modelValue.hasPriceWarning
  })
}

// Human-readable condition labels
const conditionLabels = {
  'M': 'Mint',
  'NM': 'Near Mint',
  'EX': 'Excellent',
  'GD': 'Good',
  'LP': 'Light Play',
  'PL': 'Played',
  'PR': 'Poor'
}

const getConditionLabel = (condition) => {
  return conditionLabels[condition] || condition
}

// Language flags for display
const languageFlags = {
  'English': '🇺🇸',
  'Japanese': '🇯🇵',
  'German': '🇩🇪',
  'French': '🇫🇷',
  'Italian': '🇮🇹'
}

const getLanguageDisplay = (language) => {
  const flag = languageFlags[language] || ''
  return flag ? `${flag} ${language}` : language
}
</script>

<template>
  <div class="w-full">
    <!-- Filter Toggle Button -->
    <button
      @click="expanded = !expanded"
      class="flex items-center gap-2 px-3 py-2 text-sm font-medium rounded-lg transition-colors"
      :class="activeCount > 0
        ? 'bg-blue-100 dark:bg-blue-900/40 text-blue-700 dark:text-blue-300 hover:bg-blue-200 dark:hover:bg-blue-900/60'
        : 'bg-gray-100 dark:bg-gray-700 text-gray-700 dark:text-gray-300 hover:bg-gray-200 dark:hover:bg-gray-600'"
    >
      <svg
        class="w-4 h-4 transition-transform"
        :class="{ 'rotate-180': expanded }"
        fill="none"
        stroke="currentColor"
        viewBox="0 0 24 24"
      >
        <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M19 9l-7 7-7-7" />
      </svg>
      <span>Filters</span>
      <span
        v-if="activeCount > 0"
        class="inline-flex items-center justify-center w-5 h-5 text-xs font-bold bg-blue-600 text-white rounded-full"
      >
        {{ activeCount }}
      </span>
      <!-- Quick clear button on the badge -->
      <button
        v-if="activeCount > 0"
        @click.stop="clearAll"
        class="ml-1 p-0.5 rounded-full hover:bg-blue-200 dark:hover:bg-blue-800 transition-colors"
        title="Clear all filters"
      >
        <svg class="w-3.5 h-3.5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
          <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M6 18L18 6M6 6l12 12" />
        </svg>
      </button>
    </button>

    <!-- Collapsible Filter Panel -->
    <transition
      enter-active-class="transition-all duration-200 ease-out"
      enter-from-class="opacity-0 -translate-y-2"
      enter-to-class="opacity-100 translate-y-0"
      leave-active-class="transition-all duration-150 ease-in"
      leave-from-class="opacity-100 translate-y-0"
      leave-to-class="opacity-0 -translate-y-2"
    >
      <div
        v-show="expanded"
        class="mt-3 p-4 bg-white dark:bg-gray-800 rounded-lg border border-gray-200 dark:border-gray-700 shadow-sm space-y-4"
      >
        <!-- Price Warning Filter -->
        <div class="space-y-2">
          <label class="text-sm font-medium text-gray-700 dark:text-gray-300">Alerts</label>
          <div class="flex flex-wrap gap-2">
            <button
              @click="togglePriceWarning"
              class="px-3 py-1.5 text-sm rounded-full border transition-colors flex items-center gap-1.5"
              :class="modelValue.hasPriceWarning
                ? 'bg-amber-500 border-amber-500 text-white'
                : 'bg-white dark:bg-gray-700 border-gray-300 dark:border-gray-600 text-gray-700 dark:text-gray-300 hover:border-amber-400 dark:hover:border-amber-500'"
            >
              <svg class="w-4 h-4" fill="currentColor" viewBox="0 0 20 20">
                <path fill-rule="evenodd" d="M8.257 3.099c.765-1.36 2.722-1.36 3.486 0l5.58 9.92c.75 1.334-.213 2.98-1.742 2.98H4.42c-1.53 0-2.493-1.646-1.743-2.98l5.58-9.92zM11 13a1 1 0 11-2 0 1 1 0 012 0zm-1-8a1 1 0 00-1 1v3a1 1 0 002 0V6a1 1 0 00-1-1z" clip-rule="evenodd" />
              </svg>
              Price Warning
            </button>
          </div>
        </div>

        <!-- Printing Type Filter -->
        <div v-if="availablePrintings.length > 0" class="space-y-2">
          <label class="text-sm font-medium text-gray-700 dark:text-gray-300">Printing</label>
          <div class="flex flex-wrap gap-2">
            <button
              v-for="printing in availablePrintings"
              :key="printing"
              @click="toggle('printings', printing)"
              class="px-3 py-1.5 text-sm rounded-full border transition-colors"
              :class="isSelected('printings', printing)
                ? 'bg-blue-600 border-blue-600 text-white'
                : 'bg-white dark:bg-gray-700 border-gray-300 dark:border-gray-600 text-gray-700 dark:text-gray-300 hover:border-blue-400 dark:hover:border-blue-500'"
            >
              {{ printing }}
            </button>
          </div>
        </div>

        <!-- Set Filter with Search -->
        <div v-if="availableSets.length > 0" class="space-y-2">
          <label class="text-sm font-medium text-gray-700 dark:text-gray-300">Set</label>
          <input
            v-model="setSearch"
            type="text"
            placeholder="Search sets..."
            class="w-full px-3 py-2 text-sm border border-gray-300 dark:border-gray-600 rounded-lg bg-white dark:bg-gray-700 text-gray-900 dark:text-white placeholder-gray-400 focus:ring-2 focus:ring-blue-500 focus:border-blue-500"
          />
          <div class="flex flex-wrap gap-2 max-h-32 overflow-y-auto">
            <button
              v-for="set in filteredSets"
              :key="set.code"
              @click="toggle('sets', set.code)"
              class="px-3 py-1.5 text-sm rounded-full border transition-colors"
              :class="isSelected('sets', set.code)
                ? 'bg-blue-600 border-blue-600 text-white'
                : 'bg-white dark:bg-gray-700 border-gray-300 dark:border-gray-600 text-gray-700 dark:text-gray-300 hover:border-blue-400 dark:hover:border-blue-500'"
              :title="set.name"
            >
              {{ set.name }}
            </button>
          </div>
        </div>

        <!-- Condition Filter -->
        <div v-if="availableConditions.length > 0" class="space-y-2">
          <label class="text-sm font-medium text-gray-700 dark:text-gray-300">Condition</label>
          <div class="flex flex-wrap gap-2">
            <button
              v-for="condition in availableConditions"
              :key="condition"
              @click="toggle('conditions', condition)"
              class="px-3 py-1.5 text-sm rounded-full border transition-colors"
              :class="isSelected('conditions', condition)
                ? 'bg-blue-600 border-blue-600 text-white'
                : 'bg-white dark:bg-gray-700 border-gray-300 dark:border-gray-600 text-gray-700 dark:text-gray-300 hover:border-blue-400 dark:hover:border-blue-500'"
              :title="getConditionLabel(condition)"
            >
              {{ condition }}
            </button>
          </div>
        </div>

        <!-- Language Filter -->
        <div v-if="availableLanguages.length > 0" class="space-y-2">
          <label class="text-sm font-medium text-gray-700 dark:text-gray-300">Language</label>
          <div class="flex flex-wrap gap-2">
            <button
              v-for="language in availableLanguages"
              :key="language"
              @click="toggle('languages', language)"
              class="px-3 py-1.5 text-sm rounded-full border transition-colors"
              :class="isSelected('languages', language)
                ? 'bg-blue-600 border-blue-600 text-white'
                : 'bg-white dark:bg-gray-700 border-gray-300 dark:border-gray-600 text-gray-700 dark:text-gray-300 hover:border-blue-400 dark:hover:border-blue-500'"
            >
              {{ getLanguageDisplay(language) }}
            </button>
          </div>
        </div>

        <!-- Rarity Filter -->
        <div v-if="availableRarities.length > 0" class="space-y-2">
          <label class="text-sm font-medium text-gray-700 dark:text-gray-300">Rarity</label>
          <div class="flex flex-wrap gap-2">
            <button
              v-for="rarity in availableRarities"
              :key="rarity"
              @click="toggle('rarities', rarity)"
              class="px-3 py-1.5 text-sm rounded-full border transition-colors"
              :class="isSelected('rarities', rarity)
                ? 'bg-blue-600 border-blue-600 text-white'
                : 'bg-white dark:bg-gray-700 border-gray-300 dark:border-gray-600 text-gray-700 dark:text-gray-300 hover:border-blue-400 dark:hover:border-blue-500'"
            >
              {{ rarity }}
            </button>
          </div>
        </div>

        <!-- Clear All Button -->
        <div v-if="activeCount > 0" class="pt-2 border-t border-gray-200 dark:border-gray-700">
          <button
            @click="clearAll"
            class="text-sm text-red-600 dark:text-red-400 hover:text-red-700 dark:hover:text-red-300 font-medium"
          >
            Clear All Filters
          </button>
        </div>
      </div>
    </transition>
  </div>
</template>
