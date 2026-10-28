package main

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/collectors"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"google.golang.org/protobuf/reflect/protoreflect"

	"github.com/lao-tseu-is-alive/go-cloud-k8s-poc-2026/pkg/caseexport"
	"github.com/lao-tseu-is-alive/go-cloud-k8s-poc-2026/pkg/core"
	"github.com/lao-tseu-is-alive/go-cloud-k8s-poc-2026/pkg/version"
)

// Prometheus metrics (GLD-010), served on their own listener
// (GOELAND_METRICS_ADDRESS), never on the public port. Labels stay bounded:
// a request is named by its RPC or by the route pattern that served it, never
// by its URL.

const (
	metricsNamespace = "goeland"
	// backlogScrapeTimeout bounds the database reads of one scrape.
	backlogScrapeTimeout = 2 * time.Second
	// unmatchedHandler names the requests no route or RPC matched (404s, scans).
	unmatchedHandler = "unmatched"
)

// knownHTTPMethods are the methods kept as a label; any other is "OTHER".
var knownHTTPMethods = map[string]bool{
	http.MethodGet: true, http.MethodHead: true, http.MethodPost: true, http.MethodPut: true,
	http.MethodPatch: true, http.MethodDelete: true, http.MethodOptions: true,
}

// httpMetrics counts and times the requests per handler, method and status.
type httpMetrics struct {
	requests *prometheus.CounterVec
	duration *prometheus.HistogramVec
	// routes resolves a REST path to its RPC.
	routes []restRoute
	// prefixes are the mux patterns of the transcoder ("/api/", "/goeland.v1.CaseService/").
	prefixes map[string]bool
	// procedures are the RPCs reachable on their Connect path ("/goeland.v1.CaseService/GetCase").
	procedures map[string]bool
}

// newHTTPMetrics registers the request metrics of the services named.
func newHTTPMetrics(registry prometheus.Registerer, routes []restRoute, serviceNames []string) (*httpMetrics, error) {
	m := &httpMetrics{
		requests: prometheus.NewCounterVec(prometheus.CounterOpts{
			Namespace: metricsNamespace, Name: "http_requests_total",
			Help: "HTTP requests by handler (RPC or route pattern), method and status code.",
		}, []string{"handler", "method", "code"}),
		duration: prometheus.NewHistogramVec(prometheus.HistogramOpts{
			Namespace: metricsNamespace, Name: "http_request_duration_seconds",
			Help:    "HTTP request duration by handler (RPC or route pattern) and method.",
			Buckets: []float64{.005, .01, .025, .05, .1, .25, .5, 1, 2.5, 5, 10, 30},
		}, []string{"handler", "method"}),
		routes:     routes,
		prefixes:   map[string]bool{"/api/": true},
		procedures: map[string]bool{},
	}
	for _, name := range serviceNames {
		m.prefixes["/"+name+"/"] = true
	}
	if err := eachMethod(serviceNames, func(method protoreflect.MethodDescriptor) {
		m.procedures["/"+procedureName(method)] = true
	}); err != nil {
		return nil, err
	}
	if err := registry.Register(m.requests); err != nil {
		return nil, err
	}
	return m, registry.Register(m.duration)
}

// middleware records every request. It must wrap the mux directly: the mux
// sets the matched pattern on the request it receives. The path is read
// before serving, since the transcoder rewrites it in place.
func (m *httpMetrics) middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		started, path := time.Now(), request.URL.Path
		rec := &statusRecorder{ResponseWriter: writer, status: http.StatusOK}
		next.ServeHTTP(rec, request)
		handler, method := m.handlerName(request.Pattern, request.Method, path), metricMethod(request.Method)
		m.requests.WithLabelValues(handler, method, strconv.Itoa(rec.status)).Inc()
		m.duration.WithLabelValues(handler, method).Observe(time.Since(started).Seconds())
	})
}

// handlerName names the handler of a served request: its RPC on the
// transcoder, otherwise the mux pattern that matched ("GET /health", "/" for
// the SPA).
func (m *httpMetrics) handlerName(pattern, method, path string) string {
	if !m.prefixes[pattern] {
		if pattern == "" {
			return unmatchedHandler
		}
		return pattern
	}
	if pattern == "/api/" {
		if route := routeFor(m.routes, method, splitPath(path)); route != nil {
			return route.procedure
		}
		return unmatchedHandler
	}
	if m.procedures[path] {
		return strings.TrimPrefix(path, "/")
	}
	return unmatchedHandler
}

// metricMethod bounds the method label.
func metricMethod(method string) string {
	if knownHTTPMethods[method] {
		return method
	}
	return "OTHER"
}

