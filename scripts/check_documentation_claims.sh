#!/usr/bin/env bash
#
# check_documentation_claims.sh
# Executable documentation claims (docs/DOCUMENTATION.md, "Executable
# documentation claims"). Each assertion links one stable operational or
# security fact across the surfaces that must agree: its authoritative source,
# the agent instructions and the operator-facing prose. The literals are
# intentionally exact; when a refactor changes one, verify the underlying
# contract still holds, then update source, prose and assertion together.
set -euo pipefail

cd "$(git rev-parse --show-toplevel)"

failed=0

require_literal() {
    local file="$1"
    local literal="$2"
    local claim="$3"

    if ! grep -Fq -- "$literal" "$file"; then
        echo "docs-assert: ${claim}: ${file} does not contain expected text: ${literal}" >&2
        failed=1
    fi
}

forbid_literal() {
    local file="$1"
    local literal="$2"
    local claim="$3"

    if grep -Fq -- "$literal" "$file"; then
        echo "docs-assert: ${claim}: ${file} still contains outdated text: ${literal}" >&2
        failed=1
    fi
}

# Upload and blob storage defaults are operator-facing and must agree between
# the server configuration and every document that states them.
require_literal cmd/goeland-server/config.go 'defaultMaxUploadBytes = 100 << 20 // 100 MiB' 'upload limit source'
require_literal AGENTS.md '`GOELAND_MAX_UPLOAD_BYTES` (default 100 MiB)' 'upload limit agent contract'
require_literal README.md '(default 100 MiB)' 'upload limit readme'
require_literal cmd/goeland-server/config.go 'defaultDocumentPath   = "./go_documents"' 'blob directory source'
require_literal AGENTS.md 'default `./go_documents`, gitignored' 'blob directory agent contract'
require_literal .gitignore 'go_documents/' 'blob directory is git-ignored'

# Authentication: jwt is the secure default and scopes are fixed identifiers.
require_literal cmd/goeland-server/config.go 'defaultAuthMode       = "jwt"' 'default auth mode source'
require_literal AGENTS.md '- `jwt` (default):' 'default auth mode agent contract'
require_literal pkg/core/authctx.go 'ScopeRead = "goeland:read"' 'read scope source'
require_literal pkg/core/authctx.go 'ScopeWrite = "goeland:write"' 'write scope source'
require_literal AGENTS.md 'Scopes: `goeland:read` (read RPCs), `goeland:write` (mutations).' 'scopes agent contract'

# The operator identity is derived server-side only; requests cannot forge it.
require_literal pkg/core/authctx.go 'func OperatorID(user *authadapter.AuthenticatedUser) string {' 'operator identity source'
require_literal AGENTS.md 'ALWAYS derived server-side via `core.OperatorID(user)` — never from the request' 'operator identity agent contract'

# Every migration range quoted in the docs ends at the latest embedded migration (GLD-058).
latest_migration="$(basename "$(find pkg/core/module/db/migrations -name '[0-9]*_*.sql' | sort | tail -n 1)")"
latest_version="${latest_migration%%_*}"
require_literal AGENTS.md "0001..${latest_version} (dbmate format)" 'migration range agent contract'
require_literal README.md "0001..${latest_version} (dbmate format)" 'migration range readme tree'
require_literal README.md "${latest_migration}" 'latest migration listed in the readme'
require_literal requirements/IMPLEMENTATION_STATUS.md "migrations \`0001–${latest_version}\` applied" 'migration range status snapshot'

# Document bytes are downloaded through the document (GLD-049), never a raw storage reference.
content_route='GET /api/documents/{id}/content'
require_literal cmd/goeland-server/server.go "mux.Handle(\"${content_route}\"," 'governed download route source'
require_literal AGENTS.md "${content_route}" 'governed download agent contract'
require_literal README.md "${content_route}" 'governed download readme'
require_literal docs/PRODUCTION_READINESS.md "${content_route}" 'governed download operator contract'
for doc in README.md AGENTS.md docs/PRODUCTION_READINESS.md requirements/IMPLEMENTATION_STATUS.md; do
    forbid_literal "${doc}" '`GET /download`' 'removed raw download endpoint'
done
forbid_literal docs/PRODUCTION_READINESS.md 'not filtered yet' 'searches and lists are filtered (GLD-049)'

# Migrations are serialized across replicas by one advisory lock key.
require_literal pkg/core/module/migrate.go 'const migrationLockKey = "go-cloud-k8s-poc-2026:migrations"' 'migration lock source'
require_literal AGENTS.md "under a PG advisory lock keyed to \`'go-cloud-k8s-poc-2026:migrations'\`" 'migration lock agent contract'

# Documentation governance must stay visible and run through one control chain.
require_literal AGENTS.md '`docs/DOCUMENTATION.md` is the normative documentation contract' 'agent documentation contract'
require_literal README.md '[documentation quality contract](docs/DOCUMENTATION.md)' 'contributor documentation contract'
require_literal Makefile 'docs-check: godoc-check atlas-check docs-assert' 'documentation gate composition'
require_literal Makefile 'check: front-check fmt-check lint cognitive-check test docs-check' 'quality gate includes documentation'
require_literal Makefile 'release-check: check vuln-check version-check changelog-check scripts-check roadmap-check release-traceability-check binary' 'release gate includes normal checks'
require_literal Makefile './scripts/02_tag_new_release_github.sh' 'make release uses the guarded script'
require_literal scripts/02_tag_new_release_github.sh 'git push --atomic origin main' 'release pushes main and tag atomically'
require_literal .github/workflows/ci.yml 'run: make release-check' 'CI release-equivalent gate'
for workflow in release docker-publish; do
    require_literal ".github/workflows/${workflow}.yml" 'scripts/check_release_tag.sh' "${workflow} verifies tag = version"
    require_literal ".github/workflows/${workflow}.yml" 'run: make release-check' "${workflow} runs the release gate"
done
require_literal buf.yaml '    - COMMENTS' 'protobuf COMMENTS lint enabled'

if [[ "${failed}" -ne 0 ]]; then
    exit 1
fi
echo "docs-assert: OK"
