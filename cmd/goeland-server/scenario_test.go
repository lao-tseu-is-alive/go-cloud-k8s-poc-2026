package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"math/rand/v2"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
)

// The end-to-end scenario of spec v2 §50, played over HTTP against the real
// server handler exactly as the SPA does (REST JSON through the Vanguard
// transcoder, dev-mode bearer tokens). It covers what the domain integration
// tests cannot: authentication and scopes, protovalidate, error mapping, the
// REST bindings and the module wiring. It needs a disposable PostGIS database:
//
//	GOELAND_TEST_DATABASE_URL='postgres://postgres@127.0.0.1:5432/goeland_e2e?sslmode=disable' \
//	    go test ./cmd/goeland-server -run TestScenarioV2
//
// Steps 26-27 (AI proposal, GLD-030) and 31 (ExportCase, GLD-029) are not built
// yet and are logged as skipped.

const (
	adminToken  = "e2e-admin-token"
	writerToken = "e2e-writer-token"
)

// e2e is a running server and a JSON client for it.
type e2e struct {
	t      *testing.T
	admin  *httptest.Server
	writer *httptest.Server
}

// newE2E starts two servers on the test database: one whose dev user is an
// administrator, one whose dev user only reads and writes.
func newE2E(t *testing.T) *e2e {
	t.Helper()
	dsn := strings.TrimSpace(os.Getenv("GOELAND_TEST_DATABASE_URL"))
	if dsn == "" {
		t.Skip("set GOELAND_TEST_DATABASE_URL to run the end-to-end scenario (needs PostGIS)")
	}
	start := func(token string, userID int64, name string, admin bool) *httptest.Server {
		cfg := serverConfig{
			DatabaseURL: dsn, AuthMode: "dev", DevToken: token, DevUserID: userID, DevUserEmail: name + "@e2e.test",
			DevDisplayName: name, DevUserAdmin: admin, MaxConnections: 4, RequestTimeout: 10 * time.Second,
			DocumentPath: t.TempDir(), MaxUploadBytes: 1 << 20, ShutdownPeriod: time.Second,
		}
		app, err := newApplication(context.Background(), cfg, slog.New(slog.NewTextHandler(io.Discard, nil)))
		if err != nil {
			t.Fatalf("start server: %v", err)
		}
		srv := httptest.NewServer(app.handler)
		t.Cleanup(func() { srv.Close(); app.close() })
		return srv
	}
	base := 900_000 + rand.Int64N(90_000)
	return &e2e{
		t:      t,
		admin:  start(adminToken, base, "Admin E2E", true),
		writer: start(writerToken, base+1, "Writer E2E", false),
	}
}

// reply is a decoded JSON response.
type reply struct {
	status int
	body   map[string]any
}

// call sends a JSON request to srv with token ("" for none) and decodes the reply.
func (e *e2e) call(srv *httptest.Server, token, method, path string, body any) reply {
	e.t.Helper()
	var reader io.Reader
	if body != nil {
		raw, err := json.Marshal(body)
		if err != nil {
			e.t.Fatalf("encode %s %s: %v", method, path, err)
		}
		reader = bytes.NewReader(raw)
	}
	req, err := http.NewRequest(method, srv.URL+path, reader)
	if err != nil {
		e.t.Fatalf("request %s %s: %v", method, path, err)
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	return e.send(req)
}

// send runs a request and decodes a JSON object reply (an empty object otherwise).
func (e *e2e) send(req *http.Request) reply {
	e.t.Helper()
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		e.t.Fatalf("%s %s: %v", req.Method, req.URL.Path, err)
	}
	defer func() { _ = resp.Body.Close() }()
	raw, _ := io.ReadAll(resp.Body)
	out := reply{status: resp.StatusCode, body: map[string]any{}}
	_ = json.Unmarshal(raw, &out.body)
	return out
}

