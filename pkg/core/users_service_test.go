package core_test

import (
	"errors"
	"fmt"
	"testing"

	"github.com/lao-tseu-is-alive/go-cloud-k8s-poc-2026/pkg/core"
	"github.com/lao-tseu-is-alive/go-cloud-k8s-poc-2026/pkg/core/coretest"
)

func TestBatchGetUsersValidation(t *testing.T) {
	svc := coretest.NewService(t)
	if _, err := svc.BatchGetUsers(t.Context(), []string{" ", ""}); !errors.Is(err, core.ErrInvalidInput) {
		t.Fatalf("blank ids: want ErrInvalidInput, got %v", err)
	}
	many := make([]string, core.MaxBatchUsers+1)
	for i := range many {
		many[i] = fmt.Sprint(i + 1)
	}
	if _, err := svc.BatchGetUsers(t.Context(), many); !errors.Is(err, core.ErrInvalidInput) {
		t.Fatalf("too many ids: want ErrInvalidInput, got %v", err)
	}
	// Repeated ids collapse, so the limit counts distinct users.
	repeated := make([]string, core.MaxBatchUsers+50)
	for i := range repeated {
		repeated[i] = "1"
	}
	if _, err := svc.BatchGetUsers(t.Context(), repeated); err != nil {
		t.Fatalf("repeated ids: %v", err)
	}
}
