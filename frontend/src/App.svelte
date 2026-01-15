<script>
  import { onMount } from 'svelte'
  import { fetchOverview, fetchRequests, fetchHealth } from './lib/api.js'
  import { formatNumber, formatDuration, formatBytes, formatTime, getStatusClass } from './lib/format.js'
  import SummaryCard from './components/SummaryCard.svelte'
  import RequestsTable from './components/RequestsTable.svelte'
  import RequestModal from './components/RequestModal.svelte'
  import Sparkline from './components/Sparkline.svelte'

  // State
  let currentWindow = $state('24h')
  let currentPage = $state(1)
  let currentView = $state('timing')
  const pageSize = 20

  let overviewData = $state(null)
  let requestsData = $state([])
  let health = $state({ healthy: true })
  let selectedRequest = $state(null)
  let loading = $state(true)

  // Computed
  let totalPages = $derived(Math.ceil(requestsData.length / pageSize))
  let pagedRequests = $derived(requestsData.slice((currentPage - 1) * pageSize, currentPage * pageSize))

  // Fetch data
  async function loadOverview() {
    try {
      overviewData = await fetchOverview(currentWindow)
    } catch (err) {
      console.error('Overview error:', err)
    }
  }

  async function loadRequests() {
    try {
      const data = await fetchRequests({ limit: 200, window: currentWindow })
      requestsData = data.requests || []
      loading = false
    } catch (err) {
      console.error('Requests error:', err)
      loading = false
    }
  }

  async function loadHealth() {
    try {
      health = await fetchHealth()
    } catch (err) {
      health = { healthy: false }
    }
  }

  function changeWindow(w) {
    currentWindow = w
    currentPage = 1
    loadOverview()
    loadRequests()
  }

  onMount(() => {
    loadOverview()
    loadRequests()
    loadHealth()

    // Polling intervals
    const overviewInterval = setInterval(loadOverview, 5000)
    const requestsInterval = setInterval(loadRequests, 3000)
    const healthInterval = setInterval(loadHealth, 10000)

    return () => {
      clearInterval(overviewInterval)
      clearInterval(requestsInterval)
      clearInterval(healthInterval)
    }
  })
</script>

