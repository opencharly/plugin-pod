package pod

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// unitFixture writes n dummy unit files into a temp dir and returns their paths.
func unitFixture(t *testing.T, names ...string) (string, []string) {
	t.Helper()
	dir := t.TempDir()
	paths := make([]string, 0, len(names))
	for _, n := range names {
		p := filepath.Join(dir, n)
		if err := os.WriteFile(p, []byte("[Unit]\n"), 0o644); err != nil {
			t.Fatalf("writing fixture %s: %v", p, err)
		}
		paths = append(paths, p)
	}
	return dir, paths
}

// TestRemoveUnitFiles_KeepDeployKeepsEveryUnit — the whole point of plugin-pod#18: with --keep-deploy
// the quadlet, pod AND sidecar units must survive, or `charly start` cannot resume the deployment and
// the only recovery allocates a new random port. This fails if the keep branch stops keeping, or if a
// unit list ever stops including the sidecars.
func TestRemoveUnitFiles_KeepDeployKeepsEveryUnit(t *testing.T) {
	_, paths := unitFixture(t,
		"charly-strata-sc117.container",    // quadlet
		"charly-strata-sc117.pod",          // pod unit
		"charly-strata-sc117-ts.container", // sidecar
	)
	var out bytes.Buffer
	if err := removeUnitFiles(true, paths, &out); err != nil {
		t.Fatalf("keep branch returned an error: %v", err)
	}
	for _, p := range paths {
		if _, err := os.Stat(p); err != nil {
			t.Errorf("--keep-deploy must leave %s in place: %v", filepath.Base(p), err)
		}
	}
	if got := out.String(); !strings.Contains(got, "Kept") || strings.Contains(got, "Removed") {
		t.Errorf("keep branch must report keeping only; got: %q", got)
	}
}

// TestRemoveUnitFiles_DefaultRemovesEveryUnit — without the flag nothing changes: every unit goes.
func TestRemoveUnitFiles_DefaultRemovesEveryUnit(t *testing.T) {
	_, paths := unitFixture(t,
		"charly-strata-sc117.container",
		"charly-strata-sc117.pod",
		"charly-strata-sc117-ts.container",
	)
	var out bytes.Buffer
	if err := removeUnitFiles(false, paths, &out); err != nil {
		t.Fatalf("default branch returned an error: %v", err)
	}
	for _, p := range paths {
		if _, err := os.Stat(p); err == nil {
			t.Errorf("the default path must remove %s", filepath.Base(p))
		}
	}
	if got := out.String(); !strings.Contains(got, "Removed") || strings.Contains(got, "Kept") {
		t.Errorf("default branch must report removals only; got: %q", got)
	}
}

// TestRemoveUnitFiles_MissingUnitIsNotAnError — a unit systemd already reaped must not fail the remove.
func TestRemoveUnitFiles_MissingUnitIsNotAnError(t *testing.T) {
	dir := t.TempDir()
	var out bytes.Buffer
	if err := removeUnitFiles(false, []string{filepath.Join(dir, "absent.container")}, &out); err != nil {
		t.Errorf("a missing unit must be tolerated, got: %v", err)
	}
}

// TestDeployUnitPaths_IncludesQuadletPodAndEverySidecar — the SET is the contract. Driving
// removeUnitFiles with a caller-supplied list cannot catch a dropped sidecar loop; this can.
func TestDeployUnitPaths_IncludesQuadletPodAndEverySidecar(t *testing.T) {
	got := deployUnitPaths("/q", "strata", "sc117", "charly-strata-sc117", []string{"ts", "net"})

	want := []string{
		"/q/charly-strata-sc117.container",
		"/q/charly-strata-sc117.pod",
		"/q/charly-strata-sc117-ts.container",
		"/q/charly-strata-sc117-net.container",
	}
	if len(got) != len(want) {
		t.Fatalf("deployUnitPaths returned %d paths, want %d: %v", len(got), len(want), got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("path %d = %q, want %q", i, got[i], want[i])
		}
	}
}

// TestDeployUnitPaths_NoSidecarsYieldsTwo — a sidecar-less deploy owns exactly the two units.
func TestDeployUnitPaths_NoSidecarsYieldsTwo(t *testing.T) {
	if got := deployUnitPaths("/q", "strata", "sc117", "charly-strata-sc117", nil); len(got) != 2 {
		t.Errorf("a sidecar-less deploy must yield 2 paths, got %d: %v", len(got), got)
	}
}
