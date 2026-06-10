// Command go-api is the Advanced Data API: a dependency-free (stdlib-only) HTTP
// service that answers the 7 analytic questions from the ClickHouse `analytics`
// star schema.
//
// Production-grade features required by the Advanced level:
//   - in-memory response cache with a TTL (per endpoint)
//   - structured request-logging middleware
//   - /health (warehouse connectivity) and /metrics (Prometheus text format)
//
// It talks to ClickHouse over the HTTP interface using FORMAT JSON, so no
// database driver dependency is needed — the binary builds entirely offline.
package main

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"sync"
	"sync/atomic"
	"time"
)

// ---------------------------------------------------------------------------
// configuration
// ---------------------------------------------------------------------------

type config struct {
	addr     string
	chURL    string
	chUser   string
	chPass   string
	cacheTTL time.Duration
}

func envOr(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

func loadConfig() config {
	host := envOr("CLICKHOUSE_HOST", "localhost")
	port := envOr("CLICKHOUSE_HTTP_PORT", "8123")
	ttlSec, _ := strconv.Atoi(envOr("CACHE_TTL_SECONDS", "60"))
	return config{
		addr:     ":" + envOr("API_PORT", "8090"),
		chURL:    fmt.Sprintf("http://%s:%s/", host, port),
		chUser:   envOr("CLICKHOUSE_USER", "default"),
		chPass:   envOr("CLICKHOUSE_PASSWORD", "clickhouse"),
		cacheTTL: time.Duration(ttlSec) * time.Second,
	}
}

// ---------------------------------------------------------------------------
// ClickHouse HTTP client (FORMAT JSON)
// ---------------------------------------------------------------------------

type chClient struct {
	cfg  config
	http *http.Client
}

func newCHClient(cfg config) *chClient {
	return &chClient{cfg: cfg, http: &http.Client{Timeout: 30 * time.Second}}
}

// query runs SQL and returns the rows as raw JSON ([]{...}). ClickHouse's JSON
// format wraps the rows in {"meta":..,"data":[..],"rows":N}; we forward `data`.
func (c *chClient) query(sql string) (json.RawMessage, error) {
	q := url.Values{}
	q.Set("default_format", "JSON")
	// Emit Int64/UInt64 as JSON numbers (not strings) so the dashboard's
	// charting library receives numeric values directly.
	q.Set("output_format_json_quote_64bit_integers", "0")
	req, err := http.NewRequest(http.MethodPost, c.cfg.chURL+"?"+q.Encode(),
		stringReader(sql))
	if err != nil {
		return nil, err
	}
	req.Header.Set("X-ClickHouse-User", c.cfg.chUser)
	req.Header.Set("X-ClickHouse-Key", c.cfg.chPass)

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("clickhouse %d: %s", resp.StatusCode, truncate(body, 400))
	}
	var envelope struct {
		Data json.RawMessage `json:"data"`
	}
	if err := json.Unmarshal(body, &envelope); err != nil {
		return nil, fmt.Errorf("decode clickhouse response: %w", err)
	}
	if len(envelope.Data) == 0 {
		return json.RawMessage("[]"), nil
	}
	return envelope.Data, nil
}

func (c *chClient) ping() error {
	_, err := c.query("SELECT 1")
	return err
}

// ---------------------------------------------------------------------------
// in-memory TTL cache
// ---------------------------------------------------------------------------

type cacheEntry struct {
	body    json.RawMessage
	expires time.Time
}

type ttlCache struct {
	mu  sync.RWMutex
	ttl time.Duration
	m   map[string]cacheEntry
}

func newCache(ttl time.Duration) *ttlCache {
	return &ttlCache{ttl: ttl, m: make(map[string]cacheEntry)}
}

func (c *ttlCache) get(key string) (json.RawMessage, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	e, ok := c.m[key]
	if !ok || time.Now().After(e.expires) {
		return nil, false
	}
	return e.body, true
}

func (c *ttlCache) set(key string, body json.RawMessage) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.m[key] = cacheEntry{body: body, expires: time.Now().Add(c.ttl)}
}

// ---------------------------------------------------------------------------
// metrics (Prometheus text exposition format, no client library)
// ---------------------------------------------------------------------------

type metrics struct {
	requests    int64
	errors      int64
	cacheHits   int64
	cacheMisses int64
}