// ok calls as the administrator and requires a 2xx reply.
func (e *e2e) ok(method, path string, body any) map[string]any {
	e.t.Helper()
	r := e.call(e.admin, adminToken, method, path, body)
	if r.status/100 != 2 {
		e.t.Fatalf("%s %s: HTTP %d %v", method, path, r.status, r.body)
	}
	return r.body
}

// fails calls as the administrator and requires the given gRPC status code
// (the REST error body carries it as "code").
func (e *e2e) fails(code int, method, path string, body any) {
	e.t.Helper()
	e.expect(e.call(e.admin, adminToken, method, path, body), code, method+" "+path)
}

// expect requires a reply to carry the gRPC status code.
func (e *e2e) expect(r reply, code int, what string) {
	e.t.Helper()
	if got, _ := r.body["code"].(float64); int(got) != code || r.status/100 == 2 {
		e.t.Fatalf("%s: want gRPC code %d, got HTTP %d %v", what, code, r.status, r.body)
	}
}

// gRPC status codes asserted by the scenario.
const (
	codeInvalidArgument    = 3
	codePermissionDenied   = 7
	codeFailedPrecondition = 9
	codeUnauthenticated    = 16
)

// str walks a decoded JSON object along keys and returns the string found.
func str(v any, keys ...string) string {
	for _, k := range keys {
		m, _ := v.(map[string]any)
		v = m[k]
	}
	s, _ := v.(string)
	return s
}

// list walks a decoded JSON object along keys and returns the array found.
func list(v any, keys ...string) []any {
	for _, k := range keys {
		m, _ := v.(map[string]any)
		v = m[k]
	}
	a, _ := v.([]any)
	return a
}

// square is an LV95 GeoJSON square of side metres at (e, n).
func square(e, n, side float64) string {
	return fmt.Sprintf(`{"type":"Polygon","coordinates":[[[%[1]g,%[2]g],[%[3]g,%[2]g],[%[3]g,%[4]g],[%[1]g,%[4]g],[%[1]g,%[2]g]]]}`, e, n, e+side, n+side)
}

// scenario carries the ids shared by the steps.
type scenario struct {
	*e2e
	token                              string
	userID, userSubject                string
	unitA, unitB                       string
	caseA, caseB                       string
	parcel, building                   string
	person, organization               string
	document                           string
	entry, taskA, taskB, circulationID string
}

// TestScenarioV2 plays the v2 §50 scenario end to end.
func TestScenarioV2(t *testing.T) {
	s := &scenario{e2e: newE2E(t), token: strings.ReplaceAll(uuid.NewString(), "-", "")[:10]}
	s.platform()
	s.steps01to04()
	s.steps05to12()
	s.steps13to18()
	s.steps19to21()
	s.steps22to23()
	s.steps24to25()
	t.Log("steps 26-27 (AI proposal with human validation) skipped: GLD-030 not built")
	s.step28()
	s.step29()
	s.step30()
	t.Log("step 31 (ExportCase) skipped: GLD-029 not built")
}

// platform checks the probes, the SPA fallback and authentication.
func (s *scenario) platform() {
	for _, path := range []string{"/health", "/readiness", "/cases/any/deep/link"} {
		resp, err := http.Get(s.admin.URL + path)
		if err != nil || resp.StatusCode != http.StatusOK {
			s.t.Fatalf("GET %s: %v %v", path, resp, err)
		}
		_ = resp.Body.Close()
	}
	s.expect(s.call(s.admin, "", "GET", "/api/me", nil), codeUnauthenticated, "no token")
	s.expect(s.call(s.admin, "not-the-token", "GET", "/api/me", nil), codeUnauthenticated, "wrong token")
}

