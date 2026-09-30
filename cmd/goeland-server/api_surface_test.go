package main

import (
	"fmt"
	"math/rand/v2"
	"net/url"
	"strings"
	"testing"

	"github.com/google/uuid"
)

// TestAPISurface calls, over REST against the real handler, every RPC the §50
// scenario does not reach: reads, searches, updates, deletions, the reference
// catalogues and the not-found mapping of each adapter. It proves each REST
// binding exists and that the adapters parse requests and map domain errors
// (GLD-042). Same database requirement as TestScenarioV2.
func TestAPISurface(t *testing.T) {
	s := &surface{e2e: newE2E(t), token: strings.ReplaceAll(uuid.NewString(), "-", "")[:10]}
	s.catalogues()
	s.core()
	s.actors()
	s.cases()
	s.things()
	s.documents()
	s.orgUnits()
	s.tasksAndCirculations()
}

// gRPC status codes asserted by the surface test.
const (
	codeNotFound      = 5
	codeAlreadyExists = 6
)

// surface carries the run token and the subjects shared by the sections.
type surface struct {
	*e2e
	token  string
	caseID string
}

// code is an upper-case reference code unique to this run.
func (s *surface) code(prefix string) string {
	return prefix + "_" + strings.ToUpper(s.token)
}

// missing is the not-found path of a resource under prefix (a random id).
func missing(prefix string) string {
	return prefix + "/" + uuid.NewString()
}

// catalogues creates, renames and deactivates one entry of every reference
// catalogue, and lists them.
func (s *surface) catalogues() {
	for _, path := range []string{"/api/organization-categories", "/api/case-types", "/api/document-types", "/api/org-unit-types", "/api/task-types", "/api/thing-types"} {
		code := s.code("E2E")
		body := map[string]any{"code": code, "label": "Catalogue E2E", "reason": "test"}
		if path == "/api/case-types" {
			body["businessRefNamespace"] = "E2E" + strings.ToUpper(s.token[:4])
		}
		s.ok("POST", path, body)
		s.fails(codeAlreadyExists, "POST", path, body)
		s.ok("PATCH", path+"/"+code, map[string]any{"label": "Catalogue E2E renommé", "isActive": false, "reason": "test"})
		all := s.ok("GET", path, nil)
		if len(all) == 0 {
			s.t.Fatalf("GET %s: empty reply", path)
		}
		s.fails(codeNotFound, "PATCH", path+"/NO_SUCH_CODE", map[string]any{"label": "x", "reason": "test"})
	}
	s.ok("GET", "/api/relationship-types", nil)
}

// core covers subjects, business references, lookups, relationships and users.
func (s *surface) core() {
	subject := s.ok("POST", "/api/subjects", map[string]any{"kind": "SUBJECT_KIND_CASE", "displayLabel": "Sujet E2E " + s.token})
	id := str(subject, "subjectRef", "id")
	s.ok("GET", "/api/subjects/"+id, nil)
	s.fails(codeNotFound, "GET", missing("/api/subjects"), nil)
	ref := "E2E-" + s.token
	s.ok("POST", "/api/subjects/"+id+"/business-ref", map[string]any{"businessRef": map[string]any{"namespace": "E2E", "value": ref}, "reason": "test"})
	found := s.ok("GET", "/api/subjects:lookup?businessRef="+ref+"&namespace=E2E", nil)
	if len(list(found, "subjectRefs")) != 1 {
		s.t.Fatalf("lookup by business reference: %v", found)
	}

	s.caseID = str(s.ok("POST", "/api/cases", map[string]any{"caseTypeCode": "GENERIC_REQUEST", "title": "Surface " + s.token}), "case", "subjectRef", "id")
	other := str(s.ok("POST", "/api/cases", map[string]any{"caseTypeCode": "GENERIC_REQUEST", "title": "Liée " + s.token}), "case", "subjectRef", "id")
	link := func() string {
		return str(s.ok("POST", "/api/relationships", map[string]any{"sourceSubjectId": s.caseID, "targetSubjectId": other, "relationshipTypeCode": "CASE_RELATED_TO_CASE"}), "relationship", "id")
	}
	ended := link()
	s.ok("POST", "/api/relationships/"+ended+"/end", map[string]any{"reason": "terminée"})
	mistaken := link()
	s.ok("DELETE", "/api/relationships/"+mistaken+"?reason=erreur", nil)
	s.fails(codeNotFound, "POST", missing("/api/relationships")+"/end", map[string]any{"reason": "x"})
	rels := s.ok("GET", "/api/subjects/"+s.caseID+"/relationships?outgoing=true", nil)
	if len(list(rels, "relationships")) == 0 {
		s.t.Fatalf("relationships listed: %v", rels)
	}

	me := s.ok("GET", "/api/me", nil)
	userID := str(me, "user", "id")
	users := s.ok("GET", "/api/users:batchGet?userIds="+url.QueryEscape(userID), nil)
	if len(list(users, "users")) != 1 {
		s.t.Fatalf("batch get users: %v", users)
	}
	s.ok("GET", "/api/users/search?query=Admin", nil)
}

