package core

import (
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
)

// DateRange is an inclusive range of calendar days in Europe/Zurich (GLD-074);
// a nil bound leaves that side open.
type DateRange struct {
	// From is the first day kept; nil means unbounded.
	From *time.Time
	// To is the last day kept; nil means unbounded.
	To *time.Time
}

// ParseDateRange parses the YYYY-MM-DD bounds of a list filter, an empty
// bound being open. An invalid day or a range ending before it starts is
// ErrInvalidInput.
func ParseDateRange(from, to string) (DateRange, error) {
	var r DateRange
	var err error
	if r.From, err = parseDay(from); err != nil {
		return DateRange{}, err
	}
	if r.To, err = parseDay(to); err != nil {
		return DateRange{}, err
	}
	if r.From != nil && r.To != nil && r.To.Before(*r.From) {
		return DateRange{}, fmt.Errorf("%w: the period ends (%s) before it starts (%s)", ErrInvalidInput, to, from)
	}
	return r, nil
}

// parseDay parses one YYYY-MM-DD day; empty is nil.
func parseDay(s string) (*time.Time, error) {
	if s == "" {
		return nil, nil
	}
	day, err := time.Parse(time.DateOnly, s)
	if err != nil {
		return nil, fmt.Errorf("%w: %q is not a YYYY-MM-DD date", ErrInvalidInput, s)
	}
	return &day, nil
}

// AddTo sets @<name>_from and @<name>_to on args: the day as YYYY-MM-DD, or
// NULL for an open bound.
func (r DateRange) AddTo(args pgx.NamedArgs, name string) pgx.NamedArgs {
	args[name+"_from"], args[name+"_to"] = dayArg(r.From), dayArg(r.To)
	return args
}

// dayArg is the query argument of one bound.
func dayArg(day *time.Time) any {
	if day == nil {
		return nil
	}
	return day.Format(time.DateOnly)
}

// DayRangeSQL keeps the rows whose DATE expr falls within @<name>_from and
// @<name>_to (DateRange.AddTo), both inclusive.
func DayRangeSQL(expr, name string) string {
	return `(@` + name + `_from::date IS NULL OR ` + expr + ` >= @` + name + `_from::date)
  AND (@` + name + `_to::date IS NULL OR ` + expr + ` <= @` + name + `_to::date)`
}

// TimeRangeSQL keeps the rows whose TIMESTAMPTZ expr falls on a day of
// @<name>_from .. @<name>_to (DateRange.AddTo), days read in Europe/Zurich, so
// "until 31 March" keeps the whole of 31 March. The bounds are constants of
// the query, so an index on expr still serves the range.
func TimeRangeSQL(expr, name string) string {
	return `(@` + name + `_from::date IS NULL OR ` + expr + ` >= (@` + name + `_from::date)::timestamp AT TIME ZONE '` + BusinessRefPeriodTimeZone + `')
  AND (@` + name + `_to::date IS NULL OR ` + expr + ` < (@` + name + `_to::date + 1)::timestamp AT TIME ZONE '` + BusinessRefPeriodTimeZone + `')`
}

// RecordFilter is what every subject list filters on besides its own fields
// (GLD-074): when the record was created and by whom, and whether the caller
// worked on it (GLD-088).
type RecordFilter struct {
	// Created keeps the records created on these days.
	Created DateRange
	// CreatedBy keeps the records created by this user (app_user.user_id, the
	// operator id stored in created_by); empty means anyone.
	CreatedBy string
	// ActivityOf keeps the records this user changed (audit_event) or
	// consulted (access log, a read or download allowed), and is the user of
	// the my_activity sort (MyActivitySortField). The adapter sets it to the
	// caller, never from the request; empty means anyone.
	ActivityOf string
}

// ParseRecordFilter builds a RecordFilter from the request fields.
func ParseRecordFilter(createdFrom, createdTo, createdBy string) (RecordFilter, error) {
	created, err := ParseDateRange(createdFrom, createdTo)
	if err != nil {
		return RecordFilter{}, err
	}
	return RecordFilter{Created: created, CreatedBy: createdBy}, nil
}

// AddTo sets @created_from, @created_to, @created_by and @activity_of on args.
func (f RecordFilter) AddTo(args pgx.NamedArgs) pgx.NamedArgs {
	args["created_by"], args["activity_of"] = f.CreatedBy, f.ActivityOf
	return f.Created.AddTo(args, "created")
}

// RecordFilterSQL is the predicate of a RecordFilter on the table aliased
// alias, whose created_at and created_by columns record the creation.
func RecordFilterSQL(alias string) string {
	return `(@created_by = '' OR ` + alias + `.created_by = @created_by)
  AND (@activity_of = '' OR ` + alias + `.id IN (
        SELECT ae.subject_id FROM audit_event ae WHERE ae.actor_user_id = @activity_of
        UNION
        SELECT aa.subject_id FROM access_audit_event aa
         WHERE aa.user_id = @activity_of AND aa.outcome = 'GRANTED' AND aa.action IN ` + consultedActionsSQL + `))
  AND ` + TimeRangeSQL(alias+".created_at", "created")
}

// consultedActionsSQL lists the access log actions that count as consulting a
// subject (GLD-088): reading it (READ_SENSITIVE before GLD-089) or downloading it.
const consultedActionsSQL = `('READ', 'READ_SENSITIVE', 'DOWNLOAD')`

// MyActivitySort is the order_by of a list of the caller's activity without
// one (GLD-088): its last activity first.
const MyActivitySort = "my_activity desc"

// MyActivitySortField is the my_activity sort of a subject list whose subject
// id is idExpr: the last time @activity_of (RecordFilter.ActivityOf) changed
// or consulted it; NULL for anyone else.
func MyActivitySortField(idExpr string) SortField {
	return SortField{Nullable: true, Expr: `greatest(
    (SELECT max(ae.occurred_at) FROM audit_event ae WHERE ae.subject_id = ` + idExpr + ` AND ae.actor_user_id = @activity_of),
    (SELECT max(aa.occurred_at) FROM access_audit_event aa
      WHERE aa.subject_id = ` + idExpr + ` AND aa.user_id = @activity_of AND aa.outcome = 'GRANTED' AND aa.action IN ` + consultedActionsSQL + `))`}
}

// MyActivityRequest applies a list request's my_activity flag (GLD-088): the
// filter keeps what the caller (userID) worked on, sorted by default by its
// last activity. It returns the order_by to use.
func (f *RecordFilter) MyActivityRequest(myActivity bool, userID, orderBy string) string {
	if !myActivity {
		return orderBy
	}
	f.ActivityOf = userID
	if orderBy == "" {
		return MyActivitySort
	}
	return orderBy
}
