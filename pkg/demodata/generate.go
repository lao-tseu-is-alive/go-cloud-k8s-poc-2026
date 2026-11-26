package demodata

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"time"

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

// setupTime is when the fictional municipality set up its catalogue, org
// chart and users, before the oldest story.
const setupTime = "2014-01-06 08:00"

// errNotAdmin reports a server that did not bootstrap the demo administrator.
var errNotAdmin = errors.New("the demo administrator is not ADMIN: bootstrap it (GOELAND_BOOTSTRAP_ADMINS) on the server the generator talks to")

// Options selects what Generate writes.
type Options struct {
	// Profile is the amount of data (ProfileSmall).
	Profile string
	// Language is the language of every generated text (Languages(); "fr" by default).
	Language string
	// Clock, when set, is told the story time before each step, so the caller
	// can re-date afterwards what the server stamped at generation time.
	Clock func(context.Context, time.Time) error
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
	places, err := loadPlaces()
	if err != nil {
		return Report{}, err
	}
	r := &run{ctx: ctx, c: clients, t: texts, places: places, clock: opts.Clock, units: map[string]string{}, users: map[string]*v1.User{}, count: map[string]int{}}
	r.at(setupTime)
	r.setupUsers()
	r.setupCatalogue()
	r.setupOrgChart()
	r.permitAndOpposition()
	r.citizenReport()
	report := Report{Profile: opts.Profile, Language: texts.Language, Counts: r.count}
	report.Counts["users"] = len(r.users)
	return report, r.err
}
