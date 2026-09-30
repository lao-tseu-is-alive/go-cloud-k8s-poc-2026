package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/lao-tseu-is-alive/go-cloud-k8s-poc-2026/gen/goeland/v1/goelandv1connect"
)

// allServices are the services the server mounts.
var allServices = []string{
	goelandv1connect.CoreServiceName, goelandv1connect.DocumentServiceName, goelandv1connect.ActorServiceName,
	goelandv1connect.CaseServiceName, goelandv1connect.ThingServiceName, goelandv1connect.TimelineServiceName,
	goelandv1connect.OrgUnitServiceName, goelandv1connect.TaskServiceName, goelandv1connect.CirculationServiceName,
}

func TestRestRoutesPickTheMostSpecificBinding(t *testing.T) {
	routes, err := restRoutes(allServices)
	if err != nil || len(routes) < 80 {
		t.Fatalf("every annotated RPC has a route: %d routes (%v)", len(routes), err)
	}
	cases := []struct {
		method, path, request string
	}{
		{http.MethodGet, "/api/cases/search", "goeland.v1.SearchCasesRequest"},
		{http.MethodGet, "/api/cases/0b7a4f7e-0000-0000-0000-000000000000", "goeland.v1.GetCaseRequest"},
		{http.MethodGet, "/api/subjects:lookup", "goeland.v1.LookupSubjectsRequest"},
		{http.MethodGet, "/api/subjects/x/relationships", "goeland.v1.ListRelationshipsRequest"},
		{http.MethodDelete, "/api/relationships/x", "goeland.v1.UnlinkSubjectsRequest"},
	}
	for _, c := range cases {
		route := routeFor(routes, c.method, splitPath(c.path))
		if route == nil || string(route.request.FullName()) != c.request {
			t.Fatalf("%s %s: got %v, want %s", c.method, c.path, route, c.request)
		}
	}
	if routeFor(routes, http.MethodGet, splitPath("/api/no/such/route")) != nil {
		t.Fatal("an unknown path matches no route")
	}
	if routeFor(routes, http.MethodPut, splitPath("/api/cases/search")) != nil {
		t.Fatal("the method is part of the match")
	}
}

func TestUnknownQueryParamsMiddleware(t *testing.T) {
	routes, err := restRoutes(allServices)
	if err != nil {
		t.Fatalf("routes: %v", err)
	}
	passed := false
	handler := unknownQueryParamsMiddleware(routes, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		passed = true
		w.WriteHeader(http.StatusOK)
	}))
	serve := func(method, target string) *httptest.ResponseRecorder {
		passed = false
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, httptest.NewRequest(method, target, nil))
		return rec
	}
	for _, target := range []string{
		"/api/cases/search?query=x&pageSize=5&page_size=5&caseTypeCode=OPC", // JSON and proto names
		"/api/cases/search?status=NOPE",                                     // values are the transcoder's business
		"/api/cases/search",                                                 // no query
		"/api/no/such/route?x=1",                                            // no binding: left to the transcoder
		"/api/cases/0b7a4f7e-0000-0000-0000-000000000000?id=y",              // a path variable is a field too
	} {
		if rec := serve(http.MethodGet, target); !passed || rec.Code != http.StatusOK {
			t.Fatalf("GET %s must reach the transcoder: %d", target, rec.Code)
		}
	}
	for _, target := range []string{"/api/cases/search?bogus=1", "/api/subjects/x/relationships?includeEnded=true", "/api/cases/search?query.x=1"} {
		rec := serve(http.MethodGet, target)
		var body map[string]any
		_ = json.Unmarshal(rec.Body.Bytes(), &body)
		if passed || rec.Code != http.StatusBadRequest || body["code"] != float64(3) || !strings.Contains(body["message"].(string), "unknown query parameter") {
			t.Fatalf("GET %s: want 400 INVALID_ARGUMENT, got %d %v (passed=%v)", target, rec.Code, body, passed)
		}
	}
}
