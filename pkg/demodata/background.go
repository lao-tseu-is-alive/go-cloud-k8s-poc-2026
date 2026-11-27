package demodata

import (
	"fmt"
	"math"
	"math/rand/v2"
	"slices"
	"strings"
	"time"

	v1 "github.com/lao-tseu-is-alive/go-cloud-k8s-poc-2026/gen/goeland/v1"
)

// Volumes and spread of the background of profile demo (DEMO_SCRIPT §5,
// orders of magnitude: about 300 actors, 400 cases, 1,500 documents and 200
// things).
const (
	backgroundPersons   = 230
	backgroundCompanies = 70
	// backgroundDays is how far back the background goes: four years, so none
	// of its closed cases is due for disposal (§4.5 keeps the due list to its
	// two cases of 2014).
	backgroundDays = 4 * 365
	// backgroundFirstNumber is the first reference number of the background in
	// a namespace and year, above the numbers of the stories (at most 134).
	backgroundFirstNumber = 151
	// backgroundSeed draws the same background at every generation.
	backgroundSeed = 2026_079
)

// backgroundThings is the number of background things, by thing type; a
// parcel goes with each of the first buildings.
var backgroundThings = map[string]int{"BUILDING": 90, "PARCEL": 40, "TREE": 30, "STREET": 20, "ADVERTISEMENT": 20}

// backgroundCase is how the background fills a case type: the number of
// cases, the unit and agent handling them, the thing type they concern ("":
// none), the documents filed with the request (the first always, the others
// most of the time) and those of the answer.
type backgroundCase struct {
	caseType   string
	count      int
	unit, user string
	thingType  string
	filed      []string
	answer     []string
}

// backgroundCases spreads the 400 background cases over the case types.
var backgroundCases = []backgroundCase{
	{caseBuildingPermit, 80, unitPlanning, userBruno, "BUILDING", []string{docForm, docPlan, docSupporting}, []string{docServiceOpinion, docDecision, docOutgoingLetter}},
	{caseOpposition, 15, unitPlanning, userBruno, "BUILDING", []string{docOpposition}, []string{docOutgoingLetter}},
	{caseTreeFelling, 35, unitParks, userHugo, "TREE", []string{docForm, docSupporting}, []string{docReport, docDecision}},
	{caseAdvertising, 30, unitPlanning, userBruno, "ADVERTISEMENT", []string{docForm, docPlan}, []string{docDecision, docOutgoingLetter}},
	{casePublicDomain, 45, unitRoads, userChloe, "STREET", []string{docForm, docPlan}, []string{docDecision, docOutgoingLetter}},
	{caseCitizenReport, 80, unitRoads, userChloe, "STREET", []string{docIncomingLetter, docSupporting}, []string{docReport, docOutgoingLetter}},
	{caseComplaint, 45, unitSocial, userFelix, "BUILDING", []string{docIncomingLetter, docSupporting}, []string{docReport, docOutgoingLetter}},
	{caseEnergySubsidy, 40, unitEnergy, userEmma, "BUILDING", []string{docForm, docSupporting, docIncomingLetter}, []string{docDecision, docOutgoingLetter}},
	{caseGeneric, 30, unitClerk, userAlice, "", []string{docIncomingLetter}, []string{docOutgoingLetter}},
}

// party is a background actor, planned then created at its first case.
type party struct {
	id, name string
	// person creates a person (salutation, first and last name), otherwise an
	// organization of category.
	person      bool
	salutation  v1.Salutation
	first, last string
	category    string
}

// spot is a background thing, planned then created at its first case.
type spot struct {
	id, address, geometry string
	// parcel is the parcel of a building ("" when it has none), planned with it.
	parcel *spot
	number int // parcel number or EGID suffix
}

