package pod

import (
	"errors"
	"reflect"
	"testing"

	"github.com/opencharly/sdk/deploykit"
)

// TestOwnedDeployVolumes_ExcludesSiblingInstances is the regression guard for the
// teardown/listing over-match: podman `volume ls --filter name=` is a SUBSTRING
// match and a base deploy's prefix (charly-<base>-) is a prefix of every sibling
// instance's volume name (charly-<base>-<instance>-<vol>), so scoping by prefix
// alone made `charly remove --purge githubrunner` (and `charly volume list
// githubrunner`) touch a live instance's volumes — silent data loss. The test
// pins the exact field names.
func TestOwnedDeployVolumes_ExcludesSiblingInstances(t *testing.T) {
	dc := &deploykit.DeployConfig{Deploy: map[string]deploykit.DeployNode{
		"githubrunner":      {Image: "githubrunner"},
		"githubrunner/no-1": {Image: "githubrunner"},
		"githubrunner/no-2": {Image: "githubrunner"},
		"githubrunner/no-3": {Image: "githubrunner"},
		"githubrunner/no-4": {Image: "githubrunner"},
	}}
	swapLoadPodDeployConfig(t, dc)

	// The engine lists BOTH the base volumes and the instances' volumes — exactly
	// what `volume ls --filter name=charly-githubrunner-` returns.
	listed := []string{
		"charly-githubrunner-state",
		"charly-githubrunner-storage",
		"charly-githubrunner-no-1-state",
		"charly-githubrunner-no-1-storage",
		"charly-githubrunner-no-2-state",
		"charly-githubrunner-no-2-storage",
		"charly-githubrunner-no-3-state",
		"charly-githubrunner-no-4-storage",
	}

	got := ownedDeployVolumes(listed, "githubrunner", "")
	want := []string{"charly-githubrunner-state", "charly-githubrunner-storage"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("ownedDeployVolumes(base) = %v, want ONLY the base's own volumes %v", got, want)
	}

	// An instance claims only ITS OWN volumes, not the base's or a sibling's.
	got = ownedDeployVolumes(listed, "githubrunner", "no-2")
	want = []string{"charly-githubrunner-no-2-state", "charly-githubrunner-no-2-storage"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("ownedDeployVolumes(no-2) = %v, want %v", got, want)
	}
}

// TestOwnedDeployVolumes_ReadFailureFallsBackToPrefix pins the orphaned-deploy
// contract: when the deploy config cannot be read, no siblings are known and the
// deploy's own prefix is the only signal (matching the pre-existing best-effort
// teardown).
func TestOwnedDeployVolumes_ReadFailureFallsBackToPrefix(t *testing.T) {
	orig := loadPodDeployConfig
	t.Cleanup(func() { loadPodDeployConfig = orig })
	loadPodDeployConfig = func() (*deploykit.DeployConfig, error) {
		return nil, errors.New("config unreadable")
	}

	got := ownedDeployVolumes([]string{"charly-app-data", "charly-other-data"}, "app", "")
	if !reflect.DeepEqual(got, []string{"charly-app-data"}) {
		t.Errorf("ownedDeployVolumes with unreadable config = %v, want only charly-app-data", got)
	}
}
