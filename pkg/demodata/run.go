package demodata

import (
	"context"
	"fmt"
	"io"
	"slices"
	"time"

	"connectrpc.com/connect"
	v1 "github.com/lao-tseu-is-alive/go-cloud-k8s-poc-2026/gen/goeland/v1"
	"google.golang.org/protobuf/types/known/timestamppb"
)

// run is one generation: the clients, the texts, the ids created so far and
// the first error. Its helpers do nothing once an error occurred, so a story
// reads as a list of steps and checks the error once at the end (the
// bufio.Scanner pattern).
type run struct {
	ctx   context.Context
	c     *Clients
	t     *textSet
	err   error
	units map[string]string   // unit key -> org unit id
	users map[string]*v1.User // user key -> recorded user
	// places are the things of the stories, from geodata/things.geojson.
	places map[string]place
	// now is the story time of the current step (at), in Europe/Zurich.
	now time.Time
	// today is midnight of the generation day in Europe/Zurich: the ongoing
	// work (§4.8 to §4.12) is dated from it (daysAgo, dueIn).
	today time.Time
	// clock is told each story time before its step (Options.Clock).
	clock func(context.Context, time.Time) error
	count map[string]int // kind -> number created
}

// call sends req as the demo user and returns the reply message, or nil after
// recording the first error (the getters of a nil message return zero values).
func call[Req, Res any](r *run, user, what string, fn func(context.Context, *connect.Request[Req]) (*connect.Response[Res], error), req *Req) *Res {
	if r.err != nil {
		return nil
	}
	resp, err := fn(as(r.ctx, user), connect.NewRequest(req))
	if err != nil {
		r.err = fmt.Errorf("%s (as %s): %w", what, user, err)
		return nil
	}
	return resp.Msg
}

// zurich is the time zone of the story times.
var zurich = mustLoadZurich()

func mustLoadZurich() *time.Location {
	loc, err := time.LoadLocation("Europe/Zurich") // embedded by time/tzdata
	if err != nil {
		panic(err)
	}
	return loc
}

// at starts a story step at stamp ("2006-01-02 15:04", Europe/Zurich): the
// timeline entries and documents of the step take that time, and the clock is
// told, so demo-seed can re-date afterwards what the server stamped (the API
// never backdates).
func (r *run) at(stamp string) {
	if r.err != nil {
		return
	}
	t, err := time.ParseInLocation("2006-01-02 15:04", stamp, zurich)
	if err != nil {
		panic("demodata: invalid story time " + stamp) // a typo in a story, caught by the tests
	}
	r.now = t
	if r.clock != nil {
		if err := r.clock(r.ctx, t); err != nil {
			r.err = fmt.Errorf("story clock at %s: %w", stamp, err)
		}
	}
}

// person creates a person actor.
func (r *run) person(user string, salutation v1.Salutation, first, last, email string) string {
	req := &v1.CreateActorRequest{
		ActorKind:      v1.ActorKind_ACTOR_KIND_PERSON,
		Specialization: &v1.CreateActorRequest_Person{Person: &v1.PersonDetails{Salutation: salutation, FirstName: first, LastName: last}},
	}
	if email != "" {
		req.Contacts = []*v1.ActorContact{{ContactType: v1.ContactType_CONTACT_TYPE_EMAIL, Value: email, IsPrimary: true}}
	}
	return r.actor(user, req)
}

// organization creates an organization actor of category.
func (r *run) organization(user, name, category, email string) string {
	req := &v1.CreateActorRequest{
		ActorKind:      v1.ActorKind_ACTOR_KIND_ORGANIZATION,
		DisplayName:    name,
		Specialization: &v1.CreateActorRequest_Organization{Organization: &v1.OrganizationDetails{LegalName: name, CategoryCode: category}},
	}
	if email != "" {
		req.Contacts = []*v1.ActorContact{{ContactType: v1.ContactType_CONTACT_TYPE_EMAIL, Value: email, IsPrimary: true}}
	}
	return r.actor(user, req)
}

func (r *run) actor(user string, req *v1.CreateActorRequest) string {
	r.count["actors"]++
	return call(r, user, "create actor", r.c.actors.CreateActor, req).GetActor().GetSubjectRef().GetId()
}

