<script>
  import { formatNumber, formatDuration, formatTime, getStatusClass } from '../lib/format.js'

  let { requests, view, loading, onselect } = $props()
</script>

<div class="table-container" class:view-tokens={view === 'tokens'}>
  {#if loading}
    <div class="loading">Loading requests...</div>
  {:else if requests.length === 0}
    <div class="empty-state">No requests yet</div>
  {:else}
    <table>
      <thead>
        <tr>
          <th>Time</th>
          <th>Model</th>
          <th class="col-timing">Duration</th>
          <th class="col-timing">TTFB</th>
          <th class="col-timing">Bucket</th>
          <th class="col-tokens">Est. Tokens</th>
          <th class="col-tokens">Output</th>
          <th class="col-tokens">Bucket</th>
          <th class="col-tokens">Duration</th>
          <th>Retries</th>
          <th>Status</th>
        </tr>
      </thead>
      <tbody>
        {#each requests as req}
          <tr onclick={() => onselect(req)}>
            <td>{formatTime(req.ts)}</td>
            <td>{req.model || '-'}</td>
            <td class="col-timing">{formatDuration(req.duration_ms)}</td>
            <td class="col-timing">{formatDuration(req.ttfb_ms || 0)}</td>
            <td class="col-timing">{formatNumber(req.ctx_bucket || 0)}</td>
            <td class="col-tokens">{formatNumber(req.ctx_est || 0)}</td>
            <td class="col-tokens">{formatNumber(req.completion_tokens || 0)}</td>
            <td class="col-tokens">{formatNumber(req.ctx_bucket || 0)}</td>
            <td class="col-tokens">{formatDuration(req.duration_ms)}</td>
            <td>{req.retry_count || 0}</td>
            <td>
              <span class="status-badge {getStatusClass(req.status, req.reason)}">
                {req.reason || req.status}
              </span>
            </td>
          </tr>
        {/each}
      </tbody>
    </table>
  {/if}
</div>

<style>
  .table-container {
    overflow-x: auto;
  }

  table {
    width: 100%;
    border-collapse: collapse;
  }

  thead {
    background: var(--bg-secondary);
  }

  th {
    padding: 12px 16px;
    text-align: left;
    font-size: 12px;
    text-transform: uppercase;
    letter-spacing: 0.5px;
    color: var(--text-secondary);
    font-weight: 600;
    border-bottom: 1px solid var(--border-color);
  }

  td {
    padding: 12px 16px;
    border-bottom: 1px solid var(--border-color);
    font-size: 14px;
  }

  tbody tr {
    cursor: pointer;
    transition: background 0.2s;
  }

  tbody tr:hover {
    background: var(--bg-hover);
  }

  .col-timing {
    display: table-cell;
  }

  .col-tokens {
    display: none;
  }

  .view-tokens .col-timing {
    display: none;
  }

  .view-tokens .col-tokens {
    display: table-cell;
  }

  .status-badge {
    display: inline-flex;
    align-items: center;
    gap: 6px;
    padding: 4px 10px;
    border-radius: 4px;
    font-size: 11px;
    font-weight: 600;
    text-transform: uppercase;
  }

  .status-badge::before {
    content: '';
    width: 6px;
    height: 6px;
    border-radius: 50%;
  }

  :global(.status-success) {
    background: rgba(16, 185, 129, 0.15);
    color: var(--accent-green);
  }

  :global(.status-success)::before {
    background: var(--accent-green);
  }

  :global(.status-error) {
    background: rgba(239, 68, 68, 0.15);
    color: var(--accent-red);
  }

  :global(.status-error)::before {
    background: var(--accent-red);
  }

  :global(.status-timeout) {
    background: rgba(245, 158, 11, 0.15);
    color: var(--accent-yellow);
  }

  :global(.status-timeout)::before {
    background: var(--accent-yellow);
  }

  :global(.status-inflight) {
    background: rgba(59, 130, 246, 0.15);
    color: var(--accent-blue);
  }

  :global(.status-inflight)::before {
    background: var(--accent-blue);
    animation: pulse 1.5s infinite;
  }

  @keyframes pulse {
    0%, 100% { opacity: 1; }
    50% { opacity: 0.4; }
  }

  .loading, .empty-state {
    text-align: center;
    padding: 48px 24px;
    color: var(--text-muted);
  }

  .loading::after {
    content: '';
    display: inline-block;
    width: 16px;
    height: 16px;
    border: 2px solid var(--border-color);
    border-radius: 50%;
    border-top-color: var(--accent-blue);
    animation: spin 1s linear infinite;
    margin-left: 8px;
  }

  @keyframes spin {
    to { transform: rotate(360deg); }
  }
</style>
