package demodata

import (
	"slices"

	v1 "github.com/lao-tseu-is-alive/go-cloud-k8s-poc-2026/gen/goeland/v1"
)

// Fictional register identifiers: no Swiss commune has OFS number 9990, and
// the EGRID and EGID ranges below are not real ones.
const (
	demoCommuneOFS = 9990
	tilleulsEGRID  = "CHDEMO00001101"
	tilleulsEGID   = 900_001_101
)

// Shorthands of the enums the stories use.
const (
	participants = v1.TimelineVisibility_TIMELINE_VISIBILITY_CASE_PARTICIPANTS
	internal     = v1.TimelineVisibility_TIMELINE_VISIBILITY_INTERNAL
	request      = v1.TimelineEntryType_TIMELINE_ENTRY_TYPE_REQUEST
	comment      = v1.TimelineEntryType_TIMELINE_ENTRY_TYPE_COMMENT
	decision     = v1.TimelineEntryType_TIMELINE_ENTRY_TYPE_DECISION
	closed       = v1.CaseStatus_CASE_STATUS_CLOSED
	favorable    = v1.CirculationResponse_CIRCULATION_RESPONSE_FAVORABLE
	remark       = v1.CirculationResponse_CIRCULATION_RESPONSE_COMMENT
	madame       = v1.Salutation_SALUTATION_MADAME
	monsieur     = v1.Salutation_SALUTATION_MONSIEUR
)

// permitAndOpposition is the building permit « Villa Les Tilleuls » (§4.1)
// and the neighbour's opposition linked to it (§4.2), handled by Bruno.
func (r *run) permitAndOpposition() {
	t := r.t.Permit
	owner := r.person(userBruno, madame, "Claire", "Fontaine", "claire.fontaine@example.ch")
	architect := r.organization(userBruno, "Atelier Molard SA", "BUREAU_ARCHITECTE", "contact@atelier-molard.example.ch")
	contractor := r.organization(userBruno, "Bâtisud Sàrl", "METIER_BATIMENT", "")
	parcel := r.thing(userBruno, &v1.CreateThingRequest{
		ThingTypeCode: "PARCEL", Name: t.ParcelName, GeometryGeojson: rectangle(tilleulsEast, tilleulsNorth, 40, 30),
		Specialization: &v1.CreateThingRequest_Parcel{Parcel: &v1.ParcelDetails{
			CommuneOfs: demoCommuneOFS, ParcelNumber: "1101", Egrid: tilleulsEGRID, SurfaceM2: 1200,
		}},
	})
	building := r.thing(userBruno, &v1.CreateThingRequest{
		ThingTypeCode: "BUILDING", Name: t.BuildingName, GeometryGeojson: rectangle(tilleulsEast+10, tilleulsNorth+8, 18, 12),
		Specialization: &v1.CreateThingRequest_Building{Building: &v1.BuildingDetails{Egid: tilleulsEGID, ConstructionYear: 1962}},
	})
	r.link(userBruno, parcel, owner, "THING_HAS_ACTOR_OWNER")

	permit := r.newCase(userBruno, caseBuildingPermit, t.CaseTitle, t.CaseDescription, unitPlanning, 0)
	r.link(userBruno, permit, parcel, "CASE_CONCERNS_THING")
	r.link(userBruno, permit, building, "CASE_CONCERNS_THING")
	r.link(userBruno, permit, owner, "CASE_HAS_ACTOR_REQUESTER")
	r.link(userBruno, permit, owner, "CASE_HAS_ACTOR_OWNER")
	r.link(userBruno, permit, architect, "CASE_HAS_ACTOR_ARCHITECT")
	r.link(userBruno, permit, contractor, "CASE_HAS_ACTOR_CONTRACTOR")

	form := r.document(userBruno, docForm, t.RequestForm, "2025-03-03", permit, true)
	plan := r.document(userBruno, docPlan, t.Plan, "2025-03-03", permit, false)
	r.link(userBruno, plan, building, "DOCUMENT_REPRESENTS_THING")
	r.link(userBruno, plan, architect, "DOCUMENT_AUTHORED_BY_ACTOR")
	r.entry(userBruno, permit, request, t.Filed, participants, "2025-03-03", form, plan)
	check := r.task(userBruno, permit, "DOCUMENT_CHECK", t.CheckTask, userBruno, "")
	r.complete(userBruno, check, t.CheckTaskNote)

	notice := r.document(userBruno, docInquiryNotice, t.InquiryNotice, "2025-03-21", permit, true)
	opposition := r.oppose(permit)
	r.entry(userBruno, permit, comment, t.Inquiry, participants, "2025-04-19", notice)

	recipients := r.circulate(userBruno, permit, t.CirculationTitle, t.CirculationMessage, unitRoads, unitParks, unitEnergy)
	r.respond(userChloe, recipients[unitRoads], favorable, t.RoadsAnswer)
	r.respond(userHugo, recipients[unitParks], remark, t.ParksAnswer)
	r.respond(userEmma, recipients[unitEnergy], favorable, t.EnergyAnswer)
	revised := docText{Title: t.Plan.Title, Lines: slices.Concat(t.Plan.Lines, []string{t.PlanRevisionReason})}
	r.newVersion(userBruno, plan, revised, t.PlanRevisionReason)

	granted := r.document(userBruno, docDecision, t.Decision, "2025-05-20", permit, true)
	letter := r.document(userBruno, docOutgoingLetter, t.DecisionLetter, "2025-05-21", permit, true)
	r.link(userBruno, letter, owner, "DOCUMENT_SENT_TO_ACTOR")
	r.entry(userBruno, permit, decision, t.DecisionEntry, participants, "2025-05-20", granted, letter)
	call(r, userBruno, "verify plan content", r.c.documents.VerifyDocumentContent, &v1.VerifyDocumentContentRequest{Id: plan})
	r.liftOpposition(opposition, granted)
	r.transition(userBruno, permit, closed, t.ClosingReason)
}

