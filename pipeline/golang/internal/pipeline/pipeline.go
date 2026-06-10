// Package pipeline orchestrates the multi-tenant incremental ELT.
// Each tenant is processed in its own goroutine; sync.WaitGroup synchronizes
// completion and a buffered error channel collects per-tenant failures.
package pipeline

import (
	"context"
	"log"
	"sync"
	"time"

	"github.com/parkee/de-test/elt/internal/config"
	"github.com/parkee/de-test/elt/internal/extract"
	"github.com/parkee/de-test/elt/internal/loader"
	"github.com/parkee/de-test/elt/internal/tables"
	"github.com/parkee/de-test/elt/internal/watermark"
)

const batchSize = 5000

// TenantResult summarizes one tenant's load.
type TenantResult struct {
	TenantID string
	Rows     int
	Err      error
}

// Run executes the full multi-tenant incremental load.
func Run(cfg *config.Config) error {
	ctx := context.Background()

	ext, err := extract.New(cfg)
	if err != nil {
		return err
	}
	defer ext.Close()

	ld, err := loader.New(cfg)
	if err != nil {
		return err
	}
	defer ld.Close()

	if err := ld.EnsureSchema(ctx); err != nil {
		return err
	}

	wm, err := watermark.Open(cfg.WatermarkPath)
	if err != nil {
		return err
	}

	var wg sync.WaitGroup
	results := make(chan TenantResult, len(cfg.Tenants))

	for _, tenant := range cfg.Tenants {
		wg.Add(1)
		go func(t config.Tenant) {
			defer wg.Done()
			rows, err := processTenant(ctx, ext, ld, wm, t)
			results <- TenantResult{TenantID: t.TenantID, Rows: rows, Err: err}
		}(tenant)
	}

	go func() { wg.Wait(); close(results) }()

	var firstErr error
	totalRows := 0
	for r := range results {
		if r.Err != nil {
			log.Printf("[%s] FAILED: %v", r.TenantID, r.Err)
			if firstErr == nil {
				firstErr = r.Err
			}
			continue
		}
		totalRows += r.Rows
		log.Printf("[%s] done: %d rows", r.TenantID, r.Rows)
	}

	// persist watermarks even on partial success
	if err := wm.Flush(); err != nil {
		log.Printf("WARN: failed to flush watermark: %v", err)
	}
	log.Printf("=== pipeline finished | total rows loaded: %d ===", totalRows)
	return firstErr
}

// processTenant loads every table for a single tenant.
func processTenant(
	ctx context.Context, ext *extract.Extractor, ld *loader.Loader,
	wm *watermark.Store, tenant config.Tenant,
) (int, error) {
	tenantTotal := 0
	for _, tbl := range tables.All {
		since := wm.Get(tenant.TenantID, tbl.Name)
		start := time.Now()

		count, newWM, err := ext.ExtractIncremental(
			tenant.TenantID, tenant.Schema, tbl, since, batchSize,
			func(rows [][]interface{}) error {
				return ld.InsertBatch(ctx, tbl, rows)
			})
		if err != nil {
			return tenantTotal, err
		}

		if count > 0 {
			wm.Set(tenant.TenantID, tbl.Name, newWM)
		}
		tenantTotal += count
		log.Printf("[%s.%s] %d rows (since=%s) in %s",
			tenant.TenantID, tbl.Name, count,
			since.Format(time.RFC3339), time.Since(start).Round(time.Millisecond))
	}
	return tenantTotal, nil
}
