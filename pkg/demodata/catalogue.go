package demodata

import (
	v1 "github.com/lao-tseu-is-alive/go-cloud-k8s-poc-2026/gen/goeland/v1"
)

// Org unit keys (docs/DEMO_SCRIPT.md §3).
const (
	unitMunicipality = "municipality"
	unitClerk        = "clerk"
	unitPlanning     = "planning"
	unitRoads        = "roads"
	unitLighting     = "lighting"
	unitParks        = "parks"
	unitPolice       = "police"
	unitSocial       = "social"
	unitEnergy       = "energy"
	unitArchives     = "archives"
)

// orgUnit places a unit in the tree.
type orgUnit struct {
	key, typeCode, parent string
}

// orgChart is the org chart, parents first.
var orgChart = []orgUnit{
	{unitMunicipality, "DIRECTION", ""},
	{unitClerk, "SERVICE", unitMunicipality},
	{unitPlanning, "SERVICE", unitMunicipality},
	{unitRoads, "SERVICE", unitMunicipality},
	{unitLighting, "UNIT", unitRoads},
	{unitParks, "SERVICE", unitMunicipality},
	{unitPolice, "SERVICE", unitMunicipality},
	{unitSocial, "SERVICE", unitMunicipality},
	{unitEnergy, "SERVICE", unitMunicipality},
	{unitArchives, "SERVICE", unitMunicipality},
}

// Case type codes (docs/DEMO_SCRIPT.md §2); the building permit and the
// generic request are seeded by the migrations.
const (
	caseBuildingPermit = "OPC_DEMANDE_PC"
	caseOpposition     = "OPPOSITION"
	caseTreeFelling    = "TREE_FELLING"
	caseAdvertising    = "ADVERTISING_SIGN"
	casePublicDomain   = "PUBLIC_DOMAIN"
	caseCitizenReport  = "CITIZEN_REPORT"
	caseComplaint      = "COMPLAINT"
	caseEnergySubsidy  = "ENERGY_SUBSIDY"
	caseGeneric        = "GENERIC_REQUEST"
)

// Retention policy codes.
const (
	policyPermanent = "PERMIT_ARCHIVE_50"
	policyTenYears  = "ROUTINE_DESTROY_10"
	policyFiveYears = "SHORT_DESTROY_5"
	policyReview    = "REQUEST_REVIEW_10"
	policySubsidy   = "SUBSIDY_ARCHIVE_10"
)

// retentionPolicy is the duration and final disposition of a policy.
type retentionPolicy struct {
	code        string
	years       int32
	disposition v1.FinalDisposition
}

var retentionPolicies = []retentionPolicy{
	{policyPermanent, 50, v1.FinalDisposition_FINAL_DISPOSITION_ARCHIVE},
	{policyTenYears, 10, v1.FinalDisposition_FINAL_DISPOSITION_DESTROY},
	{policyFiveYears, 5, v1.FinalDisposition_FINAL_DISPOSITION_DESTROY},
	{policyReview, 10, v1.FinalDisposition_FINAL_DISPOSITION_REVIEW},
	{policySubsidy, 10, v1.FinalDisposition_FINAL_DISPOSITION_ARCHIVE},
}

// caseType is a case type of the demo: reference namespace, retention policy
// and minimum confidentiality; seeded types are relabelled, not created.
type caseType struct {
	code, namespace, policy string
	confidentiality         int32
	seeded                  bool
}

var caseTypes = []caseType{
	{caseBuildingPermit, "OPC", policyPermanent, 0, true},
	{caseOpposition, "OPP", policyPermanent, 0, false},
	{caseTreeFelling, "ABA", policyTenYears, 0, false},
	{caseAdvertising, "REC", policyTenYears, 0, false},
	{casePublicDomain, "ODP", policyFiveYears, 0, false},
	{caseCitizenReport, "SIG", policyFiveYears, 0, false},
	{caseComplaint, "PLA", policyReview, 0, false},
	{caseEnergySubsidy, "SUB", policySubsidy, 2, false},
	{caseGeneric, "GEN", policyReview, 0, true},
}

// Document type codes; the first seven are seeded by the migrations.
const (
	docForm           = "FORM"
	docIncomingLetter = "INCOMING_LETTER"
	docOutgoingLetter = "OUTGOING_LETTER"
	docPlan           = "PLAN"
	docPhoto          = "PHOTO"
	docReport         = "REPORT"
	docDecision       = "DECISION"
	docServiceOpinion = "SERVICE_OPINION"
	docInquiryNotice  = "INQUIRY_NOTICE"
	docOpposition     = "OPPOSITION"
	docSupporting     = "SUPPORTING_DOCUMENT"
	docInternalNote   = "INTERNAL_NOTE"
)