<div class="container">
  <header>
    <h1>Ollama <span>AutoCTX</span></h1>
    <div class="header-right">
      <div class="window-selector">
        <button
          class="window-btn"
          class:active={currentWindow === '5m'}
          onclick={() => changeWindow('5m')}
        >5m</button>
        <button
          class="window-btn"
          class:active={currentWindow === '1h'}
          onclick={() => changeWindow('1h')}
        >1H</button>
        <button
          class="window-btn"
          class:active={currentWindow === '24h'}
          onclick={() => changeWindow('24h')}
        >24H</button>
        <button
          class="window-btn"
          class:active={currentWindow === '7d'}
          onclick={() => changeWindow('7d')}
        >7D</button>
      </div>
      <div class="health-badge">
        <div class="health-dot" class:unhealthy={!health.healthy}></div>
        <span>{health.healthy ? 'Healthy' : 'Unhealthy'}</span>
      </div>
    </div>
  </header>

  <!-- Summary Cards -->
  {#if overviewData}
    <div class="summary-grid">
      <SummaryCard
        label="Total Requests"
        value={formatNumber(overviewData.summary.total_requests)}
        subtext="{overviewData.summary.in_flight || 0} in-flight"
      >
        <Sparkline data={overviewData.series?.req_count} color="--accent-blue" />
      </SummaryCard>

      <SummaryCard
        label="Success Rate"
        value="{(overviewData.summary.success_rate * 100).toFixed(1)}%"
        subtext="{formatNumber(Math.round(overviewData.summary.total_requests * overviewData.summary.success_rate))} / {formatNumber(overviewData.summary.total_requests)}"
      >
        <Sparkline data={overviewData.series?.ctx_utilization} color="--accent-green" />
      </SummaryCard>

      <SummaryCard
        label="Avg Duration"
        value={formatDuration(overviewData.summary.avg_duration_ms)}
        subtext="P95: {formatDuration(overviewData.summary.p95_duration_ms)}"
      >
        <Sparkline data={overviewData.series?.duration_p95} color="--accent-purple" />
      </SummaryCard>

      <SummaryCard
        label="Total Tokens"
        value={formatNumber(overviewData.summary.total_tokens)}
        subtext="{formatBytes(overviewData.summary.total_bytes)} transferred"
      >
        <Sparkline data={overviewData.series?.gen_tok_per_s} color="--accent-green" />
      </SummaryCard>
    </div>


  <!-- Recent Requests -->
  <div class="card">
    <div class="card-header">
      <div>
        <div class="card-title">Recent Requests</div>
        <div class="card-subtitle">Click a row for details</div>
      </div>
      <div class="view-toggle">
        <button
          class="view-toggle-btn"
          class:active={currentView === 'timing'}
          onclick={() => currentView = 'timing'}
        >Timing</button>
        <button
          class="view-toggle-btn"
          class:active={currentView === 'tokens'}
          onclick={() => currentView = 'tokens'}
        >Tokens</button>
      </div>
    </div>

    <RequestsTable
      requests={pagedRequests}
      view={currentView}
      {loading}
      onselect={(req) => selectedRequest = req}
    />

    {#if totalPages > 1}
      <div class="pagination">
        <button
          class="pagination-btn"
          disabled={currentPage <= 1}
          onclick={() => currentPage--}
        >Previous</button>
        <span class="page-info">Page {currentPage} of {totalPages}</span>
        <button
          class="pagination-btn"
          disabled={currentPage >= totalPages}
          onclick={() => currentPage++}
        >Next</button>
      </div>
    {/if}
  </div>

    <!-- Prometheus Metrics Card -->
    <div class="metrics-card">
      <div class="metrics-card-title">Prometheus Metrics</div>
      <div class="metrics-card-subtitle">Aggregated statistics and performance indicators</div>
      <div class="metrics-grid">
        <div class="metric-item">
          <div class="metric-label">Total Requests</div>
          <div class="metric-value">{formatNumber(overviewData.summary.total_requests)}</div>
          <div class="metric-subtitle">All time</div>
        </div>
        <div class="metric-item">
          <div class="metric-label">Success Rate</div>
          <div class="metric-value">{(overviewData.summary.success_rate * 100).toFixed(1)}%</div>
          <div class="metric-subtitle">{Math.round(overviewData.summary.total_requests * overviewData.summary.success_rate)} / {overviewData.summary.total_requests}</div>
        </div>
        <div class="metric-item">
          <div class="metric-label">Avg Duration</div>
          <div class="metric-value">{formatDuration(overviewData.summary.avg_duration_ms)}</div>
          <div class="metric-subtitle">Request time</div>
        </div>
        <div class="metric-item">
          <div class="metric-label">Total Bytes</div>
          <div class="metric-value">{formatBytes(overviewData.summary.total_bytes)}</div>
          <div class="metric-subtitle">Forwarded</div>
        </div>
        <div class="metric-item">
          <div class="metric-label">Total Tokens</div>
          <div class="metric-value">{formatNumber(overviewData.summary.total_tokens)}</div>
          <div class="metric-subtitle">Estimated</div>
        </div>
        <div class="metric-item">
          <div class="metric-label">Timeouts</div>
          <div class="metric-value">{formatNumber(overviewData.summary.timeouts)}</div>
          <div class="metric-subtitle">All types</div>
        </div>
      </div>
      <div class="metrics-status-row">
        <div class="status-item">
          <div class="status-dot success"></div>
          <span>Success</span>
          <strong>{formatNumber(Math.round(overviewData.summary.total_requests * overviewData.summary.success_rate))}</strong>
        </div>
        <div class="status-item">
          <div class="status-dot error"></div>
          <span>Errors</span>
          <strong>{formatNumber(overviewData.summary.total_requests - Math.round(overviewData.summary.total_requests * overviewData.summary.success_rate) - overviewData.summary.timeouts)}</strong>
        </div>
        <div class="status-item">
          <div class="status-dot timeout"></div>
          <span>Timeouts</span>
          <strong>{formatNumber(overviewData.summary.timeouts)}</strong>
        </div>
        <div class="status-item">
          <div class="status-dot loop"></div>
          <span>Loops Detected</span>
          <strong>{formatNumber(overviewData.summary.loops || 0)}</strong>
        </div>
      </div>
    </div>
  {/if}
</div>

{#if selectedRequest}
  <RequestModal
    requestId={selectedRequest.id}
    onclose={() => selectedRequest = null}
  />
{/if}

<style>
  :global(:root) {
    --bg-primary: #1e293b;
    --bg-secondary: #0f172a;
    --bg-card: #1e293b;
    --bg-hover: #334155;
    --border-color: #334155;
    --text-primary: #f1f5f9;
    --text-secondary: #cbd5e1;
    --text-muted: #94a3b8;
    --accent-blue: #3b82f6;
    --accent-green: #10b981;
    --accent-red: #ef4444;
    --accent-yellow: #f59e0b;
    --accent-orange: #f97316;
    --accent-purple: #8b5cf6;
  }

  :global(*) {
    margin: 0;
    padding: 0;
    box-sizing: border-box;
  }

  :global(body) {
    font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, 'Helvetica Neue', Arial, sans-serif;
    background: var(--bg-secondary);
    color: var(--text-primary);
    line-height: 1.6;
  }

  .container {
    max-width: 1400px;
    margin: 0 auto;
    padding: 24px;
  }

  header {
    background: var(--bg-card);
    border: 1px solid var(--border-color);
    padding: 20px 24px;
    border-radius: 8px;
    margin-bottom: 24px;
    display: flex;
    justify-content: space-between;
    align-items: center;
  }

  h1 {
    color: var(--text-primary);
    font-size: 24px;
    font-weight: 600;
  }

  h1 span {
    color: var(--accent-blue);
  }

  .header-right {
    display: flex;
    align-items: center;
    gap: 16px;
  }

  .window-selector {
    display: flex;
    gap: 4px;
    background: var(--bg-secondary);
    padding: 4px;
    border-radius: 6px;
  }

  .window-btn {
    padding: 6px 12px;
    border: none;
    background: transparent;
    color: var(--text-secondary);
    border-radius: 4px;
    cursor: pointer;
    font-size: 13px;
    transition: all 0.2s;
  }

  .window-btn:hover {
    color: var(--text-primary);
  }

  .window-btn.active {
    background: var(--accent-blue);
    color: white;
  }

  .health-badge {
    display: flex;
    align-items: center;
    gap: 8px;
    padding: 6px 12px;
    background: var(--bg-secondary);
    border: 1px solid var(--border-color);
    border-radius: 4px;
    font-size: 14px;
  }

  .health-dot {
    width: 8px;
    height: 8px;
    border-radius: 50%;
    background: var(--accent-green);
  }

  .health-dot.unhealthy {
    background: var(--accent-red);
  }

  .summary-grid {
    display: grid;
    grid-template-columns: repeat(auto-fit, minmax(280px, 1fr));
    gap: 16px;
    margin-bottom: 24px;
  }

  .metrics-card {
    background: var(--bg-card);
    border: 1px solid var(--border-color);
    padding: 24px;
    border-radius: 8px;
    margin-bottom: 24px;
  }

  .metrics-card-title {
    font-size: 18px;
    font-weight: 600;
    margin-bottom: 4px;
  }

  .metrics-card-subtitle {
    font-size: 14px;
    color: var(--text-muted);
    margin-bottom: 20px;
  }

  .metrics-grid {
    display: grid;
    grid-template-columns: repeat(auto-fit, minmax(200px, 1fr));
    gap: 24px;
    margin-bottom: 20px;
  }

  .metric-item {
    padding: 16px;
    background: var(--bg-secondary);
    border-radius: 8px;
  }

  .metric-label {
    font-size: 12px;
    text-transform: uppercase;
    letter-spacing: 0.5px;
    color: var(--text-muted);
    margin-bottom: 8px;
  }

  .metric-value {
    font-size: 24px;
    font-weight: 600;
    color: var(--text-primary);
    font-variant-numeric: tabular-nums;
  }

  .metric-subtitle {
    font-size: 12px;
    color: var(--text-muted);
    margin-top: 4px;
  }

  .metrics-status-row {
    display: flex;
    gap: 24px;
    flex-wrap: wrap;
    padding-top: 16px;
    border-top: 1px solid var(--border-color);
  }

  .status-item {
    display: flex;
    align-items: center;
    gap: 8px;
    font-size: 14px;
  }

  .status-dot {
    width: 10px;
    height: 10px;
    border-radius: 50%;
  }

  .status-dot.success {
    background: var(--accent-green);
  }

  .status-dot.error {
    background: var(--accent-red);
  }

  .status-dot.timeout {
    background: var(--accent-yellow);
  }

  .status-dot.loop {
    background: var(--accent-orange);
  }

  .card {
    background: var(--bg-card);
    border: 1px solid var(--border-color);
    border-radius: 8px;
    padding: 24px;
    margin-bottom: 24px;
  }

  .card-header {
    display: flex;
    justify-content: space-between;
    align-items: center;
    margin-bottom: 20px;
  }

  .card-title {
    font-size: 18px;
    font-weight: 600;
  }

  .card-subtitle {
    font-size: 14px;
    color: var(--text-muted);
    margin-top: 4px;
  }

  .view-toggle {
    display: flex;
    gap: 4px;
    background: var(--bg-secondary);
    padding: 4px;
    border-radius: 6px;
  }

  .view-toggle-btn {
    padding: 6px 12px;
    border: none;
    background: transparent;
    color: var(--text-secondary);
    border-radius: 4px;
    cursor: pointer;
    font-size: 13px;
    transition: all 0.2s;
  }

  .view-toggle-btn:hover {
    color: var(--text-primary);
  }

  .view-toggle-btn.active {
    background: var(--accent-blue);
    color: white;
  }

  .pagination {
    display: flex;
    justify-content: center;
    align-items: center;
    gap: 16px;
    margin-top: 16px;
    padding-top: 16px;
    border-top: 1px solid var(--border-color);
  }

  .pagination-btn {
    padding: 8px 16px;
    background: var(--bg-secondary);
    border: 1px solid var(--border-color);
    color: var(--text-primary);
    border-radius: 4px;
    cursor: pointer;
    font-size: 14px;
  }

  .pagination-btn:hover:not(:disabled) {
    background: var(--bg-hover);
  }

  .pagination-btn:disabled {
    opacity: 0.5;
    cursor: not-allowed;
  }

  .page-info {
    color: var(--text-secondary);
    font-size: 14px;
  }

  @media (max-width: 768px) {
    .container {
      padding: 16px;
    }

    header {
      flex-direction: column;
      gap: 16px;
    }

    .summary-grid {
      grid-template-columns: 1fr;
    }
  }
</style>
