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
	// ProfileDemo adds the background around the heroes (DEMO_SCRIPT §5):
	// about 300 actors, 400 cases, 1,500 documents and 200 things, so lists,
	// filters, search and counts mean something.
	ProfileDemo = "demo"
)

// profiles lists the profiles built so far.
var profiles = []string{ProfileSmall, ProfileDemo}

// Profiles returns the profiles the generator supports, smallest first.
func Profiles() []string { return slices.Clone(profiles) }

// setupTime is when the fictional municipality set up its catalogue, org
// chart and users, before the oldest story.
const setupTime = "2014-01-06 08:00"

// errNotAdmin reports a server that did not bootstrap the demo administrator.
var errNotAdmin = errors.New("the demo administrator is not ADMIN: bootstrap it (GOELAND_BOOTSTRAP_ADMINS) on the server the generator talks to")

// Options selects what Generate writes.
type Options struct {
	// Profile is the amount of data (Profiles(); ProfileSmall by default).
	Profile string
	// Language is the language of every generated text (Languages(); "fr" by default).
	Language string
	// Clock, when set, is told the story time before each step, so the caller
	// can re-date afterwards what the server stamped at generation time.
	Clock func(context.Context, time.Time) error
	// BackgroundLimit, when positive, caps the background of ProfileDemo to
	// that many cases per case type (the tests: the full background takes
	// about two minutes).
	BackgroundLimit int
	// Today is the generation day the ongoing work is dated from (§4.8 to
	// §4.12); the zero value means today in Europe/Zurich.
	Today time.Time
}

// Generate writes the demo data through clients, in a brand-new database the
// caller checked. It stops at the first refused request and returns what was
// created until then.
func Generate(ctx context.Context, clients *Clients, opts Options) (Report, error) {
	if opts.Language == "" {
		opts.Language = "fr"
	}
	if opts.Profile == "" {
		opts.Profile = ProfileSmall
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
	r.today = generationDay(opts.Today)
	r.at(setupTime)
	r.setupUsers()
	r.setupCatalogue()
	r.relabelSeeded()
	r.setupOrgChart()
	cafe := r.signs()
	r.permitAndOpposition()
	tutu := r.complaint()
	r.subsidy()
	r.citizenReport()
	r.ongoing(cafe, tutu)
	if opts.Profile == ProfileDemo {
		r.background(opts.BackgroundLimit)
	}
	report := Report{Profile: opts.Profile, Language: texts.Language, Counts: r.count}
	report.Counts["users"] = len(r.users)
	return report, r.err
}

// generationDay is midnight of day (today when zero) in Europe/Zurich.
func generationDay(day time.Time) time.Time {
	if day.IsZero() {
		day = time.Now()
	}
	day = day.In(zurich)
	return time.Date(day.Year(), day.Month(), day.Day(), 0, 0, 0, 0, zurich)
}