// newMetricsRegistry builds the registry served on the metrics listener: the
// Go runtime and process, the build, the database pool and the backlogs of the
// outbox and of the case exports.
func newMetricsRegistry(pool *pgxpool.Pool, coreSvc *core.Service, exports *caseexport.Service, log *slog.Logger) (*prometheus.Registry, error) {
	registry := prometheus.NewRegistry()
	build := prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Namespace: metricsNamespace, Name: "build_info", Help: "Version and revision of the running server (always 1).",
	}, []string{"version", "revision"})
	build.WithLabelValues(version.Version, version.Revision).Set(1)
	for _, c := range []prometheus.Collector{
		collectors.NewGoCollector(),
		collectors.NewProcessCollector(collectors.ProcessCollectorOpts{}),
		build,
		poolCollector{pool: pool},
		backlogCollector{core: coreSvc, exports: exports, log: log},
	} {
		if err := registry.Register(c); err != nil {
			return nil, err
		}
	}
	return registry, nil
}

// buildMetrics builds the metrics registry, the request metrics middleware of
// the public server (with the REST routes it resolves, which the transcoder
// mount reuses) and the handler of the metrics listener.
func buildMetrics(pool *pgxpool.Pool, coreSvc *core.Service, exports *caseexport.Service,
	serviceNames []string, log *slog.Logger) (*httpMetrics, http.Handler, error) {
	routes, err := restRoutes(serviceNames)
	if err != nil {
		return nil, nil, fmt.Errorf("read REST bindings: %w", err)
	}
	registry, err := newMetricsRegistry(pool, coreSvc, exports, log)
	if err != nil {
		return nil, nil, fmt.Errorf("metrics registry: %w", err)
	}
	requestMetrics, err := newHTTPMetrics(registry, routes, serviceNames)
	if err != nil {
		return nil, nil, fmt.Errorf("request metrics: %w", err)
	}
	return requestMetrics, metricsHandler(registry, log), nil
}

// metricsHandler serves the registry in the Prometheus exposition format.
func metricsHandler(registry *prometheus.Registry, log *slog.Logger) http.Handler {
	mux := http.NewServeMux()
	mux.Handle("GET /metrics", promhttp.HandlerFor(registry, promhttp.HandlerOpts{
		ErrorLog: slog.NewLogLogger(log.Handler(), slog.LevelWarn), Registry: registry,
	}))
	return mux
}

func metricDesc(name, help string, labels ...string) *prometheus.Desc {
	return prometheus.NewDesc(prometheus.BuildFQName(metricsNamespace, "", name), help, labels, nil)
}

var (
	poolAcquiredDesc     = metricDesc("db_pool_acquired_connections", "Connections in use.")
	poolIdleDesc         = metricDesc("db_pool_idle_connections", "Idle connections.")
	poolTotalDesc        = metricDesc("db_pool_total_connections", "Open connections.")
	poolMaxDesc          = metricDesc("db_pool_max_connections", "Maximum pool size.")
	poolAcquiresDesc     = metricDesc("db_pool_acquires_total", "Connections acquired from the pool.")
	poolEmptyAcqDesc     = metricDesc("db_pool_empty_acquires_total", "Acquires that waited because the pool was empty.")
	poolCanceledAcqDesc  = metricDesc("db_pool_canceled_acquires_total", "Acquires canceled by their context.")
	poolAcquireDurDesc   = metricDesc("db_pool_acquire_duration_seconds_total", "Time spent acquiring connections.")
	outboxEventsDesc     = metricDesc("outbox_events", "Outbox events kept, by state (GLD-028).", "state")
	outboxOldestDesc     = metricDesc("outbox_oldest_pending_age_seconds", "Age of the oldest pending outbox event (0 when none).")
	outboxConsumerDesc   = metricDesc("outbox_consumer_failed_events", "Given-up outbox events per consumer that refused them.", "consumer")
	caseExportJobsDesc   = metricDesc("case_export_jobs", "Case export jobs waiting or being built, by status (GLD-029).", "status")
	backlogScrapeErrDesc = metricDesc("backlog_scrape_error", "1 when the last read of the outbox or export backlog failed.", "backlog")
)

// poolCollector reports the pgx pool statistics (formerly on /health).
type poolCollector struct {
	pool *pgxpool.Pool
}

// Describe sends the pool metric descriptions.
func (poolCollector) Describe(ch chan<- *prometheus.Desc) {
	for _, d := range []*prometheus.Desc{poolAcquiredDesc, poolIdleDesc, poolTotalDesc, poolMaxDesc,
		poolAcquiresDesc, poolEmptyAcqDesc, poolCanceledAcqDesc, poolAcquireDurDesc} {
		ch <- d
	}
}

