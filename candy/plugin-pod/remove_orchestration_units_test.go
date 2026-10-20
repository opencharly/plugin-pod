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

// TestRemoveSidecarConfigs_KeepDeployLeavesThemOnDisk — the sidecar CONFIG files are the layer after the
// sidecar UNITS: `<podBase>-<sidecar>-<purpose>.<ext>` in the sidecar config dir. Keeping the entry and
// the units but deleting these still leaves a deployment `start` cannot fully reproduce, so the keep
// branch must touch nothing. Fails if the guard is removed.
func TestRemoveSidecarConfigs_KeepDeployLeavesThemOnDisk(t *testing.T) {
	dir := t.TempDir()
	keep := filepath.Join(dir, "charly-strata-sc117-ts-serve.json")
	other := filepath.Join(dir, "charly-other-ts-serve.json")
	for _, p := range []string{keep, other} {
		if err := os.WriteFile(p, []byte("{}"), 0o644); err != nil {
			t.Fatalf("fixture: %v", err)
		}
	}
	var out bytes.Buffer
	removeSidecarConfigs(true, dir, "charly-strata-sc117", []string{"ts"}, &out)

	if _, err := os.Stat(keep); err != nil {
		t.Errorf("--keep-deploy must leave %s on disk: %v", filepath.Base(keep), err)
	}
	if got := out.String(); strings.Contains(got, "Removed") {
		t.Errorf("the keep branch must not report removals; got: %q", got)
	}
}

// TestRemoveSidecarConfigs_DefaultRemovesOnlyThisDeploysFiles — without the flag they go, and the
// sidecar-name anchoring keeps a sibling deploy's file safe.
func TestRemoveSidecarConfigs_DefaultRemovesOnlyThisDeploysFiles(t *testing.T) {
	dir := t.TempDir()
	ours := filepath.Join(dir, "charly-strata-sc117-ts-serve.json")
	other := filepath.Join(dir, "charly-other-ts-serve.json")
	for _, p := range []string{ours, other} {
		if err := os.WriteFile(p, []byte("{}"), 0o644); err != nil {
			t.Fatalf("fixture: %v", err)
		}
	}
	var out bytes.Buffer
	removeSidecarConfigs(false, dir, "charly-strata-sc117", []string{"ts"}, &out)

	if _, err := os.Stat(ours); err == nil {
		t.Errorf("the default path must remove %s", filepath.Base(ours))
	}
	if _, err := os.Stat(other); err != nil {
		t.Errorf("another deploy's file must be untouched: %v", err)
	}
	if got := out.String(); !strings.Contains(got, "Removed") {
		t.Errorf("the default path must report the removal; got: %q", got)
	}
}

// TestRemoveSidecarConfigs_FailsOnARealRemovalError — the body claims a non-IsNotExist removal error
// fails the command instead of being swallowed. That claim is only true if this returns one, so the
// unreadable-directory case is pinned: a config file that cannot be removed must surface, not vanish.
func TestRemoveSidecarConfigs_FailsOnARealRemovalError(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "charly-strata-sc117-ts-serve.json")
	if err := os.WriteFile(p, []byte("{}"), 0o644); err != nil {
		t.Fatalf("fixture: %v", err)
	}
	// A read-only directory makes os.Remove fail with EACCES while still letting ReadDir work.
	if err := os.Chmod(dir, 0o500); err != nil {
		t.Fatalf("chmod: %v", err)
	}
	defer os.Chmod(dir, 0o700) //nolint:errcheck // best-effort cleanup

	var out bytes.Buffer
	err := removeSidecarConfigs(false, dir, "charly-strata-sc117", []string{"ts"}, &out)
	if err == nil {
		t.Fatal("a removal that cannot happen must return an error, not be swallowed")
	}
	if !strings.Contains(err.Error(), "removing sidecar config") {
		t.Errorf("the error must name what failed; got: %v", err)
	}
}

// TestRemoveUnitFiles_FailsOnARealRemovalError — the mirror of the sidecar-config test: the body claims
// BOTH paths surface a non-IsNotExist removal error, so both must be pinned. A read-only directory makes
// os.Remove fail with EACCES while Stat still works.
func TestRemoveUnitFiles_FailsOnARealRemovalError(t *testing.T) {
	dir, paths := unitFixture(t, "charly-strata-sc117.container")
	if err := os.Chmod(dir, 0o500); err != nil {
		t.Fatalf("chmod: %v", err)
	}
	defer os.Chmod(dir, 0o700) //nolint:errcheck // best-effort cleanup

	var out bytes.Buffer
	err := removeUnitFiles(false, paths, &out)
	if err == nil {
		t.Fatal("a unit that cannot be removed must return an error, not be swallowed")
	}
	if !strings.Contains(err.Error(), "removing") {
		t.Errorf("the error must name what failed; got: %v", err)
	}
}

// TestRemoveUnitFiles_KeepReportsOnlyExistingUnits — the mirror of the missing-unit case on the discard
// path: with --keep-deploy, a path that does not exist must NOT be announced as kept. Announcing it is
// the same lie as announcing a removal that never happened.
func TestRemoveUnitFiles_KeepReportsOnlyExistingUnits(t *testing.T) {
	dir, paths := unitFixture(t, "charly-strata-sc117.container")
	absent := filepath.Join(dir, "charly-strata-sc117.pod")

	var out bytes.Buffer
	if err := removeUnitFiles(true, append(paths, absent), &out); err != nil {
		t.Fatalf("keep branch returned an error: %v", err)
	}
	got := out.String()
	if !strings.Contains(got, "charly-strata-sc117.container") {
		t.Errorf("an existing unit must be reported as kept; got: %q", got)
	}
	if strings.Contains(got, "charly-strata-sc117.pod") {
		t.Errorf("a unit that does not exist must NOT be reported as kept; got: %q", got)
	}
}