// backdrop generates the background: the run, a seeded random source, the
// actors and things planned (created at their first use, by the agent of that
// case and at its date) and the reference numbers used.
type backdrop struct {
	*run
	rng      *rand.Rand
	parties  []*party
	used     int // parties[:used] are created
	spots    map[string][]*spot
	spotUsed map[string]int
	numbers  map[string]int // namespace and year -> last reference number
}

// background writes the background of profile demo (§5): cases of the last
// four years over every case type, each filed with its documents, most of
// them answered and closed, with their requesters and the things they concern;
// limit, when positive, caps the number of cases per case type.
func (r *run) background(limit int) {
	b := newBackdrop(r)
	for _, bc := range backgroundCases {
		count := bc.count
		if limit > 0 {
			count = min(count, limit)
		}
		for range count {
			b.story(bc)
		}
	}
}

// newBackdrop plans the background actors and things of r.
func newBackdrop(r *run) *backdrop {
	b := &backdrop{
		run:      r,
		rng:      rand.New(rand.NewPCG(backgroundSeed, backgroundSeed)), // NOSONAR go:S2245 reproducible demo data, not a secret
		spots:    map[string][]*spot{},
		spotUsed: map[string]int{},
		numbers:  map[string]int{},
	}
	b.planParties()
	b.planSpots()
	return b
}

// planParties draws the names of the background actors, persons and
// companies mixed, each name once.
func (b *backdrop) planParties() {
	seen := map[string]bool{}
	for len(b.parties) < backgroundPersons {
		woman := b.rng.IntN(2) == 0
		p := &party{person: true, salutation: monsieur, first: pick(b.rng, backgroundFirstNames[woman]), last: pick(b.rng, backgroundLastNames)}
		if woman {
			p.salutation = madame
		}
		p.name = p.first + " " + p.last
		if !seen[p.name] {
			seen[p.name] = true
			b.parties = append(b.parties, p)
		}
	}
	for len(b.parties) < backgroundPersons+backgroundCompanies {
		trade := pick(b.rng, backgroundTrades)
		name := joinWords(trade.prefix, pick(b.rng, backgroundLastNames), trade.suffix)
		if !seen[name] {
			seen[name] = true
			b.parties = append(b.parties, &party{name: name, category: trade.category})
		}
	}
	b.rng.Shuffle(len(b.parties), func(i, j int) { b.parties[i], b.parties[j] = b.parties[j], b.parties[i] })
}

// planSpots draws the background things: addresses and LV95 geometries.
func (b *backdrop) planSpots() {
	addresses := map[string]bool{}
	address := func() string {
		for {
			a := fmt.Sprintf("%s %d", pick(b.rng, backgroundStreets), 1+b.rng.IntN(60))
			if !addresses[a] {
				addresses[a] = true
				return a
			}
		}
	}
	streets := b.rng.Perm(len(backgroundStreets))
	for i := range backgroundThings["STREET"] {
		east, north := b.point()
		angle := b.rng.Float64() * math.Pi
		end := [2]float64{east + 150*math.Cos(angle), north + 150*math.Sin(angle)}
		b.spots["STREET"] = append(b.spots["STREET"], &spot{
			address:  backgroundStreets[streets[i%len(streets)]],
			geometry: fmt.Sprintf(`{"type":"LineString","coordinates":[[%.1f,%.1f],[%.1f,%.1f]]}`, east, north, end[0], end[1]),
		})
	}
	for _, thingType := range []string{"BUILDING", "TREE", "ADVERTISEMENT"} {
		for i := range backgroundThings[thingType] {
			east, north := b.point()
			s := &spot{address: address(), geometry: pointGeoJSON(east, north), number: i + 1}
			if thingType == "BUILDING" && i < backgroundThings["PARCEL"] {
				s.parcel = &spot{address: s.address, geometry: squareGeoJSON(east, north, 15), number: 2001 + i}
			}
			b.spots[thingType] = append(b.spots[thingType], s)
		}
	}
}

