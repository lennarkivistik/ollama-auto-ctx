/**
 * Formatting utilities for the dashboard.
 */

/**
 * Format a number with locale separators.
 * @param {number|null|undefined} n
 * @returns {string}
 */
export function formatNumber(n) {
  if (n == null) return '-'
  return n.toLocaleString()
}

/**
 * Format duration in milliseconds to human-readable string.
 * @param {number|null|undefined} ms
 * @returns {string}
 */
export function formatDuration(ms) {
  if (ms == null || ms === 0) return '-'
  if (ms < 1000) return `${ms}ms`
  if (ms < 60000) return `${(ms / 1000).toFixed(2)}s`
  return `${(ms / 60000).toFixed(1)}m`
}

/**
 * Format bytes to human-readable string.
 * @param {number|null|undefined} bytes
 * @returns {string}
 */
export function formatBytes(bytes) {
  if (bytes == null || bytes === 0) return '-'
  if (bytes < 1024) return `${bytes} B`
  if (bytes < 1048576) return `${(bytes / 1024).toFixed(2)} KB`
  return `${(bytes / 1048576).toFixed(2)} MB`
}

/**
 * Format timestamp to locale time string.
 * @param {number|string|null|undefined} ts - Unix timestamp (ms) or ISO string
 * @returns {string}
 */
export function formatTime(ts) {
  if (!ts) return '-'
  const d = new Date(ts)
  if (isNaN(d)) return '-'
  // Combines extraction and formatting in one go for efficiency
  return `${d.getFullYear()}-${(d.getMonth()+1).toString().padStart(2, '0')}-${d.getDate().toString().padStart(2, '0')} ${d.getHours().toString().padStart(2, '0')}:${d.getMinutes().toString().padStart(2, '0')}:${d.getSeconds().toString().padStart(2, '0')}`
}


/**
 * Get CSS class for request status.
 * @param {string} status
 * @param {string} [reason]
 * @returns {string}
 */
export function getStatusClass(status, reason) {
  if (status === 'success') return 'status-success'
  if (status === 'in_flight') return 'status-inflight'
  if (reason && reason.includes('timeout')) return 'status-timeout'
  return 'status-error'
}