// oppose opens the neighbour's opposition, linked to the permit (§4.2).
func (r *run) oppose(permit string) string {
	t := r.t.Opposition
	neighbour := r.person(userBruno, monsieur, "Paul", "Rochat", "paul.rochat@example.ch")
	opposition := r.newCase(userBruno, caseOpposition, t.CaseTitle, t.CaseDescription, unitPlanning, 0)
	r.link(userBruno, opposition, permit, "CASE_RELATED_TO_CASE")
	r.link(userBruno, opposition, neighbour, "CASE_HAS_ACTOR_REQUESTER")
	letter := r.document(userBruno, docOpposition, t.Letter, "2025-04-10", opposition, true)
	r.link(userBruno, letter, neighbour, "DOCUMENT_AUTHORED_BY_ACTOR")
	ack := r.document(userBruno, docOutgoingLetter, t.Acknowledgement, "2025-04-14", opposition, true)
	r.link(userBruno, ack, neighbour, "DOCUMENT_SENT_TO_ACTOR")
	r.entry(userBruno, opposition, request, t.Received, participants, "2025-04-14", letter, ack)
	note := r.document(userBruno, docInternalNote, t.ConciliationNote, "2025-05-06", opposition, true)
	r.entry(userBruno, opposition, comment, t.Conciliation, internal, "2025-05-06", note)
	return opposition
}

// liftOpposition closes the opposition, citing the permit decision.
func (r *run) liftOpposition(opposition, granted string) {
	t := r.t.Opposition
	r.entry(userBruno, opposition, decision, t.Lifted, participants, "2025-05-20", granted)
	r.transition(userBruno, opposition, closed, t.ClosingReason)
}

// citizenReport is the street light out « avenue des Cèdres » (§4.6), handled
// by Chloé and fixed by the street lighting unit the same day.
func (r *run) citizenReport() {
	t := r.t.Report
	resident := r.person(userChloe, madame, "Sofia", "Keller", "sofia.keller@example.ch")
	lamp := r.thing(userChloe, &v1.CreateThingRequest{
		ThingTypeCode: "INFRASTRUCTURE", Name: t.LampName, GeometryGeojson: point(cedresEast, cedresNorth),
	})
	report := r.newCase(userChloe, caseCitizenReport, t.CaseTitle, t.CaseDescription, unitRoads, 0)
	r.link(userChloe, report, lamp, "CASE_CONCERNS_THING")
	r.link(userChloe, report, resident, "CASE_HAS_ACTOR_REQUESTER")
	photo := r.document(userChloe, docPhoto, t.Photo, "2026-02-12", report, true)
	r.link(userChloe, photo, lamp, "DOCUMENT_REPRESENTS_THING")
	r.entry(userChloe, report, request, t.Received, participants, "2026-02-12", photo)
	repair := r.task(userChloe, report, "OTHER", t.Task, "", unitLighting)
	r.complete(userChloe, repair, t.TaskNote)
	r.entry(userChloe, report, comment, t.Repaired, participants, "2026-02-12")
	r.transition(userChloe, report, closed, t.ClosingReason)
}
