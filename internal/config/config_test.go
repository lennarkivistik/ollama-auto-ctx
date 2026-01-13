package config

import (
	"os"
	"testing"
)

func TestModeDefault(t *testing.T) {
	// Clear any existing env vars
	os.Unsetenv("MODE")
	os.Unsetenv("STORAGE")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if cfg.Mode != ModeRetry {
		t.Errorf("Default mode = %v, want %v", cfg.Mode, ModeRetry)
	}
}

func TestModeOff(t *testing.T) {
	os.Setenv("MODE", "off")
	defer os.Unsetenv("MODE")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if cfg.Mode != ModeOff {
		t.Errorf("Mode = %v, want %v", cfg.Mode, ModeOff)
	}

	features := cfg.Features()
	if features.Dashboard {
		t.Error("Features.Dashboard should be false for MODE=off")
	}
	if features.API {
		t.Error("Features.API should be false for MODE=off")
	}
	if features.Storage {
		t.Error("Features.Storage should be false for MODE=off")
	}
}

func TestModeMonitor(t *testing.T) {
	os.Setenv("MODE", "monitor")
	defer os.Unsetenv("MODE")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	features := cfg.Features()
	if !features.Dashboard {
		t.Error("Features.Dashboard should be true for MODE=monitor")
	}
	if !features.API {
		t.Error("Features.API should be true for MODE=monitor")
	}
	if features.Retry {
		t.Error("Features.Retry should be false for MODE=monitor")
	}
	if features.Protect {
		t.Error("Features.Protect should be false for MODE=monitor")
	}
}

func TestModeRetry(t *testing.T) {
	os.Setenv("MODE", "retry")
	defer os.Unsetenv("MODE")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	features := cfg.Features()
	if !features.Dashboard {
		t.Error("Features.Dashboard should be true for MODE=retry")
	}
	if !features.Retry {
		t.Error("Features.Retry should be true for MODE=retry")
	}
	if features.Protect {
		t.Error("Features.Protect should be false for MODE=retry")
	}
}

func TestModeProtect(t *testing.T) {
	os.Setenv("MODE", "protect")
	defer os.Unsetenv("MODE")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	features := cfg.Features()
	if !features.Dashboard {
		t.Error("Features.Dashboard should be true for MODE=protect")
	}
	if !features.Retry {
		t.Error("Features.Retry should be true for MODE=protect")
	}
	if !features.Protect {
		t.Error("Features.Protect should be true for MODE=protect")
	}
}

func TestStorageDefaultSQLite(t *testing.T) {
	os.Setenv("MODE", "retry")
	os.Unsetenv("STORAGE")
	defer os.Unsetenv("MODE")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if cfg.Storage != StorageSQLite {
		t.Errorf("Default storage = %v, want %v", cfg.Storage, StorageSQLite)
	}
}

func TestStorageDefaultOffWhenModeOff(t *testing.T) {
	os.Setenv("MODE", "off")
	os.Unsetenv("STORAGE")
	defer os.Unsetenv("MODE")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if cfg.Storage != StorageOff {
		t.Errorf("Storage for MODE=off = %v, want %v", cfg.Storage, StorageOff)
	}
}

func TestStorageMaxRowsDefault(t *testing.T) {
	os.Unsetenv("STORAGE_MAX_ROWS")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if cfg.StorageMaxRows != 3000 {
		t.Errorf("StorageMaxRows = %v, want 3000", cfg.StorageMaxRows)
	}
}

func TestInvalidModeRejected(t *testing.T) {
	os.Setenv("MODE", "invalid")
	defer os.Unsetenv("MODE")

	_, err := Load()
	if err == nil {
		t.Error("Expected error for invalid MODE")
	}
}

func TestFeaturesMatrix(t *testing.T) {
	tests := []struct {
		mode     Mode
		storage  StorageType
		wantDash bool
		wantAPI  bool
		wantRetry bool
		wantProtect bool
		wantStorage bool
	}{
		{ModeOff, StorageOff, false, false, false, false, false},
		{ModeMonitor, StorageSQLite, true, true, false, false, true},
		{ModeMonitor, StorageMemory, true, true, false, false, true},
		{ModeMonitor, StorageOff, true, true, false, false, false},
		{ModeRetry, StorageSQLite, true, true, true, false, true},
		{ModeProtect, StorageSQLite, true, true, true, true, true},
	}

	for _, tt := range tests {
		t.Run(string(tt.mode)+"/"+string(tt.storage), func(t *testing.T) {
			cfg := Config{Mode: tt.mode, Storage: tt.storage}
			f := cfg.Features()

			if f.Dashboard != tt.wantDash {
				t.Errorf("Dashboard = %v, want %v", f.Dashboard, tt.wantDash)
			}
			if f.API != tt.wantAPI {
				t.Errorf("API = %v, want %v", f.API, tt.wantAPI)
			}
			if f.Retry != tt.wantRetry {
				t.Errorf("Retry = %v, want %v", f.Retry, tt.wantRetry)
			}
			if f.Protect != tt.wantProtect {
				t.Errorf("Protect = %v, want %v", f.Protect, tt.wantProtect)
			}
			if f.Storage != tt.wantStorage {
				t.Errorf("Storage = %v, want %v", f.Storage, tt.wantStorage)
			}
		})
	}
}
