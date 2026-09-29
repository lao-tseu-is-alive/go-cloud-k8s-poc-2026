package integration

import (
	"context"
	"io"
	"log/slog"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/lao-tseu-is-alive/go-cloud-k8s-poc-2026/pkg/actor"
	"github.com/lao-tseu-is-alive/go-cloud-k8s-poc-2026/pkg/blobstore/filestore"
	"github.com/lao-tseu-is-alive/go-cloud-k8s-poc-2026/pkg/casefile"
	"github.com/lao-tseu-is-alive/go-cloud-k8s-poc-2026/pkg/core"
	coremodule "github.com/lao-tseu-is-alive/go-cloud-k8s-poc-2026/pkg/core/module"
	"github.com/lao-tseu-is-alive/go-cloud-k8s-poc-2026/pkg/document"
	"github.com/lao-tseu-is-alive/go-cloud-k8s-poc-2026/pkg/orgunit"
	"github.com/lao-tseu-is-alive/go-cloud-k8s-poc-2026/pkg/task"
	"github.com/lao-tseu-is-alive/go-cloud-k8s-poc-2026/pkg/thing"
	"github.com/lao-tseu-is-alive/go-cloud-k8s-poc-2026/pkg/timeline"
)

// testDatabaseURLEnv names the DSN env var that enables the DB integration tests.
const testDatabaseURLEnv = "GOELAND_TEST_DATABASE_URL"

// testEnv bundles the wired-up services and pool for one integration test.
type testEnv struct {
	ctx      context.Context
	pool     *pgxpool.Pool
	coreRepo *core.PostgresRepository
	coreSvc  *core.Service
	docSvc   *document.Service
	actorSvc *actor.Service
	caseSvc  *casefile.Service
	thingSvc *thing.Service
	// orgUnitSvc is the organizational unit service.
	orgUnitSvc *orgunit.Service
	// taskSvc is the case task service.
	taskSvc *task.Service
	// timelineSvc is the case timeline service.
	timelineSvc *timeline.Service
	// blobDir is the per-test directory holding uploaded bytes.
	blobDir string
}

// newTestEnv connects to the test database named by GOELAND_TEST_DATABASE_URL,
// applies all embedded migrations, and wires the core + document services exactly
// as the server does. It skips the test when the env var is unset so the default
// `go test ./...` run needs no database.
//
// The DSN must point at a PostgreSQL with the PostGIS, pgcrypto, pg_trgm and
// unaccent extensions available (a plain postgres image without PostGIS will fail
// migration 0001). Example:
//
//	GOELAND_TEST_DATABASE_URL='postgres://postgres:postgres@127.0.0.1:5432/goeland_test?sslmode=disable' \
//	    go test ./pkg/integration/...
func newTestEnv(t *testing.T) *testEnv {
	t.Helper()

	dsn := strings.TrimSpace(os.Getenv(testDatabaseURLEnv))
	if dsn == "" {
		t.Skipf("set %s to run DB integration tests (needs PostGIS, pgcrypto, pg_trgm, unaccent)", testDatabaseURLEnv)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	t.Cleanup(cancel)

	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Fatalf("connect to test database: %v", err)
	}
	t.Cleanup(pool.Close)

	if err := pool.Ping(ctx); err != nil {
		t.Fatalf("ping test database: %v", err)
	}

	// Migrations are idempotent (bookkeeping in schema_migrations under an advisory
	// lock), so it is safe to apply them at the start of every integration test.
	if err := coremodule.Migrate(ctx, pool); err != nil {
		t.Fatalf("apply migrations: %v", err)
	}

	log := slog.New(slog.NewTextHandler(io.Discard, nil))

	coreRepo, err := core.NewPostgresRepository(pool, log)
	if err != nil {
		t.Fatalf("build core repository: %v", err)
	}
	coreSvc, err := core.NewService(coreRepo, log)
	if err != nil {
		t.Fatalf("build core service: %v", err)
	}
	docRepo, err := document.NewPostgresRepository(pool, log)
	if err != nil {
		t.Fatalf("build document repository: %v", err)
	}
	store, err := filestore.New(t.TempDir())
	if err != nil {
		t.Fatalf("build blob store: %v", err)
	}
	docSvc, err := document.NewService(docRepo, coreSvc, store, log)
	if err != nil {
		t.Fatalf("build document service: %v", err)
	}
	actorRepo, err := actor.NewPostgresRepository(pool, log)
	if err != nil {
		t.Fatalf("build actor repository: %v", err)
	}
	actorSvc, err := actor.NewService(actorRepo, coreSvc, log)
	if err != nil {
		t.Fatalf("build actor service: %v", err)
	}

	caseRepo, err := casefile.NewPostgresRepository(pool, log)
	if err != nil {
		t.Fatalf("build case repository: %v", err)
	}
	caseSvc, err := casefile.NewService(caseRepo, coreSvc, log)
	if err != nil {
		t.Fatalf("build case service: %v", err)
	}

	thingRepo, err := thing.NewPostgresRepository(pool, log)
	if err != nil {
		t.Fatalf("build thing repository: %v", err)
	}
	thingSvc, err := thing.NewService(thingRepo, coreSvc, log)
	if err != nil {
		t.Fatalf("build thing service: %v", err)
	}

	orgUnitRepo, err := orgunit.NewPostgresRepository(pool, log)
	if err != nil {
		t.Fatalf("build org unit repository: %v", err)
	}
	orgUnitSvc, err := orgunit.NewService(orgUnitRepo, coreSvc, log)
	if err != nil {
		t.Fatalf("build org unit service: %v", err)
	}

	timelineRepo, err := timeline.NewPostgresRepository(pool, log)
	if err != nil {
		t.Fatalf("build timeline repository: %v", err)
	}
	timelineSvc, err := timeline.NewService(timelineRepo, log)
	if err != nil {
		t.Fatalf("build timeline service: %v", err)
	}

	taskRepo, err := task.NewPostgresRepository(pool, log)
	if err != nil {
		t.Fatalf("build task repository: %v", err)
	}
	taskSvc, err := task.NewService(taskRepo, log)
	if err != nil {
		t.Fatalf("build task service: %v", err)
	}

	return &testEnv{ctx: ctx, pool: pool, coreRepo: coreRepo, coreSvc: coreSvc, thingSvc: thingSvc, docSvc: docSvc, actorSvc: actorSvc, caseSvc: caseSvc, orgUnitSvc: orgUnitSvc, timelineSvc: timelineSvc, taskSvc: taskSvc, blobDir: store.Root()}
}

// uniqueToken returns a lowercase, hyphen-free token safe to embed in a title and
// feed to plainto_tsquery('simple', ...) as a single matchable search word.
func uniqueToken() string {
	return "z" + strings.ReplaceAll(uuid.NewString(), "-", "")[:12]
}
