package integration

import (
	"errors"
	"strings"
	"testing"

	"github.com/lao-tseu-is-alive/go-cloud-k8s-poc-2026/pkg/actor"
	"github.com/lao-tseu-is-alive/go-cloud-k8s-poc-2026/pkg/casefile"
	"github.com/lao-tseu-is-alive/go-cloud-k8s-poc-2026/pkg/core"
	"github.com/lao-tseu-is-alive/go-cloud-k8s-poc-2026/pkg/document"
)

// referenceCode returns a fresh catalogue code for one test run.
func referenceCode(prefix string) string {
	return prefix + "_" + strings.ToUpper(uniqueToken())
}

// TestReferenceAdministration creates, renames and deactivates an entry of each
// catalogue and checks the reference change log, conflicts and unknown codes.
func TestReferenceAdministration(t *testing.T) {
	env := newTestEnv(t)
	ctx := env.ctx
	label := "Renamed"
	inactive := false

	caseCode := referenceCode("IT_CASE")
	if _, change, err := env.caseSvc.CreateCaseType(ctx, casefile.CaseTypeInput{Code: caseCode, Label: "Test case type", BusinessRefNamespace: "ITC", OperatorID: testOperator, Reason: "new procedure"}); err != nil || change.EventType != core.ReferenceCreated || change.Reason != "new procedure" {
		t.Fatalf("create case type: %+v (%v)", change, err)
	}
	if _, _, err := env.caseSvc.CreateCaseType(ctx, casefile.CaseTypeInput{Code: caseCode, Label: "again", OperatorID: testOperator}); !errors.Is(err, core.ErrConflict) {
		t.Fatalf("duplicate code: want ErrConflict, got %v", err)
	}
	if _, _, err := env.caseSvc.CreateCaseType(ctx, casefile.CaseTypeInput{Code: "lower_case", Label: "x", OperatorID: testOperator}); !errors.Is(err, core.ErrInvalidInput) {
		t.Fatalf("bad code: want ErrInvalidInput, got %v", err)
	}
	updated, change, err := env.caseSvc.UpdateCaseType(ctx, caseCode, casefile.CaseTypeUpdate{Label: &label, IsActive: &inactive, OperatorID: testOperator})
	if err != nil || updated.Label != label || updated.IsActive || updated.BusinessRefNamespace != "ITC" {
		t.Fatalf("update case type: %+v (%v)", updated, err)
	}
	if change.EventType != core.ReferenceUpdated || change.BeforeState["label"] != "Test case type" || change.AfterState["is_active"] != false {
		t.Fatalf("update not logged with before/after: %+v", change)
	}
	active, err := env.caseSvc.ListTypes(ctx, true)
	if err != nil {
		t.Fatal(err)
	}
	for _, ct := range active {
		if ct.Code == caseCode {
			t.Fatal("a deactivated case type must not be offered")
		}
	}
	if _, _, err := env.caseSvc.UpdateCaseType(ctx, "UNKNOWN_"+caseCode, casefile.CaseTypeUpdate{Label: &label, OperatorID: testOperator}); !errors.Is(err, core.ErrNotFound) {
		t.Fatalf("unknown code: want ErrNotFound, got %v", err)
	}

	docCode := referenceCode("IT_DOC")
	if _, _, err := env.docSvc.CreateDocumentType(ctx, document.DocumentTypeInput{Code: docCode, Label: "Test doc", Category: "PLAN", OperatorID: testOperator}); err != nil {
		t.Fatalf("create document type: %v", err)
	}
	if dt, _, err := env.docSvc.UpdateDocumentType(ctx, docCode, document.DocumentTypeUpdate{Label: &label, OperatorID: testOperator}); err != nil || dt.Category != "PLAN" || dt.Label != label {
		t.Fatalf("update document type keeps the category: %+v (%v)", dt, err)
	}

	catCode := referenceCode("IT_CAT")
	if _, _, err := env.actorSvc.CreateOrganizationCategory(ctx, actor.OrganizationCategoryInput{Code: catCode, Label: "Test category", OperatorID: testOperator}); err != nil {
		t.Fatalf("create organization category: %v", err)
	}

	relCode := referenceCode("IT_REL")
	if _, _, err := env.coreSvc.CreateRelationshipType(ctx, core.RelationshipTypeInput{Code: relCode, Label: "Test link", SourceKind: core.SubjectKindCase, TargetKind: core.SubjectKindActor, IsDirected: true, OperatorID: testOperator}); err != nil {
		t.Fatalf("create relationship type: %v", err)
	}
	c := openCase(t, env, "Ref "+uniqueToken())
	a := newActor(t, env)
	link(t, env, c.ID, core.LinkInput{TargetSubjectID: a.ID, RelationshipTypeCode: relCode})

	log, err := env.coreSvc.ListReferenceChanges(ctx, core.ReferenceFilter{})
	if err != nil {
		t.Fatal(err)
	}
	seen := map[string]int{}
	for _, ch := range log.Changes {
		seen[ch.Catalogue+":"+ch.Code]++
	}
	if seen["case_type:"+caseCode] != 2 || seen["document_type:"+docCode] != 2 || seen["organization_category:"+catCode] != 1 || seen["relationship_type:"+relCode] != 1 {
		t.Fatalf("reference change log incomplete: %v", seen)
	}
	only, err := env.coreSvc.ListReferenceChanges(ctx, core.ReferenceFilter{Catalogue: core.CatalogueDocumentType})
	if err != nil || len(only.Changes) == 0 || only.Changes[0].Catalogue != core.CatalogueDocumentType {
		t.Fatalf("catalogue filter: %+v (%v)", only.Changes, err)
	}
}
