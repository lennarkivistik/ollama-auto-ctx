<script>
  import { onMount } from 'svelte'
  import { fetchRequestDetail } from '../lib/api.js'
  import { formatNumber, formatDuration, formatBytes } from '../lib/format.js'
  import TimelineBar from './TimelineComponents/TimelineBar.svelte'
  import MarkerLayer from './TimelineComponents/MarkerLayer.svelte'
  import Ruler from './TimelineComponents/Ruler.svelte'
  import TokenMarble from './TokenMarble.svelte'

  let { requestId, onclose } = $props()

  let detail = $state(null)
  let activeTab = $state('overview')
  let loading = $state(true)

  onMount(async () => {
    try {
      detail = await fetchRequestDetail(requestId)
    } catch (err) {
      console.error('Failed to load request detail:', err)
    } finally {
      loading = false
    }
  })

  function handleBackdropClick(e) {
    if (e.target === e.currentTarget) onclose()
  }

  function handleKeydown(e) {
    if (e.key === 'Escape') onclose()
  }
</script>

<div
  class="modal"
  onclick={handleBackdropClick}
  onkeydown={handleKeydown}
  role="dialog"
  aria-modal="true"
  tabindex="-1"
>
  <div class="modal-content">
    <div class="modal-header">
      <h2>Request Details</h2>
      <button class="modal-close" onclick={onclose}>&times;</button>
    </div>
    
    <div class="modal-tabs">
      <button class="modal-tab" class:active={activeTab === 'overview'} onclick={() => activeTab = 'overview'}>Overview</button>
      <button class="modal-tab" class:active={activeTab === 'timings'} onclick={() => activeTab = 'timings'}>Timings</button>
      <button class="modal-tab" class:active={activeTab === 'tokens'} onclick={() => activeTab = 'tokens'}>Tokens</button>
      <button class="modal-tab" class:active={activeTab === 'bytes'} onclick={() => activeTab = 'bytes'}>Bytes</button>
    </div>

    <div class="modal-body">
      {#if loading}
        <div class="loading">Loading...</div>
      {:else if detail}
        {#if activeTab === 'overview'}
          <div class="flow-diagram">
            <div class="flow-box">
              <div class="flow-box-title">Request</div>
              <div class="flow-metric"><span class="flow-metric-label">Messages</span><span class="flow-metric-value">{detail.request.messages_count}</span></div>
              <div class="flow-metric"><span class="flow-metric-label">System</span><span class="flow-metric-value">{formatNumber(detail.request.system_chars)} chars</span></div>
              <div class="flow-metric"><span class="flow-metric-label">User</span><span class="flow-metric-value">{formatNumber(detail.request.user_chars)} chars</span></div>
              <div class="flow-metric"><span class="flow-metric-label">Tools</span><span class="flow-metric-value">{detail.request.tools_count}</span></div>
            </div>
            <div class="flow-arrow">→</div>
            <div class="flow-box">
              <div class="flow-box-title">AutoCTX</div>
              <div class="flow-metric"><span class="flow-metric-label">Estimated</span><span class="flow-metric-value">{formatNumber(detail.autoctx.ctx_est)}</span></div>
              <div class="flow-metric"><span class="flow-metric-label">Selected</span><span class="flow-metric-value">{formatNumber(detail.autoctx.ctx_selected)}</span></div>
              <div class="flow-metric"><span class="flow-metric-label">Bucket</span><span class="flow-metric-value">{formatNumber(detail.autoctx.ctx_bucket)}</span></div>
              <div class="flow-metric"><span class="flow-metric-label">Output Budget</span><span class="flow-metric-value">{formatNumber(detail.autoctx.output_budget)}</span></div>
            </div>
            <div class="flow-arrow">→</div>
            <div class="flow-box">
              <div class="flow-box-title">Ollama</div>
              <div class="flow-metric"><span class="flow-metric-label">Prompt Tokens</span><span class="flow-metric-value">{formatNumber(detail.ollama.prompt_tokens)}</span></div>
              <div class="flow-metric"><span class="flow-metric-label">Completion</span><span class="flow-metric-value">{formatNumber(detail.ollama.completion_tokens)}</span></div>
              <div class="flow-metric"><span class="flow-metric-label">Eval Time</span><span class="flow-metric-value">{formatDuration(detail.ollama.upstream_eval_ms)}</span></div>
            </div>
            <div class="flow-arrow">→</div>
            <div class="flow-box">
              <div class="flow-box-title">Response</div>
              <div class="flow-metric"><span class="flow-metric-label">Status</span><span class="flow-metric-value">{detail.status}</span></div>
              <div class="flow-metric"><span class="flow-metric-label">Duration</span><span class="flow-metric-value">{formatDuration(detail.response.duration_ms)}</span></div>
              <div class="flow-metric"><span class="flow-metric-label">TTFB</span><span class="flow-metric-value">{formatDuration(detail.response.ttfb_ms)}</span></div>
              <div class="flow-metric"><span class="flow-metric-label">Retries</span><span class="flow-metric-value">{detail.response.retry_count}</span></div>
            </div>
          </div>
        {:else if activeTab === 'timings'}
          {@const total = detail.response.duration_ms || 1}
          {@const ttfb = detail.response.ttfb_ms || 0}
          {@const load = detail.ollama.upstream_load_ms || 0}
          {@const promptEval = detail.ollama.upstream_prompt_eval_ms || 0}
          {@const evalMs = detail.ollama.upstream_eval_ms || 0}

          {@const segments = (() => {
            const segs = []
            let current = 0
            
            // Model Load segment (first - happens before TTFB)
            if (load > 0 && current < total) {
              const endMs = Math.min(current + load, total)
              if (endMs > current) {
                segs.push({
                  startMs: current,
                  endMs: endMs,
                  label: 'MODEL LOAD',
                  accent: 'var(--accent-orange)'
                })
                current = endMs
              }
            }
            
            // Prompt Eval segment (second - happens before TTFB)
            if (promptEval > 0 && current < total) {
              const endMs = Math.min(current + promptEval, total)
              if (endMs > current) {
                segs.push({
                  startMs: current,
                  endMs: endMs,
                  label: 'PROMPT EVAL',
                  accent: 'var(--accent-purple)'
                })
                current = endMs
              }
            }
            
            // Generation segment (starts at TTFB - when first byte arrives)
            // TTFB is a marker point, not a segment - it marks when first byte arrives
            // Generation can only start after the first byte is received
            const generationStart = ttfb > 0 ? ttfb : current
            if (generationStart < total) {
              segs.push({
                startMs: generationStart,
                endMs: total,
                label: 'GENERATION',
                accent: 'var(--accent-green)'
              })
            }
            
            return segs
          })()}

          {@const markers = (() => {
            const marks = []
            if (ttfb > 0 && ttfb <= total) {
              marks.push({
                atMs: ttfb,
                label: 'TIME TO FIRST BYTE',
                accent: 'var(--accent-blue)'
              })
            }
            return marks
          })()}

          {@const rulerMarkers = (() => {
            const markerTimes = [0] // Always include 0s
            let current = 0
            
            // Add end of Model Load
            if (load > 0) {
              current += load
              markerTimes.push(current)
            }
            
            // Add end of Prompt Eval
            if (promptEval > 0) {
              current += promptEval
              markerTimes.push(current)
            }
            
            // Add TTFB if it's different from current position
            if (ttfb > 0 && Math.abs(ttfb - current) > 1) {
              markerTimes.push(ttfb)
            }
            
            // Add total (end)
            if (total > 0) {
              markerTimes.push(total)
            }
            
            // Remove duplicates and sort
            return [...new Set(markerTimes)].sort((a, b) => a - b)
          })()}

          <div class="timing-cards">
            <div class="timing-card">
              <div class="timing-card-label">TTFB</div>
              <div class="timing-card-value" style="color: var(--accent-blue)">{formatDuration(ttfb)}</div>
            </div>
            <div class="timing-card">
              <div class="timing-card-label">Model Load</div>
              <div class="timing-card-value" style="color: var(--accent-orange)">{formatDuration(load)}</div>
            </div>
            <div class="timing-card">
              <div class="timing-card-label">Prompt Eval</div>
              <div class="timing-card-value" style="color: var(--accent-purple)">{formatDuration(promptEval)}</div>
            </div>
            <div class="timing-card">
              <div class="timing-card-label">Generation</div>
              <div class="timing-card-value" style="color: var(--accent-green)">{formatDuration(evalMs)}</div>
            </div>
          </div>

          <div class="timeline-section">
            <div class="timeline-header">
              <span>Timeline</span>
              <span>Total: {formatDuration(total)}</span>
            </div>
            <div class="timeline-container">
              <TimelineBar {segments} totalMs={total} />
              <MarkerLayer markers={markers} totalMs={total} />
            </div>
            <Ruler totalMs={total} customMarkers={rulerMarkers} />
          </div>
        {:else if activeTab === 'tokens'}
          {@const utilization = detail.autoctx.ctx_selected > 0 
            ? ((detail.ollama.prompt_tokens + detail.ollama.completion_tokens) / detail.autoctx.ctx_selected * 100)
            : 0}
          {@const tokPerSec = detail.ollama.upstream_eval_ms > 0 
            ? (detail.ollama.completion_tokens / detail.ollama.upstream_eval_ms * 1000)
            : 0}

          <TokenMarble
            estimatedPrompt={detail.autoctx.ctx_est || 0}
            actualPrompt={detail.ollama.prompt_tokens || 0}
            outputBudget={detail.autoctx.output_budget || 0}
            actualOutput={detail.ollama.completion_tokens || 0}
            contextSelected={detail.autoctx.ctx_selected || 0}
            contextBucket={detail.autoctx.ctx_bucket || 0}
            contextUtilizationPct={utilization}
            tokensPerSec={tokPerSec}
          />
        {:else if activeTab === 'bytes'}
          <div class="detail-grid">
            <div class="detail-item"><div class="detail-label">Client Request</div><div class="detail-value">{formatBytes(detail.request.client_in_bytes)}</div></div>
            <div class="detail-item"><div class="detail-label">Upstream Request</div><div class="detail-value">{formatBytes(detail.ollama.upstream_in_bytes)}</div></div>
            <div class="detail-item"><div class="detail-label">Upstream Response</div><div class="detail-value">{formatBytes(detail.ollama.upstream_out_bytes)}</div></div>
            <div class="detail-item"><div class="detail-label">Total Bytes</div><div class="detail-value">{formatBytes(detail.response.client_out_bytes || detail.ollama.upstream_out_bytes || 0)}</div></div>
          </div>
        {/if}
      {/if}
    </div>
  </div>
</div>

<style>
  .modal {
    position: fixed;
    top: 0;
    left: 0;
    width: 100%;
    height: 100%;
    background: rgba(0, 0, 0, 0.7);
    display: flex;
    justify-content: center;
    align-items: center;
    z-index: 1000;
  }

  .modal-content {
    background: var(--bg-card);
    border: 1px solid var(--border-color);
    border-radius: 8px;
    max-width: 900px;
    width: 95%;
    max-height: 90vh;
    overflow: hidden;
    display: flex;
    flex-direction: column;
  }

  .modal-header {
    display: flex;
    justify-content: space-between;
    align-items: center;
    padding: 20px 24px;
    border-bottom: 1px solid var(--border-color);
  }

  .modal-header h2 {
    font-size: 18px;
    margin: 0;
  }

  .modal-close {
    background: none;
    border: none;
    color: var(--text-secondary);
    font-size: 24px;
    cursor: pointer;
    padding: 4px 8px;
    border-radius: 4px;
  }

  .modal-close:hover {
    background: var(--bg-hover);
    color: var(--text-primary);
  }

  .modal-tabs {
    display: flex;
    border-bottom: 1px solid var(--border-color);
    background: var(--bg-secondary);
  }

  .modal-tab {
    padding: 12px 20px;
    border: none;
    background: transparent;
    color: var(--text-secondary);
    font-size: 14px;
    cursor: pointer;
    border-bottom: 2px solid transparent;
    margin-bottom: -1px;
  }

  .modal-tab:hover {
    color: var(--text-primary);
  }

  .modal-tab.active {
    color: var(--accent-blue);
    border-bottom-color: var(--accent-blue);
  }

  .modal-body {
    padding: 24px;
    overflow-y: auto;
    flex: 1;
  }

  .loading {
    text-align: center;
    padding: 48px;
    color: var(--text-muted);
  }

  /* Flow diagram */
  .flow-diagram {
    display: flex;
    align-items: center;
    justify-content: space-between;
    padding: 24px 0;
    gap: 16px;
  }

  .flow-box {
    flex: 1;
    background: var(--bg-secondary);
    border: 1px solid var(--border-color);
    border-radius: 8px;
    padding: 16px;
    text-align: center;
    height: 220px;
  }

  .flow-box-title {
    font-size: 12px;
    text-transform: uppercase;
    letter-spacing: 0.5px;
    color: var(--text-secondary);
    margin-bottom: 12px;
  }

  .flow-arrow {
    color: var(--text-muted);
    font-size: 40px;
    font-weight: bolder;
  }

  .flow-metric {
    display: flex;
    justify-content: space-between;
    padding: 6px 0;
    font-size: 13px;
  }

  .flow-metric-label {
    color: var(--text-secondary);
  }

  .flow-metric-value {
    color: var(--text-primary);
    font-weight: 500;
    font-variant-numeric: tabular-nums;
  }

  /* Timing cards */
  .timing-cards {
    display: flex;
    justify-content: space-around;
    gap: 16px;
    margin-bottom: 32px;
  }

  .timing-card {
    flex: 1;
    text-align: center;
    padding: 16px;
    background: var(--bg-secondary);
    border-radius: 8px;
    border: 1px solid var(--border-color);
  }

  .timing-card-label {
    font-size: 11px;
    color: var(--text-muted);
    text-transform: uppercase;
    margin-bottom: 8px;
    letter-spacing: 0.5px;
  }

  .timing-card-value {
    font-size: 24px;
    font-weight: 600;
  }

  /* Timeline */
  .timeline-section {
    margin-top: 32px;
  }

  .timeline-header {
    display: flex;
    justify-content: space-between;
    margin-bottom: 12px;
    font-weight: 600;
    font-size: 14px;
  }

  .timeline-container {
    position: relative;
    width: 100%;
  }

  /* Detail grid */
  .detail-grid {
    display: grid;
    grid-template-columns: repeat(auto-fit, minmax(200px, 1fr));
    gap: 16px;
  }

  .detail-item {
    padding: 12px;
    background: var(--bg-secondary);
    border-radius: 6px;
  }

  .detail-label {
    font-size: 11px;
    text-transform: uppercase;
    letter-spacing: 0.5px;
    color: var(--text-muted);
    margin-bottom: 4px;
  }

  .detail-value {
    font-size: 16px;
    font-weight: 500;
    color: var(--text-primary);
    font-variant-numeric: tabular-nums;
  }

  @media (max-width: 768px) {
    .flow-diagram {
      flex-direction: column;
    }

    .flow-arrow {
      transform: rotate(90deg);
    }

    .timing-cards {
      flex-direction: column;
    }
  }
</style>
