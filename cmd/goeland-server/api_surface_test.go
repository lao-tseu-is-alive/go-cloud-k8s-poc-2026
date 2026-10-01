package main

import (
	"fmt"
	"math/rand/v2"
	"net/http"
	"net/url"
	"slices"
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
	s.access()
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
	s.caseTypeDefaults()
	s.strictJSON()
}

// caseTypeDefaults covers the default access of a case type (GLD-050).
func (s *surface) caseTypeDefaults() {
	code := s.code("E2EDEF")
	s.ok("POST", "/api/case-types", map[string]any{"code": code, "label": "Défauts E2E", "defaultConfidentialityLevel": 2, "reason": "test"})
	set := s.ok("POST", "/api/case-types/"+code+"/default-grants", map[string]any{
		"grants": []any{map[string]any{"granteeKind": "GRANTEE_KIND_CREATOR_UNITS", "level": "PERMISSION_CONTRIBUTE"}}, "reason": "modèle",
	})
	if grants := list(set["caseType"].(map[string]any), "defaultGrants"); len(grants) != 1 {
		s.t.Fatalf("the template is returned: %v", set)
	}
	s.fails(codeNotFound, "POST", "/api/case-types/NO_SUCH_CODE/default-grants", map[string]any{"reason": "x"})
	s.fails(codeInvalidArgument, "POST", "/api/case-types/"+code+"/default-grants", map[string]any{
		"grants": []any{map[string]any{"granteeKind": "GRANTEE_KIND_GROUP", "granteeId": uuid.NewString(), "level": "PERMISSION_READ"}},
	})
	s.fails(codeInvalidArgument, "POST", "/api/subjects/"+uuid.NewString()+"/grants", map[string]any{
		"granteeKind": "GRANTEE_KIND_CREATOR_UNITS", "granteeId": "x", "level": "PERMISSION_READ", "reason": "x",
	})
	created := s.ok("POST", "/api/cases", map[string]any{"caseTypeCode": code, "title": "Défauts " + s.token})
	if lvl, _ := created["case"].(map[string]any)["recordMetadata"].(map[string]any)["confidentialityLevel"].(float64); lvl != 2 {
		s.t.Fatalf("the type's minimum confidentiality applies: %v", created)
	}
}

