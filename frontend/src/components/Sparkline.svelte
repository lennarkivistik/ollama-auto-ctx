<script>
  let { data, color = '--accent-blue' } = $props()

  let points = $derived.by(() => {
    if (!data || data.length === 0) return ''
    
    const values = data.map(d => d.value)
    const max = Math.max(...values, 1)
    const min = Math.min(...values, 0)
    const range = max - min || 1

    const width = 100
    const height = 30
    
    return values.map((v, i) => {
      const x = (i / (values.length - 1)) * width
      const y = height - ((v - min) / range) * height
      return `${x},${y}`
    }).join(' ')
  })

  let areaPoints = $derived(points ? `${points} 100,30 0,30` : '')
</script>

{#if data && data.length > 0}
  <svg class="sparkline" viewBox="0 0 100 30" preserveAspectRatio="none">
    <defs>
      <linearGradient id="sparklineGradient-{color}" x1="0%" y1="0%" x2="0%" y2="100%">
        <stop offset="0%" style="stop-color: var({color}); stop-opacity: 0.4" />
        <stop offset="100%" style="stop-color: var({color}); stop-opacity: 0" />
      </linearGradient>
    </defs>
    <polygon class="sparkline-area" points={areaPoints} fill="url(#sparklineGradient-{color})" />
    <polyline class="sparkline-line" points={points} style="stroke: var({color})" />
  </svg>
{/if}

<style>
  .sparkline {
    width: 100%;
    height: 100%;
  }

  .sparkline-line {
    fill: none;
    stroke-width: 1.5;
    stroke-linecap: round;
    stroke-linejoin: round;
  }

  .sparkline-area {
    opacity: 0.3;
  }
</style>
