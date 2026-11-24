package legacyimport

import (
	"log/slog"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
)

// testProgress logs to out without timestamps, on a clock the test moves.
func testProgress(out *strings.Builder, clock *time.Time) *progress {
	handler := slog.NewTextHandler(out, &slog.HandlerOptions{
		ReplaceAttr: func(_ []string, a slog.Attr) slog.Attr {
			if a.Key == slog.TimeKey {
				return slog.Attr{}
			}
			return a
		},
	})
	return &progress{log: slog.New(handler), begin: *clock, now: func() time.Time { return *clock }}
}

func TestProgressStep(t *testing.T) {
	var out strings.Builder
	clock := time.Date(2026, 11, 24, 9, 0, 0, 0, time.UTC)
	p := testProgress(&out, &clock)

	done := p.step("users", "stage", "2/30")
	clock = clock.Add(1500 * time.Millisecond)
	done("read", 3, "written", 2)

	want := "level=INFO msg=\"step started\" elapsed=0s step=users stage=2/30\n" +
		"level=INFO msg=\"step done\" elapsed=2s step=users duration=1.5s read=3 written=2\n"
	if got := out.String(); got != want {
		t.Fatalf("progress records:\n%s\nwant\n%s", got, want)
	}
}

func TestProgressCopyHeartbeat(t *testing.T) {
	var out strings.Builder
	clock := time.Date(2026, 11, 24, 9, 0, 0, 0, time.UTC)
	p := testProgress(&out, &clock)

	rows := make([][]any, heartbeatRows+1)
	src, done := p.copySource("thing", pgx.CopyFromRows(rows))
	var n int64
	for src.Next() {
		n++
		clock = clock.Add(time.Millisecond)
	}
	done(n)

	got := out.String()
	if strings.Count(got, `msg=copying`) != 1 {
		t.Fatalf("want one heartbeat for %d rows, got:\n%s", n, got)
	}
	if !strings.Contains(got, "msg=\"copy done\" elapsed=4m10s table=thing rows=250001 duration=4m10.001s rows_per_s=1000") {
		t.Fatalf("copy done record missing or wrong:\n%s", got)
	}
}

func TestProgressSilentWithoutLogger(t *testing.T) {
	p := newProgress(nil)
	p.step("step")("rows", 1)
	p.info("message") // must not panic
}
