package demodata

// ongoingEN is the English ongoing work (DEMO_SCRIPT §4.8 to §4.12).
var ongoingEN = ongoingText{
	Chalet: chaletText{
		CaseTitle:       "Chalet des Pins — conversion and insulation",
		CaseDescription: "Conversion of a 1950s chalet: insulation of the envelope, new roof and solar panels.",
		BuildingName:    "Chalet des Pins, chemin des Pins 9",
		RequestForm: docText{Title: "Building permit application — Chalet des Pins", Lines: []string{
			"Applicant: Ms Hélène Duc, chemin des Pins 9.",
			"Works: external insulation, new roof, 24 m² of solar panels.",
			"Enclosed: plans, energy statement.",
		}},
		Plan: docText{Title: "Plans and sections — Chalet des Pins", Lines: []string{
			"North and south facades, roof section.",
			"Solar panels built into the south slope, ridge height unchanged.",
		}},
		Filed:              entryText{Title: "Application filed", Body: "Permit application received at the counter, with the plans and the energy statement."},
		CheckTask:          "Check that the file is complete",
		CheckNote:          "File complete.",
		CirculationTitle:   "Opinions of the services — Chalet des Pins",
		CirculationMessage: "Please send your opinion on the conversion of the Chalet des Pins.",
		RoadsAnswer:        "Favourable opinion: site access through chemin des Pins is possible outside school hours.",
		DecisionTask:       "Prepare the decision once the opinions are in",
	},
	Cedar: cedarText{
		CaseTitle:       "Felling of a diseased cedar, Roseraie park",
		CaseDescription: "A resident reports a declining cedar whose branches fall on the park path.",
		TreeName:        "Cedar of the Roseraie park",
		Letter: docText{Title: "E-mail from Ms Isabelle Rey — cedar of the Roseraie park", Lines: []string{
			"Dear Sir or Madam,",
			"The big cedar near the park entrance has been losing branches since the autumn.",
			"A branch fell on the path yesterday, next to the playground. Could you have it checked?",
			"Isabelle Rey",
		}},
		Received:     entryText{Title: "Report received", Body: "E-mail from Ms Rey: branches fallen near the playground."},
		VisitTask:    "Check the health of the cedar on site",
		CallbackTask: "Call Ms Rey back about what comes next",
	},
	Terrace: terraceText{
		CaseTitle:       "Summer terrace of the Café du Lac",
		CaseDescription: "Request to use the pavement for a 12-table terrace, from May to September.",
		TerraceName:     "Café du Lac terrace, rue du Lac 3",
		Request: docText{Title: "Public domain occupation request — Café du Lac", Lines: []string{
			"Applicant: Café du Lac Sàrl, rue du Lac 3.",
			"Area: 18 m² of pavement, 12 tables, from May to September.",
			"Pedestrian passage kept at 1.50 m.",
		}},
		Received:       entryText{Title: "Request received", Body: "Terrace request with its layout plan."},
		CheckTask:      "Check the layout plan and the pedestrian passage",
		LightingTask:   "Check the street lighting along the terrace",
		ReassignReason: "Chloé is away: handed to the street lighting unit",
	},
	Bench: benchText{
		CaseTitle:       "Request — bench in the square des Tilleuls",
		CaseDescription: "Mr Tutu asks for a bench in the square des Tilleuls for the elderly people of the neighbourhood.",
		Letter: docText{Title: "Letter from Mr Jean Tutu — bench in the square des Tilleuls", Lines: []string{
			"Dear Sir or Madam,",
			"The elderly people of the neighbourhood have nowhere to sit in the square des Tilleuls.",
			"Please have a bench installed in the shade.",
			"Jean Tutu",
		}},
		Received:     entryText{Title: "Request received", Body: "Letter from Mr Tutu: a bench asked for in the square des Tilleuls."},
		CallbackTask: "Call Mr Tutu back",
		OrderTask:    "Order a bench",
		CancelReason: "Duplicate: the order is placed by Parks and gardens",
		VisitTask:    "Find a place for the bench",
		VisitNote:    "Place found under the big lime tree.",
		ReopenReason: "The lime tree will be pruned: the place must be reviewed",
	},
	NewSubsidy: newSubsidyText{
		CaseTitle:       "Subsidy — heat pump, rue du Midi 7",
		CaseDescription: "Subsidy application to replace an oil boiler with a heat pump.",
		BuildingName:    "House, rue du Midi 7",
		Form: docText{Title: "Subsidy application form — rue du Midi 7", Lines: []string{
			"Applicant: Mr Karim Haddad, rue du Midi 7.",
			"Works: air-to-water heat pump replacing an oil boiler.",
			"Enclosed: quote, income certificate.",
		}},
		Filed:       entryText{Title: "Application filed", Body: "Form, quote and income certificate received."},
		CheckTask:   "Check the documents of the application",
		GrantReason: "The installation needs a permit: Planning follows the case with Energy",
	},
}

// groupsEN names the English demo security groups.
var groupsEN = groupsText{
	Salubrity:            label{"Public health committee", "Roads, social cohesion and parks, for recurring complaints"},
	SalubrityGrantReason: "Recurring complaint followed by the public health committee",
	EnergyPlanning:       label{"Energy and planning unit", "Planning and energy, for the subsidies tied to a permit"},
}