// Collect reads the pool statistics.
func (c poolCollector) Collect(ch chan<- prometheus.Metric) {
	stat := c.pool.Stat()
	ch <- prometheus.MustNewConstMetric(poolAcquiredDesc, prometheus.GaugeValue, float64(stat.AcquiredConns()))
	ch <- prometheus.MustNewConstMetric(poolIdleDesc, prometheus.GaugeValue, float64(stat.IdleConns()))
	ch <- prometheus.MustNewConstMetric(poolTotalDesc, prometheus.GaugeValue, float64(stat.TotalConns()))
	ch <- prometheus.MustNewConstMetric(poolMaxDesc, prometheus.GaugeValue, float64(stat.MaxConns()))
	ch <- prometheus.MustNewConstMetric(poolAcquiresDesc, prometheus.CounterValue, float64(stat.AcquireCount()))
	ch <- prometheus.MustNewConstMetric(poolEmptyAcqDesc, prometheus.CounterValue, float64(stat.EmptyAcquireCount()))
	ch <- prometheus.MustNewConstMetric(poolCanceledAcqDesc, prometheus.CounterValue, float64(stat.CanceledAcquireCount()))
	ch <- prometheus.MustNewConstMetric(poolAcquireDurDesc, prometheus.CounterValue, stat.AcquireDuration().Seconds())
}

// backlogCollector reads the outbox and case export backlogs from the database
// at scrape time; they are shared by every replica, so any replica reports them.
type backlogCollector struct {
	core    *core.Service
	exports *caseexport.Service
	log     *slog.Logger
}

// Describe sends the backlog metric descriptions.
func (backlogCollector) Describe(ch chan<- *prometheus.Desc) {
	for _, d := range []*prometheus.Desc{outboxEventsDesc, outboxOldestDesc, outboxConsumerDesc, caseExportJobsDesc, backlogScrapeErrDesc} {
		ch <- d
	}
}

// Collect reads the backlogs; a failed read reports backlog_scrape_error=1
// instead of failing the whole scrape.
func (c backlogCollector) Collect(ch chan<- prometheus.Metric) {
	// A Prometheus collector receives no context: the scrape is bounded here.
	ctx, cancel := context.WithTimeout(context.Background(), backlogScrapeTimeout)
	defer cancel()
	ch <- scrapeError("outbox", c.collectOutbox(ctx, ch), c.log)
	ch <- scrapeError("case_export", c.collectExports(ctx, ch), c.log)
}

func (c backlogCollector) collectOutbox(ctx context.Context, ch chan<- prometheus.Metric) error {
	status, err := c.core.OutboxStatus(ctx)
	if err != nil {
		return err
	}
	ch <- prometheus.MustNewConstMetric(outboxEventsDesc, prometheus.GaugeValue, float64(status.Pending), "pending")
	ch <- prometheus.MustNewConstMetric(outboxEventsDesc, prometheus.GaugeValue, float64(status.Failed), "failed")
	ch <- prometheus.MustNewConstMetric(outboxEventsDesc, prometheus.GaugeValue, float64(status.Published), "published")
	age := 0.0
	if status.OldestPendingAt != nil {
		age = time.Since(*status.OldestPendingAt).Seconds()
	}
	ch <- prometheus.MustNewConstMetric(outboxOldestDesc, prometheus.GaugeValue, age)
	for _, consumer := range status.Consumers {
		ch <- prometheus.MustNewConstMetric(outboxConsumerDesc, prometheus.GaugeValue, float64(consumer.Failed), consumer.Name)
	}
	return nil
}

func (c backlogCollector) collectExports(ctx context.Context, ch chan<- prometheus.Metric) error {
	queued, running, err := c.exports.QueueCounts(ctx)
	if err != nil {
		return err
	}
	ch <- prometheus.MustNewConstMetric(caseExportJobsDesc, prometheus.GaugeValue, float64(queued), "QUEUED")
	ch <- prometheus.MustNewConstMetric(caseExportJobsDesc, prometheus.GaugeValue, float64(running), "RUNNING")
	return nil
}

// scrapeError reports whether reading a backlog failed, and logs why.
func scrapeError(backlog string, err error, log *slog.Logger) prometheus.Metric {
	value := 0.0
	if err != nil {
		value = 1
		log.Warn("metrics: backlog read failed", "backlog", backlog, "error", err)
	}
	return prometheus.MustNewConstMetric(backlogScrapeErrDesc, prometheus.GaugeValue, value, backlog)
}
