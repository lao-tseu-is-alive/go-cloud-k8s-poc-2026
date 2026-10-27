package core

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestOutboxRetryDelay(t *testing.T) {
	want := map[int32]time.Duration{
		0: 10 * time.Second, 1: 10 * time.Second, 2: time.Minute, 3: 6 * time.Minute,
		4: 36 * time.Minute, 5: time.Hour, OutboxMaxAttempts: time.Hour,
	}
	for attempt, delay := range want {
		if got := OutboxRetryDelay(attempt); got != delay {
			t.Errorf("OutboxRetryDelay(%d) = %s, want %s", attempt, got, delay)
		}
	}
}

// namedConsumer is a consumer for the constructor and delivery tests.
type namedConsumer struct {
	name string
	err  error
}

func (c namedConsumer) Name() string { return c.name }

func (c namedConsumer) Handle(context.Context, *OutboxEvent) error {
	if c.err != nil {
		return c.err
	}
	panic("handled")
}

func TestOutboxConsumerNames(t *testing.T) {
	for _, consumers := range [][]OutboxConsumer{{namedConsumer{}}, {namedConsumer{name: "a"}, namedConsumer{name: "a"}}, {nil}} {
		if _, err := NewOutboxWorker(nil, OutboxConfig{}, nil, consumers...); !errors.Is(err, ErrInvalidInput) {
			t.Fatalf("NewOutboxWorker(%v) = %v, want ErrInvalidInput", consumers, err)
		}
	}
}

func TestOutboxDeliverReportsEachConsumer(t *testing.T) {
	w := &OutboxWorker{consumers: []OutboxConsumer{
		namedConsumer{name: "panicking"},
		namedConsumer{name: "broken", err: errors.New("index down")},
	}}
	failed, msg := w.deliver(context.Background(), &OutboxEvent{})
	if len(failed) != 2 || failed[0] != "panicking" || failed[1] != "broken" || msg != "panicking: panic: handled; broken: index down" {
		t.Fatalf("deliver = %v %q", failed, msg)
	}
}
