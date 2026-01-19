<script setup>
import { ref, computed, watch, onMounted } from 'vue'
import { useCollectionStore } from '../stores/collection'

const store = useCollectionStore()

const periods = [
  { value: 'week', label: '7D' },
  { value: 'month', label: '1M' },
  { value: '3month', label: '3M' },
  { value: 'year', label: '1Y' },
  { value: 'all', label: 'All' }
]

const selectedPeriod = ref('month')
const hoveredPoint = ref(null)
const chartWidth = 600
const chartHeight = 200
const padding = { top: 20, right: 20, bottom: 30, left: 60 }

const snapshots = computed(() => {
  return store.valueHistory?.snapshots || []
})

const hasData = computed(() => snapshots.value.length > 0)

const valueChange = computed(() => {
  if (snapshots.value.length < 2) return null
  const first = snapshots.value[0].total_value
  const last = snapshots.value[snapshots.value.length - 1].total_value
  const change = last - first
  const percent = first > 0 ? ((change / first) * 100).toFixed(1) : 0
  return { change, percent, isPositive: change >= 0 }
})

// Chart dimensions
const innerWidth = computed(() => chartWidth - padding.left - padding.right)
const innerHeight = computed(() => chartHeight - padding.top - padding.bottom)

// Scales
const xScale = computed(() => {
  if (snapshots.value.length === 0) return { min: 0, max: 1, range: innerWidth.value }
  return {
    min: 0,
    max: snapshots.value.length - 1,
    range: innerWidth.value
  }
})

const yScale = computed(() => {
  if (snapshots.value.length === 0) return { min: 0, max: 100, range: innerHeight.value }
  const values = snapshots.value.map(s => s.total_value)
  const min = Math.min(...values) * 0.95
  const max = Math.max(...values) * 1.05
  // Ensure some range if all values are the same
  const adjustedMin = min === max ? min * 0.9 : min
  const adjustedMax = min === max ? max * 1.1 : max
  return {
    min: adjustedMin,
    max: adjustedMax,
    range: innerHeight.value
  }
})

// Convert data point to SVG coordinates
const getX = (index) => {
  const scale = xScale.value
  return padding.left + (index / (scale.max || 1)) * scale.range
}

const getY = (value) => {
  const scale = yScale.value
  const normalized = (value - scale.min) / (scale.max - scale.min || 1)
  return padding.top + (1 - normalized) * scale.range
}

// Generate SVG path for the line
const linePath = computed(() => {
  if (snapshots.value.length === 0) return ''
  const points = snapshots.value.map((s, i) => `${getX(i)},${getY(s.total_value)}`)
  return `M ${points.join(' L ')}`
})

// Generate SVG path for the area fill
const areaPath = computed(() => {
  if (snapshots.value.length === 0) return ''
  const baseline = padding.top + innerHeight.value
  const points = snapshots.value.map((s, i) => `${getX(i)},${getY(s.total_value)}`)
  return `M ${getX(0)},${baseline} L ${points.join(' L ')} L ${getX(snapshots.value.length - 1)},${baseline} Z`
})

// Y-axis ticks
const yTicks = computed(() => {
  const scale = yScale.value
  const tickCount = 4
  const ticks = []
  for (let i = 0; i <= tickCount; i++) {
    const value = scale.min + (scale.max - scale.min) * (i / tickCount)
    ticks.push({
      value,
      y: getY(value),
      label: formatPrice(value)
    })
  }
  return ticks
})

// X-axis labels (show a few dates)
const xLabels = computed(() => {
  if (snapshots.value.length === 0) return []
  const labels = []
  const step = Math.max(1, Math.floor(snapshots.value.length / 5))
  for (let i = 0; i < snapshots.value.length; i += step) {
    const snapshot = snapshots.value[i]
    labels.push({
      x: getX(i),
      label: formatDate(snapshot.snapshot_date)
    })
  }
  // Always include the last point
  if (labels.length > 0 && labels[labels.length - 1].x !== getX(snapshots.value.length - 1)) {
    labels.push({
      x: getX(snapshots.value.length - 1),
      label: formatDate(snapshots.value[snapshots.value.length - 1].snapshot_date)
    })
  }
  return labels
})

const formatPrice = (value) => {
  if (value >= 1000) {
    return '$' + (value / 1000).toFixed(1) + 'k'
  }
  return '$' + value.toFixed(0)
}

const formatDate = (dateString) => {
  const date = new Date(dateString)
  return date.toLocaleDateString(undefined, { month: 'short', day: 'numeric' })
}

const formatFullDate = (dateString) => {
  const date = new Date(dateString)
  return date.toLocaleDateString(undefined, { month: 'long', day: 'numeric', year: 'numeric' })
}

const formatFullPrice = (value) => {
  return new Intl.NumberFormat('en-US', { style: 'currency', currency: 'USD' }).format(value)
}

const handleMouseMove = (event) => {
  if (snapshots.value.length === 0) return

  const svg = event.currentTarget
  const rect = svg.getBoundingClientRect()
  const x = event.clientX - rect.left - padding.left

  // Find closest data point
  const index = Math.round((x / innerWidth.value) * (snapshots.value.length - 1))
  const clampedIndex = Math.max(0, Math.min(index, snapshots.value.length - 1))

  hoveredPoint.value = {
    index: clampedIndex,
    snapshot: snapshots.value[clampedIndex],
    x: getX(clampedIndex),
    y: getY(snapshots.value[clampedIndex].total_value)
  }
}

const handleMouseLeave = () => {
  hoveredPoint.value = null
}

