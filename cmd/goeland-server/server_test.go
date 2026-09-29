package main

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"testing"
	"time"
)

func TestWaitForDatabaseRetriesUntilReachable(t *testing.T) {
	calls := 0
	ping := func(context.Context) error {
		calls++
		if calls < 3 {
			return errors.New("connection refused")
		}
		return nil
	}
	if err := waitForDatabase(context.Background(), ping, 10*time.Second, slog.New(slog.NewTextHandler(io.Discard, nil))); err != nil || calls != 3 {
		t.Fatalf("want success after 3 attempts, got %v after %d", err, calls)
	}
}

func TestWaitForDatabaseGivesUp(t *testing.T) {
	calls := 0
	down := func(context.Context) error { calls++; return errors.New("connection refused") }
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	if err := waitForDatabase(context.Background(), down, 0, log); err == nil || calls != 1 {
		t.Fatalf("a zero timeout makes one attempt and fails, got %v after %d", err, calls)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := waitForDatabase(ctx, down, time.Minute, log); !errors.Is(err, context.Canceled) {
		t.Fatalf("a cancelled context stops the wait, got %v", err)
	}
}
