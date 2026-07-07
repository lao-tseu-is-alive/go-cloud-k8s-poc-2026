// Package version provides the version metadata for go-cloud-k8s-poc-2026 (Goéland POC).
package version

var (
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

	// Version is the semantic version starting point.
	Version = "0.1.0"

	// Revision is auto-filled by the build (do not edit manually).
	Revision = "unknown"
	// BuildStamp is auto-filled by the build (do not edit manually).
	BuildStamp = "unknown"
)
