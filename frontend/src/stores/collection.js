import { defineStore } from 'pinia'
import { collectionService, cardService } from '../services/api'

export const useCollectionStore = defineStore('collection', {
  state: () => ({
    items: [],
    stats: null,
    loading: false,
    error: null,
    searchResults: [],
    searchLoading: false
  }),

  getters: {
    totalCards: (state) => state.stats?.total_cards || 0,
    totalValue: (state) => state.stats?.total_value || 0,
    mtgCards: (state) => state.items.filter(i => i.card.game === 'mtg'),
    pokemonCards: (state) => state.items.filter(i => i.card.game === 'pokemon')
  },

  actions: {
    async fetchCollection(game = null) {
      this.loading = true
      this.error = null
      try {
        this.items = await collectionService.getAll(game)
      } catch (err) {
        this.error = err.message
      } finally {
        this.loading = false
      }
    },

    async fetchStats() {
      try {
        this.stats = await collectionService.getStats()
      } catch (err) {
        console.error('Failed to fetch stats:', err)
      }
    },

    async searchCards(query, game) {
      this.searchLoading = true
      try {
        const result = await cardService.search(query, game)
        this.searchResults = result.cards
        return result
      } catch (err) {
        this.error = err.message
        return { cards: [], total_count: 0 }
      } finally {
        this.searchLoading = false
      }
    },

    async addToCollection(cardId, options = {}) {
      try {
        const item = await collectionService.add(cardId, options)
        await this.fetchCollection()
        await this.fetchStats()
        return item
      } catch (err) {
        this.error = err.message
        throw err
      }
    },

    async updateItem(id, updates) {
      try {
        await collectionService.update(id, updates)
        await this.fetchCollection()
        await this.fetchStats()
      } catch (err) {
        this.error = err.message
        throw err
      }
    },

    async removeItem(id) {
      try {
        await collectionService.remove(id)
        await this.fetchCollection()
        await this.fetchStats()
      } catch (err) {
        this.error = err.message
        throw err
      }
    },

    async refreshPrices() {
      try {
        await collectionService.refreshPrices()
        await this.fetchCollection()
        await this.fetchStats()
      } catch (err) {
        this.error = err.message
        throw err
      }
    },

    clearSearch() {
      this.searchResults = []
    }
  }
})
