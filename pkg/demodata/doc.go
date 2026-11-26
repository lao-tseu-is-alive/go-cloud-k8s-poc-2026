// Package demodata generates the synthetic data of the public demo (GLD-079,
// docs/DEMO_SCRIPT.md): a fictional municipality, « Commune de Goéland-sur-Lac »,
// its catalogue, org chart, demo users and scripted hero cases.
//
// Every value is invented here, never sampled or anonymized from real data. The
// generator writes through the public API (the Connect clients of
// gen/goeland/v1), one demo user at a time, so the data passes the same
// validation, authorization, audit, outbox and timeline rules as the
// application; it doubles as a regression test of the business API.
//
// A Session holds the demo users and their in-process bearer tokens: its
// Verifier authenticates them on a server built for the run only (the command
// `goeland-server demo-seed` never listens on a network). Every generated text
// comes from a catalogue per language (textSet); the cases, links and story dates
// are the same in every language.
package demodata
