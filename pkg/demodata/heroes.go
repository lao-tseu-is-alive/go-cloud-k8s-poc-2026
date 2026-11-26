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
// and the neighbour's opposition linked to it (§4.2), handled by Bruno over
// eleven weeks of 2025: filing, public inquiry, opinions, revised plans,
// conciliation, decision.
func (r *run) permitAndOpposition() {
	t := r.t.Permit
	r.at("2025-03-03 09:10")
	owner := r.person(userBruno, madame, "Claire", "Fontaine", "claire.fontaine@example.ch")
	architect := r.organization(userBruno, "Atelier Molard SA", "BUREAU_ARCHITECTE", "contact@atelier-molard.example.ch")
	contractor := r.organization(userBruno, "Bâtisud Sàrl", "METIER_BATIMENT", "")
	parcel := r.thing(userBruno, &v1.CreateThingRequest{
		ThingTypeCode: "PARCEL", Name: t.ParcelName, GeometryGeojson: r.placeOf("tilleuls-parcel", "PARCEL"),
		Specialization: &v1.CreateThingRequest_Parcel{Parcel: &v1.ParcelDetails{
			CommuneOfs: demoCommuneOFS, ParcelNumber: "1101", Egrid: tilleulsEGRID, SurfaceM2: 1200,
		}},
	})
	building := r.thing(userBruno, &v1.CreateThingRequest{
		ThingTypeCode: "BUILDING", Name: t.BuildingName, GeometryGeojson: r.placeOf("tilleuls-building", "BUILDING"),
		Specialization: &v1.CreateThingRequest_Building{Building: &v1.BuildingDetails{Egid: tilleulsEGID, ConstructionYear: 1962}},
	})
	r.link(userBruno, parcel, owner, "THING_HAS_ACTOR_OWNER")

	permit := r.newCase(userBruno, caseBuildingPermit, "2025-000031", t.CaseTitle, t.CaseDescription, unitPlanning, 0)
	r.link(userBruno, permit, parcel, "CASE_CONCERNS_THING")
	r.link(userBruno, permit, building, "CASE_CONCERNS_THING")
	r.link(userBruno, permit, owner, "CASE_HAS_ACTOR_REQUESTER")
	r.link(userBruno, permit, owner, "CASE_HAS_ACTOR_OWNER")
	r.link(userBruno, permit, architect, "CASE_HAS_ACTOR_ARCHITECT")
	r.link(userBruno, permit, contractor, "CASE_HAS_ACTOR_CONTRACTOR")
	form := r.document(userBruno, docForm, t.RequestForm, permit, true)
	plan := r.document(userBruno, docPlan, t.Plan, permit, false)
	r.link(userBruno, plan, building, "DOCUMENT_REPRESENTS_THING")
	r.link(userBruno, plan, architect, "DOCUMENT_AUTHORED_BY_ACTOR")
	site := r.photo(userBruno, t.SitePhoto, "permit-villa-lime-trees.jpg", permit)
	r.link(userBruno, site, building, "DOCUMENT_REPRESENTS_THING")
	r.entry(userBruno, permit, request, t.Filed, participants, form, plan, site)
	check := r.task(userBruno, permit, "DOCUMENT_CHECK", t.CheckTask, userBruno, "")

	r.at("2025-03-05 14:20")
	r.complete(userBruno, check, t.CheckTaskNote)
	r.at("2025-03-21 08:30")
	notice := r.document(userBruno, docInquiryNotice, t.InquiryNotice, permit, true)

	opposition := r.oppose(permit)

	r.at("2025-04-22 09:00")
	r.entry(userBruno, permit, comment, t.Inquiry, participants, notice)
	recipients := r.circulate(userBruno, permit, t.CirculationTitle, t.CirculationMessage, unitRoads, unitParks, unitEnergy)
	r.at("2025-04-25 11:05")
	r.respond(userChloe, recipients[unitRoads], favorable, t.RoadsAnswer)
	r.at("2025-04-30 16:20")
	r.respond(userHugo, recipients[unitParks], remark, t.ParksAnswer)
	r.at("2025-05-02 10:05")
	r.respond(userEmma, recipients[unitEnergy], favorable, t.EnergyAnswer)

	r.conciliate(opposition)

	r.at("2025-05-12 09:30")
	revised := docText{Title: t.Plan.Title, Lines: slices.Concat(t.Plan.Lines, []string{t.PlanRevisionReason})}
	r.newVersion(userBruno, plan, revised, t.PlanRevisionReason)
	r.at("2025-05-20 10:00")
	granted := r.document(userBruno, docDecision, t.Decision, permit, true)
	call(r, userBruno, "verify plan content", r.c.documents.VerifyDocumentContent, &v1.VerifyDocumentContentRequest{Id: plan})
	r.at("2025-05-21 08:45")
	letter := r.document(userBruno, docOutgoingLetter, t.DecisionLetter, permit, true)
	r.link(userBruno, letter, owner, "DOCUMENT_SENT_TO_ACTOR")
	r.entry(userBruno, permit, decision, t.DecisionEntry, participants, granted, letter)
	r.liftOpposition(opposition, granted)
	r.at("2025-05-21 09:10")
	r.transition(userBruno, permit, closed, t.ClosingReason)
}

