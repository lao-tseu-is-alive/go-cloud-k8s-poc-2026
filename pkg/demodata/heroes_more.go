package demodata

import v1 "github.com/lao-tseu-is-alive/go-cloud-k8s-poc-2026/gen/goeland/v1"

// More enum shorthands of the stories.
const (
	restricted = v1.TimelineVisibility_TIMELINE_VISIBILITY_RESTRICTED
	reopened   = v1.CaseStatus_CASE_STATUS_OPEN
)

// complaint is the recurring complaint « Propreté et nuisances, rue du Port
// 12 » (§4.3): M. Tutu's older noise complaint of 2024, then his cleanliness
// complaint of September 2025, answered by three services, closed, reopened
// when he writes again six weeks later, and closed again. Chloé handles it;
// she files the police report as a restricted note that Félix, who answers
// for Cohésion sociale, cannot read.
func (r *run) complaint() {
	t := r.t.Complaint
	r.at("2024-06-03 10:20")
	tutu := r.person(userChloe, monsieur, "Jean", "Tutu", "jean.tutu@example.ch")
	old := r.oldComplaint(tutu)

	r.at("2025-09-15 08:20")
	manager := r.organization(userChloe, "Lacrégie SA", "GERANCE_IMMOBILIERE", "gerance@lacregie.example.ch")
	street := r.thing(userChloe, &v1.CreateThingRequest{ThingTypeCode: "STREET", Name: t.StreetName, GeometryGeojson: r.placeOf("port12-street", "STREET")})
	building := r.thing(userChloe, &v1.CreateThingRequest{ThingTypeCode: "BUILDING", Name: t.BuildingName, GeometryGeojson: r.placeOf("port12-building", "BUILDING")})
	r.link(userChloe, building, manager, "THING_HAS_ACTOR_OWNER")
	c := r.newCase(userChloe, caseComplaint, "2025-000118", t.CaseTitle, t.CaseDescription, unitRoads, 0)
	r.link(userChloe, c, tutu, "CASE_HAS_ACTOR_REQUESTER")
	r.link(userChloe, c, street, "CASE_CONCERNS_THING")
	r.link(userChloe, c, building, "CASE_CONCERNS_THING")
	letter := r.document(userChloe, docIncomingLetter, t.Letter, c, true)
	r.link(userChloe, letter, tutu, "DOCUMENT_AUTHORED_BY_ACTOR")
	litter := r.photo(userChloe, t.LitterPhoto, "complaint-litter-entrance.jpg", c)
	pavement := r.photo(userChloe, t.PavementPhoto, "complaint-pavement-dirty.jpg", c)
	r.entry(userChloe, c, request, t.Received, participants, letter, litter, pavement)

	// Chloé reads his earlier complaint before asking the services.
	r.at("2025-09-16 08:40")
	r.openCase(userChloe, old)
	r.openCase(userChloe, c)
	r.at("2025-09-16 09:00")
	recipients := r.circulate(userChloe, c, t.CirculationTitle, t.CirculationMessage, unitRoads, unitPolice, unitSocial)
	r.at("2025-09-17 08:30")
	r.openCase(userFelix, c)
	r.openDocument(userFelix, letter, litter)
	r.at("2025-09-18 10:40")
	r.openDocument(userChloe, litter, pavement)
	r.respond(userChloe, recipients[unitRoads], favorable, t.RoadsAnswer)
	cleaning := r.task(userChloe, c, "OTHER", t.CleaningTask, "", unitRoads)
	r.at("2025-09-23 16:10")
	// No police account in the demo: Chloé, managing the case, records the answer the police sent her.
	r.respond(userChloe, recipients[unitPolice], remark, t.PoliceAnswer)
	note := r.document(userChloe, docInternalNote, t.PoliceNote, c, true)
	r.entry(userChloe, c, comment, t.PoliceEntry, restricted, note)
	r.at("2025-09-25 11:30")
	r.openCase(userFelix, c)
	r.respond(userFelix, recipients[unitSocial], remark, t.SocialAnswer)
	// Félix, his part done, tries to close the case: only its owner may.
	r.at("2025-09-26 14:05")
	refused(r, userFelix, "close the complaint", r.c.cases.TransitionCase, &v1.TransitionCaseRequest{Id: c, TargetStatus: closed, Reason: t.ClosingReason})

	r.at("2025-10-02 14:00")
	r.openCase(userChloe, c)
	r.complete(userChloe, cleaning, t.CleaningNote)
	reply := r.document(userChloe, docOutgoingLetter, t.Reply, c, true)
	r.link(userChloe, reply, tutu, "DOCUMENT_SENT_TO_ACTOR")
	r.entry(userChloe, c, decision, t.ReplyEntry, participants, reply)
	r.transition(userChloe, c, closed, t.ClosingReason)

	r.complaintAgain(c, tutu)
}

