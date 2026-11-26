package demodata

import (
	"time"

	v1 "github.com/lao-tseu-is-alive/go-cloud-k8s-poc-2026/gen/goeland/v1"
	"google.golang.org/protobuf/types/known/timestamppb"
)

// The ongoing work of the demo (DEMO_SCRIPT §4.8 to §4.12, GLD-092): cases
// still in progress on the generation day, dated from it, so every demo user
// opens « Mon tableau de bord » on today's work: tasks open, in progress,
// overdue, assigned to a person or a unit, reassigned, cancelled, reopened,
// and a circulation waiting for its last opinions. Regenerate the demo
// (scripts/demo_seed.sh -r) the day before showing it.

// Shorthands of the task and circulation enums of the ongoing work.
const (
	readLevel       = v1.Permission_PERMISSION_READ
	contributeLevel = v1.Permission_PERMISSION_CONTRIBUTE
	groupGrantee    = v1.GranteeKind_GRANTEE_KIND_GROUP
)

// daysAgo starts a story step n days before the generation day at hhmm
// ("15:04", Europe/Zurich).
func (r *run) daysAgo(n int, hhmm string) {
	clock, err := time.Parse("15:04", hhmm)
	if err != nil {
		panic("demodata: invalid time " + hhmm) // a typo in a story, caught by the tests
	}
	day := r.today.AddDate(0, 0, -n)
	r.at(time.Date(day.Year(), day.Month(), day.Day(), clock.Hour(), clock.Minute(), 0, 0, zurich).Format("2006-01-02 15:04"))
}

// dueIn is a deadline n days after the generation day (n < 0: past) at hhmm.
func (r *run) dueIn(n int, hhmm string) *timestamppb.Timestamp {
	clock, err := time.Parse("15:04", hhmm)
	if err != nil {
		panic("demodata: invalid time " + hhmm)
	}
	day := r.today.AddDate(0, 0, n)
	return timestamppb.New(time.Date(day.Year(), day.Month(), day.Day(), clock.Hour(), clock.Minute(), 0, 0, zurich))
}

// year is the year of the generation day, for the references of the ongoing cases.
func (r *run) year() string {
	return r.today.Format("2006")
}

// start, reassign, cancel and reopen move a task.
func (r *run) start(user, taskID string) {
	call(r, user, "start task", r.c.tasks.StartTask, &v1.StartTaskRequest{Id: taskID})
}

func (r *run) reassignToUnit(user, taskID, unit, reason string) {
	call(r, user, "reassign task", r.c.tasks.AssignTask, &v1.AssignTaskRequest{Id: taskID, AssigneeOrgUnitId: r.units[unit], Reason: reason})
}

func (r *run) cancel(user, taskID, reason string) {
	call(r, user, "cancel task", r.c.tasks.CancelTask, &v1.CancelTaskRequest{Id: taskID, Reason: reason})
}

func (r *run) reopen(user, taskID, reason string) {
	call(r, user, "reopen task", r.c.tasks.ReopenTask, &v1.ReopenTaskRequest{Id: taskID, Reason: reason})
}

// group creates a security group as the administrator, with members.
func (r *run) group(text label, members ...string) string {
	id := call(r, userAlice, "create group "+text.Label, r.c.access.CreateGroup, &v1.CreateGroupRequest{
		Name: text.Label, Description: text.Description,
	}).GetGroup().GetSubjectRef().GetId()
	for _, member := range members {
		call(r, userAlice, "add group member", r.c.access.AddGroupMember, &v1.AddGroupMemberRequest{GroupId: id, UserId: r.users[member].GetId()})
	}
	r.count["groups"]++
	return id
}

// grantGroup gives the group level on subject, as user.
func (r *run) grantGroup(user, subject, group string, level v1.Permission, reason string) {
	call(r, user, "grant group", r.c.access.SetGrant, &v1.SetGrantRequest{
		SubjectId: subject, GranteeKind: groupGrantee, GranteeId: group, Level: level, Reason: reason,
	})
}

// salubrityCommission creates the « Commission de salubrité » and lets it
// contribute to M. Tutu's complaint (§4.3, GLD-092).
func (r *run) salubrityCommission(complaint string) {
	g := r.t.Groups
	commission := r.group(g.Salubrity, userChloe, userFelix, userHugo)
	r.grantGroup(userChloe, complaint, commission, contributeLevel, g.SalubrityGrantReason)
}

// ongoing writes §4.8 to §4.12; cafe and tutu are actors of the earlier stories.
func (r *run) ongoing(cafe, tutu string) {
	r.chalet()
	r.cedar()
	r.terrace(cafe)
	r.bench(tutu)
	r.newSubsidy()
}

