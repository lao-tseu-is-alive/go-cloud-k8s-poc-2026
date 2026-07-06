#!/bin/bash
#
# install_go_protobuf_tools.sh
# Install the local code-generation toolchain used by `make generate`
# (scripts/buf_generate.sh): buf plus the two local protoc plugins.
# The OpenAPI plugin is pulled remotely by buf, so it does not need installing.
#
# Run once per machine (or after a Go upgrade). Binaries land in $(go env GOBIN)
# or $(go env GOPATH)/bin — make sure that directory is on your PATH.
set -euo pipefail

echo "## installing buf ..."
go install github.com/bufbuild/buf/cmd/buf@latest

echo "## installing protoc-gen-go ..."
go install google.golang.org/protobuf/cmd/protoc-gen-go@latest

echo "## installing protoc-gen-connect-go ..."
go install connectrpc.com/connect/cmd/protoc-gen-connect-go@latest

echo "## done. Verify with:"
echo "   buf --version && which protoc-gen-go protoc-gen-connect-go"