// oldComplaint is M. Tutu's noise complaint of June 2024, owned by the police.
func (r *run) oldComplaint(tutu string) string {
	t := r.t.Complaint
	old := r.newCase(userChloe, caseComplaint, "2024-000052", t.OldCaseTitle, t.OldCaseDescription, unitPolice, 0)
	r.link(userChloe, old, tutu, "CASE_HAS_ACTOR_REQUESTER")
	letter := r.document(userChloe, docIncomingLetter, t.OldLetter, old, true)
	r.link(userChloe, letter, tutu, "DOCUMENT_AUTHORED_BY_ACTOR")
	r.entry(userChloe, old, request, t.OldReceived, participants, letter)
	r.at("2024-06-27 15:45")
	reply := r.document(userChloe, docOutgoingLetter, t.OldReply, old, true)
	r.link(userChloe, reply, tutu, "DOCUMENT_SENT_TO_ACTOR")
	r.entry(userChloe, old, decision, t.OldReplyEntry, participants, reply)
	r.transition(userChloe, old, closed, t.OldClosingReason)
	return old
}

// complaintAgain reopens the complaint on M. Tutu's second letter.
func (r *run) complaintAgain(c, tutu string) {
	t := r.t.Complaint
	r.at("2025-11-17 08:50")
	r.transition(userChloe, c, reopened, t.ReopenReason)
	letter := r.document(userChloe, docIncomingLetter, t.SecondLetter, c, true)
	r.link(userChloe, letter, tutu, "DOCUMENT_AUTHORED_BY_ACTOR")
	r.entry(userChloe, c, request, t.SecondReceived, participants, letter)
	check := r.task(userChloe, c, "SITE_VISIT", t.CheckTask, userChloe, "")
	r.at("2025-11-18 16:10")
	r.openCase(userFelix, c)
	r.openDocument(userFelix, letter)
	r.at("2025-11-20 07:30")
	r.openCase(userChloe, c)
	r.complete(userChloe, check, t.CheckNote)
	r.at("2025-11-24 10:15")
	reply := r.document(userChloe, docOutgoingLetter, t.SecondReply, c, true)
	r.link(userChloe, reply, tutu, "DOCUMENT_SENT_TO_ACTOR")
	r.entry(userChloe, c, decision, t.SecondReplyEntry, participants, reply)
	r.at("2025-11-25 09:00")
	r.transition(userChloe, c, closed, t.SecondClosingReason)
}

// subsidy is the confidential energy subsidy « Pompe à chaleur, chemin des
// Vignes 4 » (§4.4), handled by Emma from August to November 2025. Its type
// makes it confidential (level 2): Bruno neither finds nor opens it, and
// Emma's reads are recorded in its access log, like Bruno's refused read.
func (r *run) subsidy() {
	t := r.t.Subsidy
	r.at("2025-08-18 09:30")
	marc := r.person(userEmma, monsieur, "Marc", "Berger", "marc.berger@example.ch")
	anne := r.person(userEmma, madame, "Anne", "Berger", "")
	house := r.thing(userEmma, &v1.CreateThingRequest{ThingTypeCode: "BUILDING", Name: t.BuildingName, GeometryGeojson: r.placeOf("vignes4-building", "BUILDING")})
	c := r.newCase(userEmma, caseEnergySubsidy, "2025-000009", t.CaseTitle, t.CaseDescription, unitEnergy, 2)
	r.link(userEmma, c, marc, "CASE_HAS_ACTOR_REQUESTER")
	r.link(userEmma, c, anne, "CASE_HAS_ACTOR_REQUESTER")
	r.link(userEmma, c, house, "CASE_CONCERNS_THING")
	form := r.document(userEmma, docForm, t.Form, c, true)
	quote := r.document(userEmma, docSupporting, t.Quote, c, true)
	income := r.document(userEmma, docSupporting, t.IncomeCertificate, c, true)
	r.entry(userEmma, c, request, t.Filed, participants, form, quote, income)
	check := r.task(userEmma, c, "DOCUMENT_CHECK", t.CheckTask, userEmma, "")
	r.at("2025-08-25 14:10")
	r.openCase(userEmma, c)
	r.openDocument(userEmma, quote, income)
	r.complete(userEmma, check, t.CheckNote)
	// Bruno, who heard of the heat pump at chemin des Vignes 4, tries to open
	// the case: refused, and the refusal is in its access log.
	r.at("2025-10-06 10:30")
	refused(r, userBruno, "open the confidential subsidy", r.c.cases.GetCase, &v1.GetCaseRequest{Id: c})
	r.at("2025-09-10 10:00")
	promise := r.document(userEmma, docDecision, t.Promise, c, true)
	r.link(userEmma, promise, marc, "DOCUMENT_SENT_TO_ACTOR")
	r.entry(userEmma, c, decision, t.PromiseEntry, participants, promise)
	r.at("2025-11-03 16:40")
	photo := r.photo(userEmma, t.Photo, "subsidy-heat-pump-installed.jpg", c)
	r.link(userEmma, photo, house, "DOCUMENT_REPRESENTS_THING")
	invoice := r.document(userEmma, docSupporting, t.Invoice, c, true)
	r.at("2025-11-10 11:20")
	r.openCase(userEmma, c)
	r.openDocument(userEmma, invoice)
	payment := r.document(userEmma, docOutgoingLetter, t.Payment, c, true)
	r.link(userEmma, payment, marc, "DOCUMENT_SENT_TO_ACTOR")
	r.entry(userEmma, c, decision, t.PaymentEntry, participants, photo, invoice, payment)
	r.at("2025-11-12 08:30")
	r.transition(userEmma, c, closed, t.ClosingReason)
	r.at("2026-03-02 09:15")
	r.openCase(userEmma, c)
}

