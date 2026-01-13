package storage

import (
	"testing"
	"time"
)

func TestGetBinConfig(t *testing.T) {
	tests := []struct {
		window       time.Duration
		wantBins     int
		wantInterval time.Duration
	}{
		{30 * time.Minute, 60, time.Minute},
		{time.Hour, 60, time.Minute},
		{2 * time.Hour, 96, 15 * time.Minute},
		{24 * time.Hour, 96, 15 * time.Minute},
		{48 * time.Hour, 168, time.Hour},
		{7 * 24 * time.Hour, 168, time.Hour},
	}

	for _, tt := range tests {
		t.Run(tt.window.String(), func(t *testing.T) {
			bins, interval := GetBinConfig(tt.window)
			if bins != tt.wantBins {
				t.Errorf("bins = %d, want %d", bins, tt.wantBins)
			}
			if interval != tt.wantInterval {
				t.Errorf("interval = %v, want %v", interval, tt.wantInterval)
			}
		})
	}
}
