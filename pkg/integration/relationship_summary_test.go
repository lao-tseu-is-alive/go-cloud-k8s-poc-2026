package integration

import (
	"fmt"
	"math/rand/v2"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/lao-tseu-is-alive/go-cloud-k8s-poc-2026/pkg/casefile"
	"github.com/lao-tseu-is-alive/go-cloud-k8s-poc-2026/pkg/core"
	"github.com/lao-tseu-is-alive/go-cloud-k8s-poc-2026/pkg/document"
	"github.com/lao-tseu-is-alive/go-cloud-k8s-poc-2026/pkg/thing"
)

// summaryGroup finds the incoming group of a type in a relationship summary.
func summaryGroup(t *testing.T, groups []*core.RelationshipGroup, code string) *core.RelationshipGroup {
	t.Helper()
	for _, g := range groups {
		if g.RelationshipType.Code == code && !g.Outgoing {
			return g
		}
	}
	t.Fatalf("no incoming group %s in %+v", code, groups)
	return nil
}

// authoredDocument creates a document of a type and links the actor as its author.
func authoredDocument(t *testing.T, env *testEnv, actorID uuid.UUID, typeCode string) *document.Document {
	t.Helper()
	blob := ingest(t, env, typeCode+" "+uniqueToken()).Blob
	created, err := env.docSvc.Create(env.ctx, document.CreateInput{
		DocumentTypeCode: typeCode, Title: "Pièce " + uniqueToken(), ContentBlobID: &blob.ID, OperatorID: testOperator,
	})
	if err != nil {
		t.Fatalf("create document: %v", err)
	}
	link(t, env, created.Document.ID, core.LinkInput{TargetSubjectID: actorID, RelationshipTypeCode: "DOCUMENT_AUTHORED_BY_ACTOR"})
	return created.Document
}