// signs are the two advertising sign cases of §4.5, authorized in 2014 and
// closed in early 2015 under a 10-year destruction policy, so both are due:
// the Café du Lac sign is ready for an elimination bordereau, the Garage du
// Port sign is kept by a legal hold David placed in June 2025. The bordereau,
// its approval and the simulation are done live in the demo.
func (r *run) signs() {
	t := r.t.Signs
	r.at("2014-03-10 10:00")
	cafe := r.organization(userBruno, "Café du Lac Sàrl", "CAFE_RESTO_HOTEL", "")
	sign := r.thing(userBruno, &v1.CreateThingRequest{ThingTypeCode: "ADVERTISEMENT", Name: t.SignName, GeometryGeojson: r.placeOf("cafe-du-lac-sign", "ADVERTISEMENT")})
	c := r.newCase(userBruno, caseAdvertising, "2014-000023", t.CaseTitle, t.CaseDescription, unitPlanning, 0)
	r.link(userBruno, c, cafe, "CASE_HAS_ACTOR_REQUESTER")
	r.link(userBruno, c, sign, "CASE_CONCERNS_THING")
	form := r.document(userBruno, docForm, t.Form, c, true)
	r.entry(userBruno, c, request, t.Filed, participants, form)
	r.at("2014-04-02 11:00")
	authorization := r.document(userBruno, docDecision, t.Authorization, c, true)
	r.entry(userBruno, c, decision, t.Granted, participants, authorization)

	r.at("2014-05-05 14:30")
	garage := r.organization(userBruno, "Garage du Port SA", "GARAGE", "")
	box := r.thing(userBruno, &v1.CreateThingRequest{ThingTypeCode: "ADVERTISEMENT", Name: t.HeldSignName, GeometryGeojson: r.placeOf("garage-port-sign", "ADVERTISEMENT")})
	held := r.newCase(userBruno, caseAdvertising, "2014-000031", t.HeldTitle, t.HeldDescription, unitPlanning, 0)
	r.link(userBruno, held, garage, "CASE_HAS_ACTOR_REQUESTER")
	r.link(userBruno, held, box, "CASE_CONCERNS_THING")
	heldForm := r.document(userBruno, docForm, t.HeldForm, held, true)
	r.entry(userBruno, held, request, t.Filed, participants, heldForm)

	r.at("2015-01-15 09:20")
	photo := r.photo(userBruno, t.Photo, "sign-cafe-du-lac.jpg", c)
	r.link(userBruno, photo, sign, "DOCUMENT_REPRESENTS_THING")
	r.transition(userBruno, c, closed, t.ClosingReason)
	r.at("2015-02-03 10:00")
	heldAuthorization := r.document(userBruno, docDecision, t.HeldAuthorization, held, true)
	r.entry(userBruno, held, decision, t.Granted, participants, heldAuthorization)
	r.transition(userBruno, held, closed, t.HeldClosingReason)

	// David checks both due cases before keeping the garage's sign.
	r.at("2025-06-12 14:40")
	r.openCase(userDavid, c)
	r.openCase(userDavid, held)
	r.openDocument(userDavid, heldAuthorization)
	r.at("2025-06-12 15:00")
	call(r, userDavid, "place a legal hold", r.c.retention.PlaceRetentionHold, &v1.PlaceRetentionHoldRequest{CaseId: held, Reason: t.HoldReason})
}
