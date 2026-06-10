// Package advanced implements the production-grade Advanced ELT:
//   - true concurrency with goroutines AND channels (fan-out / fan-in)
//   - one extractor goroutine per tenant feeds a record channel
//   - a pool of loader workers reads the channel and writes to ClickHouse
//   - non-blocking error handling via a buffered error channel
//   - rate limiting on the extractor (golang.org/x/time/rate) to protect source
//   - incremental high-watermark persisted in a DWH config table
package advanced

import (
	"context"
	"log"
	"sync"
	"time"

	"golang.org/x/time/rate"

	"github.com/parkee/de-test/elt/internal/config"
	"github.com/parkee/de-test/elt/internal/dwhwatermark"
	"github.com/parkee/de-test/elt/internal/extract"
	"github.com/parkee/de-test/elt/internal/loader"
	"github.com/parkee/de-test/elt/internal/tables"
)

const (
	extractBatchSize = 2000
	loaderFlushSize  = 5000
	workerCount      = 4
	recordChanBuffer = 1000
	errChanBuffer    = 1024
	// rate limit: max source-fetch batches per second (shared across tenants)
	maxBatchesPerSecond = 50
)

// Record is one extracted row tagged with its destination table.
type Record struct {
	Table tables.Table
	Row   []interface{}
}

// Run executes the Advanced fan-out/fan-in pipeline for all tenants.
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

	wm, err := dwhwatermark.New(cfg)
	if err != nil {
		return err
	}
	defer wm.Close()

	if err := ld.EnsureSchema(ctx); err != nil {
		return err
	}
	if err := wm.EnsureTable(ctx); err != nil {
		return err
	}

	// shared limiter protects the source database across all tenant goroutines
	limiter := rate.NewLimiter(rate.Limit(maxBatchesPerSecond), maxBatchesPerSecond)

	var wg sync.WaitGroup
	results := make(chan error, len(cfg.Tenants))
	for _, t := range cfg.Tenants {
		wg.Add(1)
		go func(tenant config.Tenant) {
			defer wg.Done()
			if err := runTenant(ctx, ext, ld, wm, limiter, tenant); err != nil {
				log.Printf("[%s] FAILED: %v", tenant.TenantID, err)
				results <- err
			} else {
				results <- nil
			}
		}(t)
	}
	go func() { wg.Wait(); close(results) }()

	var firstErr error
	for err := range results {
		if err != nil && firstErr == nil {
			firstErr = err
		}
	}
	log.Printf("=== advanced pipeline finished ===")
	return firstErr
}

// runTenant wires the extractor goroutine to a pool of loader workers via a
// record channel (fan-out), collecting results back (fan-in).
func runTenant(
	ctx context.Context, ext *extract.Extractor, ld *loader.Loader,
	wm *dwhwatermark.Store, limiter *rate.Limiter, tenant config.Tenant,
) error {
	recordCh := make(chan Record, recordChanBuffer)
	errCh := make(chan error, errChanBuffer)

	var maxMu sync.Mutex
	maxWM := make(map[string]time.Time)

	// ---- extractor goroutine (single producer) ----
	go func() {
		defer close(recordCh)
		for _, tbl := range tables.All {
			since, err := wm.Get(ctx, tenant.TenantID, tbl.Name)
			if err != nil {
				errCh <- err
				continue
			}
			start := time.Now()
			count, newWM, err := ext.ExtractIncremental(
				tenant.TenantID, tenant.Schema, tbl, since, extractBatchSize,
				func(rows [][]interface{}) error {
					// rate-limit each source fetch batch
					if err := limiter.Wait(ctx); err != nil {
						return err
					}
					for _, r := range rows {
						recordCh <- Record{Table: tbl, Row: r}
					}
					return nil
				})
			if err != nil {
				errCh <- err
				continue
			}
			maxMu.Lock()
			maxWM[tbl.Name] = newWM
			maxMu.Unlock()
			log.Printf("[%s.%s] extracted %d rows (since=%s) in %s",
				tenant.TenantID, tbl.Name, count, since.Format(time.RFC3339),
				time.Since(start).Round(time.Millisecond))
		}
	}()

	// ---- loader worker pool (multiple consumers / fan-out) ----
	var wg sync.WaitGroup
	for i := 0; i < workerCount; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			loaderWorker(ctx, ld, recordCh, errCh)
		}()
	}
	wg.Wait()
	close(errCh)

	// ---- fan-in: collect errors ----
	var firstErr error
	for err := range errCh {
		log.Printf("[%s] error: %v", tenant.TenantID, err)
		if firstErr == nil {
			firstErr = err
		}
	}
	if firstErr != nil {
		// do NOT advance watermark on failure (safe re-run)
		return firstErr
	}

	// ---- persist watermarks only after a fully successful load ----
	for name, ts := range maxWM {
		if err := wm.Set(ctx, tenant.TenantID, name, ts); err != nil {
			return err
		}
	}
	return nil
}

// loaderWorker buffers records per table and flushes batches to ClickHouse.
// It always drains the channel to completion so the extractor never blocks.
func loaderWorker(ctx context.Context, ld *loader.Loader, ch <-chan Record, errCh chan<- error) {
	buffers := make(map[string][][]interface{})
	tableByName := make(map[string]tables.Table)

	flush := func(name string) {
		if len(buffers[name]) == 0 {
			return
		}
		if err := ld.InsertBatch(ctx, tableByName[name], buffers[name]); err != nil {
			errCh <- err
		}
		buffers[name] = buffers[name][:0]
	}

	for rec := range ch {
		tableByName[rec.Table.Name] = rec.Table
		buffers[rec.Table.Name] = append(buffers[rec.Table.Name], rec.Row)
		if len(buffers[rec.Table.Name]) >= loaderFlushSize {
			flush(rec.Table.Name)
		}
	}
	for name := range buffers {
		flush(name)
	}
}
