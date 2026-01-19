/**
 * Format a price value as USD currency string.
 * @param {number|null|undefined} price - The price to format
 * @param {string} fallback - What to return if price is falsy (default: '-')
 * @returns {string} Formatted price string
 */
export function formatPrice(price, fallback = '-') {
  if (!price && price !== 0) return fallback
  return `$${price.toFixed(2)}`
}

/**
 * Format a date string as relative time (e.g., "5m ago", "2h ago").
 * @param {string|null|undefined} dateString - The date to format
 * @returns {string|null} Formatted relative time or null if no date
 */
export function formatTimeAgo(dateString) {
  if (!dateString) return null
  const date = new Date(dateString)
  const now = new Date()
  const seconds = Math.floor((now - date) / 1000)

  if (seconds < 60) return 'just now'
  if (seconds < 3600) return `${Math.floor(seconds / 60)}m ago`
  if (seconds < 86400) return `${Math.floor(seconds / 3600)}h ago`
  return `${Math.floor(seconds / 86400)}d ago`
}

/**
 * Check if a card's price is considered stale (older than 24 hours).
 * @param {object} cardOrItem - Card object or collection item with card property
 * @returns {boolean} True if price is stale or missing
 */
export function isPriceStale(cardOrItem) {
  const card = cardOrItem?.card || cardOrItem
  if (!card?.price_updated_at) return true
  const date = new Date(card.price_updated_at)
  const now = new Date()
  return (now - date) > 24 * 60 * 60 * 1000 // 24 hours
}

/**
 * Check if a printing type uses foil pricing.
 * Matches backend logic in card_price.go PrintingType.IsFoilVariant()
 * @param {string} printing - The printing type (Normal, Foil, 1st Edition, etc.)
 * @returns {boolean} True if this printing type should use foil pricing
 */
export function isFoilPrinting(printing) {
  return ['Foil', '1st Edition', 'Reverse Holofoil'].includes(printing)
}

/**
 * Calculate the value of a collection item (respecting printing type and quantity).
 * @param {object} item - Collection item with card, quantity, printing properties
 * @returns {number} Total value
 */
export function getItemValue(item) {
  const card = item?.card || item
  const quantity = item?.quantity || 1
  const useFoilPrice = isFoilPrinting(item?.printing)
  if (useFoilPrice && card?.price_foil_usd) {
    return card.price_foil_usd * quantity
  }
  return (card?.price_usd || 0) * quantity
}
