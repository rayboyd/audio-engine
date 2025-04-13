// SPDX-License-Identifier: MIT
package build

import (
	"os"
	"testing"
)

var (
	origName    string
	origTime    string
	origCommit  string
	origVersion string
	origFlags   ldFlags
)

func TestMain(m *testing.M) {
	origName = buildName
	origTime = buildTime
	origCommit = buildCommit
	origVersion = buildVersion
	if buildFlags != nil {
		origFlags = *buildFlags
	}

	exitCode := m.Run()

	buildName = origName
	buildTime = origTime
	buildCommit = origCommit
	buildVersion = origVersion
	if buildFlags != nil {
		*buildFlags = origFlags
	}

	os.Exit(exitCode)
}

func TestInitialize(t *testing.T) {
	tests := []struct {
		name        string
		buildName   string
		buildTime   string
		buildCommit string
		buildVer    string
		wantErrMsg  string
	}{
		{
			"Missing BuildName",
			"",
			"2025-04-13",
			"abcdef123",
			"v1.0.0",
			"BuildName is required",
		},
		{
			"Missing BuildTime",
			"testapp",
			"",
			"abcdef123",
			"v1.0.0",
			"BuildTime is required",
		},
		{
			"Missing BuildCommit",
			"testapp",
			"2025-04-13",
			"",
			"v1.0.0",
			"BuildCommit is required",
		},
		{
			"Missing BuildVersion",
			"testapp",
			"2025-04-13",
			"abcdef123",
			"",
			"BuildVersion is required",
		},
		{
			"Success Case",
			"testapp",
			"2025-04-13",
			"abcdef123",
			"v1.0.0",
			"",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			buildFlags = &ldFlags{
				Name:    "unknown",
				Time:    "unknown",
				Commit:  "unknown",
				Version: "unknown",
			}

			buildName = tt.buildName
			buildTime = tt.buildTime
			buildCommit = tt.buildCommit
			buildVersion = tt.buildVer

			err := Initialize()

			if tt.wantErrMsg != "" {
				if err == nil {
					t.Errorf("Initialize() expected error, got nil")
					return
				}
				if err.Error() != tt.wantErrMsg {
					t.Errorf("Initialize() error = %v, want %v", err, tt.wantErrMsg)
					return
				}
				return
			}

			if err != nil {
				t.Errorf("Initialize() unexpected error: %v", err)
				return
			}

			if buildFlags.Name != tt.buildName {
				t.Errorf("buildFlags.Name = %v, want %v", buildFlags.Name, tt.buildName)
			}
			if buildFlags.Time != tt.buildTime {
				t.Errorf("buildFlags.Time = %v, want %v", buildFlags.Time, tt.buildTime)
			}
			if buildFlags.Commit != tt.buildCommit {
				t.Errorf("buildFlags.Commit = %v, want %v", buildFlags.Commit, tt.buildCommit)
			}
			if buildFlags.Version != tt.buildVer {
				t.Errorf("buildFlags.Version = %v, want %v", buildFlags.Version, tt.buildVer)
			}
		})
	}
}

func TestGetBuildFlags(t *testing.T) {
	expected := ldFlags{
		Name:    "testapp",
		Time:    "2025-04-13",
		Commit:  "abcdef123",
		Version: "v1.0.0",
	}
	buildFlags = &expected

	flags := GetBuildFlags()

	if flags.Name != expected.Name ||
		flags.Time != expected.Time ||
		flags.Commit != expected.Commit ||
		flags.Version != expected.Version {
		t.Errorf("GetBuildFlags() = %+v, want %+v", flags, expected)
	}
}
