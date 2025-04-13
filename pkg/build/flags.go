// SPDX-License-Identifier: MIT
//
// Package build provides functionality to manage and retrieve build information
// for a Go application. It allows embedding metadata such as the application
// name, build timestamp, Git commit hash, and semantic version into the binary
// at compile time using linker flags. This information can be useful for debugging,
// logging, and displaying version information to users.
package build

import "fmt"

type ldFlags struct {
	Name    string
	Time    string
	Commit  string
	Version string
}

// Package-level variables for build information. These are populated by -ldflags
// during compilation. Default values of "unknown" are used during development.
var (
	buildName    string
	buildTime    string
	buildCommit  string
	buildVersion string
	buildFlags   = &ldFlags{
		Name:    "unknown",
		Time:    "unknown",
		Commit:  "unknown",
		Version: "unknown",
	}
)

// Initialize validates and copies build information from ldflags variables
// into the buildFlags struct. This must be called early in program startup
// to ensure all build information is properly set. Returns an error if any
// required build flag is missing.
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

	buildFlags.Name = buildName
	buildFlags.Time = buildTime
	buildFlags.Commit = buildCommit
	buildFlags.Version = buildVersion

	return nil
}

// GetBuildFlags returns the current build information. Initialize()
// must be called before this function to ensure the build information
// is valid. This function is safe to call after initialization.
func GetBuildFlags() *ldFlags {
	return buildFlags
}
