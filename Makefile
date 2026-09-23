#!make
SHELL := /bin/bash
VER_SOURCE_CODE := pkg/version/version.go
APP_NAME := $(shell grep -E 'AppName\s+=' $(VER_SOURCE_CODE)| awk '{ print $$3 }'  | tr -d '"')
APP_VERSION := $(shell grep -E 'Version\s+=' $(VER_SOURCE_CODE)| awk '{ print $$3 }'  | tr -d '"')
APP_REPOSITORY := $(shell grep -E 'Repository\s+=' $(VER_SOURCE_CODE)| awk '{ print $$3 }'  | tr -d '"')
$(info  Found APP_NAME:'$(APP_NAME)', APP_VERSION:'$(APP_VERSION)', APP_REPOSITORY:'$(APP_REPOSITORY)',  in file: $(VER_SOURCE_CODE) )

ifneq ("$(wildcard .env)","")
	ENV_EXISTS := "TRUE"
	include .env
	export $(shell sed 's/=.*//' .env)
else
	$(info .env file was not found, using default values for undefined variables)
	ENV_EXISTS := "FALSE"
	DB_DRIVER ?= postgres
	DB_HOST ?= 127.0.0.1
	DB_PORT ?= 5432
	DB_NAME ?= goeland_poc_db
	DB_USER ?= goeland_poc_db
	DB_SSL_MODE ?= prefer
endif

