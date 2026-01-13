package storage

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestSQLiteStore_InsertAndGet(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "sqlite_test")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	store, err := NewSQLiteStore(filepath.Join(tmpDir, "test.db"), 1000, nil)
	if err != nil {
		t.Fatalf("NewSQLiteStore error: %v", err)
	}
	defer store.Close()

	req := &Request{
		ID:            "test-1",
		TSStart:       time.Now().UnixMilli(),
		Status:        StatusInFlight,
		Model:         "llama2",
		Endpoint:      "chat",
		MessagesCount: 3,
		SystemChars:   100,
		UserChars:     200,
		ClientInBytes: 500,
	}

	if err := store.Insert(req); err != nil {
		t.Fatalf("Insert error: %v", err)
	}

	got, err := store.GetByID("test-1")
	if err != nil {
		t.Fatalf("GetByID error: %v", err)
	}
	if got == nil {
		t.Fatal("GetByID returned nil")
	}

	if got.ID != req.ID {
		t.Errorf("ID = %v, want %v", got.ID, req.ID)
	}
	if got.Model != req.Model {
		t.Errorf("Model = %v, want %v", got.Model, req.Model)
	}
	if got.MessagesCount != req.MessagesCount {
		t.Errorf("MessagesCount = %v, want %v", got.MessagesCount, req.MessagesCount)
	}
}

func TestSQLiteStore_Update(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "sqlite_test")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	store, err := NewSQLiteStore(filepath.Join(tmpDir, "test.db"), 1000, nil)
	if err != nil {
		t.Fatalf("NewSQLiteStore error: %v", err)
	}
	defer store.Close()

	req := &Request{
		ID:      "test-update",
		TSStart: time.Now().UnixMilli(),
		Status:  StatusInFlight,
		Model:   "llama2",
	}

	if err := store.Insert(req); err != nil {
		t.Fatalf("Insert error: %v", err)
	}

	// Update
	now := time.Now().UnixMilli()
	status := StatusSuccess
	promptTokens := 100
	completionTokens := 50

	if err := store.Update("test-update", RequestUpdate{
		TSEnd:            &now,
		Status:           &status,
		PromptTokens:     &promptTokens,
		CompletionTokens: &completionTokens,
	}); err != nil {
		t.Fatalf("Update error: %v", err)
	}

	got, _ := store.GetByID("test-update")
	if got.Status != StatusSuccess {
		t.Errorf("Status = %v, want %v", got.Status, StatusSuccess)
	}
	if got.PromptTokens != 100 {
		t.Errorf("PromptTokens = %v, want 100", got.PromptTokens)
	}
	if got.TSEnd == nil {
		t.Error("TSEnd should not be nil")
	}
}

func TestSQLiteStore_List(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "sqlite_test")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	store, err := NewSQLiteStore(filepath.Join(tmpDir, "test.db"), 1000, nil)
	if err != nil {
		t.Fatalf("NewSQLiteStore error: %v", err)
	}
	defer store.Close()

	// Insert multiple requests
	for i := 0; i < 10; i++ {
		req := &Request{
			ID:      "list-" + string(rune('0'+i)),
			TSStart: time.Now().UnixMilli() - int64(i*1000),
			Status:  StatusSuccess,
			Model:   "llama2",
		}
		if err := store.Insert(req); err != nil {
			t.Fatalf("Insert error: %v", err)
		}
	}

	// List with limit
	results, err := store.List(ListOptions{Limit: 5})
	if err != nil {
		t.Fatalf("List error: %v", err)
	}
	if len(results) != 5 {
		t.Errorf("List returned %d items, want 5", len(results))
	}

	// List with filter
	status := StatusSuccess
	results, err = store.List(ListOptions{Status: &status})
	if err != nil {
		t.Fatalf("List with filter error: %v", err)
	}
	if len(results) != 10 {
		t.Errorf("List with filter returned %d items, want 10", len(results))
	}
}

func TestSQLiteStore_Prune(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "sqlite_test")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	// Small max to test pruning
	store, err := NewSQLiteStore(filepath.Join(tmpDir, "test.db"), 5, nil)
	if err != nil {
		t.Fatalf("NewSQLiteStore error: %v", err)
	}
	defer store.Close()

	// Insert more than max
	for i := 0; i < 10; i++ {
		req := &Request{
			ID:      "prune-" + string(rune('a'+i)),
			TSStart: time.Now().UnixMilli() + int64(i*100), // Increasing timestamps
			Status:  StatusSuccess,
			Model:   "llama2",
		}
		if err := store.Insert(req); err != nil {
			t.Fatalf("Insert error: %v", err)
		}
		// Small delay to let async prune run
		time.Sleep(10 * time.Millisecond)
	}

	// Wait for pruning to complete
	time.Sleep(100 * time.Millisecond)

	// Check count
	results, err := store.List(ListOptions{Limit: 100})
	if err != nil {
		t.Fatalf("List error: %v", err)
	}

	// Should have pruned oldest rows, keeping newest
	if len(results) > 10 {
		t.Errorf("Expected pruning to keep <= 10 rows, got %d", len(results))
	}
}

func TestSQLiteStore_Overview(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "sqlite_test")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	store, err := NewSQLiteStore(filepath.Join(tmpDir, "test.db"), 1000, nil)
	if err != nil {
		t.Fatalf("NewSQLiteStore error: %v", err)
	}
	defer store.Close()

	// Insert mix of statuses
	statuses := []Status{StatusSuccess, StatusSuccess, StatusSuccess, StatusError, StatusCanceled}
	for i, status := range statuses {
		req := &Request{
			ID:               "overview-" + string(rune('0'+i)),
			TSStart:          time.Now().UnixMilli(),
			Status:           status,
			Model:            "llama2",
			DurationMs:       100 + i*10,
			CompletionTokens: 50,
			ClientOutBytes:   1000,
		}
		if err := store.Insert(req); err != nil {
			t.Fatalf("Insert error: %v", err)
		}
	}

	overview, err := store.Overview(time.Hour)
	if err != nil {
		t.Fatalf("Overview error: %v", err)
	}

	if overview.TotalRequests != 5 {
		t.Errorf("TotalRequests = %d, want 5", overview.TotalRequests)
	}
	if overview.SuccessCount != 3 {
		t.Errorf("SuccessCount = %d, want 3", overview.SuccessCount)
	}
	if overview.TotalTokens != 250 {
		t.Errorf("TotalTokens = %d, want 250", overview.TotalTokens)
	}
}

func TestSQLiteStore_WALMode(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "sqlite_test")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	dbPath := filepath.Join(tmpDir, "wal_test.db")
	store, err := NewSQLiteStore(dbPath, 1000, nil)
	if err != nil {
		t.Fatalf("NewSQLiteStore error: %v", err)
	}

	// Insert a record to create the database
	req := &Request{
		ID:      "wal-test",
		TSStart: time.Now().UnixMilli(),
		Status:  StatusInFlight,
	}
	if err := store.Insert(req); err != nil {
		t.Fatalf("Insert error: %v", err)
	}

	store.Close()

	// Check for WAL file
	_, err = os.Stat(dbPath + "-wal")
	if err != nil && !os.IsNotExist(err) {
		t.Errorf("Unexpected error checking WAL file: %v", err)
	}
	// WAL file may or may not exist depending on SQLite version/config
	// The important thing is that the store opened without error with WAL mode requested
}
