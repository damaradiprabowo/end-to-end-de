// Command elt-advanced runs the Advanced production-grade ELT pipeline
// (goroutine + channel fan-out/fan-in, rate limiting, DWH-table watermark).
package main

import (
	"flag"
	"log"

	"github.com/parkee/de-test/elt/internal/advanced"
	"github.com/parkee/de-test/elt/internal/config"
)

func main() {
	configPath := flag.String("config", "config/tenants.json", "path to tenants config")
	flag.Parse()

	log.SetFlags(log.LstdFlags | log.Lmsgprefix)
	log.SetPrefix("elt-advanced | ")

	cfg, err := config.Load(*configPath)
	if err != nil {
		log.Fatalf("config: %v", err)
	}

	log.Printf("starting advanced ELT for %d tenants", len(cfg.Tenants))
	if err := advanced.Run(cfg); err != nil {
		log.Fatalf("pipeline failed: %v", err)
	}
	log.Printf("advanced ELT completed successfully")
}
