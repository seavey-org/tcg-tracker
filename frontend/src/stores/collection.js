import { defineStore } from 'pinia'
import { collectionService, cardService } from '../services/api'

export const useCollectionStore = defineStore('collection', {
  state: () => ({
    items: [],
    stats: null,
    loading: false,
    error: null,
    searchResults: [],
    searchLoading: false,
    actionLoading: false
  }),

  getters: {
    totalCards: (state) => state.stats?.total_cards || 0,
    totalValue: (state) => state.stats?.total_value || 0,
    mtgCards: (state) => state.items.filter(i => i.card?.game === 'mtg'),
    pokemonCards: (state) => state.items.filter(i => i.card?.game === 'pokemon')
  },

  actions: {
    clearError() {
      this.error = null
    },

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
      this.error = null
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
      this.actionLoading = true
      this.error = null
      try {
        const item = await collectionService.add(cardId, options)
        // Optimistic update: add to local state immediately
        this.items.unshift(item)
        // Fetch stats in background (don't await)
        this.fetchStats()
        return item
      } catch (err) {
        this.error = err.message
        throw err
      } finally {
        this.actionLoading = false
      }
    },

    async updateItem(id, updates) {
      this.actionLoading = true
      this.error = null
      try {
        const updatedItem = await collectionService.update(id, updates)
        // Optimistic update: update in local state
        const index = this.items.findIndex(i => i.id === id)
        if (index !== -1) {
          this.items[index] = updatedItem
        }
        // Fetch stats in background (don't await)
        this.fetchStats()
        return updatedItem
      } catch (err) {
        this.error = err.message
        throw err
      } finally {
        this.actionLoading = false
      }
    },

    async removeItem(id) {
      this.actionLoading = true
      this.error = null
      // Optimistic update: remove from local state immediately
      const removedIndex = this.items.findIndex(i => i.id === id)
      const removedItem = removedIndex !== -1 ? this.items[removedIndex] : null
      if (removedIndex !== -1) {
        this.items.splice(removedIndex, 1)
      }
      try {
        await collectionService.remove(id)
        // Fetch stats in background (don't await)
        this.fetchStats()
      } catch (err) {
        // Rollback on error
        if (removedItem && removedIndex !== -1) {
          this.items.splice(removedIndex, 0, removedItem)
        }
        this.error = err.message
        throw err
      } finally {
        this.actionLoading = false
      }
    },

    async refreshPrices() {
      this.actionLoading = true
      this.error = null
      try {
        const result = await collectionService.refreshPrices()
        // Refetch collection to get updated prices
        await this.fetchCollection()
        await this.fetchStats()
        return result
      } catch (err) {
        this.error = err.message
        throw err
      } finally {
        this.actionLoading = false
      }
    },

    clearSearch() {
      this.searchResults = []
    }
  }
})
