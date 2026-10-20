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
		"charly-strata-sc117.container",     // quadlet
		"charly-strata-sc117-pod.container", // pod unit
		"charly-strata-sc117-ts.container",  // sidecar
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
		"charly-strata-sc117-pod.container",
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