const changePeriod = async (period) => {
  selectedPeriod.value = period
  await store.fetchValueHistory(period)
}

onMounted(() => {
  store.fetchValueHistory(selectedPeriod.value)
})

watch(selectedPeriod, (newPeriod) => {
  store.fetchValueHistory(newPeriod)
})
</script>

<template>
  <div class="bg-white dark:bg-gray-800 rounded-lg shadow p-4">
    <div class="flex items-center justify-between mb-4">
      <div>
        <h3 class="text-sm font-medium text-gray-700 dark:text-gray-300">Collection Value</h3>
        <div v-if="hasData && valueChange" class="flex items-center gap-2 mt-1">
          <span
            :class="valueChange.isPositive ? 'text-green-600 dark:text-green-400' : 'text-red-600 dark:text-red-400'"
            class="text-xs font-medium"
          >
            {{ valueChange.isPositive ? '+' : '' }}{{ formatFullPrice(valueChange.change) }}
            ({{ valueChange.isPositive ? '+' : '' }}{{ valueChange.percent }}%)
          </span>
          <span class="text-xs text-gray-500 dark:text-gray-500">
            vs {{ periods.find(p => p.value === selectedPeriod)?.label }} ago
          </span>
        </div>
      </div>
      <div class="flex gap-1">
        <button
          v-for="period in periods"
          :key="period.value"
          @click="changePeriod(period.value)"
          :class="[
            'px-2 py-1 text-xs rounded transition-colors',
            selectedPeriod === period.value
              ? 'bg-blue-600 text-white'
              : 'bg-gray-100 dark:bg-gray-700 text-gray-600 dark:text-gray-400 hover:bg-gray-200 dark:hover:bg-gray-600'
          ]"
        >
          {{ period.label }}
        </button>
      </div>
    </div>

    <div v-if="!hasData" class="h-48 flex items-center justify-center text-gray-500 dark:text-gray-400">
      <div class="text-center">
        <p class="text-sm">No value history yet</p>
        <p class="text-xs mt-1">Daily snapshots will appear here</p>
      </div>
    </div>

    <div v-else class="relative">
      <svg
        :width="chartWidth"
        :height="chartHeight"
        class="w-full h-auto"
        :viewBox="`0 0 ${chartWidth} ${chartHeight}`"
        @mousemove="handleMouseMove"
        @mouseleave="handleMouseLeave"
      >
        <!-- Grid lines -->
        <g class="text-gray-200 dark:text-gray-700">
          <line
            v-for="tick in yTicks"
            :key="tick.value"
            :x1="padding.left"
            :y1="tick.y"
            :x2="chartWidth - padding.right"
            :y2="tick.y"
            stroke="currentColor"
            stroke-dasharray="2,2"
          />
        </g>

        <!-- Area fill -->
        <path
          :d="areaPath"
          class="fill-blue-100 dark:fill-blue-900/30"
        />

        <!-- Line -->
        <path
          :d="linePath"
          fill="none"
          class="stroke-blue-600 dark:stroke-blue-400"
          stroke-width="2"
          stroke-linecap="round"
          stroke-linejoin="round"
        />

        <!-- Data points -->
        <g v-if="snapshots.length <= 31">
          <circle
            v-for="(snapshot, i) in snapshots"
            :key="i"
            :cx="getX(i)"
            :cy="getY(snapshot.total_value)"
            r="3"
            class="fill-blue-600 dark:fill-blue-400"
          />
        </g>

        <!-- Y-axis labels -->
        <g class="text-xs fill-gray-500 dark:fill-gray-400">
          <text
            v-for="tick in yTicks"
            :key="tick.value"
            :x="padding.left - 8"
            :y="tick.y + 4"
            text-anchor="end"
          >
            {{ tick.label }}
          </text>
        </g>

        <!-- X-axis labels -->
        <g class="text-xs fill-gray-500 dark:fill-gray-400">
          <text
            v-for="label in xLabels"
            :key="label.x"
            :x="label.x"
            :y="chartHeight - 8"
            text-anchor="middle"
          >
            {{ label.label }}
          </text>
        </g>

        <!-- Hover indicator -->
        <g v-if="hoveredPoint">
          <line
            :x1="hoveredPoint.x"
            :y1="padding.top"
            :x2="hoveredPoint.x"
            :y2="chartHeight - padding.bottom"
            class="stroke-gray-400 dark:stroke-gray-500"
            stroke-dasharray="4,4"
          />
          <circle
            :cx="hoveredPoint.x"
            :cy="hoveredPoint.y"
            r="5"
            class="fill-blue-600 dark:fill-blue-400 stroke-white dark:stroke-gray-800"
            stroke-width="2"
          />
        </g>
      </svg>

      <!-- Tooltip -->
      <div
        v-if="hoveredPoint"
        class="absolute bg-gray-800 dark:bg-gray-900 text-white text-xs rounded px-2 py-1 pointer-events-none shadow-lg"
        :style="{
          left: `${Math.min(hoveredPoint.x, chartWidth - 100)}px`,
          top: `${Math.max(hoveredPoint.y - 40, 0)}px`
        }"
      >
        <div class="font-semibold">{{ formatFullPrice(hoveredPoint.snapshot.total_value) }}</div>
        <div class="text-gray-400">{{ formatFullDate(hoveredPoint.snapshot.snapshot_date) }}</div>
        <div class="text-gray-400">{{ hoveredPoint.snapshot.total_cards }} cards</div>
      </div>
    </div>
  </div>
</template>
