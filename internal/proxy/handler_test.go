package proxy

import (
	"testing"

	"ollama-auto-ctx/internal/config"
)

func TestChooseFinalCtx(t *testing.T) {
	desired := 8192
	hardMax := 16384

	// No user value: always set.
	ctx, override, clamped := chooseFinalCtx(desired, hardMax, 0, false, config.OverrideIfTooSmall)
	if ctx != desired || !override || clamped {
		t.Fatalf("expected ctx=%d override=true clamped=false, got ctx=%d override=%v clamped=%v", desired, ctx, override, clamped)
	}

	// User ctx smaller, policy if_too_small -> increase.
	ctx, override, clamped = chooseFinalCtx(desired, hardMax, 4096, true, config.OverrideIfTooSmall)
	if ctx != desired || !override || clamped {
		t.Fatalf("expected ctx=%d override=true clamped=false, got ctx=%d override=%v clamped=%v", desired, ctx, override, clamped)
	}

	// User ctx larger, policy if_too_small -> keep user ctx.
	ctx, override, clamped = chooseFinalCtx(desired, hardMax, 12288, true, config.OverrideIfTooSmall)
	if ctx != 12288 || override || clamped {
		t.Fatalf("expected ctx=12288 override=false clamped=false, got ctx=%d override=%v clamped=%v", ctx, override, clamped)
	}

	// User ctx larger than hardMax -> clamp down.
	ctx, override, clamped = chooseFinalCtx(desired, 8192, 16384, true, config.OverrideIfTooSmall)
	if ctx != 8192 || !override || !clamped {
		t.Fatalf("expected ctx=8192 override=true clamped=true, got ctx=%d override=%v clamped=%v", ctx, override, clamped)
	}

	// Policy always overrides user ctx.
	ctx, override, clamped = chooseFinalCtx(desired, hardMax, 4096, true, config.OverrideAlways)
	if ctx != desired || !override || clamped {
		t.Fatalf("expected ctx=%d override=true clamped=false, got ctx=%d override=%v clamped=%v", desired, ctx, override, clamped)
	}

	// Policy if_missing leaves user ctx unchanged.
	ctx, override, clamped = chooseFinalCtx(desired, hardMax, 4096, true, config.OverrideIfMissing)
	if ctx != 4096 || override || clamped {
		t.Fatalf("expected ctx=4096 override=false clamped=false, got ctx=%d override=%v clamped=%v", ctx, override, clamped)
	}
}