// point draws a position inside backgroundArea, rounded to 10 cm.
func (b *backdrop) point() (east, north float64) {
	east = backgroundArea[0] + b.rng.Float64()*(backgroundArea[2]-backgroundArea[0])
	north = backgroundArea[1] + b.rng.Float64()*(backgroundArea[3]-backgroundArea[1])
	return float64(int(east*10)) / 10, float64(int(north*10)) / 10
}

// story writes one background case of bc: filed on a working day of the last
// four years, then, as time allows, taken in hand, answered and closed.
func (b *backdrop) story(bc backgroundCase) {
	opened := b.workday(14 + b.rng.IntN(backgroundDays-14))
	requester := b.party(bc.user, b.rng.IntN(len(b.parties)))
	thing := b.spot(bc.user, bc.thingType)
	address := pick(b.rng, backgroundStreets)
	if thing != nil {
		address = thing.address
	}
	text := b.t.Background
	title := fmt.Sprintf(pick(b.rng, text.Subjects[bc.caseType]), address)
	ct := caseTypeByCode(bc.caseType)
	number := b.number(ct.namespace, b.now.Year())
	c := b.newCase(bc.user, bc.caseType, number, title, fmt.Sprintf(text.Description, requester.name), bc.unit, ct.confidentiality)
	b.link(bc.user, c, requester.id, "CASE_HAS_ACTOR_REQUESTER")
	if thing != nil {
		b.link(bc.user, c, thing.id, "CASE_CONCERNS_THING")
		if thing.parcel != nil {
			b.link(bc.user, c, thing.parcel.id, "CASE_CONCERNS_THING")
		}
	}
	ref := ct.namespace + " " + number
	filed := b.documents(bc.user, c, title, ref, requester.name, bc.filed, true)
	b.entry(bc.user, c, request, text.Received, participants, filed...)

	// Each step is two days or more after the previous one, so moving it off a
	// weekend never puts it before that step.
	started := opened - 2 - b.rng.IntN(5)
	if started < 2 {
		return
	}
	started = b.workday(started)
	b.transition(bc.user, c, v1.CaseStatus_CASE_STATUS_IN_PROGRESS, "")
	answered := started - 5 - b.rng.IntN(60)
	if answered < 1 || b.rng.IntN(10) == 0 { // one case in ten is still in progress
		return
	}
	b.workday(answered)
	sent := b.documents(bc.user, c, title, ref, requester.name, bc.answer, false)
	b.entry(bc.user, c, decision, text.Decided, participants, sent...)
	b.transition(bc.user, c, closed, text.ClosingReason)
}

// documents files a final document of each type in c (the first always, the
// others three times in four when optional) and returns their ids.
func (b *backdrop) documents(user, c, title, ref, requester string, types []string, optional bool) []string {
	lines := b.t.Background.Lines
	var ids []string
	for i, docType := range types {
		if optional && i > 0 && b.rng.IntN(4) == 0 {
			continue
		}
		doc := docText{
			Title: b.t.DocumentTypes[docType].Label + " — " + title,
			Lines: []string{
				fmt.Sprintf(lines.Case, title), fmt.Sprintf(lines.Reference, ref), fmt.Sprintf(lines.Party, requester),
				fmt.Sprintf(lines.Date, b.now.Format("02.01.2006")), "", b.t.Background.Bodies[docType],
			},
		}
		ids = append(ids, b.document(user, docType, doc, c, true))
	}
	return ids
}

// party returns the actor of the next case: the next planned one while some
// are not created yet, otherwise the one at index (a returning requester).
func (b *backdrop) party(user string, index int) *party {
	if b.used < len(b.parties) {
		index = b.used
		b.used++
	}
	p := b.parties[index]
	if p.id != "" {
		return p
	}
	if p.person {
		p.id = b.person(user, p.salutation, p.first, p.last, "")
	} else {
		p.id = b.organization(user, p.name, p.category, "")
	}
	return p
}

