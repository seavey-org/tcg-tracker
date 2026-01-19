import { defineStore } from 'pinia'
import { collectionService, cardService } from '../services/api'

export const useCollectionStore = defineStore('collection', {
  state: () => ({
    items: [],           // Raw collection items (for compatibility)
    groupedItems: [],    // Collection items grouped by card_id
    stats: null,
    valueHistory: null,  // Historical value snapshots for charting
    valueHistoryPeriod: 'month',
    loading: false,
    error: null,
    searchResults: [],
    searchLoading: false,
    actionLoading: false,
    lastUpdateResult: null  // Stores last update operation info (split/merge/updated)
  }),

  getters: {
    totalCards: (state) => state.stats?.total_cards || 0,
    totalValue: (state) => state.stats?.total_value || 0,
    mtgCards: (state) => state.items.filter(i => i.card?.game === 'mtg'),
    pokemonCards: (state) => state.items.filter(i => i.card?.game === 'pokemon'),
    // Grouped getters
    mtgGrouped: (state) => state.groupedItems.filter(g => g.card?.game === 'mtg'),
    pokemonGrouped: (state) => state.groupedItems.filter(g => g.card?.game === 'pokemon')
  },

  actions: {
    clearError() {
      this.error = null
    },

    clearLastUpdateResult() {
      this.lastUpdateResult = null
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

    /**
     * Fetch collection grouped by card_id
     * This is the primary method for displaying collection
     */
    async fetchGroupedCollection(game = null) {
      this.loading = true
      this.error = null
      try {
        this.groupedItems = await collectionService.getGrouped(game)
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
        this.error = 'Failed to load collection statistics'
      }
    },

    async fetchValueHistory(period = 'month') {
      try {
        this.valueHistoryPeriod = period
        this.valueHistory = await collectionService.getValueHistory(period)
      } catch (err) {
        console.error('Failed to fetch value history:', err)
        // Don't set error - this is optional data
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
        // Refresh grouped collection to reflect changes
        await this.fetchGroupedCollection()
        this.fetchStats()
        return item
      } catch (err) {
        this.error = err.message
        throw err
      } finally {
        this.actionLoading = false
      }
    },

    /**
     * Update a collection item
     * The backend may split or merge items depending on the changes
     * Returns { item, operation, message }
     */
    async updateItem(id, updates) {
      this.actionLoading = true
      this.error = null
      this.lastUpdateResult = null
      try {
        const result = await collectionService.update(id, updates)
        // Store the operation result for UI feedback
        this.lastUpdateResult = {
          operation: result.operation,
          message: result.message,
          item: result.item
        }
        // Refresh grouped collection to reflect changes (splits, merges, etc.)
        await this.fetchGroupedCollection()
        this.fetchStats()
        return result
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
      try {
        await collectionService.remove(id)
        // Refresh grouped collection
        await this.fetchGroupedCollection()
        this.fetchStats()
      } catch (err) {
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
        await this.fetchGroupedCollection()
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