// steps01to04: internal user, two org units, an OPC case with its business reference.
func (s *scenario) steps01to04() {
	me := s.ok("GET", "/api/me", nil)
	s.userID, s.userSubject = str(me, "user", "id"), str(me, "user", "subjectId")
	if s.userSubject == "" {
		s.t.Fatalf("step 1: the caller is recorded as a USER subject: %v", me)
	}
	unit := func(label string) string {
		return str(s.ok("POST", "/api/org-units", map[string]any{"orgUnitTypeCode": "SERVICE", "label": label + " " + s.token, "abbreviation": "E2E"}), "orgUnit", "subjectRef", "id")
	}
	s.unitA, s.unitB = unit("Service urbanisme"), unit("Service du feu")
	s.expect(s.call(s.writer, writerToken, "POST", "/api/org-units", map[string]any{"orgUnitTypeCode": "SERVICE", "label": "Interdit"}),
		codePermissionDenied, "step 2: org units need goeland:admin")

	c := s.ok("POST", "/api/cases", map[string]any{"caseTypeCode": "OPC_DEMANDE_PC", "title": "Permis E2E " + s.token})
	s.caseA = str(c, "case", "subjectRef", "id")
	if str(c, "case", "subjectRef", "businessRefNamespace") != "OPC" || str(c, "case", "subjectRef", "businessRef") == "" {
		s.t.Fatalf("step 4: the OPC reference is allocated: %v", c)
	}
	s.fails(codeInvalidArgument, "POST", "/api/cases", map[string]any{"caseTypeCode": "OPC_DEMANDE_PC", "title": ""})
}

// steps05to12: parcel, building, person, organization and their links to the case.
func (s *scenario) steps05to12() {
	e0, n0 := 2_538_000+float64(rand.IntN(1000))*10, 1_152_000+float64(rand.IntN(1000))*10
	s.parcel = str(s.ok("POST", "/api/things", map[string]any{
		"thingTypeCode": "PARCEL", "name": "Parcelle E2E", "geometryGeojson": square(e0, n0, 50),
		"parcel": map[string]any{"communeOfs": 5586, "parcelNumber": fmt.Sprintf("E2E%d", rand.IntN(1_000_000))},
	}), "thing", "subjectRef", "id")
	s.building = str(s.ok("POST", "/api/things", map[string]any{
		"thingTypeCode": "BUILDING", "name": "Bâtiment E2E", "geometryGeojson": square(e0+10, n0+10, 20),
		"building": map[string]any{"egid": 100_000_000 + rand.IntN(800_000_000)},
	}), "thing", "subjectRef", "id")
	s.person = str(s.ok("POST", "/api/actors", map[string]any{
		"actorKind": "ACTOR_KIND_PERSON", "person": map[string]any{"lastName": "Requérant " + s.token, "firstName": "Jeanne"},
	}), "actor", "subjectRef", "id")
	s.organization = str(s.ok("POST", "/api/actors", map[string]any{
		"actorKind": "ACTOR_KIND_ORGANIZATION", "displayName": "Architectes E2E " + s.token, "organization": map[string]any{"legalName": "Architectes E2E SA " + s.token},
	}), "actor", "subjectRef", "id")
	for _, l := range [][3]string{
		{s.caseA, s.parcel, "CASE_CONCERNS_THING"},
		{s.caseA, s.building, "CASE_CONCERNS_THING"},
		{s.caseA, s.person, "CASE_HAS_ACTOR_REQUESTER"},
		{s.caseA, s.organization, "CASE_HAS_ACTOR_MANDATEE"},
	} {
		s.link(l[0], l[1], l[2])
	}
	s.fails(codeFailedPrecondition, "POST", "/api/relationships", map[string]any{
		"sourceSubjectId": s.caseA, "targetSubjectId": s.person, "relationshipTypeCode": "CASE_CONCERNS_THING",
	})
}

// link creates a typed relationship.
func (s *scenario) link(source, target, typeCode string) {
	s.ok("POST", "/api/relationships", map[string]any{"sourceSubjectId": source, "targetSubjectId": target, "relationshipTypeCode": typeCode})
}

