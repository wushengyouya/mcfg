package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestReleaseScript_BuildsBinaryAndChecksum(t *testing.T) {
	outputDir := filepath.Join(t.TempDir(), "dist")
	cmd := exec.Command("bash", "scripts/build-release.sh", "v0.1.0-test", outputDir)
	cmd.Env = append(os.Environ(),
		"GOCACHE=/tmp/go-build",
		"GOMODCACHE=/tmp/gomodcache",
		"COMMIT=testcommit",
		"BUILD_DATE=2026-03-13T00:00:00Z",
	)
	cmd.Dir = "."

	output, err := cmd.CombinedOutput()
	require.NoError(t, err, string(output))
	require.Contains(t, string(output), "built ")

	binaryPath := filepath.Join(outputDir, "mcfg-"+runtime.GOOS+"-"+runtime.GOARCH)
	checksumPath := binaryPath + ".sha256"

	_, err = os.Stat(binaryPath)
	require.NoError(t, err)

	data, err := os.ReadFile(checksumPath)
	require.NoError(t, err)
	require.Contains(t, string(data), filepath.Base(binaryPath))

	versionCmd := exec.Command(binaryPath, "version")
	versionOutput, err := versionCmd.CombinedOutput()
	require.NoError(t, err, string(versionOutput))
	require.True(t, strings.Contains(string(versionOutput), "Version: v0.1.0-test"))
	require.True(t, strings.Contains(string(versionOutput), "Commit: testcommit"))
}
