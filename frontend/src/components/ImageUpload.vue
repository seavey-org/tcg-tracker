<script setup>
import { ref, computed, onMounted } from 'vue'
import { cardService } from '../services/api'

const emit = defineEmits(['results', 'error'])

const props = defineProps({
  game: {
    type: String,
    required: true,
    validator: (value) => ['pokemon', 'mtg'].includes(value)
  }
})

const dragActive = ref(false)
const isProcessing = ref(false)
const selectedFile = ref(null)
const previewUrl = ref(null)
const ocrStatus = ref(null)

const isOCRAvailable = computed(() => ocrStatus.value?.server_ocr_available === true)

onMounted(async () => {
  try {
    ocrStatus.value = await cardService.getOCRStatus()
  } catch (e) {
    ocrStatus.value = { server_ocr_available: false }
  }
})

const handleDragEnter = (e) => {
  e.preventDefault()
  dragActive.value = true
}

const handleDragLeave = (e) => {
  e.preventDefault()
  dragActive.value = false
}

const handleDragOver = (e) => {
  e.preventDefault()
}

const handleDrop = (e) => {
  e.preventDefault()
  dragActive.value = false

  const files = e.dataTransfer.files
  if (files.length > 0) {
    handleFileSelect(files[0])
  }
}

const handleFileInput = (e) => {
  const files = e.target.files
  if (files.length > 0) {
    handleFileSelect(files[0])
  }
}

const handleFileSelect = (file) => {
  // Validate file type
  if (!file.type.startsWith('image/')) {
    emit('error', 'Please select an image file')
    return
  }

  // Validate file size (max 10MB)
  if (file.size > 10 * 1024 * 1024) {
    emit('error', 'Image must be smaller than 10MB')
    return
  }

  selectedFile.value = file
  previewUrl.value = URL.createObjectURL(file)
}

const clearSelection = () => {
  if (previewUrl.value) {
    URL.revokeObjectURL(previewUrl.value)
  }
  selectedFile.value = null
  previewUrl.value = null
}

const processImage = async () => {
  if (!selectedFile.value || !isOCRAvailable.value) return

  isProcessing.value = true

  try {
    const result = await cardService.identifyFromImage(selectedFile.value, props.game)
    emit('results', result)
    clearSelection()
  } catch (e) {
    emit('error', e.message || 'Failed to process image')
  } finally {
    isProcessing.value = false
  }
}
</script>

<template>
  <div class="image-upload">
    <!-- OCR Status Warning -->
    <div v-if="ocrStatus && !isOCRAvailable" class="mb-4 p-4 bg-yellow-100 dark:bg-yellow-900 rounded-lg">
      <div class="flex items-center">
        <svg class="w-5 h-5 text-yellow-600 dark:text-yellow-400 mr-2" fill="none" stroke="currentColor" viewBox="0 0 24 24">
          <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M12 9v2m0 4h.01m-6.938 4h13.856c1.54 0 2.502-1.667 1.732-3L13.732 4c-.77-1.333-2.694-1.333-3.464 0L3.34 16c-.77 1.333.192 3 1.732 3z" />
        </svg>
        <span class="text-yellow-700 dark:text-yellow-300 text-sm">
          Server-side image scanning is not available. Please use text search instead.
        </span>
      </div>
    </div>

    <!-- Drop Zone -->
    <div
      v-if="!selectedFile && isOCRAvailable"
      class="border-2 border-dashed rounded-lg p-8 text-center transition-colors"
      :class="{
        'border-blue-500 bg-blue-50 dark:bg-blue-900/20': dragActive,
        'border-gray-300 dark:border-gray-600 hover:border-gray-400 dark:hover:border-gray-500': !dragActive
      }"
      @dragenter="handleDragEnter"
      @dragleave="handleDragLeave"
      @dragover="handleDragOver"
      @drop="handleDrop"
    >
      <svg class="mx-auto h-12 w-12 text-gray-400" fill="none" stroke="currentColor" viewBox="0 0 24 24">
        <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M4 16l4.586-4.586a2 2 0 012.828 0L16 16m-2-2l1.586-1.586a2 2 0 012.828 0L20 14m-6-6h.01M6 20h12a2 2 0 002-2V6a2 2 0 00-2-2H6a2 2 0 00-2 2v12a2 2 0 002 2z" />
      </svg>
      <p class="mt-2 text-sm text-gray-600 dark:text-gray-400">
        <span class="font-semibold">Drop a card image here</span> or
        <label class="text-blue-600 dark:text-blue-400 hover:text-blue-700 cursor-pointer">
          browse
          <input type="file" class="hidden" accept="image/*" @change="handleFileInput" />
        </label>
      </p>
      <p class="mt-1 text-xs text-gray-500 dark:text-gray-500">PNG, JPG up to 10MB</p>
    </div>

    <!-- Image Preview -->
    <div v-if="selectedFile" class="mt-4">
      <div class="relative inline-block">
        <img
          :src="previewUrl"
          alt="Selected card"
          class="max-h-64 rounded-lg shadow-md"
        />
        <button
          @click="clearSelection"
          class="absolute -top-2 -right-2 bg-red-500 hover:bg-red-600 text-white rounded-full p-1"
          :disabled="isProcessing"
        >
          <svg class="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
            <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M6 18L18 6M6 6l12 12" />
          </svg>
        </button>
      </div>

      <div class="mt-4">
        <button
          @click="processImage"
          :disabled="isProcessing || !isOCRAvailable"
          class="w-full bg-blue-600 hover:bg-blue-700 disabled:bg-gray-400 text-white font-medium py-2 px-4 rounded-lg transition-colors flex items-center justify-center"
        >
          <svg v-if="isProcessing" class="animate-spin -ml-1 mr-2 h-5 w-5" fill="none" viewBox="0 0 24 24">
            <circle class="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" stroke-width="4"></circle>
            <path class="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4zm2 5.291A7.962 7.962 0 014 12H0c0 3.042 1.135 5.824 3 7.938l3-2.647z"></path>
          </svg>
          {{ isProcessing ? 'Processing...' : 'Identify Card' }}
        </button>
      </div>
    </div>

    <!-- Set Identifier Status -->
    <div v-if="ocrStatus?.set_identifier?.healthy" class="mt-4 text-xs text-gray-500 dark:text-gray-400 flex items-center">
      <svg class="w-4 h-4 mr-1 text-green-500" fill="none" stroke="currentColor" viewBox="0 0 24 24">
        <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M5 13l4 4L19 7" />
      </svg>
      Set icon matching enabled
    </div>
  </div>
</template>