// thing creates a thing.
func (r *run) thing(user string, req *v1.CreateThingRequest) string {
	r.count["things"]++
	return call(r, user, "create thing "+req.GetName(), r.c.things.CreateThing, req).GetThing().GetSubjectRef().GetId()
}

// newCase opens a case owned by unit, at confidentiality level, with the
// reference number of its story ("2025-000031", in its type's namespace).
func (r *run) newCase(user, caseType, number, title, description, unit string, confidentiality int32) string {
	r.count["cases"]++
	return call(r, user, "create case "+title, r.c.cases.CreateCase, &v1.CreateCaseRequest{
		CaseTypeCode: caseType, Title: title, Description: description,
		BusinessRef: &v1.BusinessRefRequest{
			Namespace: namespaceOf(caseType), Source: &v1.BusinessRefRequest_Value{Value: number},
		},
		InitialGovernance: &v1.RecordMetadata{OwnerOrgId: r.units[unit], ConfidentialityLevel: confidentiality},
	}).GetCase().GetSubjectRef().GetId()
}

// namespaceOf is the reference namespace of a demo case type.
func namespaceOf(caseType string) string {
	for _, ct := range caseTypes {
		if ct.code == caseType {
			return ct.namespace
		}
	}
	return ""
}

// link relates two subjects.
func (r *run) link(user, source, target, typeCode string) {
	call(r, user, "link "+typeCode, r.c.core.LinkSubjects, &v1.LinkSubjectsRequest{
		SourceSubjectId: source, TargetSubjectId: target, RelationshipTypeCode: typeCode,
	})
}

// document uploads the PDF rendering of doc and files it in caseID, dated
// the story day; final documents can no longer change.
func (r *run) document(user, docType string, doc docText, caseID string, final bool) string {
	return r.file(user, docType, doc, caseID, final, "document.pdf", renderPDF(doc.Title, doc.Lines, r.t.Footer))
}

// photo files the demo photograph asset (assets/) in caseID as a final photo.
func (r *run) photo(user string, doc docText, asset, caseID string) string {
	return r.file(user, docPhoto, doc, caseID, true, asset, r.asset(asset))
}

// file uploads content as filename and files it as a document of caseID.
func (r *run) file(user, docType string, doc docText, caseID string, final bool, filename string, content []byte) string {
	blob := r.uploadAs(user, filename, content)
	r.count["documents"]++
	return call(r, user, "create document "+doc.Title, r.c.documents.CreateDocument, &v1.CreateDocumentRequest{
		DocumentTypeCode: docType, Title: doc.Title, OfficialDate: r.now.Format(time.DateOnly), Language: r.t.Language,
		LinkToCaseId: caseID, ContentBlobId: blob, IsFinal: final,
	}).GetDocument().GetSubjectRef().GetId()
}

// newVersion files a new version of documentID with doc's rendering.
func (r *run) newVersion(user, documentID string, doc docText, reason string) {
	blob := r.uploadAs(user, "document.pdf", renderPDF(doc.Title, doc.Lines, r.t.Footer))
	call(r, user, "add version to "+doc.Title, r.c.documents.AddDocumentVersion, &v1.AddDocumentVersionRequest{
		DocumentId: documentID, ContentBlobId: blob, Reason: reason,
	})
}

func (r *run) uploadAs(user, filename string, content []byte) string {
	if r.err != nil {
		return ""
	}
	blob, err := r.c.upload(as(r.ctx, user), filename, content)
	if err != nil {
		r.err = err
	}
	return blob
}

// entry writes a validated timeline entry of the case at the story time,
// citing documents.
func (r *run) entry(user, caseID string, kind v1.TimelineEntryType, e entryText, visibility v1.TimelineVisibility, documents ...string) {
	created := call(r, user, "timeline entry "+e.Title, r.c.timeline.CreateTimelineEntry, &v1.CreateTimelineEntryRequest{
		CaseId: caseID, EntryType: kind, Title: e.Title, Body: e.Body, Visibility: visibility,
		OccurredAt: timestamppb.New(r.now), DocumentIds: slices.DeleteFunc(documents, func(id string) bool { return id == "" }),
	})
	call(r, user, "validate entry "+e.Title, r.c.timeline.ValidateTimelineEntry, &v1.ValidateTimelineEntryRequest{Id: created.GetEntry().GetId()})
	r.count["timeline entries"]++
}