// chalet is the permit « Chalet des Pins » (§4.8): checked, its circulation
// answered by Voirie, overdue for Parcs, in progress for Énergie, and Bruno's
// decision task open.
func (r *run) chalet() {
	t := r.t.Ongoing.Chalet
	r.daysAgo(21, "09:00")
	owner := r.person(userBruno, madame, "Hélène", "Duc", "helene.duc@example.ch")
	chalet := r.thing(userBruno, &v1.CreateThingRequest{ThingTypeCode: "BUILDING", Name: t.BuildingName, GeometryGeojson: r.placeOf("pins-building", "BUILDING")})
	c := r.newCase(userBruno, caseBuildingPermit, r.year()+"-000087", t.CaseTitle, t.CaseDescription, unitPlanning, 0)
	r.link(userBruno, c, owner, "CASE_HAS_ACTOR_REQUESTER")
	r.link(userBruno, c, chalet, "CASE_CONCERNS_THING")
	form := r.document(userBruno, docForm, t.RequestForm, c, true)
	plan := r.document(userBruno, docPlan, t.Plan, c, false)
	r.entry(userBruno, c, request, t.Filed, participants, form, plan)
	check := r.taskDue(userBruno, c, "DOCUMENT_CHECK", t.CheckTask, userBruno, "", r.dueIn(-14, "17:00"))
	r.daysAgo(20, "10:30")
	r.openDocument(userBruno, form, plan)
	r.complete(userBruno, check, t.CheckNote)

	r.daysAgo(14, "10:00")
	recipients, tasks := r.circulateDue(userBruno, c, t.CirculationTitle, t.CirculationMessage, r.dueIn(-2, "17:00"), unitRoads, unitParks, unitEnergy)
	r.daysAgo(10, "14:20")
	r.openCase(userChloe, c)
	r.openDocument(userChloe, plan)
	r.respond(userChloe, recipients[unitRoads], favorable, t.RoadsAnswer)
	r.daysAgo(1, "15:10")
	r.openCase(userEmma, c)
	r.openDocument(userEmma, plan)
	r.start(userEmma, tasks[unitEnergy])
	r.daysAgo(1, "16:30")
	r.openCase(userBruno, c)
	r.taskDue(userBruno, c, "OTHER", t.DecisionTask, userBruno, "", r.dueIn(14, "17:00"))
}

// cedar is the tree felling request (§4.9): Hugo's site visit in progress,
// a call-back waiting in the parks unit.
func (r *run) cedar() {
	t := r.t.Ongoing.Cedar
	r.daysAgo(6, "08:30")
	resident := r.person(userHugo, madame, "Isabelle", "Rey", "isabelle.rey@example.ch")
	tree := r.thing(userHugo, &v1.CreateThingRequest{ThingTypeCode: "TREE", Name: t.TreeName, GeometryGeojson: r.placeOf("roseraie-cedar", "TREE")})
	c := r.newCase(userHugo, caseTreeFelling, r.year()+"-000012", t.CaseTitle, t.CaseDescription, unitParks, 0)
	r.link(userHugo, c, resident, "CASE_HAS_ACTOR_REQUESTER")
	r.link(userHugo, c, tree, "CASE_CONCERNS_THING")
	letter := r.document(userHugo, docIncomingLetter, t.Letter, c, true)
	r.link(userHugo, letter, resident, "DOCUMENT_AUTHORED_BY_ACTOR")
	r.entry(userHugo, c, request, t.Received, participants, letter)
	visit := r.taskDue(userHugo, c, "SITE_VISIT", t.VisitTask, userHugo, "", r.dueIn(1, "12:00"))
	r.taskDue(userHugo, c, "CALLBACK", t.CallbackTask, "", unitParks, r.dueIn(5, "17:00"))
	r.daysAgo(2, "07:45")
	r.openCase(userHugo, c)
	r.openThing(userHugo, tree)
	r.start(userHugo, visit)
}

// terrace is the café terrace (§4.10): Chloé's check open, the lighting
// check reassigned to the lighting unit while she is away.
func (r *run) terrace(cafe string) {
	t := r.t.Ongoing.Terrace
	r.daysAgo(3, "09:15")
	area := r.thing(userChloe, &v1.CreateThingRequest{ThingTypeCode: "INFRASTRUCTURE", Name: t.TerraceName, GeometryGeojson: r.placeOf("cafe-du-lac-terrace", "INFRASTRUCTURE")})
	c := r.newCase(userChloe, casePublicDomain, r.year()+"-000041", t.CaseTitle, t.CaseDescription, unitRoads, 0)
	r.link(userChloe, c, cafe, "CASE_HAS_ACTOR_REQUESTER")
	r.link(userChloe, c, area, "CASE_CONCERNS_THING")
	req := r.document(userChloe, docForm, t.Request, c, true)
	r.link(userChloe, req, cafe, "DOCUMENT_AUTHORED_BY_ACTOR")
	r.entry(userChloe, c, request, t.Received, participants, req)
	r.taskDue(userChloe, c, "DOCUMENT_CHECK", t.CheckTask, userChloe, "", r.dueIn(4, "17:00"))
	lighting := r.taskDue(userChloe, c, "OTHER", t.LightingTask, userChloe, "", r.dueIn(7, "17:00"))
	r.daysAgo(1, "11:00")
	r.reassignToUnit(userChloe, lighting, unitLighting, t.ReassignReason)
}

