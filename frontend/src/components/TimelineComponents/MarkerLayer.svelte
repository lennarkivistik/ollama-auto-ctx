<script>
  let { markers = [], totalMs = 1 } = $props()

  let markerRefs = $state({})

  function getMarkerPosition(marker) {
    const position = (marker.atMs / totalMs) * 100
    // Clamp to prevent bubble overflow (assuming bubble is ~80px wide)
    // Leave 5% margin on each side
    return Math.max(5, Math.min(95, position))
  }

  function getBubbleOffset(marker) {
    const position = (marker.atMs / totalMs) * 100
    // If near left edge, align bubble to left of marker
    if (position < 10) return 'left'
    // If near right edge, align bubble to right of marker
    if (position > 90) return 'right'
    // Otherwise center
    return 'center'
  }

  function formatTime(ms) {
    if (ms >= 1000) {
      return `${(ms / 1000).toFixed(2)}s`
    }
    return `${ms}ms`
  }
</script>

<div class="marker-layer">
  {#each markers as marker, i (marker)}
    {@const position = getMarkerPosition(marker)}
    {@const bubbleAlign = getBubbleOffset(marker)}
    <div
      class="marker"
      style="--marker-position: {position}%; --marker-accent: {marker.accent || 'var(--accent-blue)'}; --bubble-align: {bubbleAlign}"
      role="button"
      tabindex="0"
      aria-label="{marker.label}: {formatTime(marker.atMs)}"
      title="{marker.label}: {formatTime(marker.atMs)}"
    >
      <div class="marker-bubble" data-align={bubbleAlign}>
        <div class="bubble-label-small">{marker.label}</div>
        <div class="bubble-value">{formatTime(marker.atMs)}</div>
      </div>
      <div class="marker-line"></div>
      <div class="marker-pointer"></div>
    </div>
  {/each}
</div>

<style>
  .marker-layer {
    position: absolute;
    top: 0;
    left: 0;
    right: 0;
    height: 100%;
    pointer-events: none;
    z-index: 20;
  }

  .marker {
    position: absolute;
    left: var(--marker-position);
    top: 0;
    bottom: 0;
    width: 0;
    pointer-events: auto;
    cursor: pointer;
    transform: translateX(-50%);
  }

  .marker:focus {
    outline: 2px solid var(--marker-accent);
    outline-offset: 2px;
    border-radius: 2px;
  }

  .marker-bubble {
    position: absolute;
    bottom: calc(100% + 8px);
    left: 50%;
    transform: translateX(calc(var(--bubble-align) === 'left' ? 0% : var(--bubble-align) === 'right' ? -100% : -50%));
    background: var(--marker-accent);
    border-radius: 6px;
    padding: 6px 10px;
    white-space: nowrap;
    box-shadow: 0 2px 8px rgba(0, 0, 0, 0.3),
                0 0 0 1px rgba(255, 255, 255, 0.1) inset,
                0 1px 2px rgba(255, 255, 255, 0.2) inset;
    min-width: 60px;
    text-align: center;
  }

  .bubble-label-small {
    font-size: 9px;
    color: rgba(255, 255, 255, 0.8);
    text-transform: uppercase;
    letter-spacing: 0.5px;
    margin-bottom: 2px;
    font-weight: 500;
  }

  .bubble-value {
    font-size: 13px;
    color: white;
    font-weight: 600;
    font-variant-numeric: tabular-nums;
  }

  .marker-line {
    position: absolute;
    top: 0;
    bottom: 0;
    left: 50%;
    width: 2px;
    background: var(--marker-accent);
    transform: translateX(-50%);
    box-shadow: 0 0 4px var(--marker-accent);
  }

  .marker-pointer {
    position: absolute;
    bottom: 0;
    left: 50%;
    transform: translateX(-50%);
    width: 0;
    height: 0;
    border-left: 6px solid transparent;
    border-right: 6px solid transparent;
    border-bottom: 6px solid var(--marker-accent);
    filter: drop-shadow(0 2px 2px rgba(0, 0, 0, 0.3));
  }
</style>