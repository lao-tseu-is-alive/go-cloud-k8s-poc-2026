#!/usr/bin/env bash
#
# check_build_inputs.sh
# Immutable build inputs (GLD-059): refuse a container image or a third-party
# GitHub action referenced by a mutable tag, and a Go builder image older than
# the go.mod version.
#   - every FROM of the Dockerfile (except scratch and earlier stages) and every
#     `image:` of the workflows and k8s manifests carries @sha256:<digest>
#     (the project's own image, published by docker-publish, is exempt);
#   - every `uses:` outside GitHub's own actions (actions/*, github/*) is pinned
#     by a 40-character commit SHA (AGENTS.md, "CI actions");
#   - the golang builder tag has go.mod's major.minor and at least its patch
#     (the image sets GOTOOLCHAIN=local and refuses an older one).
# Dependabot (.github/dependabot.yml) proposes the digest and SHA updates.
set -euo pipefail

cd "$(git rev-parse --show-toplevel)"

own_image="ghcr.io/lao-tseu-is-alive/go-cloud-k8s-poc-2026"
digest_pattern='@sha256:[0-9a-f]{64}$'
failures=0

fail() {
    local message="$1"
    echo "build-inputs-check: ${message}" >&2
    failures=$((failures + 1))
}

# Dockerfile base images (stage names are lower-cased by Docker).
stages=()
while read -r image alias; do
    if [[ "${image}" == "scratch" ]] || [[ " ${stages[*]-} " == *" ${image,,} "* ]]; then
        :
    elif ! [[ "${image}" =~ ${digest_pattern} ]]; then
        fail "Dockerfile: base image ${image} is not pinned by digest"
    fi
    if [[ -n "${alias}" ]]; then
        stages+=("${alias,,}")
    fi
done < <(sed -nE 's/^FROM[[:space:]]+([^[:space:]]+)([[:space:]]+[Aa][Ss][[:space:]]+([^[:space:]]+))?.*/\1 \3/p' Dockerfile)

# Container images of the workflows and k8s manifests.
while IFS=: read -r file _ image; do
    image="$(printf '%s' "${image}" | sed -E 's/^[[:space:]]*image:[[:space:]]*//; s/[[:space:]]*(#.*)?$//; s/^["'\'']|["'\'']$//g')"
    if [[ "${image}" == "${own_image}:"* ]]; then
        continue
    fi
    if ! [[ "${image}" =~ ${digest_pattern} ]]; then
        fail "${file}: image ${image} is not pinned by digest"
    fi
done < <(grep -nE '^[[:space:]]*image:' .github/workflows/*.yml deployments/k8s/*.yaml || true)

# Third-party actions.
while IFS=: read -r file _ action; do
    action="$(printf '%s' "${action}" | sed -E 's/^[[:space:]]*(-[[:space:]]*)?uses:[[:space:]]*//; s/[[:space:]]*(#.*)?$//')"
    case "${action}" in
        actions/* | github/* | ./*) continue ;;
    esac
    if ! [[ "${action}" =~ @[0-9a-f]{40}$ ]]; then
        fail "${file}: action ${action} is not pinned by commit SHA"
    fi
done < <(grep -nE '^[[:space:]]*(-[[:space:]]*)?uses:' .github/workflows/*.yml || true)

# Go builder version against go.mod.
go_mod_version="$(sed -nE 's/^go ([0-9]+\.[0-9]+\.[0-9]+)$/\1/p' go.mod)"
builder_version="$(sed -nE 's/^FROM golang:([0-9]+\.[0-9]+\.[0-9]+)-.*/\1/p' Dockerfile)"
if [[ -z "${go_mod_version}" ]] || [[ -z "${builder_version}" ]]; then
    fail "cannot read the go.mod version (${go_mod_version:-none}) or the golang builder version (${builder_version:-none})"
elif [[ "${go_mod_version%.*}" != "${builder_version%.*}" ]]; then
    fail "golang builder ${builder_version} is not the go.mod minor ${go_mod_version%.*}"
elif [[ "$(printf '%s\n%s\n' "${go_mod_version}" "${builder_version}" | sort -V | head -n1)" != "${go_mod_version}" ]]; then
    fail "golang builder ${builder_version} is older than go.mod ${go_mod_version}"
fi

if [[ "${failures}" -gt 0 ]]; then
    exit 1
fi
echo "build-inputs-check: OK"