// task creates a task of the case, assigned to a user key or a unit key.
func (r *run) task(user, caseID, taskType, title, assigneeUser, assigneeUnit string) string {
	return r.taskDue(user, caseID, taskType, title, assigneeUser, assigneeUnit, nil)
}

// taskDue is task with a deadline (nil: none).
func (r *run) taskDue(user, caseID, taskType, title, assigneeUser, assigneeUnit string, due *timestamppb.Timestamp) string {
	req := &v1.CreateTaskRequest{CaseId: caseID, TaskTypeCode: taskType, Title: title, DueAt: due}
	if assigneeUser != "" {
		req.AssigneeUserId = r.users[assigneeUser].GetId()
	}
	if assigneeUnit != "" {
		req.AssigneeOrgUnitId = r.units[assigneeUnit]
	}
	r.count["tasks"]++
	return call(r, user, "create task "+title, r.c.tasks.CreateTask, req).GetTask().GetId()
}

// complete starts then completes a task.
func (r *run) complete(user, taskID, note string) {
	call(r, user, "start task", r.c.tasks.StartTask, &v1.StartTaskRequest{Id: taskID})
	call(r, user, "complete task", r.c.tasks.CompleteTask, &v1.CompleteTaskRequest{Id: taskID, Note: note})
}

// circulate opens a one-step circulation to units and returns its recipient
// ids by unit key.
func (r *run) circulate(user, caseID, title, message string, units ...string) map[string]string {
	recipients, _ := r.circulateDue(user, caseID, title, message, nil, units...)
	return recipients
}

// circulateDue is circulate with a deadline (nil: none); it also returns the
// task id of each recipient, by unit key.
func (r *run) circulateDue(user, caseID, title, message string, due *timestamppb.Timestamp, units ...string) (recipients, tasks map[string]string) {
	req := &v1.CreateCirculationRequest{CaseId: caseID, Title: title, Message: message, DueAt: due}
	for _, unit := range units {
		req.Recipients = append(req.Recipients, &v1.CirculationRecipientInput{Step: 1, AssigneeOrgUnitId: r.units[unit]})
	}
	created := call(r, user, "create circulation "+title, r.c.circulations.CreateCirculation, req)
	r.count["circulations"]++
	recipients, tasks = map[string]string{}, map[string]string{}
	for _, recipient := range created.GetCirculation().GetRecipients() {
		for _, unit := range units {
			if recipient.GetAssigneeOrgUnitId() == r.units[unit] {
				recipients[unit], tasks[unit] = recipient.GetId(), recipient.GetTaskId()
			}
		}
	}
	return recipients, tasks
}

// respond answers a circulation recipient.
func (r *run) respond(user, recipientID string, response v1.CirculationResponse, text string) {
	call(r, user, "answer circulation", r.c.circulations.RespondToCirculation, &v1.RespondToCirculationRequest{
		RecipientId: recipientID, Response: response, Text: text,
	})
}

// transition moves the case to status.
func (r *run) transition(user, caseID string, status v1.CaseStatus, reason string) {
	call(r, user, "transition case", r.c.cases.TransitionCase, &v1.TransitionCaseRequest{Id: caseID, TargetStatus: status, Reason: reason})
}

// Report counts what a generation created.
type Report struct {
	// Profile is the profile generated.
	Profile string
	// Language is the language of the generated texts.
	Language string
	// Counts is the number of records created, by kind.
	Counts map[string]int
}

// Print writes the counts, one kind per line, sorted.
func (rep Report) Print(w io.Writer) {
	fmt.Fprintf(w, "demo data generated: profile %s, language %s\n", rep.Profile, rep.Language)
	kinds := make([]string, 0, len(rep.Counts))
	for kind := range rep.Counts {
		kinds = append(kinds, kind)
	}
	slices.Sort(kinds)
	for _, kind := range kinds {
		fmt.Fprintf(w, "  %-18s %d\n", kind, rep.Counts[kind])
	}
}
