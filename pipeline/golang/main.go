// Command elt is the Intermediate multi-tenant incremental ELT loader.
//
// It loads every configured tenant (postgres schema) into the ClickHouse raw
// database concurrently (one goroutine per tenant) using an incremental
// high-watermark on updated_at, persisted to a JSON file.
//
// Usage:
//
//	elt --config config/tenants.json --watermark state/watermark.json
package main

import (
	"flag"
	"log"

	"github.com/parkee/de-test/elt/internal/config"
	"github.com/parkee/de-test/elt/internal/pipeline"
)

func main() {
	cfgPath := flag.String("config", "config/tenants.json", "path to tenants config")
	wmPath := flag.String("watermark", "", "path to watermark state file (overrides env)")
	flag.Parse()

	log.SetFlags(log.LstdFlags | log.Lmsgprefix)
	log.SetPrefix("el-go | ")

	cfg, err := config.Load(*cfgPath)
	if err != nil {
		log.Fatalf("config error: %v", err)
	}
	if *wmPath != "" {
		cfg.WatermarkPath = *wmPath
	}

	log.Printf("starting ELT for %d tenants -> clickhouse db %q",
		len(cfg.Tenants), cfg.CHRawDB)
	if err := pipeline.Run(cfg); err != nil {
		log.Fatalf("pipeline failed: %v", err)
	}
}
