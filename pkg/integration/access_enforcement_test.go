package integration

import (
	"errors"
	"testing"

	"github.com/lao-tseu-is-alive/go-cloud-k8s-poc-2026/pkg/access"
	"github.com/lao-tseu-is-alive/go-cloud-k8s-poc-2026/pkg/casefile"
	"github.com/lao-tseu-is-alive/go-cloud-k8s-poc-2026/pkg/circulation"
	"github.com/lao-tseu-is-alive/go-cloud-k8s-poc-2026/pkg/core"
	"github.com/lao-tseu-is-alive/go-cloud-k8s-poc-2026/pkg/document"
	"github.com/lao-tseu-is-alive/go-cloud-k8s-poc-2026/pkg/task"
	"github.com/lao-tseu-is-alive/go-cloud-k8s-poc-2026/pkg/timeline"
)

// caseInput is a generic case created by the test operator at a confidentiality level.
func caseInput(title string, confidentiality int32) casefile.CreateInput {
	return casefile.CreateInput{
		CaseTypeCode: "GENERIC_REQUEST", Title: title, OperatorID: testOperator,
		Governance: core.CreateSubjectInput{ConfidentialityLevel: confidentiality},
	}
}

// denied requires err to be the access refusal.
func denied(t *testing.T, what string, err error) {
	t.Helper()
	if !errors.Is(err, core.ErrPermissionDenied) {
		t.Fatalf("%s: want ErrPermissionDenied, got %v", what, err)
	}
}

// TestCaseWorkNeedsAccess covers the checks made inside the transactions of the
// case-owned entities (GLD-048): timeline entries, tasks and circulations.
func TestCaseWorkNeedsAccess(t *testing.T) {
	env := newTestEnv(t)
	ctx := env.ctx
	c := openCase(t, env, "Travail protégé "+uniqueToken())
	x, y := newUser(t, env, "Xavier Travail"), newUser(t, env, "Yasmine Travail")

	note := timeline.CreateInput{CaseID: c.ID, Type: timeline.TypeComment, Body: "note", OperatorID: x.UserID}
	_, _, err := env.timelineSvc.Create(ctx, note)
	denied(t, "a timeline entry with READ only", err)
	grant(t, env, c.ID, core.GranteeUser, x.UserID, core.LevelContribute)
	entry, _, err := env.timelineSvc.Create(ctx, note)
	if err != nil {
		t.Fatalf("a timeline entry with CONTRIBUTE: %v", err)
	}
	_, _, err = env.timelineSvc.Lock(ctx, entry.ID, x.UserID, "archivage")
	denied(t, "locking an entry with CONTRIBUTE", err)

	_, _, err = env.taskSvc.Create(ctx, task.CreateInput{CaseID: c.ID, TypeCode: "OTHER", Title: "à faire", OperatorID: x.UserID})
	denied(t, "a task with CONTRIBUTE", err)
	assigned, _, err := env.taskSvc.Create(ctx, task.CreateInput{CaseID: c.ID, TypeCode: "OTHER", Title: "pour Yasmine", Assignee: task.Assignee{UserID: &y.UserID}, OperatorID: testOperator})
	if err != nil {
		t.Fatalf("create task: %v", err)
	}
	if _, _, err := env.taskSvc.ChangeStatus(ctx, assigned.ID, task.MoveStart, y.UserID, ""); err != nil {
		t.Fatalf("the assignee starts its task without a grant: %v", err)
	}
	_, _, err = env.taskSvc.ChangeStatus(ctx, assigned.ID, task.MoveCancel, y.UserID, "inutile")
	denied(t, "the assignee cancelling (MANAGE)", err)

	circ, _, err := env.circulationSvc.Create(ctx, circulation.CreateInput{
		CaseID: c.ID, Title: "Préavis", OperatorID: testOperator,
		Recipients: []circulation.RecipientInput{{Assignee: task.Assignee{UserID: &y.UserID}}},
	})
	if err != nil {
		t.Fatalf("create circulation: %v", err)
	}
	answer := circulation.RespondInput{RecipientID: circ.Recipients[0].ID, Response: circulation.ResponseFavorable, OperatorID: x.UserID}
	_, _, err = env.circulationSvc.Respond(ctx, answer)
	denied(t, "answering for another recipient", err)
	answer.OperatorID = y.UserID
	if _, _, err := env.circulationSvc.Respond(ctx, answer); err != nil {
		t.Fatalf("the recipient answers without a grant: %v", err)
	}
}

