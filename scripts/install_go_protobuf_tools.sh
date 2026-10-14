#!/bin/bash
#
# install_go_protobuf_tools.sh
# Install buf, the code-generation driver of `make generate`
# (scripts/buf_generate.sh), at the version CI uses. The two Go protoc plugins
# run as go.mod tools (`go tool protoc-gen-go`, GLD-008) and the OpenAPI plugin
# is pulled remotely by buf at a pinned version, so neither needs installing.
#
# Run once per machine (or after a Go upgrade). Binaries land in $(go env GOBIN)
# or $(go env GOPATH)/bin — make sure that directory is on your PATH.
set -euo pipefail

echo "## installing buf ..."
go install github.com/bufbuild/buf/cmd/buf@v1.70.0

echo "## done. Verify with:"
echo "   buf --version && go tool protoc-gen-go --version"
