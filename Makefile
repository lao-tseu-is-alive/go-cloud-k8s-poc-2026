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
PACKAGES := $(shell go list ./... | grep -vE '/vendor/|/node_modules/')
COVER_PACKAGES := $(shell go list ./... | grep -vE '/vendor/|/node_modules/' | paste -sd,)
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
build: clean mod-download front-build test
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
## release:	build a clean repo and tag a version release
release: build
	@echo "  >  Preparing release $(APP_EXECUTABLE) v$(APP_VERSION) rev: $(APP_REVISION) ..."
ifeq ($(shell git status -s),)
	echo "OK : your repo is clean"
	@git fetch  ||  (echo "ERROR : git fetch failed" && exit 1)
	@git tag -l  "v${APP_VERSION}"  ||  (echo "ERROR : this git tag v${APP_VERSION} already exist" && exit 1)
	git tag "v${APP_VERSION}" -m "v${APP_VERSION} bump"
else
	(echo "ERROR : your local git repo is dirty" && ( git status -s) && exit 1)
endif

.PHONY: help
help: Makefile
	@echo
	@echo " Choose a make target from one of  :"
	@echo
	@sed -n 's/^##//p' $< | column -t -s ':' |  sed -e 's/^/ /'
	@echo