// upload posts bytes to the out-of-proto upload endpoint and returns the blob id.
func (s *scenario) upload(content string) (string, bool) {
	var buf bytes.Buffer
	w := multipart.NewWriter(&buf)
	part, _ := w.CreateFormFile("file", "plan.pdf")
	_, _ = part.Write([]byte(content))
	_ = w.Close()
	req, _ := http.NewRequest("POST", s.admin.URL+"/api/documents/upload", &buf)
	req.Header.Set("Content-Type", w.FormDataContentType())
	req.Header.Set("Authorization", "Bearer "+adminToken)
	r := s.send(req)
	if r.status != http.StatusOK && r.status != http.StatusCreated {
		s.t.Fatalf("upload: HTTP %d %v", r.status, r.body)
	}
	reused, _ := r.body["reused"].(bool)
	return str(r.body, "contentBlobId"), reused
}

// steps13to18: document, unique content blob, version 1, two cases, represents the building.
func (s *scenario) steps13to18() {
	content := "%PDF plan de bâtiment " + s.token
	blob, _ := s.upload(content)
	doc := s.ok("POST", "/api/documents", map[string]any{"documentTypeCode": "PLAN", "title": "Plan de bâtiment " + s.token, "contentBlobId": blob})
	s.document = str(doc, "document", "subjectRef", "id")
	versions := s.ok("GET", "/api/documents/"+s.document+"/versions", nil)
	if len(list(versions, "versions")) != 1 {
		s.t.Fatalf("step 15: one version: %v", versions)
	}
	again, reused := s.upload(content)
	if again != blob || !reused {
		s.t.Fatalf("step 14: identical bytes reuse the content blob (%s vs %s, reused=%v)", again, blob, reused)
	}
	s.link(s.caseA, s.document, "CASE_HAS_DOCUMENT")
	s.caseB = str(s.ok("POST", "/api/cases", map[string]any{"caseTypeCode": "GENERIC_REQUEST", "title": "Affaire B " + s.token}), "case", "subjectRef", "id")
	s.link(s.caseB, s.document, "CASE_HAS_DOCUMENT")
	s.link(s.document, s.building, "DOCUMENT_REPRESENTS_THING")
}

// steps19to21: a follow-up citing the document, validated then immutable.
func (s *scenario) steps19to21() {
	e := s.ok("POST", "/api/cases/"+s.caseA+"/timeline", map[string]any{
		"entryType": "TIMELINE_ENTRY_TYPE_OPINION", "title": "Préavis", "body": "Préavis favorable sous conditions",
	})
	s.entry = str(e, "entry", "id")
	s.ok("POST", "/api/timeline-entries/"+s.entry+"/documents", map[string]any{"documentId": s.document})
	v := s.ok("POST", "/api/timeline-entries/"+s.entry+"/validate", map[string]any{})
	if str(v, "entry", "status") != "TIMELINE_ENTRY_STATUS_VALIDATED" || str(list(v, "entry", "documents")[0], "documentVersionId") == "" {
		s.t.Fatalf("step 21: validated with the document version pinned: %v", v)
	}
	s.fails(codeFailedPrecondition, "PATCH", "/api/timeline-entries/"+s.entry, map[string]any{"entryType": "TIMELINE_ENTRY_TYPE_OPINION", "body": "réécrit"})
}

// steps22to23: tasks, and a reassignment kept as history.
func (s *scenario) steps22to23() {
	task := func(title string, assignee map[string]any) string {
		body := map[string]any{"taskTypeCode": "DOCUMENT_CHECK", "title": title}
		for k, v := range assignee {
			body[k] = v
		}
		return str(s.ok("POST", "/api/cases/"+s.caseA+"/tasks", body), "task", "id")
	}
	s.taskA = task("Contrôler le dossier", map[string]any{"assigneeUserId": s.userID})
	s.taskB = task("Vérifier les plans", map[string]any{"assigneeOrgUnitId": s.unitA})
	s.ok("POST", "/api/tasks/"+s.taskB+"/assign", map[string]any{"assigneeOrgUnitId": s.unitB, "reason": "compétence du service du feu"})
	history := s.ok("GET", "/api/tasks/"+s.taskB, nil)
	if len(list(history, "task", "assignments")) != 2 {
		s.t.Fatalf("step 23: the reassignment is kept as history: %v", history)
	}
}

