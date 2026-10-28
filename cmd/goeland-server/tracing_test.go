package main

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"go.opentelemetry.io/otel"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
)

// recordSpans installs a tracer provider recording in memory for the test.
func recordSpans(t *testing.T) *tracetest.SpanRecorder {
	t.Helper()
	recorder := tracetest.NewSpanRecorder()
	previous := otel.GetTracerProvider()
	otel.SetTracerProvider(sdktrace.NewTracerProvider(sdktrace.WithSpanProcessor(recorder)))
	t.Cleanup(func() { otel.SetTracerProvider(previous) })
	return recorder
}

func TestRequestSpanIsNamedAfterItsRPC(t *testing.T) {
	recorder := recordSpans(t)
	_, handler := newTestHTTPMetrics(t)
	traced := tracingMiddleware(handler)
	for _, path := range []string{"/api/cases/search", "/health"} {
		traced.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, path, nil))
	}
	spans := recorder.Ended()
	if len(spans) != 2 {
		t.Fatalf("want 2 spans, got %d", len(spans))
	}
	for i, want := range []string{"GET goeland.v1.CaseService/SearchCases", "GET /health"} {
		if got := spans[i].Name(); got != want {
			t.Errorf("span %d: want %q, got %q", i, want, got)
		}
	}
}

func TestTraceLogHandlerAddsTheTraceIDs(t *testing.T) {
	recordSpans(t)
	var out bytes.Buffer
	log := slog.New(traceLogHandler{slog.NewJSONHandler(&out, nil)}).With("component", "test")
	ctx, span := otel.Tracer(tracerName).Start(context.Background(), "op")
	log.InfoContext(ctx, "traced")
	span.End()
	log.InfoContext(context.Background(), "untraced")

	lines := bytes.Split(bytes.TrimSpace(out.Bytes()), []byte("\n"))
	var traced, untraced map[string]any
	if err := json.Unmarshal(lines[0], &traced); err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(lines[1], &untraced); err != nil {
		t.Fatal(err)
	}
	if traced["trace_id"] != span.SpanContext().TraceID().String() || traced["span_id"] == nil || traced["component"] != "test" {
		t.Errorf("traced record lacks its ids: %v", traced)
	}
	if _, ok := untraced["trace_id"]; ok {
		t.Errorf("untraced record has a trace id: %v", untraced)
	}
}

func TestTracingIsOffWithoutAnOTLPEndpoint(t *testing.T) {
	cases := []struct {
		endpoint, traces, disabled string
		want                       bool
	}{
		{"", "", "", false},
		{"http://collector:4318", "", "", true},
		{"", "http://collector:4318/v1/traces", "", true},
		{"http://collector:4318", "", "TRUE", false},
	}
	for _, c := range cases {
		t.Setenv("OTEL_EXPORTER_OTLP_ENDPOINT", c.endpoint)
		t.Setenv("OTEL_EXPORTER_OTLP_TRACES_ENDPOINT", c.traces)
		t.Setenv("OTEL_SDK_DISABLED", c.disabled)
		if got := tracingEnabled(); got != c.want {
			t.Errorf("%+v: want %v, got %v", c, c.want, got)
		}
	}
}
