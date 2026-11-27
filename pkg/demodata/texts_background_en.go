package demodata

// backgroundEN is the English background of profile demo (DEMO_SCRIPT §5).
var backgroundEN = backgroundText{
	Subjects: map[string][]string{
		caseBuildingPermit: {
			"Conversion of a villa, %s", "Extension of a building, %s", "Construction of a garage, %s",
			"Solar panels, %s", "New conservatory, %s", "Demolition of an annex, %s",
			"Extra storey on a building, %s", "Attic conversion, %s",
		},
		caseOpposition: {
			"Opposition to the project, %s", "Neighbours' opposition, %s", "Opposition to the extra storey, %s",
		},
		caseTreeFelling: {
			"Felling of a tree, %s", "Felling of a diseased maple, %s", "Felling of a lime tree, %s",
			"Felling and replanting, %s",
		},
		caseAdvertising: {
			"Illuminated sign, %s", "Billboard, %s", "Shop sign, %s", "Advertising awning, %s",
		},
		casePublicDomain: {
			"Pavement terrace, %s", "Building skip, %s", "Scaffolding on the public domain, %s",
			"Market stall, %s", "Removal, reserved parking spaces, %s",
		},
		caseCitizenReport: {
			"Pothole, %s", "Faulty street light, %s", "Fly-tipping, %s", "Damaged bench, %s",
			"Graffiti, %s", "Blocked drain, %s",
		},
		caseComplaint: {
			"Noise nuisance, %s", "Complaint about smells, %s", "Obstructive parking, %s", "Cleanliness, %s",
			"Complaint about smoke, %s",
		},
		caseEnergySubsidy: {
			"Subsidy — heat pump, %s", "Subsidy — solar panels, %s",
			"Subsidy — roof insulation, %s", "Subsidy — window replacement, %s",
		},
		caseGeneric: {
			"Information request, %s", "Certificate of residence, %s", "Appointment request, %s",
		},
	},
	Description: "Case opened at the request of %s.",
	Things: map[string]string{
		"BUILDING":      "Building, %s",
		"PARCEL":        "Parcel, %s",
		"TREE":          "Tree, %s",
		"STREET":        "Street section, %s",
		"ADVERTISEMENT": "Sign, %s",
	},
	Lines: documentLines{
		Case:      "Case: %s",
		Reference: "Reference: %s",
		Party:     "Requester: %s",
		Date:      "Date: %s",
	},
	Bodies: map[string]string{
		docForm:           "Form filled in and signed, enclosures checked.",
		docIncomingLetter: "Letter received at the counter and registered.",
		docPlan:           "Site plan and sections, scale 1:100.",
		docSupporting:     "Supporting document handed in with the request.",
		docServiceOpinion: "Opinion of the service: favourable, with the usual conditions.",
		docOpposition:     "The signatories oppose the project and ask to be heard.",
		docReport:         "Report drawn up on site.",
		docDecision:       "The Municipal Council ruled on the request.",
		docOutgoingLetter: "We inform you of the follow-up given to your request.",
	},
	Received:      entryText{Title: "Request received", Body: "Request and enclosures registered."},
	Decided:       entryText{Title: "Answer sent", Body: "Decision notified to the requester."},
	ClosingReason: "Case handled",
}