// TestRelationshipSummary covers GLD-063 on the server: the groups of an
// actor counted as a viewer sees them (an unreadable case left out), the open
// cases, the split by case and document type, the roles filled by migration
// 0031, and the list filters and details that open a group.
func TestRelationshipSummary(t *testing.T) {
	env := newTestEnv(t)
	reader := newUser(t, env, "Rita Résumé")
	actorRef := newActor(t, env)
	open := openCase(t, env, "Dossier ouvert "+uniqueToken())
	closed := openCase(t, env, "Dossier clos "+uniqueToken())
	if _, _, err := env.caseSvc.Transition(env.ctx, closed.ID, casefile.TransitionInput{Target: casefile.StatusClosed, OperatorID: testOperator, Reason: "terminé"}); err != nil {
		t.Fatalf("close case: %v", err)
	}
	secret, _, err := env.caseSvc.Create(env.ctx, caseInput("Dossier secret "+uniqueToken(), core.ConfidentialLevel))
	if err != nil {
		t.Fatalf("create confidential case: %v", err)
	}
	for _, c := range []uuid.UUID{open.ID, closed.ID, secret.ID} {
		link(t, env, c, core.LinkInput{TargetSubjectID: actorRef.ID, RelationshipTypeCode: "CASE_HAS_ACTOR_REQUESTER"})
	}
	authoredDocument(t, env, actorRef.ID, "PLAN")
	authoredDocument(t, env, actorRef.ID, "REPORT")
	egid := int32(100_000_000 + rand.IntN(800_000_000))
	building, _, err := env.thingSvc.Create(env.ctx, thing.CreateInput{
		TypeCode: "BUILDING", GeometryGeoJSON: fmt.Sprintf(`{"type":"Point","coordinates":[%d,%d]}`, 2_538_000+rand.IntN(1000), 1_152_000+rand.IntN(1000)),
		Building: &thing.Building{EGID: &egid, Status: 4}, OperatorID: testOperator,
	})
	if err != nil {
		t.Fatalf("create building: %v", err)
	}
	link(t, env, building.ID, core.LinkInput{TargetSubjectID: actorRef.ID, RelationshipTypeCode: "THING_HAS_ACTOR_OWNER"})

	viewer := viewerOf(t, env, reader.UserID)
	groups, err := env.coreSvc.SummarizeRelationships(env.ctx, actorRef.ID, viewer)
	if err != nil {
		t.Fatalf("summarize: %v", err)
	}
	requesters := summaryGroup(t, groups, "CASE_HAS_ACTOR_REQUESTER")
	if requesters.Count != 2 || requesters.OpenCount != 1 || requesters.OtherKind != core.SubjectKindCase {
		t.Fatalf("two readable cases, one open (the confidential one is not counted): %+v", requesters)
	}
	if rt := requesters.RelationshipType; rt.RoleLabel != "Requérant" || rt.RoleEnd != core.RoleEndTarget || rt.RolePreposition != "de" {
		t.Fatalf("migration 0031 names the role of the seeded type: %+v", rt)
	}
	if len(requesters.Families) != 1 || requesters.Families[0].Code == "" || requesters.Families[0].Count != 2 || requesters.Families[0].OpenCount != 1 {
		t.Fatalf("the cases are split by case type: %+v", requesters.Families)
	}
	if owners := summaryGroup(t, groups, "THING_HAS_ACTOR_OWNER"); len(owners.Families) != 1 || owners.Families[0].Code != "BUILDING" {
		t.Fatalf("the objects are split by thing type: %+v", owners.Families)
	}
	authors := summaryGroup(t, groups, "DOCUMENT_AUTHORED_BY_ACTOR")
	if authors.Count != 2 || len(authors.Families) != 2 || authors.RelationshipType.RoleLabel != "Auteur" {
		t.Fatalf("two documents split by document type: %+v", authors)
	}

	list := func(f core.RelationshipFilter) core.RelationshipResult {
		t.Helper()
		f.SubjectID, f.Viewer = actorRef.ID, viewer
		res, err := env.coreSvc.ListRelationships(env.ctx, f)
		if err != nil {
			t.Fatalf("list %+v: %v", f, err)
		}
		return res
	}
	byCase := core.RelationshipFilter{RelationshipTypeCode: "CASE_HAS_ACTOR_REQUESTER", IncludeDetails: true}
	all := list(byCase)
	if all.TotalSize != requesters.Count {
		t.Fatalf("the list matches the count: %d listed, %d counted", all.TotalSize, requesters.Count)
	}
	for _, rel := range all.Relationships {
		if d := rel.OtherDetail; d == nil || d.TypeCode != requesters.Families[0].Code || d.Date == "" || d.CaseStatus == "" {
			t.Fatalf("each edge describes its case: %+v", rel.OtherDetail)
		}
	}
	openOnly, closedOnly := byCase, byCase
	openOnly.OtherCaseStatus, closedOnly.OtherCaseStatus = "open", "closed"
	if list(openOnly).TotalSize != 1 || list(closedOnly).Relationships[0].OtherDetail.CaseStatus != "CLOSED" {
		t.Fatal("the list filters open and closed cases")
	}
	thisYear, pastOnly := byCase, byCase
	thisYear.OtherYearFrom = int32(time.Now().Year())
	pastOnly.OtherYearTo = int32(time.Now().Year() - 1)
	if list(thisYear).TotalSize != 2 || list(pastOnly).TotalSize != 0 {
		t.Fatal("the list filters by the year the cases were opened")
	}
	byText := byCase
	byText.OtherQuery = "  OUVÉRT "
	if found := list(byText); found.TotalSize != 1 || found.Relationships[0].OtherDetail.CaseStatus == "CLOSED" {
		t.Fatalf("the text filter matches the label, accent- and case-insensitive: %d", found.TotalSize)
	}
	byRef := byCase
	byRef.OtherQuery = open.Subject.BusinessRef
	if list(byRef).TotalSize < 1 {
		t.Fatal("the text filter matches the business reference")
	}
	if list(core.RelationshipFilter{RelationshipTypeCode: "THING_HAS_ACTOR_OWNER", OtherTypeCode: "BUILDING"}).TotalSize != 1 {
		t.Fatal("the list filters by thing type")
	}
	plans := list(core.RelationshipFilter{RelationshipTypeCode: "DOCUMENT_AUTHORED_BY_ACTOR", OtherTypeCode: "PLAN"})
	if plans.TotalSize != 1 {
		t.Fatalf("the list filters by document type: %d", plans.TotalSize)
	}
}
