package demodata

// Invented names of the background (DEMO_SCRIPT §5): common first and last
// names of French-speaking Switzerland combined at random, generic street
// names and company names built from them. They are not texts: like the names
// of the heroes, they stay the same in every language.

// backgroundFirstNames are first names of the background persons, by salutation.
var backgroundFirstNames = map[bool][]string{ // true: Madame
	true: {
		"Anne", "Camille", "Catherine", "Céline", "Chantal", "Christine", "Élodie", "Fanny",
		"Florence", "Isabelle", "Julie", "Laure", "Léa", "Manon", "Marie", "Mathilde",
		"Nathalie", "Nicole", "Sandrine", "Sophie", "Valérie", "Yvonne",
	},
	false: {
		"Alain", "André", "Bernard", "Cédric", "Christophe", "Daniel", "Didier", "Éric",
		"Fabien", "François", "Gilles", "Jacques", "Jean-Marc", "Julien", "Laurent", "Luc",
		"Marc", "Nicolas", "Olivier", "Patrick", "Pierre", "Yves",
	},
}

// backgroundLastNames are last names of the background persons and companies.
var backgroundLastNames = []string{
	"Aebischer", "Baud", "Berset", "Bovet", "Brélaz", "Buchs", "Burnand", "Chappuis",
	"Chevalley", "Cornaz", "Cuendet", "Curchod", "Décosterd", "Delacrétaz", "Desmeules", "Dubois",
	"Dufour", "Duperrex", "Estoppey", "Favez", "Forney", "Gavillet", "Genton", "Gilliéron",
	"Golay", "Gonin", "Grandjean", "Guignard", "Jaccard", "Jaunin", "Jordan", "Junod",
	"Lambelet", "Leuba", "Loup", "Magnin", "Marendaz", "Mermoud", "Meylan", "Monod",
	"Mottier", "Nicod", "Pasche", "Perrin", "Piguet", "Pittet", "Rapin", "Reymond",
	"Rochat", "Roulet", "Ruchet", "Savary", "Schmid", "Taillens", "Testuz", "Thévoz",
	"Vallotton", "Vuagniaux", "Vulliemin", "Zbinden",
}

// backgroundTrade is a kind of company: the words around its name and its
// organization category.
type backgroundTrade struct {
	prefix, suffix, category string
}

// backgroundTrades are the companies of the background.
var backgroundTrades = []backgroundTrade{
	{"Atelier", "Architectes SA", "BUREAU_ARCHITECTE"},
	{"Bureau", "Architecture Sàrl", "BUREAU_ARCHITECTE"},
	{"", "Ingénieurs Conseils SA", "BUREAU_INGENIEUR"},
	{"Menuiserie", "Sàrl", "METIER_BATIMENT"},
	{"", "Toitures Sàrl", "METIER_BATIMENT"},
	{"", "Chauffage Sanitaire SA", "METIER_BATIMENT"},
	{"", "Construction SA", "ENTREPRISE_TP"},
	{"Régie", "SA", "GERANCE_IMMOBILIERE"},
	{"Café", "", "CAFE_RESTO_HOTEL"},
	{"Boulangerie", "", "COMMERCE"},
	{"Garage", "SA", "COMMERCE"},
	{"Étude", "Avocats", "AVOCAT_JURISTE"},
	{"Fondation", "", "FONDATION"},
	{"PPE", "", "PPE"},
}

// backgroundStreets are the streets of the background addresses.
var backgroundStreets = []string{
	"avenue de la Gare", "avenue des Alpes", "avenue du Léman", "avenue de Rumine", "avenue de Morges",
	"avenue de Beaulieu", "avenue de Cour", "boulevard de Grancy", "chemin des Aubépines", "chemin des Cerisiers",
	"chemin du Couchant", "chemin des Écureuils", "chemin de la Forêt", "chemin des Glycines", "chemin du Levant",
	"chemin des Mésanges", "chemin du Muguet", "chemin des Roches", "place de la Riponne", "place du Marché",
	"route de Berne", "route du Signal", "rue Centrale", "rue de Bourg", "rue du Midi",
	"rue de la Paix", "rue du Petit-Chêne", "rue des Terreaux", "rue du Valentin", "rue de la Tour",
}

// backgroundArea is the part of the map the background things lie in, in LV95
// metres (east min, north min, east max, north max): the town north of the
// lake shore, inside mapExtent.
var backgroundArea = [4]float64{2_536_500, 1_152_300, 2_540_500, 1_155_500}