// documentType is a document type and its category; seeded types are relabelled.
type documentType struct {
	code, category string
	seeded         bool
}

var documentTypes = []documentType{
	{docForm, "ENTREE", true},
	{docIncomingLetter, "ENTREE", true},
	{docOutgoingLetter, "SORTIE", true},
	{docPlan, "PLAN", true},
	{docPhoto, "JUSTIFICATIF", true},
	{docReport, "JUSTIFICATIF", true},
	{docDecision, "DECISION", true},
	{docServiceOpinion, "JUSTIFICATIF", false},
	{docInquiryNotice, "SORTIE", false},
	{docOpposition, "ENTREE", false},
	{docSupporting, "JUSTIFICATIF", false},
	{docInternalNote, "JUSTIFICATIF", false},
}

// setupUsers records every demo user (Alice first, bootstrapped as
// administrator) and makes David a records manager.
func (r *run) setupUsers() {
	for _, u := range demoUsers {
		r.users[u.Key] = call(r, u.Key, "record user", r.c.core.GetCurrentUser, &v1.GetCurrentUserRequest{}).GetUser()
	}
	if r.err == nil && !r.users[userAlice].GetIsAdmin() {
		r.err = errNotAdmin
		return
	}
	call(r, userAlice, "grant records manager", r.c.core.GrantUserRole, &v1.GrantUserRoleRequest{
		UserId: r.users[userDavid].GetId(), RoleCode: "RECORDS_MANAGER", Reason: r.t.Reason,
	})
}

// setupOrgChart creates the org units and the memberships of the demo users.
func (r *run) setupOrgChart() {
	for _, u := range orgChart {
		text := r.t.Units[u.key]
		r.units[u.key] = call(r, userAlice, "create org unit "+u.key, r.c.orgUnits.CreateOrgUnit, &v1.CreateOrgUnitRequest{
			OrgUnitTypeCode: u.typeCode, Label: text.Label, Abbreviation: text.Abbreviation,
			ParentId: r.units[u.parent], Reason: r.t.Reason,
		}).GetOrgUnit().GetSubjectRef().GetId()
	}
	for _, u := range demoUsers {
		if u.Unit != "" {
			r.link(userAlice, r.users[u.Key].GetSubjectId(), r.units[u.Unit], "USER_MEMBER_OF_ORG_UNIT")
		}
	}
	r.count["org units"] = len(orgChart)
}

// setupCatalogue creates the retention policies and the case and document
// types of the demo, and relabels the seeded ones in the demo language.
func (r *run) setupCatalogue() {
	for _, p := range retentionPolicies {
		text := r.t.Policies[p.code]
		call(r, userAlice, "create retention policy "+p.code, r.c.retention.CreateRetentionPolicy, &v1.CreateRetentionPolicyRequest{
			Code: p.code, Label: text.Label, Description: text.Description, LegalBasis: text.LegalBasis,
			DurationYears: p.years, FinalDisposition: p.disposition, Reason: r.t.Reason,
		})
	}
	for _, ct := range caseTypes {
		r.setupCaseType(ct)
	}
	for _, dt := range documentTypes {
		r.setupDocumentType(dt)
	}
}

func (r *run) setupCaseType(ct caseType) {
	text := r.t.CaseTypes[ct.code]
	if ct.seeded {
		call(r, userAlice, "update case type "+ct.code, r.c.cases.UpdateCaseType, &v1.UpdateCaseTypeRequest{
			Code: ct.code, Label: &text.Label, Description: &text.Description, BusinessRefNamespace: &ct.namespace,
			DefaultConfidentialityLevel: &ct.confidentiality, DefaultRetentionPolicyCode: &ct.policy, Reason: r.t.Reason,
		})
		return
	}
	call(r, userAlice, "create case type "+ct.code, r.c.cases.CreateCaseType, &v1.CreateCaseTypeRequest{
		Code: ct.code, Label: text.Label, Description: text.Description, BusinessRefNamespace: ct.namespace,
		DefaultConfidentialityLevel: ct.confidentiality, DefaultRetentionPolicyCode: ct.policy, Reason: r.t.Reason,
	})
}

func (r *run) setupDocumentType(dt documentType) {
	text := r.t.DocumentTypes[dt.code]
	if dt.seeded {
		call(r, userAlice, "update document type "+dt.code, r.c.documents.UpdateDocumentType, &v1.UpdateDocumentTypeRequest{
			Code: dt.code, Label: &text.Label, Description: &text.Description, Category: &dt.category, Reason: r.t.Reason,
		})
		return
	}
	call(r, userAlice, "create document type "+dt.code, r.c.documents.CreateDocumentType, &v1.CreateDocumentTypeRequest{
		Code: dt.code, Label: text.Label, Description: text.Description, Category: dt.category, Reason: r.t.Reason,
	})
}
