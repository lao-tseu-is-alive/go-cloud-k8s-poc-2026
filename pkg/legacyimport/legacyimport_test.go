package legacyimport

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/lao-tseu-is-alive/go-cloud-k8s-poc-2026/pkg/actor"
	"github.com/lao-tseu-is-alive/go-cloud-k8s-poc-2026/pkg/core"
)

// newTestImporter is an importer with its maps, without databases.
func newTestImporter() *Importer {
	return &Importer{
		now: time.Date(2026, 10, 1, 12, 0, 0, 0, time.UTC), subjects: map[uuid.UUID]core.SubjectKind{},
		employees: map[int64]bool{}, activeEmployees: map[int64]bool{}, employeeUnits: map[int64]int64{},
		liveUnits: map[uuid.UUID]bool{}, liveGroups: map[uuid.UUID]bool{}, caseTypes: map[int64]uuid.UUID{},
		caseClosedAt: map[uuid.UUID]time.Time{}, relTypes: map[string]uuid.UUID{},
	}
}

func TestIDIsDeterministic(t *testing.T) {
	if ID("CASE", 42) != ID("CASE", 42) {
		t.Fatal("the same legacy row must give the same id")
	}
	if ID("CASE", 42) == ID("ACTOR", 42) || ID("CASE", 42) == ID("CASE", 43) {
		t.Fatal("the kind and the legacy id must both change the id")
	}
	if ID("CASE", 42).Version() != 5 {
		t.Fatal("ids are UUIDv5")
	}
}

func TestRoleCode(t *testing.T) {
	cases := map[string]string{
		"Responsable projet":     "RESPONSABLE_PROJET",
		"Bénéficiaire d'un DDP":  "BENEFICIAIRE_D_UN_DDP",
		"Auteur-e de la requête": "AUTEUR_E_DE_LA_REQUETE",
		"  Maître d'ouvrage ":    "MAITRE_D_OUVRAGE",
		"Étape 2":                "ETAPE_2",
	}
	for in, want := range cases {
		if got := roleCode(in, 7); got != want {
			t.Errorf("roleCode(%q) = %q, want %q", in, got, want)
		}
	}
	if got := roleCode("—", 7); got != "ROLE_7" {
		t.Errorf("a name without letters falls back to the id, got %q", got)
	}
}

func TestUnitTree(t *testing.T) {
	p := func(v int64) *int64 { return &v }
	units := []*SourceUnit{
		{ID: 3, ParentID: p(2), Active: true},  // live leaf under an inactive parent
		{ID: 2, ParentID: p(1), Active: false}, // kept live by its child
		{ID: 1, Active: true},
		{ID: 4, ParentID: p(1), Active: false}, // inactive leaf: dissolved
		{ID: 5, ParentID: p(99), Active: true}, // parent missing: a root
	}
	ordered := TopDown(units)
	pos := map[int64]int{}
	for i, u := range ordered {
		pos[u.ID] = i
	}
	if pos[1] > pos[2] || pos[2] > pos[3] {
		t.Fatalf("parents come first: %v", pos)
	}
	if units[4].ParentID != nil {
		t.Fatal("a unit whose parent is missing becomes a root")
	}
	live := liveUnits(ordered)
	if !live[1] || !live[2] || !live[3] || live[4] || !live[5] {
		t.Fatalf("live units: %v", live)
	}
}

func TestCaseStatus(t *testing.T) {
	imp := newTestImporter()
	end := time.Date(2020, 1, 2, 0, 0, 0, 0, time.UTC)
	for _, tc := range []struct {
		name                  string
		suspended, terminated bool
		want                  int16
	}{
		{"open", false, false, statusOpen},
		{"suspended", true, false, statusSuspended},
		{"terminated", false, true, statusClosed},
		{"suspended and terminated", true, true, statusClosed},
	} {
		a := &sourceCase{ID: 1, Title: "x", Suspended: tc.suspended, Terminated: tc.terminated, EndAt: &end}
		s := imp.caseSubject(a)
		row := imp.caseRow(s, uuid.New(), a)
		if got := row[4].(int16); got != tc.want {
			t.Errorf("%s: status %d, want %d", tc.name, got, tc.want)
		}
		if closed := row[6].(*time.Time); (closed != nil) != (tc.want == statusClosed) {
			t.Errorf("%s: closed_at %v", tc.name, closed)
		}
	}
	if s := imp.caseSubject(&sourceCase{ID: 9, Confidential: true}); s.confidentiality != core.ConfidentialLevel || s.businessRef != "9" || s.label != "Affaire 9" {
		t.Fatalf("confidentiality, business reference and placeholder title: %+v", s)
	}
}