// actors covers read, search, update and deletion of an organization.
func (s *surface) actors() {
	id := str(s.ok("POST", "/api/actors", map[string]any{
		"actorKind": "ACTOR_KIND_ORGANIZATION", "displayName": "Surface SA " + s.token, "organization": map[string]any{"legalName": "Surface SA " + s.token},
	}), "actor", "subjectRef", "id")
	s.ok("GET", "/api/actors/"+id, nil)
	s.fails(codeNotFound, "GET", missing("/api/actors"), nil)
	s.ok("PATCH", "/api/actors/"+id, map[string]any{"displayName": "Surface Sàrl " + s.token, "isActive": true, "reason": "test"})
	found := s.ok("GET", "/api/actors/search?query="+s.token, nil)
	if len(list(found, "actors")) != 1 {
		s.t.Fatalf("actor search: %v", found)
	}
	s.ok("DELETE", "/api/actors/"+id+"?reason=test", nil)
	s.fails(codeNotFound, "DELETE", missing("/api/actors")+"?reason=test", nil)
}

// cases covers read, search and deletion of a case.
func (s *surface) cases() {
	s.ok("GET", "/api/cases/"+s.caseID, nil)
	s.fails(codeNotFound, "GET", missing("/api/cases"), nil)
	found := s.ok("GET", "/api/cases/search?query=Surface+"+s.token, nil)
	if len(list(found, "cases")) == 0 {
		s.t.Fatalf("case search: %v", found)
	}
	doomed := str(s.ok("POST", "/api/cases", map[string]any{"caseTypeCode": "GENERIC_REQUEST", "title": "Supprimée " + s.token}), "case", "subjectRef", "id")
	s.ok("DELETE", "/api/cases/"+doomed+"?reason=test", nil)
}

// things covers read, update, search and deletion of a parcel.
func (s *surface) things() {
	e0 := 2_540_000 + float64(rand.IntN(1000))*10
	id := str(s.ok("POST", "/api/things", map[string]any{
		"thingTypeCode": "PARCEL", "geometryGeojson": square(e0, 1_155_000, 40),
		"parcel": map[string]any{"communeOfs": 5586, "parcelNumber": fmt.Sprintf("S%s", s.token)},
	}), "thing", "subjectRef", "id")
	s.ok("GET", "/api/things/"+id, nil)
	s.fails(codeNotFound, "GET", missing("/api/things"), nil)
	s.ok("PATCH", "/api/things/"+id, map[string]any{
		"name": "Parcelle surface", "description": "Parcelle mise à jour", "geometryGeojson": square(e0, 1_155_000, 40),
		"parcel": map[string]any{"communeOfs": 5586, "parcelNumber": fmt.Sprintf("S%s", s.token)}, "reason": "test",
	})
	bbox := fmt.Sprintf("%g,1154990,%g,1155050", e0-10, e0+50)
	found := s.ok("GET", "/api/things/search?bbox="+url.QueryEscape(bbox), nil)
	if len(list(found, "things")) == 0 {
		s.t.Fatalf("thing search by bbox: %v", found)
	}
	s.ok("DELETE", "/api/things/"+id+"?reason=test", nil)
}

// documents covers versions, metadata, finalization, integrity, search, links and deletion.
func (s *surface) documents() {
	sc := &scenario{e2e: s.e2e}
	blob, _ := sc.upload("%PDF surface v1 " + s.token)
	id := str(s.ok("POST", "/api/documents", map[string]any{"documentTypeCode": "PLAN", "title": "Plan surface " + s.token, "contentBlobId": blob}), "document", "subjectRef", "id")
	s.ok("GET", "/api/documents/"+id, nil)
	s.fails(codeNotFound, "GET", missing("/api/documents"), nil)
	s.ok("PATCH", "/api/documents/"+id, map[string]any{"title": "Plan surface révisé " + s.token, "reason": "test"})
	second, _ := sc.upload("%PDF surface v2 " + s.token)
	s.ok("POST", "/api/documents/"+id+"/versions", map[string]any{"contentBlobId": second, "reason": "nouvelle version"})
	author := str(s.ok("POST", "/api/actors", map[string]any{
		"actorKind": "ACTOR_KIND_PERSON", "person": map[string]any{"lastName": "Auteur " + s.token, "firstName": "Ada"},
	}), "actor", "subjectRef", "id")
	s.ok("POST", "/api/documents/"+id+"/links", map[string]any{"targetSubjectId": author, "relationshipTypeCode": "DOCUMENT_AUTHORED_BY_ACTOR"})
	s.ok("POST", "/api/relationships", map[string]any{"sourceSubjectId": s.caseID, "targetSubjectId": id, "relationshipTypeCode": "CASE_HAS_DOCUMENT"})
	found := s.ok("GET", "/api/documents/search?caseId="+s.caseID, nil)
	if len(list(found, "documents")) != 1 {
		s.t.Fatalf("documents of the case: %v", found)
	}
	s.ok("POST", "/api/documents/"+id+"/finalize", map[string]any{"reason": "définitif"})
	integrity := s.ok("GET", "/api/documents/"+id+"/integrity", nil)
	if len(integrity) == 0 {
		s.t.Fatalf("integrity report: %v", integrity)
	}
	s.ok("DELETE", "/api/documents/"+id+"?reason=test", nil)
}

