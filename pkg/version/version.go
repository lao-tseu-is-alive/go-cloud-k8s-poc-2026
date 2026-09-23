// Package version provides the version metadata for go-cloud-k8s-poc-2026 (Goéland POC).
//
// Identity and release values are constants: Version is the single source of
// truth read by the Makefile, the release scripts and cmd/doccheck, so it must
// not be overridable at link time. Only the build provenance variables below
// are injected with -ldflags -X.
package version

const (
	// AppName is the CamelCase name of the application.
	AppName = "goelandPoc"

	// GoPackage is the name of the main service go package.
	GoPackage = "goeland"

	// ServiceName is the human-readable name of the main service.
	ServiceName = "Goeland"

	// DbSchemaName is the PostgreSQL schema / database name used by the POC.
	DbSchemaName = "goeland_poc_db"

	// AppNameKebab is the kebab-case name matching the GitHub repository.
	AppNameKebab = "go-cloud-k8s-poc-2026"

	// AppNameSnake is the snake-case name for databases or directories.
	AppNameSnake = "go_cloud_k8s_poc_2026"

	// Repository is the full Go module path.
	Repository = "github.com/lao-tseu-is-alive/go-cloud-k8s-poc-2026"

	// Version is the semantic version of the source tree, without the "v"
	// prefix. A release tag must equal "v" + Version.
	Version = "0.4.0"
)

var (
	// Revision is the git revision injected by the build (do not edit manually).
	Revision = "unknown"
	// BuildStamp is the UTC build time injected by the build (do not edit manually).
	BuildStamp = "unknown"
)
