import axios from 'axios'

const api = axios.create({
  baseURL: import.meta.env.VITE_API_URL || 'http://localhost:8080/api',
  timeout: 10000,
})

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
