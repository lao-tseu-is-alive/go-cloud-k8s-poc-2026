package integration

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"math/rand/v2"
	"sync/atomic"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/lao-tseu-is-alive/go-cloud-k8s-poc-2026/pkg/actor"
	"github.com/lao-tseu-is-alive/go-cloud-k8s-poc-2026/pkg/casefile"
	"github.com/lao-tseu-is-alive/go-cloud-k8s-poc-2026/pkg/core/coretest"
	"github.com/lao-tseu-is-alive/go-cloud-k8s-poc-2026/pkg/document"
	"github.com/lao-tseu-is-alive/go-cloud-k8s-poc-2026/pkg/orgunit"
	"github.com/lao-tseu-is-alive/go-cloud-k8s-poc-2026/pkg/thing"
)

// queryCounter is a pgx tracer counting the queries a pool runs.
type queryCounter struct{ n atomic.Int64 }

// TraceQueryStart counts one query.
func (c *queryCounter) TraceQueryStart(ctx context.Context, _ *pgx.Conn, _ pgx.TraceQueryStartData) context.Context {
	c.n.Add(1)
	return ctx
}

// TraceQueryEnd does nothing.
func (c *queryCounter) TraceQueryEnd(context.Context, *pgx.Conn, pgx.TraceQueryEndData) {}

// countingPool opens a second pool on the test database whose queries are counted.
func countingPool(t *testing.T, env *testEnv) (*pgxpool.Pool, *queryCounter) {
	t.Helper()
	cfg, err := pgxpool.ParseConfig(coretest.TestDatabaseURL(t, "DB integration tests"))
	if err != nil {
		t.Fatalf("parse dsn: %v", err)
	}
	counter := &queryCounter{}
	cfg.ConnConfig.Tracer = counter
	pool, err := pgxpool.NewWithConfig(env.ctx, cfg)
	if err != nil {
		t.Fatalf("open counting pool: %v", err)
	}
	t.Cleanup(pool.Close)
	return pool, counter
}

// assertQueries runs search on a fresh count and checks it stays within max
// queries whatever the page size.
func assertQueries(t *testing.T, counter *queryCounter, label string, maxQueries int64, search func() int) {
	t.Helper()
	counter.n.Store(0)
	rows := search()
	if got := counter.n.Load(); got > maxQueries {
		t.Fatalf("%s: %d rows cost %d queries, want at most %d (no per-row hydration)", label, rows, got, maxQueries)
	}
}

// TestSearchHydratesInBatches checks that the case, actor, document, thing and
// org unit searches load a page's related rows in a fixed number of queries
// (GLD-042), with every result fully hydrated.
func TestSearchHydratesInBatches(t *testing.T) {
	env := newTestEnv(t)
	pool, counter := countingPool(t, env)
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	const pageRows = 6
	token := uniqueToken()

	for i := range pageRows {
		openCase(t, env, fmt.Sprintf("Lot %s %d", token, i))
	}
	caseRepo, _ := casefile.NewPostgresRepository(pool, log)
	assertQueries(t, counter, "cases", 4, func() int {
		res, err := caseRepo.Search(env.ctx, casefile.SearchFilter{Query: token, Limit: 50})
		if err != nil || len(res.Cases) != pageRows {
			t.Fatalf("search cases: %d (%v)", len(res.Cases), err)
		}
		for _, c := range res.Cases {
			if c.Subject == nil || c.RecordMetadata == nil || c.Type == nil {
				t.Fatalf("case not hydrated: %+v", c)
			}
		}
		return len(res.Cases)
	})

	assertActorBatch(t, env, pool, counter, token, pageRows)
	assertDocumentBatch(t, env, pool, counter, token, pageRows)
	assertThingBatch(t, env, pool, counter, token, pageRows)

	for i := range pageRows {
		newUnit(t, env, "SERVICE", fmt.Sprintf("Lot %s %d", token, i), nil)
	}
	unitRepo, _ := orgunit.NewPostgresRepository(pool, log)
	assertQueries(t, counter, "org units", 4, func() int {
		res, err := unitRepo.Search(env.ctx, orgunit.SearchFilter{Query: token, Limit: 50})
		if err != nil || len(res.Units) != pageRows || res.Units[0].Type == nil || res.Units[0].Subject == nil {
			t.Fatalf("search org units: %+v (%v)", res.Units, err)
		}
		return len(res.Units)
	})
}

