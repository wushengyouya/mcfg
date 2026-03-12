package buildinfo_test

import (
	"runtime"
	"testing"

	"github.com/stretchr/testify/require"

	"mcfg/internal/buildinfo"
)

func TestCurrent_Defaults(t *testing.T) {
	originalVersion := buildinfo.Version
	originalCommit := buildinfo.Commit
	originalBuildDate := buildinfo.BuildDate
	t.Cleanup(func() {
		buildinfo.Version = originalVersion
		buildinfo.Commit = originalCommit
		buildinfo.BuildDate = originalBuildDate
	})

	buildinfo.Version = "dev"
	buildinfo.Commit = "unknown"
	buildinfo.BuildDate = "unknown"

	info := buildinfo.Current()
	require.Equal(t, "dev", info.Version)
	require.Equal(t, "unknown", info.Commit)
	require.Equal(t, "unknown", info.BuildDate)
	require.Equal(t, runtime.Version(), info.GoVersion)
	require.Equal(t, runtime.GOOS+"/"+runtime.GOARCH, info.Platform)
}

func TestCurrent_UsesInjectedValues(t *testing.T) {
	originalVersion := buildinfo.Version
	originalCommit := buildinfo.Commit
	originalBuildDate := buildinfo.BuildDate
	t.Cleanup(func() {
		buildinfo.Version = originalVersion
		buildinfo.Commit = originalCommit
		buildinfo.BuildDate = originalBuildDate
	})

	buildinfo.Version = "v0.1.0"
	buildinfo.Commit = "abc1234"
	buildinfo.BuildDate = "2026-03-13T00:00:00Z"

	info := buildinfo.Current()
	require.Equal(t, "v0.1.0", info.Version)
	require.Equal(t, "abc1234", info.Commit)
	require.Equal(t, "2026-03-13T00:00:00Z", info.BuildDate)
}
