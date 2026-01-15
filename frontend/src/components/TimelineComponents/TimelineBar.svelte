<script>
  let { segments = [], totalMs = 1 } = $props()
</script>

<div class="timeline-bar">
  {#each segments as segment (segment)}
    {@const duration = segment.endMs - segment.startMs}
    {@const widthPct = (duration / totalMs) * 100}
    {@const width = Math.max((duration / totalMs) * 100, 0.1)}
    {@const left = (segment.startMs / totalMs) * 100}
    {@const accent = segment.accent || 'var(--accent-blue)'}
    <div
      class="timeline-segment"
      style="width: {width}%; left: {left}%; --segment-accent: {accent}; background-color: {accent};"
      title="{segment.label}: {((segment.endMs - segment.startMs) / 1000).toFixed(2)}s"
    >
      {#if widthPct > 8}
        <span class="segment-label">
          {duration >= 1000 ? `${(duration / 1000).toFixed(2)}s` : `${duration}ms`}
        </span>
      {/if}
    </div>
  {/each}
</div>

<style>
  .timeline-bar {
    position: relative;
    width: 100%;
    height: 48px;
    background: var(--bg-secondary);
    border-radius: 8px;
    overflow: hidden;
    border: 1px solid var(--border-color);
    box-shadow: 0 0 0 1px rgba(255, 255, 255, 0.05) inset,
                0 1px 2px rgba(0, 0, 0, 0.2);
  }

  .timeline-bar::before {
    content: '';
    position: absolute;
    top: 0;
    left: 0;
    right: 0;
    height: 1px;
    background: linear-gradient(
      to right,
      transparent,
      rgba(255, 255, 255, 0.1) 20%,
      rgba(255, 255, 255, 0.1) 80%,
      transparent
    );
    pointer-events: none;
  }

  .timeline-bar::after {
    content: '';
    position: absolute;
    bottom: 0;
    left: 0;
    right: 0;
    height: 1px;
    background: linear-gradient(
      to right,
      transparent,
      rgba(255, 255, 255, 0.1) 20%,
      rgba(255, 255, 255, 0.1) 80%,
      transparent
    );
    pointer-events: none;
  }

  .timeline-segment {
    position: absolute;
    top: 0;
    height: 100%;
    display: flex;
    align-items: center;
    justify-content: center;
    border-right: 1px solid rgba(0, 0, 0, 0.2);
    box-sizing: border-box;
    z-index: 1;
    min-width: 1px;
  }

  .timeline-segment:last-child {
    border-right: none;
  }

  .segment-label {
    color: white;
    font-size: 11px;
    font-weight: 600;
    text-shadow: 0 1px 2px rgba(0, 0, 0, 0.3);
    white-space: nowrap;
    position: relative;
    z-index: 2;
    pointer-events: none;
  }
</style>