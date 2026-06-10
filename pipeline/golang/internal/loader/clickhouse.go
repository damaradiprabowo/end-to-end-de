// Package loader writes extracted rows into the ClickHouse raw database.
package loader

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/ClickHouse/clickhouse-go/v2"
	"github.com/ClickHouse/clickhouse-go/v2/lib/driver"

	"github.com/parkee/de-test/elt/internal/config"
	"github.com/parkee/de-test/elt/internal/tables"
)

// Loader wraps a ClickHouse connection.
type Loader struct {
	conn driver.Conn
	db   string
}

// New opens a ClickHouse connection.
func New(cfg *config.Config) (*Loader, error) {
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
		return nil, fmt.Errorf("open clickhouse: %w", err)
	}
	if err := conn.Ping(context.Background()); err != nil {
		return nil, fmt.Errorf("ping clickhouse: %w", err)
	}
	return &Loader{conn: conn, db: cfg.CHRawDB}, nil
}

func (l *Loader) Close() error { return l.conn.Close() }

// EnsureSchema creates the raw database and all raw tables.
func (l *Loader) EnsureSchema(ctx context.Context) error {
	if err := l.conn.Exec(ctx,
		fmt.Sprintf("CREATE DATABASE IF NOT EXISTS `%s`", l.db)); err != nil {
		return fmt.Errorf("create database: %w", err)
	}
	for _, t := range tables.All {
		if err := l.conn.Exec(ctx, t.CreateDDL(l.db)); err != nil {
			return fmt.Errorf("create table %s: %w", t.Name, err)
		}
	}
	return nil
}

// InsertBatch inserts rows (each row already prefixed with tenant_id) into the
// raw table. ReplacingMergeTree dedupes by (tenant_id, pk) keeping latest.
func (l *Loader) InsertBatch(ctx context.Context, t tables.Table, rows [][]interface{}) error {
	if len(rows) == 0 {
		return nil
	}
	cols := t.CHColumns()
	stmt := fmt.Sprintf("INSERT INTO `%s`.`%s` (%s)",
		l.db, t.Name, "`"+strings.Join(cols, "`, `")+"`")
	batch, err := l.conn.PrepareBatch(ctx, stmt)
	if err != nil {
		return fmt.Errorf("prepare batch %s: %w", t.Name, err)
	}
	for _, row := range rows {
		if err := batch.Append(row...); err != nil {
			return fmt.Errorf("append %s: %w", t.Name, err)
		}
	}
	return batch.Send()
}
