package core

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	goelandv1 "github.com/lao-tseu-is-alive/go-cloud-k8s-poc-2026/gen/goeland/v1"
)

// The caller's activity (GLD-088, « Mon tableau de bord »): the subjects a user
// changed (audit_event) or consulted (access log), the filter and sort of the
// subject lists being RecordFilter.ActivityOf and MyActivitySortField.

// subjectActivitySQL reads the last change and the last consultation by
// @user_id of each subject of @ids (GLD-088).
const subjectActivitySQL = `
SELECT s.id AS subject_id,
       (SELECT max(ae.occurred_at) FROM audit_event ae WHERE ae.subject_id = s.id AND ae.actor_user_id = @user_id) AS last_changed_at,
       (SELECT max(aa.occurred_at) FROM access_audit_event aa
         WHERE aa.subject_id = s.id AND aa.user_id = @user_id AND aa.outcome = 'GRANTED' AND aa.action IN ` + consultedActionsSQL + `) AS last_viewed_at
  FROM unnest(@ids::uuid[]) AS s(id);`

// SubjectActivity is a user's last activity on a subject (GLD-088).
type SubjectActivity struct {
	// SubjectID is the subject.
	SubjectID uuid.UUID `db:"subject_id"`
	// LastChangedAt is the user's last change of the subject (audit_event); nil when never.
	LastChangedAt *time.Time `db:"last_changed_at"`
	// LastViewedAt is the user's last consultation of it (access log); nil when never.
	LastViewedAt *time.Time `db:"last_viewed_at"`
}

// SubjectActivitiesTx returns userID's last activity on each of ids, in the
// order of ids; no query when ids is empty.
func SubjectActivitiesTx(ctx context.Context, q Querier, userID string, ids []uuid.UUID) ([]SubjectActivity, error) {
	if len(ids) == 0 {
		return nil, nil
	}
	rows, err := q.Query(ctx, subjectActivitySQL, pgx.NamedArgs{"user_id": userID, "ids": ids})
	if err != nil {
		return nil, fmt.Errorf("read the activity: %w", err)
	}
	activities, err := pgx.CollectRows(rows, pgx.RowToStructByName[SubjectActivity])
	if err != nil {
		return nil, fmt.Errorf("read the activity: %w", err)
	}
	return activities, nil
}

// SubjectActivitiesToProto converts activities to their messages.
func SubjectActivitiesToProto(activities []SubjectActivity) []*goelandv1.SubjectActivity {
	out := make([]*goelandv1.SubjectActivity, len(activities))
	for i, a := range activities {
		out[i] = &goelandv1.SubjectActivity{
			SubjectId: a.SubjectID.String(), LastChangedAt: TimestampPtrOrNil(a.LastChangedAt), LastViewedAt: TimestampPtrOrNil(a.LastViewedAt),
		}
	}
	return out
}

// MyActivity returns, when myActivity is set, the authenticated caller's last
// activity on each of ids for a list response (GLD-088), nil otherwise;
// failures are Connect errors.
func (a *Authorizer) MyActivity(ctx context.Context, myActivity bool, viewer Viewer, ids []uuid.UUID) ([]*goelandv1.SubjectActivity, error) {
	if !myActivity {
		return nil, nil
	}
	activities, err := SubjectActivitiesTx(ctx, a.q, viewer.UserID, ids)
	if err != nil {
		return nil, ToConnectError(a.log, "activity", err)
	}
	return SubjectActivitiesToProto(activities), nil
}