// strictJSON checks that a body naming an unknown field is refused on both
// surfaces (GLD-050) instead of being applied without it.
func (s *surface) strictJSON() {
	s.fails(codeInvalidArgument, "POST", "/api/cases", map[string]any{
		"caseTypeCode": "GENERIC_REQUEST", "title": "Faute de frappe", "initialGovernance": map[string]any{"confidentialityLevl": 2},
	})
	req, err := http.NewRequest("POST", s.admin.URL+"/goeland.v1.CaseService/ListCaseTypes", strings.NewReader(`{"onlyActive":true,"onlyActiv":true}`))
	if err != nil {
		s.t.Fatal(err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Connect-Protocol-Version", "1")
	req.Header.Set("Authorization", "Bearer "+adminToken)
	if r := s.send(req); r.status != http.StatusBadRequest || r.body["code"] != "invalid_argument" {
		s.t.Fatalf("an unknown field on the Connect path: HTTP %d %v", r.status, r.body)
	}
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
	s.roles(userID)
}

// roles covers the application roles (GLD-047): the administrator's own ADMIN
// role and scope, a grant and a revocation of the writer, and the refusals.
func (s *surface) roles(adminID string) {
	me := s.ok("GET", "/api/me", nil)
	scopes := list(me, "scopes")
	if !slices.Contains(list(me, "user", "roles"), any("ADMIN")) || !slices.Contains(scopes, any("goeland:admin")) {
		s.t.Fatalf("the bootstrap administrator holds ADMIN and goeland:admin: %v", me)
	}
	if len(list(s.ok("GET", "/api/app-roles", nil), "roles")) == 0 {
		s.t.Fatal("the role catalogue is listed")
	}
	holders := s.ok("GET", "/api/app-roles/ADMIN/holders", nil)
	if !slices.ContainsFunc(list(holders, "users"), func(u any) bool { return str(u, "id") == adminID }) {
		s.t.Fatalf("the administrator is among the holders: %v", holders)
	}
	writer := str(s.call(s.writer, writerToken, "GET", "/api/me", nil).body, "user", "id")
	s.expect(s.call(s.writer, writerToken, "POST", "/api/users/"+writer+"/roles", map[string]any{"roleCode": "ADMIN", "reason": "moi-même"}),
		codePermissionDenied, "a writer cannot grant roles")
	s.ok("POST", "/api/users/"+writer+"/roles", map[string]any{"roleCode": "ADMIN", "reason": "suppléance"})
	s.fails(codeAlreadyExists, "POST", "/api/users/"+writer+"/roles", map[string]any{"roleCode": "ADMIN", "reason": "encore"})
	s.ok("POST", "/api/users/"+writer+"/roles/ADMIN/revoke", map[string]any{"reason": "fin de suppléance"})
	s.fails(codeNotFound, "POST", "/api/users/"+writer+"/roles/ADMIN/revoke", map[string]any{"reason": "encore"})
	history := s.ok("GET", "/api/users/"+writer+"/roles?includeRevoked=true", nil)
	if len(list(history, "roles")) != 1 || str(list(history, "roles")[0], "revokedBy") == "" {
		s.t.Fatalf("the revoked assignment is kept as history: %v", history)
	}
	s.fails(codeNotFound, "POST", "/api/users/it-never-seen/roles", map[string]any{"roleCode": "ADMIN", "reason": "x"})
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
	s.fails(codeInvalidArgument, "GET", "/api/cases/search?querry=typo", nil) // GLD-043: 400, not 500
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
	if r := s.call(s.admin, adminToken, "GET", "/api/documents/"+id+"/content", nil); r.status != 200 {
		s.t.Fatalf("download the current version: HTTP %d", r.status)
	}
	if r := s.call(s.admin, adminToken, "GET", missing("/api/documents")+"/content", nil); r.status != 404 {
		s.t.Fatalf("download an unknown document: HTTP %d", r.status)
	}
	if r := s.call(s.admin, adminToken, "GET", "/api/documents/"+id+"/content?ref=internal://x", nil); r.status != 400 {
		s.t.Fatalf("a raw storage reference is refused: HTTP %d", r.status)
	}
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

// access covers the grants and groups (GLD-048) and their enforcement in the
// adapters: the writer reads a public case, is refused a confidential one,
// edits only once granted MANAGE and deletes only with FULL_CONTROL.
func (s *surface) access() {
	writer := str(s.call(s.writer, writerToken, "GET", "/api/me", nil).body, "user", "id")
	asWriter := func(method, path string, body any) reply { return s.call(s.writer, writerToken, method, path, body) }

	open := str(s.ok("POST", "/api/cases", map[string]any{"caseTypeCode": "GENERIC_REQUEST", "title": "Accès " + s.token}), "case", "subjectRef", "id")
	secret := str(s.ok("POST", "/api/cases", map[string]any{
		"caseTypeCode": "GENERIC_REQUEST", "title": "Confidentielle " + s.token, "initialGovernance": map[string]any{"confidentialityLevel": 2},
	}), "case", "subjectRef", "id")
	if r := asWriter("GET", "/api/cases/"+open, nil); r.status != 200 {
		s.t.Fatalf("a public case is readable by everyone: %v", r.body)
	}
	s.expect(asWriter("GET", "/api/cases/"+secret, nil), codePermissionDenied, "a confidential case needs a grant")
	s.readFilters(asWriter, secret)
	s.expect(asWriter("PATCH", "/api/cases/"+open, map[string]any{"title": "Réécrit"}), codePermissionDenied, "editing with READ")
	if lvl := str(asWriter("GET", "/api/subjects/"+open+"/access", nil).body, "access", "level"); lvl != "PERMISSION_READ" {
		s.t.Fatalf("the writer's baseline is READ: %s", lvl)
	}

	granted := s.ok("POST", "/api/subjects/"+open+"/grants", map[string]any{
		"granteeKind": "GRANTEE_KIND_USER", "granteeId": writer, "level": "PERMISSION_MANAGE", "reason": "co-gestion",
	})
	if r := asWriter("PATCH", "/api/cases/"+open, map[string]any{"title": "Réécrit avec MANAGE " + s.token}); r.status != 200 {
		s.t.Fatalf("editing with MANAGE: %v", r.body)
	}
	s.expect(asWriter("DELETE", "/api/cases/"+open+"?reason=x", nil), codePermissionDenied, "deleting needs FULL_CONTROL")
	s.expect(asWriter("POST", "/api/subjects/"+open+"/grants", map[string]any{
		"granteeKind": "GRANTEE_KIND_USER", "granteeId": writer, "level": "PERMISSION_FULL_CONTROL", "reason": "moi",
	}), codePermissionDenied, "granting needs FULL_CONTROL")
	if len(list(s.ok("GET", "/api/subjects/"+open+"/grants?includeRevoked=true", nil), "grants")) < 2 {
		s.t.Fatal("the creator's and the writer's grants are listed")
	}
	s.ok("POST", "/api/grants/"+str(granted, "grant", "id")+"/revoke", map[string]any{"reason": "fin"})
	s.fails(codeNotFound, "POST", missing("/api/grants")+"/revoke", map[string]any{"reason": "x"})
	s.groups(writer)
}

// readFilters covers GLD-049 over HTTP: a confidential case is found by its
// creator only, and a confidential document's content is refused to others.
func (s *surface) readFilters(asWriter func(method, path string, body any) reply, secret string) {
	search := "/api/cases/search?query=Confidentielle+" + s.token
	if n := len(list(s.ok("GET", search, nil), "cases")); n != 1 {
		s.t.Fatalf("the creator finds its confidential case: %d", n)
	}
	if r := asWriter("GET", search, nil); r.status != 200 || len(list(r.body, "cases")) != 0 {
		s.t.Fatalf("another user does not find it: %v", r.body)
	}
	sc := &scenario{e2e: s.e2e}
	blob, _ := sc.upload("%PDF confidentiel " + s.token)
	doc := str(s.ok("POST", "/api/documents", map[string]any{
		"documentTypeCode": "PLAN", "title": "Plan confidentiel " + s.token, "contentBlobId": blob, "linkToCaseId": secret,
	}), "document", "subjectRef", "id")
	if r := asWriter("GET", "/api/documents/"+doc+"/content", nil); r.status != 403 {
		s.t.Fatalf("the content of a confidential document needs READ: HTTP %d", r.status)
	}
}

// groups covers the security group RPCs.
func (s *surface) groups(writer string) {
	id := str(s.ok("POST", "/api/groups", map[string]any{"name": "Commission " + s.token, "description": "Groupe de test"}), "group", "subjectRef", "id")
	s.fails(codeAlreadyExists, "POST", "/api/groups", map[string]any{"name": "Commission " + s.token})
	s.ok("PATCH", "/api/groups/"+id, map[string]any{"name": "Commission renommée " + s.token, "reason": "test"})
	s.ok("POST", "/api/groups/"+id+"/members", map[string]any{"userId": writer})
	group := s.ok("GET", "/api/groups/"+id, nil)
	if len(list(group, "members")) != 1 {
		s.t.Fatalf("the writer is a member: %v", group)
	}
	if len(list(s.ok("GET", "/api/groups?query=renommée+"+s.token, nil), "groups")) != 1 {
		s.t.Fatal("the group is found by name")
	}
	s.ok("POST", "/api/groups/"+id+"/members/"+writer+"/remove", map[string]any{"reason": "départ"})
	s.ok("POST", "/api/groups/"+id+"/archive", map[string]any{"reason": "commission dissoute"})
	s.fails(codeNotFound, "GET", missing("/api/groups"), nil)
}
