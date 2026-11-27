package demodata

// backgroundText holds the texts of the background of profile demo
// (DEMO_SCRIPT §5): templates the generator fills with invented names and
// addresses (background_names.go).
type backgroundText struct {
	// Subjects are the case title templates, by case type code; %s is the address.
	Subjects map[string][]string
	// Description is the case description template; %s is the requester's name.
	Description string
	// Things names the things, by thing type code; %s is the address.
	Things map[string]string
	// Lines are the body lines every document starts with.
	Lines documentLines
	// Bodies is the closing line of a document, by document type code.
	Bodies map[string]string
	// Received is the timeline entry of the filing.
	Received entryText
	// Decided is the timeline entry of the answer or decision.
	Decided entryText
	// ClosingReason closes a handled case.
	ClosingReason string
}

// documentLines are the templates of the first body lines of a background
// document; each %s is the value named.
type documentLines struct {
	// Case is the case title line.
	Case string
	// Reference is the case reference line.
	Reference string
	// Party is the requester line.
	Party string
	// Date is the date line.
	Date string
}