APP_EXECUTABLE := goeland-server
FRONTEND_DIR := cmd/$(APP_EXECUTABLE)/goeland-front
MIGRATIONS_DIR := pkg/core/module/db/migrations
APP_REVISION := $(shell git describe --dirty --always 2>/dev/null || echo "unknown")
BUILD := $(shell date -u '+%Y-%m-%d_%I:%M:%S%p')
# Exclude vendored code and any Go packages that live inside the frontend's
# node_modules tree (e.g. flatted/golang) so `bun install` cannot pollute
# Go package discovery for test/vet/coverage.
# Recursive (=) on purpose: expanded when a recipe runs, i.e. after front-check
# or front-build produced dist/, so a clean checkout still lists cmd/goeland-server.
PACKAGES = $(shell go list ./... | grep -vE '/vendor/|/node_modules/')
COVER_PACKAGES = $(shell go list ./... | grep -vE '/vendor/|/node_modules/' | paste -sd,)
# Expected start of `goeland-server --version` for this build (see cmd/goeland-server/main.go).
version_prefix = go-cloud-k8s-poc-2026 v$(APP_VERSION) (revision $(APP_REVISION), built
LDFLAGS := -ldflags "-X ${APP_REPOSITORY}/pkg/version.Revision=${APP_REVISION} -X ${APP_REPOSITORY}/pkg/version.BuildStamp=${BUILD}"

MAKEFLAGS += --silent

.PHONY: run
## run:	generate, build the frontend, then run the goeland-server binary [DEFAULT RULE]
run: generate mod-download front-build
	go run $(LDFLAGS) ./cmd/$(APP_EXECUTABLE)

.PHONY: mod-download
mod-download:
	@echo "  >  Downloading go modules dependencies..."
	go mod download

.PHONY: front-build
## front-build:	install deps + build the embedded Vue/Vuetify frontend (dist/)
front-build:
	@echo "  >  Building embedded frontend with bun in $(FRONTEND_DIR) ..."
	cd $(FRONTEND_DIR) && bun install && bun run build

.PHONY: generate
## generate:	run buf lint + generate (protobuf, ConnectRPC, OpenAPI)
generate:
	./scripts/buf_generate.sh

.PHONY: build
## build:	build the frontend, run tests, then compile the server binary into bin/
build: clean mod-download front-build test binary

.PHONY: binary
## binary:	compile bin/goeland-server only (expects dist/ to exist)
binary:
	@echo "  >  Building your app binary inside bin directory..."
	CGO_ENABLED=0 go build ${LDFLAGS} -a -o bin/$(APP_EXECUTABLE) ./cmd/$(APP_EXECUTABLE)

.PHONY: test
## test:	run all Go tests with the race detector + coverage
test: mod-download
	@echo "  >  Running all tests..."
	go test -race -coverprofile coverage.out -coverpkg=$(COVER_PACKAGES) $(PACKAGES)

.PHONY: lint
## lint:	run go vet + buf lint
lint:
	go vet $(PACKAGES)
	buf lint

.PHONY: fmt
## fmt:	format all Go source files
fmt:
	gofmt -w .

.PHONY: fmt-check
## fmt-check:	fail if any Go file needs gofmt or any proto needs buf format
fmt-check:
	@unformatted="$$(gofmt -l $$(git ls-files --cached --others --exclude-standard '*.go'))"; \
		test -z "$$unformatted" || { echo "fmt-check: run gofmt -w on:"; echo "$$unformatted"; exit 1; }
	buf format -d --exit-code

.PHONY: front-check
## front-check:	frozen bun install + vue-tsc type-check + eslint + vite build (dist/)
front-check:
	@echo "  >  Checking embedded frontend in $(FRONTEND_DIR) ..."
	cd $(FRONTEND_DIR) && bun install --frozen-lockfile && bun run type-check && bun run lint && bun run build-only

# --- Documentation contract (docs/DOCUMENTATION.md) ---------------------------

.PHONY: godoc-check
## godoc-check:	require package + exported API GoDoc comments (cmd/doccheck)
godoc-check:
	go run ./cmd/doccheck --scope go

.PHONY: atlas-check
## atlas-check:	require docs/atlas.md to list every non-ignored file exactly once
atlas-check:
	go run ./cmd/doccheck --scope atlas

.PHONY: docs-assert
## docs-assert:	verify load-bearing prose claims against their sources
docs-assert:
	bash scripts/check_documentation_claims.sh

.PHONY: docs-check
## docs-check:	godoc-check + atlas-check + docs-assert
docs-check: godoc-check atlas-check docs-assert

# --- Quality and release gates ---------------------------------------------------

.PHONY: check
## check:	full local quality gate (frontend, format, lint, tests, documentation)
check: front-check fmt-check lint test docs-check
	git diff --check

.PHONY: version-check
## version-check:	README must announce the version declared in pkg/version/version.go
version-check:
	@test -n "$(APP_VERSION)" || { echo "version-check: cannot read $(VER_SOURCE_CODE)"; exit 1; }
	@grep -qF 'Current version: **v$(APP_VERSION)**' README.md || { echo "version-check: README does not announce v$(APP_VERSION)"; exit 1; }
	@echo "version-check: v$(APP_VERSION)"

.PHONY: changelog-check
## changelog-check:	CHANGELOG.md must hold one dated section for the current version
changelog-check:
	@duplicates="$$(sed -n 's/^## \[\([0-9][0-9.]*\)\].*/\1/p' CHANGELOG.md | sort | uniq -d)"; \
		test -z "$$duplicates" || { echo "changelog-check: duplicate versions: $$duplicates"; exit 1; }
	@grep -qE '^## \[$(APP_VERSION)\] - [0-9]{4}-[0-9]{2}-[0-9]{2}$$' CHANGELOG.md || { echo "changelog-check: no dated v$(APP_VERSION) section"; exit 1; }
	@echo "changelog-check: v$(APP_VERSION) documented"

.PHONY: scripts-check
## scripts-check:	bash syntax check of every helper script + checker self-tests
scripts-check:
	for script in scripts/*.sh; do bash -n "$$script" || exit 1; done
	bash scripts/check_release_traceability_test.sh

.PHONY: roadmap-check
## roadmap-check:	docs/ROADMAP.md tracks the current version, has unique GLD-NNN IDs and a next action
roadmap-check:
	@grep -qF 'Tracked version: **v$(APP_VERSION)**.' docs/ROADMAP.md || { echo "roadmap-check: tracked version does not match v$(APP_VERSION)"; exit 1; }
	@ids="$$(grep -oE '^- \[[ x~]\] \*\*GLD-[0-9]{3}' docs/ROADMAP.md | grep -oE 'GLD-[0-9]{3}')"; \
		test -n "$$ids" || { echo "roadmap-check: no task IDs found"; exit 1; }; \
		duplicates="$$(printf '%s\n' "$$ids" | sort | uniq -d)"; \
		test -z "$$duplicates" || { echo "roadmap-check: duplicate task IDs: $$duplicates"; exit 1; }
	@grep -q '^## Next action$$' docs/ROADMAP.md || { echo "roadmap-check: missing next-action section"; exit 1; }
	@echo "roadmap-check: OK"

.PHONY: release-traceability-check
## release-traceability-check:	done roadmap tasks <-> dated changelog sections, both ways
release-traceability-check:
	bash scripts/check_release_traceability.sh

.PHONY: release-check
## release-check:	check + version/changelog/roadmap/traceability + binary reporting the version (CI runs this)
release-check: check version-check changelog-check scripts-check roadmap-check release-traceability-check binary
	@./bin/$(APP_EXECUTABLE) --version | grep -qF '$(version_prefix)' || { echo "release-check: binary does not report $(version_prefix)"; exit 1; }
	@echo "release-check: v$(APP_VERSION) OK"

.PHONY: clean
## clean:	remove binaries and coverage files
clean:
	@echo "  >  Removing binaries and coverage..."
	rm -rf bin/$(APP_EXECUTABLE) coverage.out coverage-all.out

.PHONY: db-status
## db-status:	show dbmate migration status
db-status:
	dbmate --env-file .env --migrations-dir $(MIGRATIONS_DIR) status

.PHONY: db-up
## db-up:	apply pending dbmate migrations
db-up:
	dbmate --env-file .env --migrations-dir $(MIGRATIONS_DIR) --no-dump-schema up

.PHONY: db-down
## db-down:	roll back the latest dbmate migration
db-down:
	dbmate --env-file .env --migrations-dir $(MIGRATIONS_DIR) --no-dump-schema down

.PHONY: db-new
## db-new:	create a new dbmate migration (usage: make db-new name=add_case)
db-new:
	dbmate --migrations-dir $(MIGRATIONS_DIR) new $(name)

.PHONY: release
## release:	guarded tag + atomic push of v<Version> (CONFIRM_RELEASE=vX.Y.Z make release)
release:
	./scripts/02_tag_new_release_github.sh

.PHONY: help
help: Makefile
	@echo
	@echo " Choose a make target from one of  :"
	@echo
	@sed -n 's/^##//p' $< | column -t -s ':' |  sed -e 's/^/ /'
	@echo