// assertActorBatch covers organizations with a category and a contact each.
func assertActorBatch(t *testing.T, env *testEnv, pool *pgxpool.Pool, counter *queryCounter, token string, rows int) {
	t.Helper()
	for i := range rows {
		_, _, err := env.actorSvc.Create(env.ctx, actor.CreateInput{
			ActorKind: actor.KindOrganization, DisplayName: fmt.Sprintf("Lot %s %d", token, i), LegalName: "Lot SA", OperatorID: testOperator,
			CategoryCode: "COMMERCE",
			Contacts:     []actor.ContactInput{{ContactType: actor.ContactTypeEmail, Value: fmt.Sprintf("lot%d@example.ch", i)}},
		})
		if err != nil {
			t.Fatalf("create actor: %v", err)
		}
	}
	repo, _ := actor.NewPostgresRepository(pool, slog.New(slog.NewTextHandler(io.Discard, nil)))
	assertQueries(t, counter, "actors", 6, func() int {
		res, err := repo.Search(env.ctx, actor.SearchFilter{Query: token, Limit: 50})
		if err != nil || len(res.Actors) != rows {
			t.Fatalf("search actors: %d (%v)", len(res.Actors), err)
		}
		for _, a := range res.Actors {
			if a.Subject == nil || a.RecordMetadata == nil || a.Category == nil || len(a.Contacts) != 1 {
				t.Fatalf("actor not hydrated: %+v", a)
			}
		}
		return len(res.Actors)
	})
}

// assertDocumentBatch covers documents with a current version and its content.
func assertDocumentBatch(t *testing.T, env *testEnv, pool *pgxpool.Pool, counter *queryCounter, token string, rows int) {
	t.Helper()
	for i := range rows {
		blob := ingest(t, env, fmt.Sprintf("lot %s %d", token, i)).Blob
		if _, err := env.docSvc.Create(env.ctx, document.CreateInput{
			DocumentTypeCode: "PLAN", Title: fmt.Sprintf("Lot %s %d", token, i), ContentBlobID: &blob.ID, OperatorID: testOperator,
		}); err != nil {
			t.Fatalf("create document: %v", err)
		}
	}
	repo, _ := document.NewPostgresRepository(pool, slog.New(slog.NewTextHandler(io.Discard, nil)))
	assertQueries(t, counter, "documents", 6, func() int {
		res, err := repo.Search(env.ctx, document.SearchFilter{Query: token, Limit: 50})
		if err != nil || len(res.Documents) != rows {
			t.Fatalf("search documents: %d (%v)", len(res.Documents), err)
		}
		for _, d := range res.Documents {
			if d.Subject == nil || d.Type == nil || d.CurrentVersion == nil || d.CurrentVersion.Content == nil {
				t.Fatalf("document not hydrated: %+v", d)
			}
		}
		return len(res.Documents)
	})
}

// assertThingBatch covers parcels with their register details.
func assertThingBatch(t *testing.T, env *testEnv, pool *pgxpool.Pool, counter *queryCounter, token string, rows int) {
	t.Helper()
	e0 := 2_530_000 + float64(rand.IntN(20_000))*10
	for i := range rows {
		if _, _, err := env.thingSvc.Create(env.ctx, thing.CreateInput{
			TypeCode: "PARCEL", GeometryGeoJSON: square(e0+float64(i)*200, 1_160_000, 100), OperatorID: testOperator,
			Parcel: &thing.Parcel{CommuneOFS: 5586, ParcelNumber: fmt.Sprintf("Lot-%s-%d", token, i)},
		}); err != nil {
			t.Fatalf("create parcel: %v", err)
		}
	}
	repo, _ := thing.NewPostgresRepository(pool, slog.New(slog.NewTextHandler(io.Discard, nil)))
	assertQueries(t, counter, "things", 6, func() int {
		res, err := repo.Search(env.ctx, thing.SearchFilter{Query: token, Limit: 50})
		if err != nil || len(res.Things) != rows {
			t.Fatalf("search things: %d (%v)", len(res.Things), err)
		}
		for _, th := range res.Things {
			if th.Subject == nil || th.Type == nil || th.Parcel == nil {
				t.Fatalf("thing not hydrated: %+v", th)
			}
		}
		return len(res.Things)
	})
}
