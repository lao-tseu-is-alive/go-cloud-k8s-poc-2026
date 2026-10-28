package main

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/testutil"
)

// newTestHTTPMetrics mounts the services' prefixes and a few literal routes on
// a mux, behind the request metrics.
func newTestHTTPMetrics(t *testing.T) (*httpMetrics, http.Handler) {
	t.Helper()
	routes, err := restRoutes(allServices)
	if err != nil {
		t.Fatal(err)
	}
	m, err := newHTTPMetrics(prometheus.NewRegistry(), routes, allServices)
	if err != nil {
		t.Fatal(err)
	}
	ok := http.HandlerFunc(func(http.ResponseWriter, *http.Request) {})
	mux := http.NewServeMux()
	mux.Handle("GET /health", ok)
	mux.Handle("POST /api/documents/upload", ok)
	// Like the transcoder, rewrite the path in place.
	mux.Handle("/api/", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		r.URL.Path = "/goeland.v1.CoreService/Rewritten"
		http.NotFound(w, r)
	}))
	for _, name := range allServices {
		mux.Handle("/"+name+"/", ok)
	}
	mux.Handle("/", ok)
	return m, m.middleware(mux)
}

func TestRequestMetricsNameBoundedHandlers(t *testing.T) {
	m, handler := newTestHTTPMetrics(t)
	cases := []struct{ method, path, handler, code string }{
		{http.MethodGet, "/api/cases/0b7f5a52-34f6-4a5a-9d55-2f6b1f3a0c11", "goeland.v1.CaseService/GetCase", "404"},
		{http.MethodGet, "/api/cases/search", "goeland.v1.CaseService/SearchCases", "404"},
		{http.MethodPost, "/goeland.v1.CaseService/GetCase", "goeland.v1.CaseService/GetCase", "200"},
		{http.MethodPost, "/goeland.v1.CaseService/NoSuchMethod", unmatchedHandler, "200"},
		{http.MethodGet, "/api/no/such/route", unmatchedHandler, "404"},
		{http.MethodGet, "/health", "GET /health", "200"},
		{http.MethodPost, "/api/documents/upload", "POST /api/documents/upload", "200"},
		{http.MethodGet, "/cases/0b7f5a52-34f6-4a5a-9d55-2f6b1f3a0c11", "/", "200"},
		{"BREW", "/health", "/", "200"},
	}
	for _, c := range cases {
		handler.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(c.method, c.path, nil))
		method := metricMethod(c.method)
		if got := testutil.ToFloat64(m.requests.WithLabelValues(c.handler, method, c.code)); got != 1 {
			t.Errorf("%s %s: want one request as (%s, %s, %s), got %v", c.method, c.path, c.handler, method, c.code, got)
		}
	}
	if got := testutil.CollectAndCount(m.duration); got != len(cases) {
		t.Errorf("want one duration series per case, got %d", got)
	}
}
