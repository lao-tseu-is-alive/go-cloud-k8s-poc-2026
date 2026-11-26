package demodata

import (
	"bytes"
	"context"
	"math"
	"os"
	"reflect"
	"strconv"
	"strings"
	"testing"
)

// TestTextsAreComplete checks that every language fills every text and names
// every unit, case type, document type and retention policy of the demo.
func TestTextsAreComplete(t *testing.T) {
	for _, lang := range Languages() {
		texts, err := textsFor(lang)
		if err != nil {
			t.Fatal(err)
		}
		checkFilled(t, lang, reflect.ValueOf(*texts))
		for _, u := range orgChart {
			if _, ok := texts.Units[u.key]; !ok {
				t.Errorf("%s: unit %s has no text", lang, u.key)
			}
		}
		for _, ct := range caseTypes {
			if _, ok := texts.CaseTypes[ct.code]; !ok {
				t.Errorf("%s: case type %s has no text", lang, ct.code)
			}
		}
		for _, dt := range documentTypes {
			if _, ok := texts.DocumentTypes[dt.code]; !ok {
				t.Errorf("%s: document type %s has no text", lang, dt.code)
			}
		}
		for _, p := range retentionPolicies {
			if _, ok := texts.Policies[p.code]; !ok {
				t.Errorf("%s: retention policy %s has no text", lang, p.code)
			}
		}
	}
	if _, err := textsFor("xx"); err == nil {
		t.Error("an unknown language must be refused")
	}
}

// checkFilled fails on any empty string reachable from v.
func checkFilled(t *testing.T, path string, v reflect.Value) {
	t.Helper()
	switch v.Kind() {
	case reflect.String:
		if strings.TrimSpace(v.String()) == "" {
			t.Errorf("%s is empty", path)
		}
	case reflect.Struct:
		for i := range v.NumField() {
			checkFilled(t, path+"."+v.Type().Field(i).Name, v.Field(i))
		}
	case reflect.Slice:
		if v.Len() == 0 {
			t.Errorf("%s is empty", path)
		}
		for i := range v.Len() {
			checkFilled(t, path+"["+strconv.Itoa(i)+"]", v.Index(i))
		}
	case reflect.Map:
		for _, key := range v.MapKeys() {
			checkFilled(t, path+"["+key.String()+"]", v.MapIndex(key))
		}
	default:
	}
}

// TestRenderPDF checks the structure of a generated PDF: header, objects at
// the offsets the cross-reference table gives, trailer.
func TestRenderPDF(t *testing.T) {
	pdf := renderPDF("Décision — (n° 1)", []string{"Ligne « une »", "€ 42"}, textsFR.Footer)
	if !bytes.HasPrefix(pdf, []byte("%PDF-1.4\n")) || !bytes.HasSuffix(pdf, []byte("%%EOF\n")) {
		t.Fatalf("not a PDF: %q", pdf[:20])
	}
	xref := bytes.Index(pdf, []byte("\nxref\n")) + 1
	start := bytes.LastIndex(pdf, []byte("startxref\n"))
	if got := strings.TrimSpace(strings.Split(string(pdf[start+len("startxref\n"):]), "\n")[0]); got != strconv.Itoa(xref) {
		t.Errorf("startxref %s, xref at %d", got, xref)
	}
	lines := strings.Split(string(pdf[xref:]), "\n")
	for i := 1; i <= 5; i++ {
		offset, err := strconv.Atoi(lines[2+i][:10])
		if err != nil || !bytes.HasPrefix(pdf[offset:], []byte(strconv.Itoa(i)+" 0 obj")) {
			t.Errorf("object %d is not at offset %s", i, lines[2+i][:10])
		}
	}
	if !bytes.Contains(pdf, []byte(`(D\351cision \227 \(n\260 1\))`)) {
		t.Errorf("the title is not encoded in WinAnsi with its parentheses escaped")
	}
}

// TestSessionVerifier checks that each demo user's token authenticates that
// user only, and that requests carry the token of the user they act as.
func TestSessionVerifier(t *testing.T) {
	s, err := NewSession()
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	for _, u := range Users() {
		token, err := s.tokenFor(as(ctx, u.Key))
		if err != nil {
			t.Fatal(err)
		}
		user, err := s.Verifier().VerifyBearerToken(ctx, token)
		if err != nil || user.AppUserID != u.ID || user.DisplayName != u.DisplayName {
			t.Errorf("%s: %v %v", u.Key, user, err)
		}
	}
	if _, err := s.Verifier().VerifyBearerToken(ctx, "not-a-demo-token"); err == nil {
		t.Error("an unknown token must be refused")
	}
	if _, err := s.tokenFor(ctx); err == nil {
		t.Error("a request acting as nobody must fail")
	}
	if AdminUserID() != "1" || Users()[0].Key != userAlice {
		t.Error("Alice is the first user and the bootstrap administrator")
	}
}

// TestPublicSession checks the public tokens of the demo mode: each signs in
// as its demo user, and a token of another session does not.
func TestPublicSession(t *testing.T) {
	ctx := context.Background()
	verifier := PublicSession().Verifier()
	for _, u := range Users() {
		user, err := verifier.VerifyBearerToken(ctx, PublicToken(u.Key))
		if err != nil || user.AppUserID != u.ID {
			t.Errorf("%s: %v %v", u.Key, user, err)
		}
	}
	drawn, err := NewSession()
	if err != nil {
		t.Fatal(err)
	}
	token, _ := drawn.tokenFor(as(ctx, userAlice))
	if _, err := verifier.VerifyBearerToken(ctx, token); err == nil {
		t.Error("a token drawn for demo-seed must not sign in on the demo mode")
	}
}

// TestPlaces checks things.geojson: every feature converts to LV95 inside the
// map, and the conversion agrees with swisstopo's reference point (Bern).
func TestPlaces(t *testing.T) {
	places, err := loadPlaces()
	if err != nil {
		t.Fatal(err)
	}
	for _, key := range []string{"tilleuls-parcel", "tilleuls-building", "cedres-streetlight"} {
		if places[key].geometry == "" {
			t.Errorf("geodata/things.geojson lacks %s", key)
		}
	}
	east, north := wgs84ToLV95(7.438632495, 46.951082877)
	if math.Abs(east-2_600_000) > 1.5 || math.Abs(north-1_200_000) > 1.5 {
		t.Errorf("Bern converts to %.1f, %.1f", east, north)
	}
	if _, err := toLV95([]any{2.35, 48.85}); err == nil {
		t.Error("a position outside the map tiles (Paris) must be refused")
	}
}

// TestAssets checks the demo photographs: JPEG files under 400 KB, each
// recorded with its prompt in assets/README.md.
func TestAssets(t *testing.T) {
	entries, err := assets.ReadDir("assets")
	if err != nil || len(entries) == 0 {
		t.Fatalf("no demo photograph: %v", err)
	}
	readme, err := os.ReadFile("assets/README.md")
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range entries {
		content, _ := assets.ReadFile("assets/" + e.Name())
		if !bytes.HasPrefix(content, []byte{0xFF, 0xD8, 0xFF}) || len(content) > 400_000 {
			t.Errorf("%s: a JPEG under 400 KB expected (%d bytes)", e.Name(), len(content))
		}
		if !bytes.Contains(readme, []byte("`"+e.Name()+"`")) {
			t.Errorf("%s is not recorded in assets/README.md", e.Name())
		}
	}
}
