<script>
  let { label, value, subtext, children } = $props()
</script>

<div class="summary-card">
  <div class="summary-card-header">
    <div>
      <div class="summary-label">{label}</div>
      <div class="summary-value">{value}</div>
      <div class="summary-subtext">{subtext}</div>
    </div>
  </div>
  <div class="sparkline-container">
    {@render children?.()}
  </div>
</div>

<style>
  .summary-card {
    background: var(--bg-card);
    border: 1px solid var(--border-color);
    padding: 20px;
    border-radius: 8px;
    position: relative;
  }

  .summary-card-header {
    display: flex;
    justify-content: space-between;
    align-items: flex-start;
    margin-bottom: 8px;
  }

  .summary-label {
    color: var(--text-secondary);
    font-size: 12px;
    text-transform: uppercase;
    letter-spacing: 0.5px;
    font-weight: 600;
  }

  .summary-value {
    font-size: 28px;
    font-weight: 600;
    color: var(--text-primary);
    font-variant-numeric: tabular-nums;
  }

  .summary-subtext {
    color: var(--text-muted);
    font-size: 13px;
    margin-top: 4px;
  }

  .sparkline-container {
    height: 40px;
    margin-top: 12px;
  }
</style>
