package demodata

import (
	"context"
	"errors"
	"fmt"
	"slices"

	v1 "github.com/lao-tseu-is-alive/go-cloud-k8s-poc-2026/gen/goeland/v1"
)

// Profiles of the generator (GLD-079).
const (
	// ProfileSmall is the catalogue, the org chart, the demo users and the hero
	// cases: what the tests and a first demo need.
	ProfileSmall = "small"
)

// profiles lists the profiles built so far; "demo" (background volume) and
// "volume" come later.
var profiles = []string{ProfileSmall}

// errNotAdmin reports a server that did not bootstrap the demo administrator.
var errNotAdmin = errors.New("the demo administrator is not ADMIN: bootstrap it (GOELAND_BOOTSTRAP_ADMINS) on the server the generator talks to")

// Options selects what Generate writes.
type Options struct {
	// Profile is the amount of data (ProfileSmall).
	Profile string
	// Language is the language of every generated text (Languages(); "fr" by default).
	Language string
}

// Generate writes the demo data through clients, in a brand-new database the
// caller checked. It stops at the first refused request and returns what was
// created until then.
func Generate(ctx context.Context, clients *Clients, opts Options) (Report, error) {
	if opts.Language == "" {
		opts.Language = "fr"
	}
	texts, err := textsFor(opts.Language)
	if err != nil {
		return Report{}, err
	}
	if !slices.Contains(profiles, opts.Profile) {
		return Report{}, fmt.Errorf("demo profile %q is not supported (supported: %v)", opts.Profile, profiles)
	}
	r := &run{ctx: ctx, c: clients, t: texts, units: map[string]string{}, users: map[string]*v1.User{}, count: map[string]int{}}
	r.setupUsers()
	r.setupCatalogue()
	r.setupOrgChart()
	r.permitAndOpposition()
	r.citizenReport()
	report := Report{Profile: opts.Profile, Language: texts.Language, Counts: r.count}
	report.Counts["users"] = len(r.users)
	return report, r.err
}

// Story coordinates: the fictional municipality lies on the territory of the
// City of Lausanne (LV95, EPSG:2056), inside the extent of its map tiles.
const (
	tilleulsEast, tilleulsNorth = 2_539_420.0, 1_153_310.0
	cedresEast, cedresNorth     = 2_537_850.0, 1_152_640.0
)

// rectangle is a GeoJSON polygon of width × height metres from its south-west corner.
func rectangle(east, north, width, height float64) string {
	return fmt.Sprintf(`{"type":"Polygon","coordinates":[[[%.1f,%.1f],[%.1f,%.1f],[%.1f,%.1f],[%.1f,%.1f],[%.1f,%.1f]]]}`,
		east, north, east+width, north, east+width, north+height, east, north+height, east, north)
}

// point is a GeoJSON point.
func point(east, north float64) string {
	return fmt.Sprintf(`{"type":"Point","coordinates":[%.1f,%.1f]}`, east, north)
}
