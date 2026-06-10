// Package watermark provides a simple file-based high-watermark store for
// incremental loads. State is persisted as JSON: { "tenant|table": "RFC3339" }.
package watermark

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// Epoch is used when no watermark exists yet (first run -> full load).
var Epoch = time.Date(1970, 1, 1, 0, 0, 0, 0, time.UTC)

// Store is a concurrency-safe watermark store backed by a JSON file.
type Store struct {
	mu    sync.Mutex
	path  string
	state map[string]string
}

func key(tenant, table string) string { return tenant + "|" + table }

// Open loads an existing watermark file or starts empty.
func Open(path string) (*Store, error) {
	s := &Store{path: path, state: map[string]string{}}
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return s, nil
		}
		return nil, err
	}
	if len(data) > 0 {
		if err := json.Unmarshal(data, &s.state); err != nil {
			return nil, err
		}
	}
	return s, nil
}

// Get returns the high-watermark for a tenant/table, or Epoch if none.
func (s *Store) Get(tenant, table string) time.Time {
	s.mu.Lock()
	defer s.mu.Unlock()
	v, ok := s.state[key(tenant, table)]
	if !ok {
		return Epoch
	}
	t, err := time.Parse(time.RFC3339, v)
	if err != nil {
		return Epoch
	}
	return t
}

// Set updates the in-memory watermark (call Flush to persist).
func (s *Store) Set(tenant, table string, ts time.Time) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.state[key(tenant, table)] = ts.UTC().Format(time.RFC3339)
}

// Flush atomically writes the watermark state to disk.
func (s *Store) Flush() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if dir := filepath.Dir(s.path); dir != "" {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return err
		}
	}
	data, err := json.MarshalIndent(s.state, "", "  ")
	if err != nil {
		return err
	}
	tmp := s.path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, s.path)
}
