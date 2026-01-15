/**
 * API client for the AutoCTX backend.
 * All endpoints are under /autoctx/api/v1/
 */

const API_BASE = '/autoctx/api/v1'

/**
 * Fetch overview statistics and time series data.
 * @param {string} window - Time window: '1h', '24h', or '7d'
 * @returns {Promise<{summary: Object, series: Object}>}
 */
export async function fetchOverview(window = '24h') {
  const res = await fetch(`${API_BASE}/overview?window=${window}`)
  if (!res.ok) throw new Error('Failed to fetch overview')
  return res.json()
}

/**
 * Fetch paginated list of requests.
 * @param {Object} options
 * @param {number} options.limit - Max items to return
 * @param {number} options.offset - Offset for pagination
 * @param {string} options.window - Time window
 * @param {string} [options.status] - Filter by status
 * @param {string} [options.model] - Filter by model
 * @returns {Promise<{requests: Array, total: number, limit: number, offset: number}>}
 */
export async function fetchRequests({ limit = 50, offset = 0, window = '24h', status, model } = {}) {
  const params = new URLSearchParams({ limit, offset, window })
  if (status) params.set('status', status)
  if (model) params.set('model', model)
  
  const res = await fetch(`${API_BASE}/requests?${params}`)
  if (!res.ok) throw new Error('Failed to fetch requests')
  return res.json()
}

/**
 * Fetch details for a single request.
 * @param {string} id - Request ID
 * @returns {Promise<Object>}
 */
export async function fetchRequestDetail(id) {
  const res = await fetch(`${API_BASE}/requests/${id}`)
  if (!res.ok) throw new Error('Failed to fetch request detail')
  return res.json()
}

/**
 * Fetch per-model statistics.
 * @param {string} window - Time window
 * @returns {Promise<{models: Array}>}
 */
export async function fetchModels(window = '24h') {
  const res = await fetch(`${API_BASE}/models?window=${window}`)
  if (!res.ok) throw new Error('Failed to fetch models')
  return res.json()
}

/**
 * Fetch current configuration.
 * @returns {Promise<Object>}
 */
export async function fetchConfig() {
  const res = await fetch(`${API_BASE}/config`)
  if (!res.ok) throw new Error('Failed to fetch config')
  return res.json()
}

/**
 * Check upstream health.
 * @returns {Promise<{healthy: boolean, last_check: string, last_error?: string}>}
 */
export async function fetchHealth() {
  const res = await fetch('/healthz/upstream')
  if (!res.ok) return { healthy: false, last_check: new Date().toISOString() }
  return res.json()
}
