package demodata

import (
	"fmt"
	"slices"
	"strings"
)

// textSet is the catalogue of every text the generator writes, in one language.
// Each language fills every field (TestTextsAreComplete); codes, names of
// people and companies, dates and identifiers are not texts and stay the same
// in every language.
type textSet struct {
	// Language is the ISO 639-1 code stored on the generated documents.
	Language string
	// Footer is printed on every generated document.
	Footer string
	// Reason is the reason given for the catalogue and org chart changes.
	Reason string
	// Municipality is the name of the fictional municipality.
	Municipality string
	// Units names the org units, by unit key.
	Units map[string]unitText
	// CaseTypes labels the case types, by code.
	CaseTypes map[string]label
	// DocumentTypes labels the document types, by code.
	DocumentTypes map[string]label
	// Policies labels the retention policies, by code.
	Policies map[string]policyText
	// Permit is the building permit hero (§4.1).
	Permit permitText
	// Opposition is the opposition hero (§4.2).
	Opposition oppositionText
	// Report is the citizen report hero (§4.6).
	Report reportText
}

// label is the label and description of a catalogue entry.
type label struct {
	// Label is the entry's label.
	Label string
	// Description is the entry's description.
	Description string
}

// unitText is the label and abbreviation of an org unit.
type unitText struct {
	// Label is the unit's name.
	Label string
	// Abbreviation is the unit's short name.
	Abbreviation string
}

// policyText is the label, description and legal basis of a retention policy.
type policyText struct {
	// Label is the policy's label.
	Label string
	// Description says what happens to the cases under the policy.
	Description string
	// LegalBasis is the (fictional) legal basis of the policy.
	LegalBasis string
}

// docText is the title and body lines of a generated document.
type docText struct {
	// Title is the document title, printed as the heading of its PDF.
	Title string
	// Lines are the body lines of its PDF.
	Lines []string
}

// entryText is a timeline entry.
type entryText struct {
	// Title is the entry title.
	Title string
	// Body is the entry text.
	Body string
}

// permitText holds the texts of the building permit hero (§4.1).
type permitText struct {
	// CaseTitle is the title of the permit case.
	CaseTitle string
	// CaseDescription is its description.
	CaseDescription string
	// ParcelName is the name of the parcel.
	ParcelName string
	// BuildingName is the name of the building.
	BuildingName string
	// RequestForm is the application form.
	RequestForm docText
	// Plan is the architect's plan (version 1).
	Plan docText
	// SitePhoto is the photograph of the house from the street.
	SitePhoto docText
	// InquiryNotice is the public inquiry notice.
	InquiryNotice docText
	// Decision is the permit granted.
	Decision docText
	// DecisionLetter is the letter sending the permit.
	DecisionLetter docText
	// PlanRevisionReason is the reason of the plan's version 2.
	PlanRevisionReason string
	// Filed is the timeline entry of the filing.
	Filed entryText
	// Inquiry is the timeline entry of the public inquiry.
	Inquiry entryText
	// DecisionEntry is the timeline entry of the decision.
	DecisionEntry entryText
	// CheckTask is the title of the completeness check task.
	CheckTask string
	// CheckTaskNote is its completion note.
	CheckTaskNote string
	// CirculationTitle is the title of the circulation for opinions.
	CirculationTitle string
	// CirculationMessage is its message.
	CirculationMessage string
	// RoadsAnswer is the answer of Voirie et propreté.
	RoadsAnswer string
	// ParksAnswer is the answer of Parcs et promenades.
	ParksAnswer string
	// EnergyAnswer is the answer of Énergie.
	EnergyAnswer string
	// ClosingReason is the reason given when closing the case.
	ClosingReason string
}

// oppositionText holds the texts of the opposition hero (§4.2).
type oppositionText struct {
	// CaseTitle is the title of the opposition case.
	CaseTitle string
	// CaseDescription is its description.
	CaseDescription string
	// Letter is the neighbour's opposition letter.
	Letter docText
	// HeatPumpPhoto is the neighbour's photograph of the planned heat pump site.
	HeatPumpPhoto docText
	// Acknowledgement is the acknowledgement of receipt.
	Acknowledgement docText
	// ConciliationNote is the minutes of the conciliation meeting.
	ConciliationNote docText
	// Received is the timeline entry of the receipt.
	Received entryText
	// Conciliation is the internal timeline entry of the meeting.
	Conciliation entryText
	// Lifted is the timeline entry lifting the opposition.
	Lifted entryText
	// ClosingReason is the reason given when closing the case.
	ClosingReason string
}

// reportText holds the texts of the citizen report hero (§4.6).
type reportText struct {
	// CaseTitle is the title of the report case.
	CaseTitle string
	// CaseDescription is its description.
	CaseDescription string
	// LampName is the name of the street light.
	LampName string
	// Email is the resident's e-mail reporting the street light.
	Email docText
	// Photo is the resident's photograph.
	Photo docText
	// Received is the timeline entry of the report.
	Received entryText
	// Repaired is the timeline entry of the repair.
	Repaired entryText
	// Task is the title of the repair task.
	Task string
	// TaskNote is its completion note.
	TaskNote string
	// ClosingReason is the reason given when closing the case.
	ClosingReason string
}

// catalogues lists the languages the generator writes, by code.
var catalogues = map[string]*textSet{"fr": &textsFR, "en": &textsEN}

// Languages returns the language codes the generator supports, sorted.
func Languages() []string {
	codes := make([]string, 0, len(catalogues))
	for code := range catalogues {
		codes = append(codes, code)
	}
	slices.Sort(codes)
	return codes
}

// textsFor returns the catalogue of language lang.
func textsFor(lang string) (*textSet, error) {
	texts, ok := catalogues[strings.ToLower(strings.TrimSpace(lang))]
	if !ok {
		return nil, fmt.Errorf("demo language %q is not supported (supported: %s)", lang, strings.Join(Languages(), ", "))
	}
	return texts, nil
}
