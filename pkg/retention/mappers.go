package retention

import (
	"time"

	goelandv1 "github.com/lao-tseu-is-alive/go-cloud-k8s-poc-2026/gen/goeland/v1"

	"github.com/lao-tseu-is-alive/go-cloud-k8s-poc-2026/pkg/core"
)

var dispositionToProto = map[Disposition]goelandv1.FinalDisposition{
	DispositionArchive: goelandv1.FinalDisposition_FINAL_DISPOSITION_ARCHIVE,
	DispositionDestroy: goelandv1.FinalDisposition_FINAL_DISPOSITION_DESTROY,
	DispositionReview:  goelandv1.FinalDisposition_FINAL_DISPOSITION_REVIEW,
}

// DispositionToProto converts a disposition (UNSPECIFIED when unknown).
func DispositionToProto(d Disposition) goelandv1.FinalDisposition {
	return dispositionToProto[d]
}

// DispositionFromProto converts a proto disposition ("" for UNSPECIFIED).
func DispositionFromProto(d goelandv1.FinalDisposition) Disposition {
	for domain, wire := range dispositionToProto {
		if wire == d {
			return domain
		}
	}
	return ""
}

// PolicyToProto converts a policy (nil for nil).
func PolicyToProto(p *Policy) *goelandv1.RetentionPolicy {
	if p == nil {
		return nil
	}
	return &goelandv1.RetentionPolicy{
		Id: p.ID.String(), Code: p.Code, Label: p.Label, Description: p.Description, LegalBasis: p.LegalBasis,
		DurationYears: int32(p.DurationYears), FinalDisposition: DispositionToProto(p.FinalDisposition), IsActive: p.IsActive,
	}
}

// RetentionToProto converts a retention (nil for nil).
func RetentionToProto(r *Retention) *goelandv1.SubjectRetention {
	if r == nil {
		return nil
	}
	out := &goelandv1.SubjectRetention{
		Id: r.ID.String(), SubjectId: r.SubjectID.String(), PolicyCode: r.PolicyCode, PolicyLabel: r.PolicyLabel,
		DurationYears: int32(r.DurationYears), FinalDisposition: DispositionToProto(r.FinalDisposition),
		StartsOn: r.StartsOn.Format(time.DateOnly), UntilOn: r.UntilOn.Format(time.DateOnly),
		CreatedAt: core.TimestampOrNil(r.CreatedAt), CreatedBy: r.CreatedBy,
		EndedAt: core.TimestampPtrOrNil(r.EndedAt), EndReason: r.EndReason,
	}
	if r.EndedBy != nil {
		out.EndedBy = *r.EndedBy
	}
	return out
}

// HoldToProto converts a hold (nil for nil).
func HoldToProto(h *Hold) *goelandv1.RetentionHold {
	if h == nil {
		return nil
	}
	out := &goelandv1.RetentionHold{
		Id: h.ID.String(), SubjectId: h.SubjectID.String(), Reason: h.Reason,
		PlacedAt: core.TimestampOrNil(h.PlacedAt), PlacedBy: h.PlacedBy,
		ReleasedAt: core.TimestampPtrOrNil(h.ReleasedAt), ReleaseReason: h.ReleaseReason,
	}
	if h.ReleasedBy != nil {
		out.ReleasedBy = *h.ReleasedBy
	}
	return out
}

// CaseRetentionToProto converts the retention of a case.
func CaseRetentionToProto(c *CaseRetention) *goelandv1.GetCaseRetentionResponse {
	out := &goelandv1.GetCaseRetentionResponse{
		Current: RetentionToProto(c.Current), Hold: HoldToProto(c.Hold), DefaultPolicy: PolicyToProto(c.DefaultPolicy),
		Retentions: make([]*goelandv1.SubjectRetention, 0, len(c.Retentions)),
		Holds:      make([]*goelandv1.RetentionHold, 0, len(c.Holds)),
	}
	for _, r := range c.Retentions {
		out.Retentions = append(out.Retentions, RetentionToProto(r))
	}
	for _, h := range c.Holds {
		out.Holds = append(out.Holds, HoldToProto(h))
	}
	return out
}