// spot returns a thing of thingType for the next case ("" : none): the next
// planned one while some are not created yet, otherwise one already created.
func (b *backdrop) spot(user, thingType string) *spot {
	planned := b.spots[thingType]
	if len(planned) == 0 {
		return nil
	}
	index := b.rng.IntN(len(planned))
	if b.spotUsed[thingType] < len(planned) {
		index = b.spotUsed[thingType]
		b.spotUsed[thingType]++
	}
	s := planned[index]
	if s.id == "" {
		b.createSpot(user, thingType, s)
	}
	return s
}

// createSpot creates a planned thing and, for a building, its parcel.
func (b *backdrop) createSpot(user, thingType string, s *spot) {
	req := &v1.CreateThingRequest{ThingTypeCode: thingType, Name: fmt.Sprintf(b.t.Background.Things[thingType], s.address), GeometryGeojson: s.geometry}
	if thingType == "BUILDING" {
		req.Specialization = &v1.CreateThingRequest_Building{Building: &v1.BuildingDetails{Egid: int32(900_002_000 + s.number)}}
	}
	s.id = b.thing(user, req)
	if p := s.parcel; p != nil {
		p.id = b.thing(user, &v1.CreateThingRequest{
			ThingTypeCode: "PARCEL", Name: fmt.Sprintf(b.t.Background.Things["PARCEL"], p.address), GeometryGeojson: p.geometry,
			Specialization: &v1.CreateThingRequest_Parcel{Parcel: &v1.ParcelDetails{CommuneOfs: demoCommuneOFS, ParcelNumber: fmt.Sprint(p.number)}},
		})
	}
}

// number returns the next background reference number of namespace in year.
func (b *backdrop) number(namespace string, year int) string {
	key := fmt.Sprintf("%s/%d", namespace, year)
	if b.numbers[key] == 0 {
		b.numbers[key] = backgroundFirstNumber - 1
	}
	b.numbers[key]++
	return fmt.Sprintf("%d-%06d", year, b.numbers[key])
}

// workday starts a step n days before the generation day, at an office hour;
// a Saturday moves to the Friday before, a Sunday to the Monday after (or the
// Friday before when that Monday is not past yet). It returns the day used.
func (b *backdrop) workday(n int) int {
	switch b.today.AddDate(0, 0, -n).Weekday() {
	case time.Saturday:
		n++
	case time.Sunday:
		if n > 1 {
			n--
		} else {
			n += 2
		}
	default:
	}
	b.daysAgo(n, fmt.Sprintf("%02d:%02d", 8+b.rng.IntN(9), b.rng.IntN(60)))
	return n
}

// caseTypeByCode returns the demo case type of code.
func caseTypeByCode(code string) caseType {
	for _, ct := range caseTypes {
		if ct.code == code {
			return ct
		}
	}
	panic("demodata: unknown case type " + code) // a typo in backgroundCases, caught by the tests
}

// pointGeoJSON is an LV95 GeoJSON point.
func pointGeoJSON(east, north float64) string {
	return fmt.Sprintf(`{"type":"Point","coordinates":[%.1f,%.1f]}`, east, north)
}

// squareGeoJSON is an LV95 GeoJSON square of half side h metres centred on a point.
func squareGeoJSON(east, north, h float64) string {
	return fmt.Sprintf(`{"type":"Polygon","coordinates":[[[%.1f,%.1f],[%.1f,%.1f],[%.1f,%.1f],[%.1f,%.1f],[%.1f,%.1f]]]}`,
		east-h, north-h, east+h, north-h, east+h, north+h, east-h, north+h, east-h, north-h)
}

// joinWords joins the non-empty words with spaces.
func joinWords(words ...string) string {
	return strings.Join(slices.DeleteFunc(words, func(w string) bool { return w == "" }), " ")
}

// pick returns a random element of list.
func pick[T any](rng *rand.Rand, list []T) T { return list[rng.IntN(len(list))] }
