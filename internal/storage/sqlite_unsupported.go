//go:build !cgo || mips64 || mips64le || ppc64 || s390x

package storage

import (
	"errors"
	"log/slog"
	"time"
)

// SQLiteStore implements Store using SQLite with WAL mode.
// This is a stub implementation for unsupported platforms.
type SQLiteStore struct{}

// NewSQLiteStore creates a new SQLite store at the given path.
// On unsupported platforms, this returns an error.
func NewSQLiteStore(path string, maxRows int, logger *slog.Logger) (*SQLiteStore, error) {
	return nil, errors.New("SQLite storage is not supported on this platform, use memory storage instead")
}

// Insert creates a new request record.
func (s *SQLiteStore) Insert(req *Request) error {
	return errors.New("SQLite storage not available")
}

// Update modifies an existing request.
func (s *SQLiteStore) Update(id string, upd RequestUpdate) error {
	return errors.New("SQLite storage not available")
}

// GetByID retrieves a single request.
func (s *SQLiteStore) GetByID(id string) (*Request, error) {
	return nil, errors.New("SQLite storage not available")
}

// List retrieves requests with filtering.
func (s *SQLiteStore) List(opts ListOptions) ([]Request, error) {
	return nil, errors.New("SQLite storage not available")
}

// Overview returns aggregate statistics.
func (s *SQLiteStore) Overview(window time.Duration) (*Overview, error) {
	return nil, errors.New("SQLite storage not available")
}

// ModelStats returns per-model statistics.
func (s *SQLiteStore) ModelStats(window time.Duration) ([]ModelStat, error) {
	return nil, errors.New("SQLite storage not available")
}

// Series returns time-binned data for charts.
func (s *SQLiteStore) Series(opts SeriesOptions) ([]DataPoint, error) {
	return nil, errors.New("SQLite storage not available")
}

// InFlightCount returns the number of in-flight requests.
func (s *SQLiteStore) InFlightCount() (int, error) {
	return 0, errors.New("SQLite storage not available")
}

// Close closes the database connection.
func (s *SQLiteStore) Close() error {
	return nil
}