package demodata

// Texts of the ongoing work (DEMO_SCRIPT §4.8 to §4.12, GLD-092), of the demo
// security groups, and the English labels of the seeded catalogues.

// ongoingText holds the cases still in progress on the generation day.
type ongoingText struct {
	// Chalet is the building permit awaiting its opinions (§4.8).
	Chalet chaletText
	// Cedar is the tree felling request (§4.9).
	Cedar cedarText
	// Terrace is the café terrace on the public domain (§4.10).
	Terrace terraceText
	// Bench is M. Tutu's request for a bench (§4.11).
	Bench benchText
	// NewSubsidy is the confidential subsidy shared with a group (§4.12).
	NewSubsidy newSubsidyText
}

// chaletText holds the texts of the permit « Chalet des Pins » (§4.8).
type chaletText struct {
	// CaseTitle is the title of the case.
	CaseTitle string
	// CaseDescription is its description.
	CaseDescription string
	// BuildingName is the name of the chalet.
	BuildingName string
	// RequestForm is the application form.
	RequestForm docText
	// Plan is the architect's plan.
	Plan docText
	// Filed is the timeline entry of the filing.
	Filed entryText
	// CheckTask is the completeness check task.
	CheckTask string
	// CheckNote is its completion note.
	CheckNote string
	// CirculationTitle is the title of the circulation for opinions.
	CirculationTitle string
	// CirculationMessage is its message.
	CirculationMessage string
	// RoadsAnswer is the opinion of Voirie et propreté.
	RoadsAnswer string
	// DecisionTask is Bruno's task to prepare the decision.
	DecisionTask string
}

// cedarText holds the texts of the tree felling request (§4.9).
type cedarText struct {
	// CaseTitle is the title of the case.
	CaseTitle string
	// CaseDescription is its description.
	CaseDescription string
	// TreeName is the name of the tree.
	TreeName string
	// Letter is the resident's request.
	Letter docText
	// Received is the timeline entry of the request.
	Received entryText
	// VisitTask is Hugo's site visit.
	VisitTask string
	// CallbackTask is the call-back assigned to the parks unit.
	CallbackTask string
}

// terraceText holds the texts of the café terrace request (§4.10).
type terraceText struct {
	// CaseTitle is the title of the case.
	CaseTitle string
	// CaseDescription is its description.
	CaseDescription string
	// TerraceName is the name of the terrace area.
	TerraceName string
	// Request is the café's request.
	Request docText
	// Received is the timeline entry of the request.
	Received entryText
	// CheckTask is Chloé's completeness check.
	CheckTask string
	// LightingTask is the lighting check, reassigned to the lighting unit.
	LightingTask string
	// ReassignReason is why it was reassigned.
	ReassignReason string
}

// benchText holds the texts of M. Tutu's request for a bench (§4.11).
type benchText struct {
	// CaseTitle is the title of the case.
	CaseTitle string
	// CaseDescription is its description.
	CaseDescription string
	// Letter is M. Tutu's letter.
	Letter docText
	// Received is the timeline entry of the request.
	Received entryText
	// CallbackTask is Félix's call-back, due today.
	CallbackTask string
	// OrderTask is the order of a bench, cancelled.
	OrderTask string
	// CancelReason is why it was cancelled.
	CancelReason string
	// VisitTask is the site visit, done then reopened.
	VisitTask string
	// VisitNote is its completion note.
	VisitNote string
	// ReopenReason is why it was reopened.
	ReopenReason string
}

// newSubsidyText holds the texts of the new confidential subsidy (§4.12).
type newSubsidyText struct {
	// CaseTitle is the title of the case.
	CaseTitle string
	// CaseDescription is its description.
	CaseDescription string
	// BuildingName is the name of the house.
	BuildingName string
	// Form is the application form.
	Form docText
	// Filed is the timeline entry of the filing.
	Filed entryText
	// CheckTask is Emma's completeness check.
	CheckTask string
	// GrantReason is why the group may read the case.
	GrantReason string
}

// groupsText names the demo security groups.
type groupsText struct {
	// Salubrity is the « Commission de salubrité » (Chloé, Félix, Hugo).
	Salubrity label
	// SalubrityGrantReason is why it may contribute to the complaint (§4.3).
	SalubrityGrantReason string
	// EnergyPlanning is the « Cellule énergie et urbanisme » (Bruno, Emma).
	EnergyPlanning label
}

// seededText relabels the catalogues the migrations seed in French, for a
// demo in another language (nil keeps the migrations' labels).
type seededText struct {
	// TaskTypes labels the task types, by code.
	TaskTypes map[string]label
	// OrgUnitTypes labels the org unit types, by code.
	OrgUnitTypes map[string]label
	// ThingTypes labels the thing types, by code.
	ThingTypes map[string]label
	// RelationshipTypes labels the relationship types and their roles, by code.
	RelationshipTypes map[string]relationshipText
}

// relationshipText labels a relationship type and the role it names.
type relationshipText struct {
	// Label reads from the source.
	Label string
	// InverseLabel reads from the target.
	InverseLabel string
	// Description explains the type.
	Description string
	// RoleLabel is the role one end plays.
	RoleLabel string
	// RoleLabelPlural is its plural.
	RoleLabelPlural string
}

// Codes the migrations seed, which a seededText must all relabel.
var (
	seededTaskTypes     = []string{"CALLBACK", "CIRCULATION_RESPONSE", "DOCUMENT_CHECK", "OPINION_REQUEST", "OTHER", "SITE_VISIT"}
	seededOrgUnitTypes  = []string{"BUREAU", "DIRECTION", "DIVISION", "ENTERPRISE", "OFFICE", "SERVICE", "UNIT"}
	seededThingTypes    = []string{"ADVERTISEMENT", "BUILDING", "INFRASTRUCTURE", "PARCEL", "SPORT_ZONE", "STREET", "TREE"}
	seededRelationTypes = []string{
		"ACTOR_BRANCH_OF_ACTOR", "ACTOR_CONTACT_PERSON_OF_ACTOR", "CASE_CONCERNS_THING", "CASE_HAS_ACTOR_ARCHITECT",
		"CASE_HAS_ACTOR_CONTRACTOR", "CASE_HAS_ACTOR_MANDATEE", "CASE_HAS_ACTOR_OWNER", "CASE_HAS_ACTOR_REQUESTER",
		"CASE_HAS_DOCUMENT", "CASE_HAS_ORG_UNIT_LEADER", "CASE_HAS_ORG_UNIT_MANAGER", "CASE_HAS_ORG_UNIT_PARTICIPANT",
		"CASE_PARENT_OF_CASE", "CASE_RELATED_TO_CASE", "DOCUMENT_AUTHORED_BY_ACTOR", "DOCUMENT_PREVIOUS_VERSION",
		"DOCUMENT_REPRESENTS_THING", "DOCUMENT_SENT_TO_ACTOR", "THING_CONTAINS_THING", "THING_HAS_ACTOR_EASEMENT_HOLDER",
		"THING_HAS_ACTOR_FARMER", "THING_HAS_ACTOR_OWNER", "THING_HAS_ACTOR_SURFACE_RIGHT_HOLDER", "THING_HAS_ACTOR_TENANT",
		"USER_MEMBER_OF_GROUP", "USER_MEMBER_OF_ORG_UNIT",
	}
)
