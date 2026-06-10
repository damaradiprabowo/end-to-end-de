// Package dwhwatermark implements a high-watermark store backed by a
// configuration table inside the Data Warehouse (ClickHouse), as required by
// the Advanced level (watermark persisted in a DWH config table rather than a
// local JSON file).
package dwhwatermark

import (
	"context"
	"fmt"
	"time"

	"github.com/ClickHouse/clickhouse-go/v2"
	"github.com/ClickHouse/clickhouse-go/v2/lib/driver"

	"github.com/parkee/de-test/elt/internal/config"
)

// Epoch is returned when no watermark exists yet (triggers an initial load).
var Epoch = time.Date(1970, 1, 1, 0, 0, 0, 0, time.UTC)

// Store persists watermarks in `<rawdb>.etl_watermark`.
type Store struct {
	conn  driver.Conn
	db    string
	table string
}

// New opens a ClickHouse connection for the watermark store.
func New(cfg *config.Config) (*Store, error) {
	conn, err := clickhouse.Open(&clickhouse.Options{
		Addr: []string{fmt.Sprintf("%s:%s", cfg.CHHost, cfg.CHPort)},
		Auth: clickhouse.Auth{
			Database: "default",
			Username: cfg.CHUser,
			Password: cfg.CHPassword,
		},
		DialTimeout: 10 * time.Second,
	})
	if err != nil {
		return nil, fmt.Errorf("open clickhouse (watermark): %w", err)
	}
	return &Store{conn: conn, db: cfg.CHRawDB, table: "etl_watermark"}, nil
}

func (s *Store) Close() error { return s.conn.Close() }

func (s *Store) fqtn() string { return fmt.Sprintf("`%s`.`%s`", s.db, s.table) }

// EnsureTable creates the watermark config table if it does not exist.
func (s *Store) EnsureTable(ctx context.Context) error {
	if err := s.conn.Exec(ctx,
		fmt.Sprintf("CREATE DATABASE IF NOT EXISTS `%s`", s.db)); err != nil {
		return err
	}
	ddl := fmt.Sprintf(`CREATE TABLE IF NOT EXISTS %s (
        tenant_id   String,
        table_name  String,
        watermark   DateTime,
        updated_at  DateTime DEFAULT now()
    ) ENGINE = ReplacingMergeTree(updated_at)
    ORDER BY (tenant_id, table_name)`, s.fqtn())
	return s.conn.Exec(ctx, ddl)
}

// Get returns the stored high-watermark for a tenant/table, or Epoch if none.
// max() is robust to the ReplacingMergeTree keeping multiple unmerged versions.
func (s *Store) Get(ctx context.Context, tenant, table string) (time.Time, error) {
	var wm time.Time
	row := s.conn.QueryRow(ctx,
		fmt.Sprintf("SELECT max(watermark) FROM %s WHERE tenant_id = ? AND table_name = ?", s.fqtn()),
		tenant, table)
	if err := row.Scan(&wm); err != nil {
		return Epoch, err
	}
	if wm.IsZero() || wm.Year() <= 1970 {
		return Epoch, nil
	}
	return wm, nil
}

// Set records a new high-watermark for a tenant/table.
func (s *Store) Set(ctx context.Context, tenant, table string, ts time.Time) error {
	return s.conn.Exec(ctx,
		fmt.Sprintf("INSERT INTO %s (tenant_id, table_name, watermark) VALUES (?, ?, ?)", s.fqtn()),
		tenant, table, ts.UTC())
}
