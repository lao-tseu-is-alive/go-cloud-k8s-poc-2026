package retention

import (
	"context"
	"fmt"
	"strings"

	"github.com/google/uuid"

	"github.com/lao-tseu-is-alive/go-cloud-k8s-poc-2026/pkg/core"
)

// CreateBordereau validates and prepares a bordereau of due cases.
func (s *Service) CreateBordereau(ctx context.Context, in BordereauInput) (*Bordereau, error) {
	if in.Disposition != DispositionArchive && in.Disposition != DispositionDestroy {
		return nil, fmt.Errorf("%w: a bordereau archives or destroys", core.ErrInvalidInput)
	}
	in.Reason = strings.TrimSpace(in.Reason)
	if err := validReason(in.Reason); err != nil {
		return nil, err
	}
	if len(in.CaseIDs) == 0 || len(in.CaseIDs) > MaxBordereauCases {
		return nil, fmt.Errorf("%w: a bordereau holds 1 to %d cases", core.ErrInvalidInput, MaxBordereauCases)
	}
	seen := make(map[uuid.UUID]bool, len(in.CaseIDs))
	for _, id := range in.CaseIDs {
		if id == uuid.Nil || seen[id] {
			return nil, fmt.Errorf("%w: case ids must be distinct and set", core.ErrInvalidInput)
		}
		seen[id] = true
	}
	b, err := s.repo.CreateBordereau(ctx, in)
	if err != nil {
		return nil, err
	}
	s.log.Info("disposition bordereau prepared", "bordereau_id", b.ID, "cases", len(in.CaseIDs), "operator", in.OperatorID)
	return b, nil
}

// GetBordereau reads a bordereau with its cases.
func (s *Service) GetBordereau(ctx context.Context, id uuid.UUID) (*Bordereau, error) {
	return s.repo.GetBordereau(ctx, id)
}

// ListBordereaux pages the bordereaux.
func (s *Service) ListBordereaux(ctx context.Context, filter BordereauFilter) (BordereauList, error) {
	limit, err := core.NormalizePageSize(filter.Limit)
	if err != nil {
		return BordereauList{}, err
	}
	filter.Limit, filter.Offset = limit, max(filter.Offset, 0)
	return s.repo.ListBordereaux(ctx, filter)
}

// MyApprovals lists the cases waiting for operatorID's decision.
func (s *Service) MyApprovals(ctx context.Context, operatorID string) ([]Approval, error) {
	return s.repo.MyApprovals(ctx, operatorID)
}

// Decide approves or rejects a case of a bordereau.
func (s *Service) Decide(ctx context.Context, in DecisionInput) (*Item, error) {
	in.Reason = strings.TrimSpace(in.Reason)
	if in.ItemID == uuid.Nil {
		return nil, fmt.Errorf("%w: item id is required", core.ErrInvalidInput)
	}
	if err := validReason(in.Reason); err != nil {
		return nil, err
	}
	item, err := s.repo.Decide(ctx, in)
	if err != nil {
		return nil, err
	}
	s.log.Info("disposition decided", "item_id", in.ItemID, "approved", in.Approve, "operator", in.OperatorID)
	return item, nil
}

// Execute handles the approved cases of a fully decided bordereau.
func (s *Service) Execute(ctx context.Context, id uuid.UUID, operatorID string) (*Bordereau, error) {
	b, err := s.repo.Execute(ctx, id, operatorID)
	if err != nil {
		return nil, err
	}
	s.log.Info("disposition bordereau executed", "bordereau_id", id, "proof_sha256", b.ProofSHA256, "operator", operatorID)
	return b, nil
}

// RebuildTransfer queues again a failed or expired transfer package.
func (s *Service) RebuildTransfer(ctx context.Context, itemID uuid.UUID, operatorID string) (*Item, error) {
	item, err := s.repo.RebuildTransfer(ctx, itemID, operatorID)
	if err != nil {
		return nil, err
	}
	s.log.Info("transfer package queued again", "item_id", itemID, "operator", operatorID)
	return item, nil
}

// Cancel abandons an open bordereau.
func (s *Service) Cancel(ctx context.Context, id uuid.UUID, reason, operatorID string) (*Bordereau, error) {
	reason = strings.TrimSpace(reason)
	if err := validReason(reason); err != nil {
		return nil, err
	}
	return s.repo.Cancel(ctx, id, reason, operatorID)
}
