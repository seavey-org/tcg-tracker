import axios from 'axios'

const api = axios.create({
  baseURL: import.meta.env.VITE_API_URL || '/api',
  timeout: 10000,
})

// Response interceptor for error handling
api.interceptors.response.use(
  response => response,
  error => {
    // Handle timeout errors
    if (error.code === 'ECONNABORTED') {
      error.message = 'Request timed out. Please try again.'
    }
    // Handle network errors
    else if (!error.response) {
      error.message = 'Network error. Please check your connection.'
    }
    // Handle server errors
    else if (error.response.status >= 500) {
      error.message = error.response.data?.error || 'Server error. Please try again later.'
    }
    // Handle client errors
    else if (error.response.status >= 400) {
      error.message = error.response.data?.error || 'Request failed.'
    }
    return Promise.reject(error)
  }
)

export const cardService = {
  async search(query, game) {
    const response = await api.get('/cards/search', {
      params: { q: query, game }
    })
    return response.data
  },

  async getCard(id, game) {
    const response = await api.get(`/cards/${id}`, {
      params: { game }
    })
    return response.data
  },

  async identify(text, game) {
    const response = await api.post('/cards/identify', { text, game })
    return response.data
  },

  /**
   * Identify cards from an uploaded image using server-side OCR
   * @param {File} file - The image file to process
   * @param {string} game - 'pokemon' or 'mtg'
   * @returns {Promise<Object>} - The identification result with cards and parsed data
   */
  async identifyFromImage(file, game) {
    const formData = new FormData()
    formData.append('image', file)
    formData.append('game', game)

    const response = await api.post('/cards/identify-image', formData, {
      headers: { 'Content-Type': 'multipart/form-data' },
      timeout: 60000, // 60 second timeout for image processing
    })
    return response.data
  },

  /**
   * Check if server-side OCR is available
   * @returns {Promise<Object>} - Status of OCR and set identifier services
   */
  async getOCRStatus() {
    const response = await api.get('/cards/ocr-status')
    return response.data
  }
}

export const collectionService = {
  async getAll(game = null) {
    const response = await api.get('/collection', {
      params: game ? { game } : {}
    })
    return response.data
  },

  async add(cardId, options = {}) {
    const response = await api.post('/collection', {
      card_id: cardId,
      quantity: options.quantity || 1,
      condition: options.condition || 'NM',
      foil: options.foil || false,
      notes: options.notes || ''
    })
    return response.data
  },

  async update(id, updates) {
    const response = await api.put(`/collection/${id}`, updates)
    return response.data
  },

  async remove(id) {
    await api.delete(`/collection/${id}`)
  },

  async getStats() {
    const response = await api.get('/collection/stats')
    return response.data
  },

  async refreshPrices() {
    const response = await api.post('/collection/refresh-prices')
    return response.data
  }
}

export const priceService = {
  async getStatus() {
    const response = await api.get('/prices/status')
    return response.data
  },

  async refreshCardPrice(cardId) {
    const response = await api.post(`/cards/${cardId}/refresh-price`)
    return response.data
  }
}

export default api