// TestRelationshipsNeedAccess covers the link rules of core.LinkSubjectsTx:
// MANAGE on the source, READ on the target, CONTRIBUTE to attach a document to
// a case, and memberships managed from the group, never from oneself.
func TestRelationshipsNeedAccess(t *testing.T) {
	env := newTestEnv(t)
	ctx := env.ctx
	c := openCase(t, env, "Liens protégés "+uniqueToken())
	x := newUser(t, env, "Xavier Liens")
	a := newActor(t, env)

	link := core.LinkInput{SourceSubjectID: c.ID, TargetSubjectID: a.ID, RelationshipTypeCode: "CASE_HAS_ACTOR_REQUESTER", OperatorID: x.UserID}
	_, _, err := env.coreSvc.LinkSubjects(ctx, link)
	denied(t, "linking from a case with READ", err)
	grant(t, env, c.ID, core.GranteeUser, x.UserID, core.LevelManage)
	if _, _, err := env.coreSvc.LinkSubjects(ctx, link); err != nil {
		t.Fatalf("linking with MANAGE on the case and READ on the actor: %v", err)
	}

	group, _, err := env.accessSvc.CreateGroup(ctx, access.GroupInput{Name: "Cercle " + uniqueToken(), OperatorID: testOperator})
	if err != nil {
		t.Fatalf("create group: %v", err)
	}
	_, _, err = env.coreSvc.LinkSubjects(ctx, core.LinkInput{SourceSubjectID: x.SubjectID, TargetSubjectID: group.ID, RelationshipTypeCode: "USER_MEMBER_OF_GROUP", OperatorID: x.UserID})
	denied(t, "joining a group by oneself", err)
	_, _, err = env.accessSvc.AddMember(ctx, access.MemberInput{GroupID: group.ID, UserID: x.UserID, OperatorID: x.UserID})
	denied(t, "adding oneself through the access service", err)
}

// TestDepositFromCaseCopiesAccess covers a document deposited from a case: it
// copies the case's grants and confidentiality once, while attaching an
// existing document changes neither.
func TestDepositFromCaseCopiesAccess(t *testing.T) {
	env := newTestEnv(t)
	ctx := env.ctx
	x, reader := newUser(t, env, "Xavier Dépôt"), newUser(t, env, "Rita Lectrice")
	c, _, err := env.caseSvc.Create(ctx, caseInput("Dépôt confidentiel "+uniqueToken(), core.ConfidentialLevel))
	if err != nil {
		t.Fatalf("create confidential case: %v", err)
	}
	grant(t, env, c.ID, core.GranteeUser, x.UserID, core.LevelContribute)
	grant(t, env, c.ID, core.GranteeUser, reader.UserID, core.LevelRead)

	blob := ingest(t, env, "dépôt "+uniqueToken()).Blob
	deposited, err := env.docSvc.Create(ctx, document.CreateInput{
		DocumentTypeCode: "PLAN", Title: "Plan déposé " + uniqueToken(), ContentBlobID: &blob.ID, LinkToCaseID: &c.ID, OperatorID: x.UserID,
	})
	if err != nil {
		t.Fatalf("deposit from the case: %v", err)
	}
	doc := deposited.Document
	if doc.RecordMetadata == nil || doc.RecordMetadata.ConfidentialityLevel != core.ConfidentialLevel {
		t.Fatalf("the deposited document takes the case's confidentiality: %+v", doc.RecordMetadata)
	}
	expectAccess(t, env, x.UserID, doc.ID, core.LevelFullControl, core.SourcePersonal) // its depositor
	expectAccess(t, env, reader.UserID, doc.ID, core.LevelRead, core.SourcePersonal)   // copied from the case

	public := newVersionedDocument(t, env)
	if _, _, err := env.coreSvc.LinkSubjects(ctx, core.LinkInput{SourceSubjectID: c.ID, TargetSubjectID: public.ID, RelationshipTypeCode: "CASE_HAS_DOCUMENT", OperatorID: x.UserID}); err != nil {
		t.Fatalf("attaching a public document with CONTRIBUTE on the case: %v", err)
	}
	expectAccess(t, env, reader.UserID, public.ID, core.LevelRead, core.SourceBaseline) // still public, nothing copied
}
