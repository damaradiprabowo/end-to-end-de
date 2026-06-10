package config

import (
	"encoding/json"
	"fmt"
	"os"
)

// Tenant represents a single store/branch source (one postgres schema).
type Tenant struct {
	TenantID    string `json:"tenant_id"`
	Schema      string `json:"schema"`
	DisplayName string `json:"display_name"`
}

type tenantsFile struct {
	Tenants []Tenant `json:"tenants"`
}

// Config holds all runtime configuration for the ELT binary.
type Config struct {
	// Postgres
	PGHost, PGPort, PGDB, PGUser, PGPassword string
	// ClickHouse
	CHHost, CHPort, CHUser, CHPassword, CHRawDB string
	// runtime
	Tenants       []Tenant
	WatermarkPath string
}

func env(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

// Load reads environment variables and the tenants.json config file.
func Load(tenantsPath string) (*Config, error) {
	raw, err := os.ReadFile(tenantsPath)
	if err != nil {
		return nil, fmt.Errorf("read tenants config %q: %w", tenantsPath, err)
	}
	var tf tenantsFile
	if err := json.Unmarshal(raw, &tf); err != nil {
		return nil, fmt.Errorf("parse tenants config: %w", err)
	}
	if len(tf.Tenants) == 0 {
		return nil, fmt.Errorf("no tenants configured in %q", tenantsPath)
	}

	return &Config{
		PGHost:        env("POSTGRES_HOST", "localhost"),
		PGPort:        env("POSTGRES_PORT", "5432"),
		PGDB:          env("POSTGRES_DB", "pos"),
		PGUser:        env("POSTGRES_USER", "pos_user"),
		PGPassword:    env("POSTGRES_PASSWORD", "pos_password"),
		CHHost:        env("CLICKHOUSE_HOST", "localhost"),
		CHPort:        env("CLICKHOUSE_PORT", "9000"),
		CHUser:        env("CLICKHOUSE_USER", "default"),
		CHPassword:    env("CLICKHOUSE_PASSWORD", "clickhouse"),
		CHRawDB:       env("CLICKHOUSE_RAW_DB", "raw"),
		Tenants:       tf.Tenants,
		WatermarkPath: env("WATERMARK_PATH", "state/watermark.json"),
	}, nil
}

// PGDSN builds the postgres connection string.
func (c *Config) PGDSN() string {
	return fmt.Sprintf("host=%s port=%s dbname=%s user=%s password=%s sslmode=disable",
		c.PGHost, c.PGPort, c.PGDB, c.PGUser, c.PGPassword)
}
