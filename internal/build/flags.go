package build

import "fmt"

// ldFlags holds build-time information that is injected during compilation.
// The fields are populated via -ldflags during the build process, for example:
//
//	go build -ldflags "-X build.buildName=grec -X build.buildVersion=0.1.0"
//
// Required flags for production builds:
// - Name: Application name (e.g., "grec")
// - Time: Build timestamp (RFC3339 format)
// - Commit: Git commit hash
// - Version: Semantic version (e.g., "0.1.0")
// - Uuid: Unique build identifier
type ldFlags struct {
	Name    string // Application name
	Time    string // Build timestamp
	Commit  string // Git commit hash
	Version string // Semantic version
	Uuid    string // Unique build identifier
}

// Package-level variables for build information.
// These are populated by -ldflags during compilation.
// Default values of "unknown" are used during development.
var (
	buildName    string
	buildTime    string
	buildCommit  string
	buildVersion string
	buildUuid    string
	buildFlags   = &ldFlags{
		Name:    "unknown",
		Time:    "unknown",
		Commit:  "unknown",
		Version: "unknown",
		Uuid:    "unknown",
	}
)

// Initialize validates and copies build information from ldflags variables
// into the buildFlags struct. This must be called early in program startup
// to ensure all build information is properly set.
//
// Returns an error if any required build flag is missing.
func Initialize() error {
	if buildName == "" {
		return fmt.Errorf("BuildName is required")
	}
	if buildTime == "" {
		return fmt.Errorf("BuildTime is required")
	}
	if buildCommit == "" {
		return fmt.Errorf("BuildCommit is required")
	}
	if buildVersion == "" {
		return fmt.Errorf("BuildVersion is required")
	}
	if buildUuid == "" {
		return fmt.Errorf("BuildUuid is required")
	}

	buildFlags.Name = buildName
	buildFlags.Time = buildTime
	buildFlags.Commit = buildCommit
	buildFlags.Version = buildVersion
	buildFlags.Uuid = buildUuid

	return nil
}

// GetBuildFlags returns the current build information.
// Initialize() must be called before this function.
//
// Returns:
// - *ldFlags: Struct containing all build information
func GetBuildFlags() *ldFlags {
	return buildFlags
}