// steps24to25: a circulation to the two units, answers recorded in the timeline.
func (s *scenario) steps24to25() {
	c := s.ok("POST", "/api/cases/"+s.caseA+"/circulations", map[string]any{
		"title": "Préavis des services", "recipients": []map[string]any{{"assigneeOrgUnitId": s.unitA}, {"assigneeOrgUnitId": s.unitB}},
	})
	s.circulationID = str(c, "circulation", "id")
	for _, rec := range list(c, "circulation", "recipients") {
		s.ok("POST", "/api/circulation-recipients/"+str(rec, "id")+"/respond", map[string]any{"response": "CIRCULATION_RESPONSE_FAVORABLE", "text": "Sans remarque"})
	}
	done := s.ok("GET", "/api/circulations/"+s.circulationID, nil)
	if str(done, "circulation", "status") != "CIRCULATION_STATUS_COMPLETED" {
		s.t.Fatalf("step 25: the circulation completes: %v", done)
	}
	responses := s.ok("GET", "/api/cases/"+s.caseA+"/timeline?entryTypes=TIMELINE_ENTRY_TYPE_RESPONSE", nil)
	if len(list(responses, "entries")) != 2 {
		s.t.Fatalf("step 25: each answer is a timeline entry: %v", responses)
	}
}

// step28: close the case once its tasks are done (closing earlier is refused).
func (s *scenario) step28() {
	closing := map[string]any{"targetStatus": "CASE_STATUS_CLOSED", "reason": "Permis délivré"}
	s.fails(codeFailedPrecondition, "POST", "/api/cases/"+s.caseA+"/transition", closing)
	for _, id := range []string{s.taskA, s.taskB} {
		s.ok("POST", "/api/tasks/"+id+"/complete", map[string]any{"note": "fait"})
	}
	closed := s.ok("POST", "/api/cases/"+s.caseA+"/transition", closing)
	if str(closed, "case", "status") != "CASE_STATUS_CLOSED" {
		s.t.Fatalf("step 28: closed: %v", closed)
	}
}

// step29: a closed case refuses every change.
func (s *scenario) step29() {
	s.fails(codeFailedPrecondition, "PATCH", "/api/cases/"+s.caseA, map[string]any{"title": "Réécrit"})
	s.fails(codeFailedPrecondition, "POST", "/api/cases/"+s.caseA+"/timeline", map[string]any{"entryType": "TIMELINE_ENTRY_TYPE_COMMENT", "body": "trop tard"})
	s.fails(codeFailedPrecondition, "POST", "/api/cases/"+s.caseA+"/tasks", map[string]any{"taskTypeCode": "OTHER", "title": "trop tard"})
	s.fails(codeFailedPrecondition, "POST", "/api/tasks/"+s.taskA+"/reopen", map[string]any{"reason": "oups"})
	s.fails(codeFailedPrecondition, "POST", "/api/cases/"+s.caseA+"/circulations", map[string]any{
		"title": "trop tard", "recipients": []map[string]any{{"assigneeOrgUnitId": s.unitA}},
	})
}

// step30: the case audit trail holds the whole history.
func (s *scenario) step30() {
	audit := s.ok("GET", "/api/subjects/"+s.caseA+"/audit?pageSize=200", nil)
	seen := map[string]bool{}
	for _, ev := range list(audit, "events") {
		seen[str(ev, "eventType")] = true
	}
	for _, want := range []string{
		"CASE_CREATED", "RELATIONSHIP_LINKED", "TIMELINE_ENTRY_ADDED", "TIMELINE_DOCUMENT_LINKED", "TIMELINE_ENTRY_VALIDATED",
		"TASK_CREATED", "TASK_ASSIGNED", "TASK_COMPLETED", "CIRCULATION_CREATED", "CIRCULATION_RESPONDED",
		"CIRCULATION_COMPLETED", "CASE_STATUS_CHANGED",
	} {
		if !seen[want] {
			s.t.Errorf("step 30: the case audit misses %s (has %v)", want, seen)
		}
	}
}
