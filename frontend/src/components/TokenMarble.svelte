<script>
  let {
    estimatedPrompt = 0,
    actualPrompt = 0,
    outputBudget = 0,
    actualOutput = 0,
    contextSelected = 0,
    contextBucket = 0,
    contextUtilizationPct = 0,
    tokensPerSec = 0
  } = $props()

  // Derive calculations
  const contextSelectedPct = $derived(contextBucket > 0 ? Math.min(contextSelected / contextBucket, 1) : 0)
  const contextUtilPct = $derived(contextUtilizationPct / 100)
  const promptMax = $derived(Math.max(estimatedPrompt, actualPrompt, 1))
  const actualPromptPct = $derived(actualPrompt / promptMax)
  const estimatedPromptPct = $derived(estimatedPrompt / promptMax)
  const outputPct = $derived(outputBudget > 0 ? Math.min(actualOutput / outputBudget, 1) : 0)
  const promptDelta = $derived(actualPrompt - estimatedPrompt)

  function formatNumber(n) {
    if (n == null) return '-'
    return n.toLocaleString()
  }
</script>

<div class="token-marble">
  <div class="marble-header">
    <h3 class="marble-title">TOKENS</h3>
    <div class="throughput-badge">
      {tokensPerSec.toFixed(1)} tok/s
    </div>
  </div>

  <!-- Track 1: Context Bucket -->
  <div class="track">
    <div class="track-header">
      <span class="track-title">CONTEXT BUCKET</span>
    </div>
    <div class="meter-track track-blue">
      <div
        class="meter-fill"
        style="width: {Math.max(contextSelectedPct * 100, contextSelected > 0 ? 0.5 : 0)}%; --fill-pct: {contextSelectedPct * 100}%"
      >
        <div
          class="utilization-overlay"
          style="width: {contextUtilPct * 100}%"
        ></div>
      </div>
    </div>
    <div class="track-text">
      <span class="number">{formatNumber(contextSelected)}</span> selected of <span class="number">{formatNumber(contextBucket)}</span> • Utilization <span class="number">{contextUtilizationPct.toFixed(1)}%</span>
    </div>
  </div>

  <!-- Track 2: Prompt Tokens -->
  <div class="track">
    <div class="track-header">
      <span class="track-title">PROMPT TOKENS</span>
    </div>
    <div class="meter-track track-purple">
      <div
        class="meter-fill"
        style="width: {Math.max(actualPromptPct * 100, actualPrompt > 0 ? 0.5 : 0)}%; --fill-pct: {actualPromptPct * 100}%"
      ></div>
      {#if estimatedPromptPct > actualPromptPct}
        <div
          class="estimate-marker"
          style="left: {estimatedPromptPct * 100}%"
        ></div>
      {/if}
      {#if promptDelta !== 0}
        <div class="delta-badge">
          {promptDelta > 0 ? '+' : ''}{formatNumber(promptDelta)} tok
        </div>
      {/if}
    </div>
    <div class="track-text">
      <span class="number">{formatNumber(actualPrompt)}</span> actual • <span class="number">{formatNumber(estimatedPrompt)}</span> estimated • Δ <span class="number">{promptDelta > 0 ? '+' : ''}{formatNumber(promptDelta)}</span> tok
    </div>
  </div>

  <!-- Track 3: Output Budget -->
  <div class="track">
    <div class="track-header">
      <span class="track-title">OUTPUT BUDGET</span>
    </div>
    <div class="meter-track track-green">
      <div
        class="meter-fill"
        style="width: {Math.max(outputPct * 100, actualOutput > 0 ? 0.5 : 0)}%; --fill-pct: {outputPct * 100}%"
      ></div>
    </div>
    <div class="track-text">
      <span class="number">{formatNumber(actualOutput)}</span> used of <span class="number">{formatNumber(outputBudget)}</span> • <span class="number">{(outputPct * 100).toFixed(1)}%</span>
    </div>
  </div>
</div>

<style>
  .token-marble {
    background: var(--bg-card);

    padding: 24px;
    position: relative;
  }

  .token-marble::before {
    content: '';
    position: absolute;
    top: 0;
    left: 0;
    right: 0;
    height: 1px;
    pointer-events: none;
  }

  .marble-header {
    display: flex;
    justify-content: space-between;
    align-items: center;
    margin-bottom: 24px;
  }

  .marble-title {
    font-size: 12px;
    font-weight: 600;
    text-transform: uppercase;
    letter-spacing: 0.1em;
    color: var(--text-secondary);
    margin: 0;
  }

  .throughput-badge {
    background: var(--bg-secondary);
    border: 1px solid rgba(255, 255, 255, 0.1);
    border-radius: 12px;
    padding: 4px 10px;
    font-size: 11px;
    color: var(--text-muted);
    font-variant-numeric: tabular-nums;
  }

  .track {
    margin-bottom: 20px;
  }

  .track:last-child {
    margin-bottom: 0;
  }

  .track-header {
    display: flex;
    justify-content: space-between;
    align-items: center;
    margin-bottom: 8px;
  }

  .track-title {
    font-size: 10px;
    font-weight: 600;
    text-transform: uppercase;
    letter-spacing: 0.08em;
    color: var(--text-secondary);
  }

  .meter-track {
    position: relative;
    height: 8px;
    background: var(--bg-secondary);
    border-radius: 4px;
  }

  .meter-track::before {
    content: '';
    position: absolute;
    top: 0;
    left: 0;
    right: 0;
    bottom: 0;

    pointer-events: none;
  }

  .meter-fill {
    position: absolute;
    top: 0;
    left: 0;
    height: 100%;
    border-radius: 12px;
    min-width: 6px;
    transition: width 0.3s ease;
  }

  .track-blue .meter-fill {
    background: var(--accent-blue);
    box-shadow: 0 0 8px rgba(59, 130, 246, 0.3);
  }

  .track-purple .meter-fill {
    background: var(--accent-purple);
    box-shadow: 0 0 8px rgba(139, 92, 246, 0.3);
  }

  .track-green .meter-fill {
    background: var(--accent-green);
    box-shadow: 0 0 8px rgba(16, 185, 129, 0.3);
  }

  .utilization-overlay {
    position: absolute;
    bottom: 0;
    left: 0;
    height: 8px;
    background: rgba(255, 255, 255, 0.4);
    border-radius: 4px;
  }

  .estimate-marker {
    position: absolute;
    top: 0;
    bottom: 0;
    width: 0;
    transform: translateX(-50%);
    border-left: 5px solid rgba(0, 255, 112, 0.5);
    pointer-events: none;
    z-index: 2;
    border-radius: 42%;
  }

  .delta-badge {
    position: absolute;
    right: 8px;
    top: 50%;
    transform: translateY(-50%);
    background: var(--bg-secondary);
    border: 1px solid rgba(255, 255, 255, 0.1);
    border-radius: 8px;
    padding: 2px 6px;
    font-size: 10px;
    color: var(--text-muted);
    font-variant-numeric: tabular-nums;
  }

  .track-text {
    margin-top: 8px;
    font-size: 12px;
    color: var(--text-muted);
    line-height: 1.4;
  }

  .track-text .number {
    color: var(--text-primary);
    font-weight: 500;
    font-variant-numeric: tabular-nums;
  }

</style>