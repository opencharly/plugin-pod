package pod

import (
	"encoding/json"
	"reflect"
	"testing"

	"github.com/opencharly/sdk/deploykit"
)

// TestPurgeDeployArtifacts_DropsOverlay proves `charly remove --purge` drops the synthesized
// <deploy-key>-overlay images (the leak fix). Every disposable pod check bed tears down via
// `charly remove --purge` (check_bed_run.go's default cleanup), so without this drop the add_candy:
// overlay images accumulate (dozens leaked before the fix). Stubs the engine-facing drop so no live
// container engine is needed; the removeVolumes/RemoveEncryptedVolumes side effects are best-effort
// no-ops against a fixture deploy name that matches nothing. Relocated from
// charly/purge_overlay_test.go (Cutover B unit 2 remove-verb completion) alongside
// purgeDeployArtifacts itself.
func TestPurgeDeployArtifacts_DropsOverlay(t *testing.T) {
	var called bool
	var gotEngine, gotRef string
	prev := dropOverlayImagesByRef
	dropOverlayImagesByRef = func(engine, ref string) { called, gotEngine, gotRef = true, engine, ref }
	t.Cleanup(func() { dropOverlayImagesByRef = prev })

	purgeDeployArtifacts("podman", "check-addcandy-pod-unit-fixture", "")
	if !called {
		t.Fatal("--purge must drop the <name>-overlay image (dropOverlayImagesByRef was never called)")
	}
	if gotEngine != "podman" || gotRef != "check-addcandy-pod-unit-fixture-overlay" {
		t.Fatalf("overlay drop targeted the wrong ref: engine=%q ref=%q "+
			"(want podman / check-addcandy-pod-unit-fixture-overlay)", gotEngine, gotRef)
	}

	// Instance form: the overlay ref carries the deploy KEY (base/instance), matching the
	// DeployName build_overlay stamps onto the overlay image.
	gotRef = ""
	purgeDeployArtifacts("podman", "selkies", "work")
	if gotRef != "selkies/work-overlay" {
		t.Fatalf("instance overlay ref = %q, want selkies/work-overlay", gotRef)
	}
}

// TestSidecarNamesFromDeployConfig covers the pure extraction logic resolveSidecarNames' seam call
// feeds into — enumerating EXACT sidecar names attached to a deploy so `charly remove`'s
// quadlet-mode sidecar sweep never over-matches a same-image sibling instance. Split out of
// resolveSidecarNames (Cutover B unit 2: that function now calls the loaderkit overlay read,
// which needs a live reverse channel a plain unit test doesn't have — see remove_orchestration.go's
// header) so this logic — the part charly/remove_sidecar_test.go actually exercised — stays
// unit-testable directly against a constructed *deploykit.DeployConfig, no YAML/loader/seam needed.
func TestSidecarNamesFromDeployConfig(t *testing.T) {
	raw := func(s string) json.RawMessage { return json.RawMessage(s) }

	tests := []struct {
		name     string
		deploy   map[string]deploykit.DeployNode
		image    string
		instance string
		want     []string
	}{
		{
			name:     "no entry — returns nil",
			deploy:   map[string]deploykit.DeployNode{"other": {Image: "other"}},
			image:    "missing",
			instance: "",
			want:     nil,
		},
		{
			name:     "entry without sidecars — returns nil",
			deploy:   map[string]deploykit.DeployNode{"foo": {Image: "foo"}},
			image:    "foo",
			instance: "",
			want:     nil,
		},
		{
			name: "entry with one sidecar — single-name slice",
			deploy: map[string]deploykit.DeployNode{
				"foo": {Image: "foo", Sidecar: map[string]json.RawMessage{"tailscale": raw("{}")}},
			},
			image:    "foo",
			instance: "",
			want:     []string{"tailscale"},
		},
		{
			name: "entry with multiple sidecars — sorted",
			deploy: map[string]deploykit.DeployNode{
				"foo": {Image: "foo", Sidecar: map[string]json.RawMessage{
					"vault": raw("{}"), "tailscale": raw("{}"),
				}},
			},
			image:    "foo",
			instance: "",
			want:     []string{"tailscale", "vault"},
		},
		{
			name: "Pattern-A instance entry with sidecar",
			deploy: map[string]deploykit.DeployNode{
				"foo/inst1": {Image: "foo", Sidecar: map[string]json.RawMessage{"tailscale": raw("{}")}},
			},
			image:    "foo",
			instance: "inst1",
			want:     []string{"tailscale"},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			dc := &deploykit.DeployConfig{Deploy: tc.deploy}
			got := sidecarNamesFromDeployConfig(dc, tc.image, tc.instance)
			if !reflect.DeepEqual(got, tc.want) {
				t.Errorf("sidecarNamesFromDeployConfig(%q, %q) = %v; want %v", tc.image, tc.instance, got, tc.want)
			}
		})
	}

	t.Run("nil deploy config — returns nil", func(t *testing.T) {
		if got := sidecarNamesFromDeployConfig(nil, "foo", ""); got != nil {
			t.Errorf("sidecarNamesFromDeployConfig(nil, ...) = %v, want nil", got)
		}
	})
}