// bench is M. Tutu's request for a bench (§4.11): Félix's call-back due
// today, a duplicate order cancelled, the site visit done then reopened.
func (r *run) bench(tutu string) {
	t := r.t.Ongoing.Bench
	r.daysAgo(8, "10:00")
	c := r.newCase(userFelix, caseComplaint, r.year()+"-000134", t.CaseTitle, t.CaseDescription, unitSocial, 0)
	r.link(userFelix, c, tutu, "CASE_HAS_ACTOR_REQUESTER")
	letter := r.document(userFelix, docIncomingLetter, t.Letter, c, true)
	r.link(userFelix, letter, tutu, "DOCUMENT_AUTHORED_BY_ACTOR")
	r.entry(userFelix, c, request, t.Received, participants, letter)
	r.taskDue(userFelix, c, "CALLBACK", t.CallbackTask, userFelix, "", r.dueIn(0, "16:00"))
	visit := r.taskDue(userFelix, c, "SITE_VISIT", t.VisitTask, userFelix, "", r.dueIn(3, "12:00"))
	r.daysAgo(7, "09:30")
	order := r.task(userFelix, c, "OTHER", t.OrderTask, userFelix, "")
	r.daysAgo(5, "14:00")
	r.cancel(userFelix, order, t.CancelReason)
	r.daysAgo(4, "10:15")
	r.openCase(userFelix, c)
	r.complete(userFelix, visit, t.VisitNote)
	r.daysAgo(2, "08:50")
	r.reopen(userFelix, visit, t.ReopenReason)
}

// newSubsidy is the new confidential subsidy (§4.12): Emma's check open, and
// the « Cellule énergie et urbanisme » may read it, so Bruno, refused the
// subsidy of §4.4, opens this one.
func (r *run) newSubsidy() {
	t := r.t.Ongoing.NewSubsidy
	g := r.t.Groups
	r.daysAgo(2, "14:00")
	requester := r.person(userEmma, monsieur, "Karim", "Haddad", "karim.haddad@example.ch")
	house := r.thing(userEmma, &v1.CreateThingRequest{ThingTypeCode: "BUILDING", Name: t.BuildingName, GeometryGeojson: r.placeOf("midi7-building", "BUILDING")})
	c := r.newCase(userEmma, caseEnergySubsidy, r.year()+"-000021", t.CaseTitle, t.CaseDescription, unitEnergy, 2)
	r.link(userEmma, c, requester, "CASE_HAS_ACTOR_REQUESTER")
	r.link(userEmma, c, house, "CASE_CONCERNS_THING")
	form := r.document(userEmma, docForm, t.Form, c, true)
	r.entry(userEmma, c, request, t.Filed, participants, form)
	r.taskDue(userEmma, c, "DOCUMENT_CHECK", t.CheckTask, userEmma, "", r.dueIn(5, "17:00"))
	cell := r.group(g.EnergyPlanning, userBruno, userEmma)
	r.grantGroup(userEmma, c, cell, readLevel, t.GrantReason)
	r.daysAgo(1, "09:40")
	r.openCase(userBruno, c)
}

// relabelSeeded relabels the catalogues the migrations seed in French, for a
// demo in another language (GLD-092); a French demo keeps them.
func (r *run) relabelSeeded() {
	s := r.t.Seeded
	if s == nil {
		return
	}
	for _, code := range seededTaskTypes {
		l := s.TaskTypes[code]
		call(r, userAlice, "relabel task type "+code, r.c.tasks.UpdateTaskType, &v1.UpdateTaskTypeRequest{Code: code, Label: &l.Label, Description: &l.Description, Reason: r.t.Reason})
	}
	for _, code := range seededOrgUnitTypes {
		l := s.OrgUnitTypes[code]
		call(r, userAlice, "relabel org unit type "+code, r.c.orgUnits.UpdateOrgUnitType, &v1.UpdateOrgUnitTypeRequest{Code: code, Label: &l.Label, Description: &l.Description, Reason: r.t.Reason})
	}
	for _, code := range seededThingTypes {
		l := s.ThingTypes[code]
		call(r, userAlice, "relabel thing type "+code, r.c.things.UpdateThingType, &v1.UpdateThingTypeRequest{Code: code, Label: &l.Label, Description: &l.Description, Reason: r.t.Reason})
	}
	for _, code := range seededRelationTypes {
		l := s.RelationshipTypes[code]
		call(r, userAlice, "relabel relationship type "+code, r.c.core.UpdateRelationshipType, &v1.UpdateRelationshipTypeRequest{
			Code: code, Label: &l.Label, InverseLabel: &l.InverseLabel, Description: &l.Description,
			RoleLabel: &l.RoleLabel, RoleLabelPlural: &l.RoleLabelPlural, Reason: r.t.Reason,
		})
	}
}
