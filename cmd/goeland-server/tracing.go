package main

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"strings"

	"github.com/jackc/pgx/v5"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.43.0"
	"go.opentelemetry.io/otel/trace"

	"github.com/lao-tseu-is-alive/go-cloud-k8s-poc-2026/pkg/version"
)

// OpenTelemetry traces (GLD-010): a span per HTTP request, named by its RPC or
// route pattern like the metrics, and a child span per SQL query. Off unless
// the standard OTEL_EXPORTER_OTLP_ENDPOINT or OTEL_EXPORTER_OTLP_TRACES_ENDPOINT
// names an OTLP/HTTP collector; the other OTEL_* variables (service name,
// sampler, headers...) apply as the SDK reads them.

// tracerName names the instrumentation scope of the server's own spans.
const tracerName = "github.com/lao-tseu-is-alive/go-cloud-k8s-poc-2026/cmd/goeland-server"

// tracingEnabled reports whether an OTLP endpoint is configured and the SDK
// is not disabled (OTEL_SDK_DISABLED=true).
func tracingEnabled() bool {
	if strings.EqualFold(strings.TrimSpace(os.Getenv("OTEL_SDK_DISABLED")), "true") {
		return false
	}
	return os.Getenv("OTEL_EXPORTER_OTLP_ENDPOINT") != "" || os.Getenv("OTEL_EXPORTER_OTLP_TRACES_ENDPOINT") != ""
}

// setupTracing installs the global tracer provider and the W3C propagators
// when tracing is enabled, and returns the function flushing the spans at
// shutdown (a no-op when tracing is off).
func setupTracing(ctx context.Context, log *slog.Logger) (func(context.Context) error, error) {
	if !tracingEnabled() {
		return func(context.Context) error { return nil }, nil
	}
	exporter, err := otlptracehttp.New(ctx)
	if err != nil {
		return nil, fmt.Errorf("OTLP trace exporter: %w", err)
	}
	res, err := resource.New(ctx,
		resource.WithAttributes(semconv.ServiceName(version.AppNameKebab), semconv.ServiceVersion(version.Version)),
		resource.WithFromEnv(), // OTEL_SERVICE_NAME and OTEL_RESOURCE_ATTRIBUTES win
		resource.WithTelemetrySDK(),
		resource.WithHost(),
	)
	if err != nil {
		return nil, fmt.Errorf("trace resource: %w", err)
	}
	provider := sdktrace.NewTracerProvider(sdktrace.WithBatcher(exporter), sdktrace.WithResource(res))
	otel.SetTracerProvider(provider)
	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(propagation.TraceContext{}, propagation.Baggage{}))
	log.Info("OpenTelemetry traces enabled")
	return provider.Shutdown, nil
}

// tracingMiddleware starts the span of each request, continuing the caller's
// W3C trace context; httpMetrics.middleware names it once the route is known.
// It records the method, the route and the status, never the URL: paths carry
// ids and queries carry search text. With tracing off, the global provider is
// a no-op one.
func tracingMiddleware(next http.Handler) http.Handler {
	tracer := otel.Tracer(tracerName)
	return http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		ctx := otel.GetTextMapPropagator().Extract(request.Context(), propagation.HeaderCarrier(request.Header))
		ctx, span := tracer.Start(ctx, request.Method, trace.WithSpanKind(trace.SpanKindServer),
			trace.WithAttributes(semconv.HTTPRequestMethodKey.String(metricMethod(request.Method))))
		defer span.End()
		rec := &statusRecorder{ResponseWriter: writer, status: http.StatusOK}
		next.ServeHTTP(rec, request.WithContext(ctx))
		span.SetAttributes(semconv.HTTPResponseStatusCode(rec.status))
		if rec.status >= http.StatusInternalServerError {
			span.SetStatus(codes.Error, http.StatusText(rec.status))
		}
	})
}

// nameSpan renames the request span after the RPC or route pattern that served it.
func nameSpan(ctx context.Context, method, handler string) {
	span := trace.SpanFromContext(ctx)
	if !span.IsRecording() {
		return
	}
	name := handler
	if !strings.HasPrefix(handler, method+" ") { // "GET /health" already names its method
		name = method + " " + handler
	}
	span.SetName(name)
	span.SetAttributes(semconv.HTTPRoute(handler))
}

// queryTracer records a span per SQL query: the statement text, never its
// arguments (they carry business data).
type queryTracer struct {
	tracer trace.Tracer
}

// newQueryTracer returns the pgx tracer, or nil when tracing is off.
func newQueryTracer() pgx.QueryTracer {
	if !tracingEnabled() {
		return nil
	}
	return queryTracer{tracer: otel.Tracer(tracerName)}
}

// TraceQueryStart starts the query span when the query runs inside a traced request.
func (t queryTracer) TraceQueryStart(ctx context.Context, _ *pgx.Conn, data pgx.TraceQueryStartData) context.Context {
	if !trace.SpanFromContext(ctx).SpanContext().IsValid() {
		return ctx // a worker's query: no request to attach it to
	}
	ctx, _ = t.tracer.Start(ctx, "db.query", trace.WithSpanKind(trace.SpanKindClient),
		trace.WithAttributes(semconv.DBSystemNamePostgreSQL, semconv.DBQueryText(strings.TrimSpace(data.SQL))))
	return ctx
}

// TraceQueryEnd ends the query span, marking a failure.
func (queryTracer) TraceQueryEnd(ctx context.Context, _ *pgx.Conn, data pgx.TraceQueryEndData) {
	span := trace.SpanFromContext(ctx)
	if data.Err != nil {
		span.RecordError(data.Err)
		span.SetStatus(codes.Error, "query failed")
	}
	span.End()
}

// traceLogHandler adds the trace and span ids to the records logged with a
// traced context, so logs and traces join.
type traceLogHandler struct {
	slog.Handler
}

// Handle adds trace_id and span_id when the context carries a span.
func (h traceLogHandler) Handle(ctx context.Context, record slog.Record) error {
	if sc := trace.SpanContextFromContext(ctx); sc.IsValid() {
		record.AddAttrs(slog.String("trace_id", sc.TraceID().String()), slog.String("span_id", sc.SpanID().String()))
	}
	return h.Handler.Handle(ctx, record)
}

// WithAttrs keeps the trace ids on derived loggers.
func (h traceLogHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	return traceLogHandler{h.Handler.WithAttrs(attrs)}
}

// WithGroup keeps the trace ids on derived loggers.
func (h traceLogHandler) WithGroup(name string) slog.Handler {
	return traceLogHandler{h.Handler.WithGroup(name)}
}