// oppose opens the neighbour's opposition, received during the public
// inquiry and linked to the permit (§4.2).
func (r *run) oppose(permit string) string {
	t := r.t.Opposition
	r.at("2025-04-11 10:15")
	neighbour := r.person(userBruno, monsieur, "Paul", "Rochat", "paul.rochat@example.ch")
	opposition := r.newCase(userBruno, caseOpposition, "2025-000007", t.CaseTitle, t.CaseDescription, unitPlanning, 0)
	r.link(userBruno, opposition, permit, "CASE_RELATED_TO_CASE")
	r.link(userBruno, opposition, neighbour, "CASE_HAS_ACTOR_REQUESTER")
	letter := r.document(userBruno, docOpposition, t.Letter, opposition, true)
	r.link(userBruno, letter, neighbour, "DOCUMENT_AUTHORED_BY_ACTOR")
	pump := r.photo(userBruno, t.HeatPumpPhoto, "permit-heat-pump.jpg", opposition)
	r.link(userBruno, pump, neighbour, "DOCUMENT_AUTHORED_BY_ACTOR")
	r.at("2025-04-14 15:00")
	ack := r.document(userBruno, docOutgoingLetter, t.Acknowledgement, opposition, true)
	r.link(userBruno, ack, neighbour, "DOCUMENT_SENT_TO_ACTOR")
	r.entry(userBruno, opposition, request, t.Received, participants, letter, pump, ack)
	return opposition
}

// conciliate records the conciliation meeting of the opposition.
func (r *run) conciliate(opposition string) {
	r.at("2025-05-06 14:00")
	note := r.document(userBruno, docInternalNote, r.t.Opposition.ConciliationNote, opposition, true)
	r.entry(userBruno, opposition, comment, r.t.Opposition.Conciliation, internal, note)
}

// liftOpposition closes the opposition, citing the permit decision.
func (r *run) liftOpposition(opposition, granted string) {
	t := r.t.Opposition
	r.entry(userBruno, opposition, decision, t.Lifted, participants, granted)
	r.transition(userBruno, opposition, closed, t.ClosingReason)
}

// citizenReport is the street light out « avenue des Cèdres » (§4.6): the
// resident's e-mail on a Monday morning, the street lighting unit replaces the
// lamp on Thursday, Chloé closes the case on Friday.
func (r *run) citizenReport() {
	t := r.t.Report
	r.at("2026-02-09 07:40")
	resident := r.person(userChloe, madame, "Sofia", "Keller", "sofia.keller@example.ch")
	lamp := r.thing(userChloe, &v1.CreateThingRequest{
		ThingTypeCode: "INFRASTRUCTURE", Name: t.LampName, GeometryGeojson: r.placeOf("cedres-streetlight", "INFRASTRUCTURE"),
	})
	report := r.newCase(userChloe, caseCitizenReport, "2026-000014", t.CaseTitle, t.CaseDescription, unitRoads, 0)
	r.link(userChloe, report, lamp, "CASE_CONCERNS_THING")
	r.link(userChloe, report, resident, "CASE_HAS_ACTOR_REQUESTER")
	email := r.document(userChloe, docIncomingLetter, t.Email, report, true)
	r.link(userChloe, email, resident, "DOCUMENT_AUTHORED_BY_ACTOR")
	photo := r.photo(userChloe, t.Photo, "report-streetlight-off.jpg", report)
	r.link(userChloe, photo, lamp, "DOCUMENT_REPRESENTS_THING")
	r.entry(userChloe, report, request, t.Received, participants, email, photo)
	repair := r.task(userChloe, report, "OTHER", t.Task, "", unitLighting)
	r.at("2026-02-12 13:30")
	r.complete(userChloe, repair, t.TaskNote)
	r.entry(userChloe, report, comment, t.Repaired, participants)
	r.at("2026-02-13 08:15")
	r.transition(userChloe, report, closed, t.ClosingReason)
}
