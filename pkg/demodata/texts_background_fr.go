package demodata

// backgroundFR is the French background of profile demo (DEMO_SCRIPT §5).
var backgroundFR = backgroundText{
	Subjects: map[string][]string{
		caseBuildingPermit: {
			"Transformation d'une villa, %s", "Agrandissement d'un immeuble, %s", "Construction d'un garage, %s",
			"Pose de panneaux solaires, %s", "Création d'une véranda, %s", "Démolition d'une annexe, %s",
			"Surélévation d'un immeuble, %s", "Aménagement des combles, %s",
		},
		caseOpposition: {
			"Opposition au projet, %s", "Opposition de voisins, %s", "Opposition à la surélévation, %s",
		},
		caseTreeFelling: {
			"Abattage d'un arbre, %s", "Abattage d'un érable malade, %s", "Abattage d'un tilleul, %s",
			"Abattage et replantation, %s",
		},
		caseAdvertising: {
			"Enseigne lumineuse, %s", "Panneau publicitaire, %s", "Enseigne de commerce, %s", "Store publicitaire, %s",
		},
		casePublicDomain: {
			"Terrasse sur le trottoir, %s", "Benne de chantier, %s", "Échafaudage sur le domaine public, %s",
			"Stand de marché, %s", "Déménagement, places de parc réservées, %s",
		},
		caseCitizenReport: {
			"Nid-de-poule, %s", "Lampadaire défectueux, %s", "Dépôt sauvage, %s", "Banc endommagé, %s",
			"Graffiti, %s", "Grille d'égout bouchée, %s",
		},
		caseComplaint: {
			"Nuisances sonores, %s", "Plainte pour odeurs, %s", "Stationnement gênant, %s", "Propreté, %s",
			"Plainte pour fumée, %s",
		},
		caseEnergySubsidy: {
			"Subvention — pompe à chaleur, %s", "Subvention — panneaux solaires, %s",
			"Subvention — isolation de la toiture, %s", "Subvention — remplacement des fenêtres, %s",
		},
		caseGeneric: {
			"Demande d'information, %s", "Demande d'attestation de domicile, %s", "Demande de rendez-vous, %s",
		},
	},
	Description: "Dossier ouvert à la demande de %s.",
	Things: map[string]string{
		"BUILDING":      "Immeuble, %s",
		"PARCEL":        "Parcelle, %s",
		"TREE":          "Arbre, %s",
		"STREET":        "Tronçon, %s",
		"ADVERTISEMENT": "Enseigne, %s",
	},
	Lines: documentLines{
		Case:      "Affaire : %s",
		Reference: "Référence : %s",
		Party:     "Requérant·e : %s",
		Date:      "Date : %s",
	},
	Bodies: map[string]string{
		docForm:           "Formulaire rempli et signé, pièces jointes contrôlées.",
		docIncomingLetter: "Courrier reçu au guichet et enregistré.",
		docPlan:           "Plan de situation et coupes, échelle 1:100.",
		docSupporting:     "Pièce justificative remise avec la demande.",
		docServiceOpinion: "Préavis du service : favorable, avec les charges usuelles.",
		docOpposition:     "Les signataires s'opposent au projet et demandent à être entendus.",
		docReport:         "Constat établi sur place.",
		docDecision:       "La Municipalité a statué sur la demande.",
		docOutgoingLetter: "Nous vous informons de la suite donnée à votre demande.",
	},
	Received:      entryText{Title: "Demande reçue", Body: "Demande et pièces jointes enregistrées."},
	Decided:       entryText{Title: "Réponse envoyée", Body: "Décision notifiée au requérant."},
	ClosingReason: "Dossier traité",
}
