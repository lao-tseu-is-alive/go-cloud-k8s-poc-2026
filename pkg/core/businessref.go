package core

import (
	"fmt"
	"regexp"
	"strings"
	"unicode/utf8"
)

const (
	// MaxBusinessRefLength is the maximum number of code points in a business reference.
	MaxBusinessRefLength = 100
	// BusinessRefPeriodTimeZone is the time zone whose calendar year prefixes
	// allocated references; the year is computed by PostgreSQL in this zone.
	BusinessRefPeriodTimeZone = "Europe/Zurich"
	// businessRefSequenceDigits is the zero-padded width of the allocated counter.
	businessRefSequenceDigits = 6
)

// businessRefNamespacePattern accepts upper-case codes such as OPC or PERMIS_2;
// it mirrors the protovalidate rule on BusinessRefRequest.namespace.
var businessRefNamespacePattern = regexp.MustCompile(`^[A-Z][A-Z0-9_]{0,31}$`)

// BusinessRefRequest asks for a business reference on a subject: either an
// explicit Value or a server-allocated one (Allocate). Exactly one of the two
// must be set, and Allocate requires a Namespace.
type BusinessRefRequest struct {
	// Namespace scopes uniqueness (e.g. OPC). Empty allows a free, non-unique
	// explicit Value such as an imported legacy number.
	Namespace string
	// Value is an explicit reference; mutually exclusive with Allocate.
	Value string
	// Allocate asks for the next "<YYYY>-<NNNNNN>" reference of Namespace for the
	// current calendar year in BusinessRefPeriodTimeZone.
	Allocate bool
}

// IsZero reports whether no business reference was requested.
func (r BusinessRefRequest) IsZero() bool {
	return r.Namespace == "" && r.Value == "" && !r.Allocate
}

// Normalized trims r and validates it, returning core.ErrInvalidInput on any
// violation. It is applied by the service before any persistence.
func (r BusinessRefRequest) Normalized() (BusinessRefRequest, error) {
	r.Namespace = strings.TrimSpace(r.Namespace)
	r.Value = strings.TrimSpace(r.Value)
	switch {
	case r.Allocate && r.Value != "":
		return r, fmt.Errorf("%w: business_ref: set either an explicit value or allocate, not both", ErrInvalidInput)
	case !r.Allocate && r.Value == "":
		return r, fmt.Errorf("%w: business_ref: an explicit value or allocate is required", ErrInvalidInput)
	case r.Allocate && r.Namespace == "":
		return r, fmt.Errorf("%w: business_ref: allocate requires a namespace", ErrInvalidInput)
	case r.Namespace != "" && !businessRefNamespacePattern.MatchString(r.Namespace):
		return r, fmt.Errorf("%w: business_ref: namespace must match %s", ErrInvalidInput, businessRefNamespacePattern)
	case utf8.RuneCountInString(r.Value) > MaxBusinessRefLength:
		return r, fmt.Errorf("%w: business_ref exceeds %d characters", ErrInvalidInput, MaxBusinessRefLength)
	}
	return r, nil
}

// FormatAllocatedBusinessRef renders an allocated reference as
// "<period>-<zero-padded sequence>", e.g. FormatAllocatedBusinessRef("2026", 1245)
// returns "2026-001245". Sequences wider than the padding keep all their digits.
func FormatAllocatedBusinessRef(period string, sequence int64) string {
	return fmt.Sprintf("%s-%0*d", period, businessRefSequenceDigits, sequence)
}

// LookupFilter selects subjects by business reference.
type LookupFilter struct {
	// BusinessRef is the required, exact reference to find.
	BusinessRef string
	// Namespace restricts the match to one namespace; empty matches any namespace,
	// including references without one.
	Namespace string
	// Kind restricts the match to one subject kind; SubjectKindUnspecified means any.
	Kind SubjectKind
}

// maxLookupResults bounds LookupSubjects; a reference without namespace is not
// unique, but a lookup is not a search endpoint.
const maxLookupResults = 50
