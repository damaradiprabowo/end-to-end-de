package watermark

import (
	"path/filepath"
	"testing"
	"time"
)

func TestGetReturnsEpochWhenMissing(t *testing.T) {
	s, _ := Open(filepath.Join(t.TempDir(), "wm.json"))
	if got := s.Get("tenant_01", "customers"); !got.Equal(Epoch) {
		t.Fatalf("expected epoch, got %v", got)
	}
}

func TestSetGetFlushRoundTrip(t *testing.T) {
	path := filepath.Join(t.TempDir(), "wm.json")
	s, _ := Open(path)
	ts := time.Date(2024, 3, 1, 10, 0, 0, 0, time.UTC)
	s.Set("tenant_02", "transactions", ts)
	if err := s.Flush(); err != nil {
		t.Fatalf("flush: %v", err)
	}

	reopened, err := Open(path)
	if err != nil {
		t.Fatalf("reopen: %v", err)
	}
	got := reopened.Get("tenant_02", "transactions")
	if !got.Equal(ts) {
		t.Fatalf("expected %v, got %v", ts, got)
	}
}

func TestKeysAreTenantScoped(t *testing.T) {
	s, _ := Open(filepath.Join(t.TempDir(), "wm.json"))
	a := time.Now().UTC().Truncate(time.Second)
	s.Set("tenant_01", "customers", a)
	if !s.Get("tenant_02", "customers").Equal(Epoch) {
		t.Fatal("tenant_02 should be isolated from tenant_01")
	}
}
