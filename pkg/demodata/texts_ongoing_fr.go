package demodata

// ongoingFR is the French ongoing work (DEMO_SCRIPT §4.8 to §4.12).
var ongoingFR = ongoingText{
	Chalet: chaletText{
		CaseTitle:       "Chalet des Pins — transformation et isolation",
		CaseDescription: "Transformation d'un chalet des années 1950 : isolation de l'enveloppe, nouvelle toiture et panneaux solaires.",
		BuildingName:    "Chalet des Pins, chemin des Pins 9",
		RequestForm: docText{Title: "Demande de permis de construire — Chalet des Pins", Lines: []string{
			"Requérante : Mme Hélène Duc, chemin des Pins 9.",
			"Travaux : isolation périphérique, réfection de la toiture, 24 m² de panneaux solaires.",
			"Pièces jointes : plans, descriptif énergétique.",
		}},
		Plan: docText{Title: "Plans et coupes — Chalet des Pins", Lines: []string{
			"Façades nord et sud, coupe de la toiture.",
			"Panneaux solaires intégrés au pan sud, hauteur au faîte inchangée.",
		}},
		Filed:              entryText{Title: "Demande déposée", Body: "Demande de permis reçue au guichet, avec les plans et le descriptif énergétique."},
		CheckTask:          "Contrôler la complétude du dossier",
		CheckNote:          "Dossier complet.",
		CirculationTitle:   "Préavis des services — Chalet des Pins",
		CirculationMessage: "Merci de transmettre votre préavis sur la transformation du Chalet des Pins.",
		RoadsAnswer:        "Préavis favorable : l'accès de chantier par le chemin des Pins est possible hors des heures scolaires.",
		DecisionTask:       "Préparer la décision une fois les préavis reçus",
	},
	Cedar: cedarText{
		CaseTitle:       "Abattage d'un cèdre malade, parc de la Roseraie",
		CaseDescription: "Une riveraine signale un cèdre dépérissant dont des branches tombent sur le chemin du parc.",
		TreeName:        "Cèdre du parc de la Roseraie",
		Letter: docText{Title: "Courriel de Mme Isabelle Rey — cèdre du parc de la Roseraie", Lines: []string{
			"Madame, Monsieur,",
			"Le grand cèdre près de l'entrée du parc perd des branches depuis l'automne.",
			"Une branche est tombée hier sur le chemin, à côté de la place de jeux. Pourriez-vous le faire contrôler ?",
			"Isabelle Rey",
		}},
		Received:     entryText{Title: "Signalement reçu", Body: "Courriel de Mme Rey : branches tombées près de la place de jeux."},
		VisitTask:    "Contrôler l'état sanitaire du cèdre sur place",
		CallbackTask: "Rappeler Mme Rey pour l'informer de la suite",
	},
	Terrace: terraceText{
		CaseTitle:       "Terrasse estivale du Café du Lac",
		CaseDescription: "Demande d'occupation du trottoir pour une terrasse de 12 tables, de mai à septembre.",
		TerraceName:     "Terrasse du Café du Lac, rue du Lac 3",
		Request: docText{Title: "Demande d'occupation du domaine public — Café du Lac", Lines: []string{
			"Requérant : Café du Lac Sàrl, rue du Lac 3.",
			"Emprise : 18 m² de trottoir, 12 tables, de mai à septembre.",
			"Passage piéton maintenu à 1,50 m.",
		}},
		Received:       entryText{Title: "Demande reçue", Body: "Demande de terrasse avec plan d'emprise."},
		CheckTask:      "Vérifier le plan d'emprise et le passage piéton",
		LightingTask:   "Vérifier l'éclairage public au droit de la terrasse",
		ReassignReason: "Absence de Chloé : confié à l'unité Éclairage public",
	},
	Bench: benchText{
		CaseTitle:       "Requête — banc au square des Tilleuls",
		CaseDescription: "M. Tutu demande l'installation d'un banc au square des Tilleuls pour les personnes âgées du quartier.",
		Letter: docText{Title: "Lettre de M. Jean Tutu — banc au square des Tilleuls", Lines: []string{
			"Madame, Monsieur,",
			"Les personnes âgées du quartier n'ont nulle part où s'asseoir au square des Tilleuls.",
			"Je vous prie d'y faire installer un banc à l'ombre.",
			"Jean Tutu",
		}},
		Received:     entryText{Title: "Requête reçue", Body: "Lettre de M. Tutu : demande d'un banc au square des Tilleuls."},
		CallbackTask: "Rappeler M. Tutu",
		OrderTask:    "Commander un banc",
		CancelReason: "Doublon : la commande est faite par Parcs et promenades",
		VisitTask:    "Repérer l'emplacement du banc",
		VisitNote:    "Emplacement repéré sous le grand tilleul.",
		ReopenReason: "Le tilleul sera taillé : l'emplacement est à revoir",
	},
	NewSubsidy: newSubsidyText{
		CaseTitle:       "Subvention — pompe à chaleur, rue du Midi 7",
		CaseDescription: "Demande de subvention pour le remplacement d'une chaudière à mazout par une pompe à chaleur.",
		BuildingName:    "Maison, rue du Midi 7",
		Form: docText{Title: "Formulaire de demande de subvention — rue du Midi 7", Lines: []string{
			"Requérant : M. Karim Haddad, rue du Midi 7.",
			"Travaux : pompe à chaleur air-eau en remplacement d'une chaudière à mazout.",
			"Pièces jointes : devis, attestation de revenu.",
		}},
		Filed:       entryText{Title: "Demande déposée", Body: "Formulaire, devis et attestation de revenu reçus."},
		CheckTask:   "Contrôler les pièces de la demande",
		GrantReason: "La pose demande un permis : l'Urbanisme suit le dossier avec l'Énergie",
	},
}

// groupsFR names the French demo security groups.
var groupsFR = groupsText{
	Salubrity:            label{"Commission de salubrité", "Voirie, cohésion sociale et parcs, pour les plaintes récurrentes"},
	SalubrityGrantReason: "Plainte récurrente suivie par la commission de salubrité",
	EnergyPlanning:       label{"Cellule énergie et urbanisme", "Urbanisme et énergie, pour les subventions liées à un permis"},
}