func TestGrantRow(t *testing.T) {
	imp := newTestImporter()
	caseID := ID(string(core.SubjectKindCase), 1)
	unit := ID(string(core.SubjectKindOrgUnit), 10)
	imp.subjects[caseID], imp.subjects[unit] = core.SubjectKindCase, core.SubjectKindOrgUnit
	imp.employees[7] = true
	c := &StageCounts{}
	if row := imp.grantRow(caseID, "E", 7, 2, c); row == nil || row[2] != "7" || row[4] != int16(core.LevelManage) {
		t.Fatalf("an employee grant (Edition → MANAGE): %v", row)
	}
	if row := imp.grantRow(caseID, "O", 10, 3, c); row == nil || row[3] != unit || row[4] != int16(core.LevelContribute) {
		t.Fatalf("a unit grant (Ajout suivis → CONTRIBUTE): %v", row)
	}
	for _, left := range []struct {
		kind  string
		id    int64
		right int16
	}{{"E", 7, 5}, {"E", 8, 1}, {"O", 11, 1}, {"G", 1, 1}, {"X", 1, 1}} {
		if row := imp.grantRow(caseID, left.kind, left.id, left.right, c); row != nil {
			t.Errorf("%+v must be left out", left)
		}
	}
	if len(c.Skipped) != 5 {
		t.Fatalf("every left-out grant is counted by reason: %v", c.Skipped)
	}
}

func TestContactAndAddressRows(t *testing.T) {
	imp := newTestImporter()
	actorID := ID(string(core.SubjectKindActor), 1)
	imp.subjects[actorID] = core.SubjectKindActor
	c := &StageCounts{}
	if row := imp.contactRow(actorID, 5, "Adresse E-mail", " Someone@Example.org ", c); row == nil || row[1] != int16(actor.ContactTypeEmail) {
		t.Fatalf("a valid e-mail is imported normalized: %v", row)
	}
	if row := imp.contactRow(actorID, 5, "Adresse E-mail", "not an e-mail", c); row != nil {
		t.Fatal("an invalid e-mail is left out")
	}
	if row := imp.contactRow(actorID, 19, "IBAN", "CH00", c); row != nil {
		t.Fatal("bank references are not imported")
	}
	if row := imp.contactRow(actorID, 16, "Groupe Conseil Communal", "Groupe A", c); row == nil || row[1] != int16(actor.ContactTypeOther) || row[3] != "Groupe Conseil Communal" {
		t.Fatalf("other complements become OTHER with the legacy label: %v", row)
	}
	countries := map[string]string{"France": "FR"}
	ok := &sourceAddress{ActorID: 1, Street: "Rue A", PostalCode: "1003", Locality: "Lausanne", PostalBox: "12"}
	if row := imp.addressRow(ok, countries, c); row == nil || row[6] != "CH" || row[3] != "Case postale 12" {
		t.Fatalf("a blank country is Switzerland: %v", row)
	}
	for _, bad := range []*sourceAddress{
		{ActorID: 1, Street: "Rue A", PostalCode: "75001", Locality: "Lausanne"},                   // not a Swiss code
		{ActorID: 1, Street: "", PostalCode: "1003", Locality: "Lausanne"},                         // incomplete
		{ActorID: 1, Street: "Rue A", PostalCode: "1", Locality: "Ailleurs", Country: "Atlantide"}, // unknown country
		{ActorID: 2, Street: "Rue A", PostalCode: "1003", Locality: "Lausanne"},                    // actor not imported
	} {
		if row := imp.addressRow(bad, countries, c); row != nil {
			t.Errorf("%+v must be left out", bad)
		}
	}
	if row := imp.addressRow(&sourceAddress{ActorID: 1, Street: "Rue B", PostalCode: "75001", Locality: "Paris", Country: "France"}, countries, c); row == nil || row[6] != "FR" {
		t.Fatalf("a foreign address keeps its postal code: %v", row)
	}
}