// orgUnits covers the tree, search, read, update and dissolution.
func (s *surface) orgUnits() {
	id := str(s.ok("POST", "/api/org-units", map[string]any{"orgUnitTypeCode": "SERVICE", "label": "Unité surface " + s.token, "abbreviation": "SRF"}), "orgUnit", "subjectRef", "id")
	s.ok("GET", "/api/org-units", nil)
	s.ok("GET", "/api/org-units/"+id, nil)
	s.fails(codeNotFound, "GET", missing("/api/org-units"), nil)
	s.ok("PATCH", "/api/org-units/"+id, map[string]any{"orgUnitTypeCode": "SERVICE", "label": "Unité surface renommée " + s.token, "abbreviation": "SRF", "reason": "test"})
	found := s.ok("GET", "/api/org-units/search?query=surface+renommée+"+s.token, nil)
	if len(list(found, "units")) != 1 {
		s.t.Fatalf("org unit search: %v", found)
	}
	s.ok("POST", "/api/org-units/"+id+"/dissolve", map[string]any{"reason": "réorganisation"})
}

// tasksAndCirculations covers task listing, update, start and cancel, the
// timeline entry read, lock and withdraw, and circulation listing and cancel.
func (s *surface) tasksAndCirculations() {
	me := str(s.ok("GET", "/api/me", nil), "user", "id")
	task := str(s.ok("POST", "/api/cases/"+s.caseID+"/tasks", map[string]any{"taskTypeCode": "DOCUMENT_CHECK", "title": "Tâche surface", "assigneeUserId": me}), "task", "id")
	s.ok("PATCH", "/api/tasks/"+task, map[string]any{"taskTypeCode": "DOCUMENT_CHECK", "title": "Tâche surface révisée", "reason": "test"})
	s.ok("POST", "/api/tasks/"+task+"/start", map[string]any{})
	if len(list(s.ok("GET", "/api/cases/"+s.caseID+"/tasks?statuses=TASK_STATUS_IN_PROGRESS", nil), "tasks")) != 1 {
		s.t.Fatal("the started task is listed by status")
	}
	if len(list(s.ok("GET", "/api/tasks/mine?includeUnits=true", nil), "tasks")) == 0 {
		s.t.Fatal("the assigned task is in \"my tasks\"")
	}
	s.ok("POST", "/api/tasks/"+task+"/cancel", map[string]any{"reason": "inutile"})
	s.fails(codeNotFound, "POST", missing("/api/tasks")+"/start", map[string]any{})

	entry := func() string {
		return str(s.ok("POST", "/api/cases/"+s.caseID+"/timeline", map[string]any{"entryType": "TIMELINE_ENTRY_TYPE_COMMENT", "body": "Note surface"}), "entry", "id")
	}
	locked := entry()
	s.ok("GET", "/api/timeline-entries/"+locked, nil)
	s.fails(codeNotFound, "GET", missing("/api/timeline-entries"), nil)
	s.ok("POST", "/api/timeline-entries/"+locked+"/lock", map[string]any{"reason": "archivage"})
	withdrawn := entry()
	sc := &scenario{e2e: s.e2e}
	blob, _ := sc.upload("%PDF surface cité " + s.token)
	doc := str(s.ok("POST", "/api/documents", map[string]any{"documentTypeCode": "PLAN", "title": "Cité " + s.token, "contentBlobId": blob}), "document", "subjectRef", "id")
	s.ok("POST", "/api/timeline-entries/"+withdrawn+"/documents", map[string]any{"documentId": doc})
	s.ok("DELETE", "/api/timeline-entries/"+withdrawn+"/documents/"+doc, nil)
	s.ok("POST", "/api/timeline-entries/"+withdrawn+"/withdraw", map[string]any{"reason": "doublon"})

	circ := str(s.ok("POST", "/api/cases/"+s.caseID+"/circulations", map[string]any{
		"title": "Circulation surface", "recipients": []map[string]any{{"assigneeUserId": me}},
	}), "circulation", "id")
	if len(list(s.ok("GET", "/api/cases/"+s.caseID+"/circulations", nil), "circulations")) != 1 {
		s.t.Fatal("the circulation is listed on its case")
	}
	s.ok("POST", "/api/circulations/"+circ+"/cancel", map[string]any{"reason": "retirée"})
	s.fails(codeNotFound, "POST", missing("/api/circulations")+"/cancel", map[string]any{"reason": "x"})
}
