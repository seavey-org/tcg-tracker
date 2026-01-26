import axios from 'axios'

const ADMIN_KEY_STORAGE = 'tcg-admin-key'

const api = axios.create({
  baseURL: import.meta.env.VITE_API_URL || '/api',
  timeout: 10000,
})

// Auth error callback - set by the app to handle 401 errors
let onAuthError = null

/**
 * Register a callback to be called when a 401 error occurs
 * @param {Function} callback - Called with the failed request config
 */
export function setAuthErrorHandler(callback) {
  onAuthError = callback
}

/**
 * Get the stored admin key
 */
export function getStoredAdminKey() {
  return localStorage.getItem(ADMIN_KEY_STORAGE)
}

// Request interceptor to add auth header
api.interceptors.request.use(
  config => {
    const adminKey = getStoredAdminKey()
    if (adminKey) {
      config.headers.Authorization = `Bearer ${adminKey}`
    }
    return config
  },
  error => Promise.reject(error)
)

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
    // Handle auth errors
    else if (error.response.status === 401) {
      error.message = error.response.data?.error || 'Authentication required'
      error.isAuthError = true
      // Notify the app about the auth error
      if (onAuthError) {
        onAuthError(error)
      }
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

  /**
   * Search for cards by name and group results by set for 2-phase selection
   * @param {string} query - Card name to search for
   * @param {string} game - 'pokemon' or 'mtg'
   * @returns {Promise<Object>} - { card_name, set_groups: [...], total_sets }
   */
  async searchGrouped(query, game) {
    const response = await api.get('/cards/search/grouped', {
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
   * Identify cards from an uploaded image using Gemini Vision API
   * @param {File} file - The image file to process
   * @returns {Promise<Object>} - The identification result with cards and parsed data
   */
  async identifyFromImage(file) {
    const formData = new FormData()
    formData.append('image', file)

    const response = await api.post('/cards/identify-image', formData, {
      headers: { 'Content-Type': 'multipart/form-data' },
      timeout: 90000, // 90 second timeout for Gemini multi-turn identification
    })
    return response.data
  }
}

export const setService = {
  /**
   * List sets with optional query filter
   * @param {string} query - Optional search query (name, series, or code)
   * @param {string} game - 'pokemon' or 'mtg'
   * @returns {Promise<Object>} - { sets: [...] }
   */
  async list(query, game) {
    const response = await api.get('/sets', {
      params: { q: query, game }
    })
    return response.data
  },

  /**
   * Get all cards in a specific set
   * @param {string} setCode - Set code (e.g., 'swsh4', 'MH2')
   * @param {string} game - 'pokemon' or 'mtg'
   * @param {string} [nameFilter] - Optional name filter
   * @returns {Promise<Object>} - { cards: [...], total_count, has_more }
   */
  async getCards(setCode, game, nameFilter = '') {
    const params = { game }
    if (nameFilter) params.q = nameFilter
    const response = await api.get(`/sets/${setCode}/cards`, { params })
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

  /**
   * Get collection items grouped by card_id
   * Returns array of grouped items with total quantities, variants, and scans info
   *
   * @param {Object} options - Filter options
   * @param {string} [options.game] - Filter by game ('pokemon' or 'mtg')
   * @param {string} [options.q] - Search by card name or set
   * @param {string} [options.sort] - Sort by ('added_at', 'name', 'price_updated')
   */
  async getGrouped(options = {}) {
    const params = {}
    if (options.game) params.game = options.game
    if (options.q) params.q = options.q
    if (options.sort) params.sort = options.sort

    const response = await api.get('/collection/grouped', { params })
    return response.data
  },

  async add(cardId, options = {}) {
    const payload = {
      card_id: cardId,
      quantity: options.quantity || 1,
      condition: options.condition || 'NM',
      printing: options.printing || 'Normal',
      notes: options.notes || ''
    }
    // Include language if specified (for Japanese/foreign cards with different pricing)
    if (options.language) {
      payload.language = options.language
    }
    const response = await api.post('/collection', payload)
    return response.data
  },

  /**
   * Update a collection item
   * Returns { item, operation, message } where operation is 'updated', 'split', or 'merged'
   * @param {number} id - Collection item ID
   * @param {Object} updates - Fields to update: { quantity, condition, printing, language }
   */
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
  },

  /**
   * Get collection value history for charting
   * @param {string} period - 'week', 'month', '3month', 'year', or 'all'
   */
  async getValueHistory(period = 'month') {
    const response = await api.get('/collection/stats/history', {
      params: { period }
    })
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

export const authService = {
  /**
   * Check if authentication is enabled on the server
   */
  async getStatus() {
    const response = await api.get('/auth/status')
    return response.data
  },

  /**
   * Verify an admin key
   * @param {string} key - The admin key to verify
   */
  async verifyKey(key) {
    const response = await api.post('/auth/verify', null, {
      headers: { Authorization: `Bearer ${key}` }
    })
    return response.data
  }
}

export default api