func (m *metrics) render() string {
	return fmt.Sprintf(
		"# HELP api_requests_total Total API requests served.\n"+
			"# TYPE api_requests_total counter\n"+
			"api_requests_total %d\n"+
			"# HELP api_errors_total Total API requests that returned 5xx.\n"+
			"# TYPE api_errors_total counter\n"+
			"api_errors_total %d\n"+
			"# HELP api_cache_hits_total Total cache hits.\n"+
			"# TYPE api_cache_hits_total counter\n"+
			"api_cache_hits_total %d\n"+
			"# HELP api_cache_misses_total Total cache misses.\n"+
			"# TYPE api_cache_misses_total counter\n"+
			"api_cache_misses_total %d\n",
		atomic.LoadInt64(&m.requests),
		atomic.LoadInt64(&m.errors),
		atomic.LoadInt64(&m.cacheHits),
		atomic.LoadInt64(&m.cacheMisses),
	)
}

// ---------------------------------------------------------------------------
// server
// ---------------------------------------------------------------------------

type server struct {
	cfg     config
	ch      *chClient
	cache   *ttlCache
	metrics *metrics
}

// statusRecorder captures the status code for the logging middleware.
type statusRecorder struct {
	http.ResponseWriter
	status int
}

func (r *statusRecorder) WriteHeader(code int) {
	r.status = code
	r.ResponseWriter.WriteHeader(code)
}

// middleware wraps a handler with CORS, request logging and metrics counting.
func (s *server) middleware(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		w.Header().Set("Access-Control-Allow-Origin", "*")
		rec := &statusRecorder{ResponseWriter: w, status: http.StatusOK}

		atomic.AddInt64(&s.metrics.requests, 1)
		next(rec, r)

		if rec.status >= 500 {
			atomic.AddInt64(&s.metrics.errors, 1)
		}
		log.Printf("%s %s -> %d (%s)",
			r.Method, r.URL.Path, rec.status, time.Since(start).Round(time.Microsecond))
	}
}

func writeJSON(w http.ResponseWriter, status int, body json.RawMessage) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_, _ = w.Write(body)
}

func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, json.RawMessage(
		fmt.Sprintf(`{"error":%s}`, strconvQuote(msg))))
}

// dataHandler serves one analytic endpoint, with cache lookup → query → cache fill.
func (s *server) dataHandler(ep endpoint) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if body, ok := s.cache.get(ep.path); ok {
			atomic.AddInt64(&s.metrics.cacheHits, 1)
			w.Header().Set("X-Cache", "HIT")
			writeJSON(w, http.StatusOK, body)
			return
		}
		atomic.AddInt64(&s.metrics.cacheMisses, 1)

		body, err := s.ch.query(ep.sql)
		if err != nil {
			log.Printf("query failed for %s: %v", ep.path, err)
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		s.cache.set(ep.path, body)
		w.Header().Set("X-Cache", "MISS")
		writeJSON(w, http.StatusOK, body)
	}
}

func (s *server) healthHandler(w http.ResponseWriter, r *http.Request) {
	if err := s.ch.ping(); err != nil {
		writeError(w, http.StatusServiceUnavailable, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, json.RawMessage(`{"status":"ok"}`))
}

func (s *server) metricsHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/plain; version=0.0.4")
	_, _ = io.WriteString(w, s.metrics.render())
}

// indexHandler advertises the available endpoints (used as a simple API home).
func (s *server) indexHandler(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		writeError(w, http.StatusNotFound, "not found")
		return
	}
	type route struct {
		Path string `json:"path"`
		Desc string `json:"description"`
	}
	routes := make([]route, 0, len(endpoints))
	for _, ep := range endpoints {
		routes = append(routes, route{ep.path, ep.desc})
	}
	body, _ := json.Marshal(map[string]any{
		"service":   "minimarket-advanced-data-api",
		"endpoints": routes,
		"cache_ttl": s.cfg.cacheTTL.String(),
	})
	writeJSON(w, http.StatusOK, body)
}

func (s *server) routes() http.Handler {
	mux := http.NewServeMux()
	for _, ep := range endpoints {
		mux.HandleFunc(ep.path, s.middleware(s.dataHandler(ep)))
	}
	mux.HandleFunc("/health", s.middleware(s.healthHandler))
	mux.HandleFunc("/metrics", s.middleware(s.metricsHandler))
	mux.HandleFunc("/", s.middleware(s.indexHandler))
	return mux
}

func main() {
	cfg := loadConfig()
	log.SetFlags(log.LstdFlags | log.Lmsgprefix)
	log.SetPrefix("go-api | ")

	s := &server{
		cfg:     cfg,
		ch:      newCHClient(cfg),
		cache:   newCache(cfg.cacheTTL),
		metrics: &metrics{},
	}

	log.Printf("listening on %s | clickhouse=%s | cache_ttl=%s",
		cfg.addr, cfg.chURL, cfg.cacheTTL)
	srv := &http.Server{
		Addr:              cfg.addr,
		Handler:           s.routes(),
		ReadHeaderTimeout: 10 * time.Second,
	}
	if err := srv.ListenAndServe(); err != nil {
		log.Fatalf("server error: %v", err)
	}
}
