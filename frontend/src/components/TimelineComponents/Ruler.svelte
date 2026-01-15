<script>
  import { onMount } from 'svelte'

  let { totalMs = 1, customMarkers = [] } = $props()

  let rulerRef = $state(null)
  let visibleLabels = $state(new Set())

  // Calculate "nice" major step
  function calculateMajorStep(totalMs) {
    if (totalMs <= 0) {
      return { majorStep: 100, minorStep: 20 }
    }
    
    const targetTicks = 8 // Target 6-10, use 8 as middle
    const rawStep = totalMs / targetTicks
    
    if (rawStep <= 0) {
      return { majorStep: 100, minorStep: 20 }
    }
    
    // Find the order of magnitude
    const magnitude = Math.pow(10, Math.floor(Math.log10(rawStep)))
    
    // Normalize to 1-10 range
    const normalized = rawStep / magnitude
    
    // Snap to 1, 2, 2.5, or 5
    let niceNormalized
    if (normalized <= 1) niceNormalized = 1
    else if (normalized <= 2) niceNormalized = 2
    else if (normalized <= 2.5) niceNormalized = 2.5
    else if (normalized <= 5) niceNormalized = 5
    else niceNormalized = 10
    
    const majorStep = niceNormalized * magnitude
    
    // Calculate minor step (typically 1/5 of major)
    const minorStep = majorStep / 5
    
    return { majorStep, minorStep }
  }

  function formatLabel(ms) {
    if (ms >= 1000) {
      return `${(ms / 1000).toFixed(2)}s`
    }
    return `${ms}ms`
  }

  // Calculate step sizes
  const stepSizes = $derived(calculateMajorStep(totalMs))
  const majorStepMs = $derived(stepSizes.majorStep)
  const minorStepMs = $derived(stepSizes.minorStep)

  // Generate all ticks
  const ticks = $derived.by(() => {
    if (totalMs <= 0 || minorStepMs <= 0) return []
    const result = []
    for (let ms = 0; ms <= totalMs + minorStepMs / 2; ms += minorStepMs) {
      const isMajor = Math.abs(ms % majorStepMs) < minorStepMs / 2
      result.push({ ms: Math.min(ms, totalMs), isMajor })
    }
    return result
  })

  // Get indices of major ticks
  const majorTickIndices = $derived(ticks.map((tick, i) => tick.isMajor ? i : -1).filter(i => i >= 0))

  // Prevent label overlaps by measuring and hiding colliding labels
  function updateVisibleLabels() {
    if (!rulerRef) return
    
    const labelElements = Array.from(rulerRef.querySelectorAll('.ruler-label'))
    if (labelElements.length === 0) {
      visibleLabels = new Set()
      return
    }
    
    const visible = new Set()
    let lastRight = -Infinity
    const minGap = 6 // Minimum gap between labels in pixels
    
    // Map label elements to their tick indices
    labelElements.forEach((el, labelIndex) => {
      const tickIndex = majorTickIndices[labelIndex]
      if (tickIndex === undefined) return
      
      const rect = el.getBoundingClientRect()
      const left = rect.left
      const right = rect.right
      
      if (left - lastRight >= minGap) {
        visible.add(tickIndex)
        lastRight = right
      }
    })
    
    visibleLabels = visible
  }

  onMount(() => {
    updateVisibleLabels()
  })

  $effect(() => {
    if (rulerRef && ticks.length > 0) {
      // Use setTimeout to ensure DOM is updated after reactive statements
      setTimeout(() => {
        updateVisibleLabels()
      }, 0)
    }
  })
</script>

<div class="ruler" bind:this={rulerRef}>
  {#each ticks as tick, i (tick)}
    {@const position = (tick.ms / totalMs) * 100}
    {@const tolerance = Math.max(minorStepMs / 3, 5)}
    {@const isCustomMarker = customMarkers.some(m => Math.abs(m - tick.ms) < tolerance)}
    <div
      class="ruler-tick"
      class:major={tick.isMajor}
      class:custom={isCustomMarker}
      style="left: {position}%"
    >
      {#if (tick.isMajor && visibleLabels.has(i)) || isCustomMarker}
        <div class="ruler-label" class:custom-label={isCustomMarker}>{formatLabel(tick.ms)}</div>
      {/if}
    </div>
  {/each}
  
  <!-- Render custom markers that don't align with regular ticks -->
  {#each customMarkers as markerMs (markerMs)}
    {@const position = (markerMs / totalMs) * 100}
    {@const tolerance = Math.max(minorStepMs / 3, 5)}
    {@const isNearTick = ticks.some(t => Math.abs(t.ms - markerMs) < tolerance)}
    {#if !isNearTick && markerMs >= 0 && markerMs <= totalMs}
      <div
        class="ruler-tick custom"
        style="left: {position}%"
      >
        <div class="ruler-label custom-label">{formatLabel(markerMs)}</div>
      </div>
    {/if}
  {/each}
</div>

<style>
  .ruler {
    position: relative;
    width: 100%;
    height: 40px;
    margin-top: 12px;
  }

  .ruler::before {
    content: '';
    position: absolute;
    top: 0;
    left: 0;
    right: 0;
    height: 1px;
    background: linear-gradient(
      to right,
      transparent,
      rgba(255, 255, 255, 0.15) 10%,
      rgba(255, 255, 255, 0.15) 90%,
      transparent
    );
  }

  .ruler-tick {
    position: absolute;
    top: 0;
    width: 0;
    height: 7px;
    border-left: 1px solid rgba(203, 213, 225, 0.4);
    opacity: 1;
    transition: opacity 0.2s;
  }

  .ruler-tick.major {
    height: 11px;
    border-left: 1.5px solid rgba(203, 213, 225, 0.6);
    opacity: 1;
  }

  .ruler-tick.custom {
    height: 13px;
    border-left: 2px solid rgba(59, 130, 246, 0.7);
    opacity: 1;
    z-index: 2;
  }

  .ruler-label {
    position: absolute;
    top: 16px;
    left: 50%;
    transform: translateX(-50%);
    font-size: 11px;
    color: rgba(203, 213, 225, 0.9);
    white-space: nowrap;
    font-variant-numeric: tabular-nums;
    font-weight: 500;
    text-shadow: 0 1px 2px rgba(0, 0, 0, 0.3);
    letter-spacing: 0.02em;
  }

  .ruler-label.custom-label {
    font-size: 6px;
    margin-top: 12px;
    font-weight: 600;
    color: rgba(147, 197, 253, 1);
    text-shadow: 0 1px 3px rgba(0, 0, 0, 0.5);
  }
</style>