// Package extract reads rows incrementally from a PostgreSQL tenant schema.
package extract

import (
	"database/sql"
	"fmt"
	"time"

	_ "github.com/lib/pq"

	"github.com/parkee/de-test/elt/internal/config"
	"github.com/parkee/de-test/elt/internal/tables"
)

// Extractor reads from the source PostgreSQL database.
type Extractor struct {
	db *sql.DB
}

// New opens the postgres connection.
func New(cfg *config.Config) (*Extractor, error) {
	db, err := sql.Open("postgres", cfg.PGDSN())
	if err != nil {
		return nil, fmt.Errorf("open postgres: %w", err)
	}
	db.SetMaxOpenConns(len(cfg.Tenants) + 2)
	if err := db.Ping(); err != nil {
		return nil, fmt.Errorf("ping postgres: %w", err)
	}
	return &Extractor{db: db}, nil
}

func (e *Extractor) Close() error { return e.db.Close() }

// BatchFunc receives a batch of rows (each prefixed with tenant_id).
type BatchFunc func(rows [][]interface{}) error

// ExtractIncremental streams rows where watermark_col > since, in batches.
// Returns total row count and the new high-watermark.
func (e *Extractor) ExtractIncremental(
	tenantID, schema string, t tables.Table, since time.Time,
	batchSize int, emit BatchFunc,
) (int, time.Time, error) {

	query := fmt.Sprintf(
		`SELECT %s FROM "%s".%s WHERE %s > $1 ORDER BY %s`,
		t.SelectList(), schema, t.Name, t.WatermarkCol, t.WatermarkCol)

	rows, err := e.db.Query(query, since)
	if err != nil {
		return 0, since, fmt.Errorf("query %s.%s: %w", schema, t.Name, err)
	}
	defer rows.Close()

	wmIdx := watermarkIndex(t)
	nCols := len(t.Columns)
	maxWM := since
	total := 0
	batch := make([][]interface{}, 0, batchSize)

	for rows.Next() {
		raw := make([]interface{}, nCols)
		ptrs := make([]interface{}, nCols)
		for i := range raw {
			ptrs[i] = &raw[i]
		}
		if err := rows.Scan(ptrs...); err != nil {
			return total, maxWM, fmt.Errorf("scan %s: %w", t.Name, err)
		}

		// normalize driver values + track watermark
		row := make([]interface{}, 0, nCols+1)
		row = append(row, tenantID)
		for i, v := range raw {
			v = normalize(v)
			if i == wmIdx {
				if ts, ok := v.(time.Time); ok && ts.After(maxWM) {
					maxWM = ts
				}
			}
			row = append(row, v)
		}
		batch = append(batch, row)
		total++

		if len(batch) >= batchSize {
			if err := emit(batch); err != nil {
				return total, maxWM, err
			}
			batch = batch[:0]
		}
	}
	if err := rows.Err(); err != nil {
		return total, maxWM, err
	}
	if len(batch) > 0 {
		if err := emit(batch); err != nil {
			return total, maxWM, err
		}
	}
	return total, maxWM, nil
}

// normalize converts lib/pq []byte (text/varchar) values to string so the
// ClickHouse driver maps them onto String columns.
func normalize(v interface{}) interface{} {
	if b, ok := v.([]byte); ok {
		return string(b)
	}
	return v
}

func watermarkIndex(t tables.Table) int {
	for i, c := range t.Columns {
		if c.Name == t.WatermarkCol {
			return i
		}
	}
	return -1
}
