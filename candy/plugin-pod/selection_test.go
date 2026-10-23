package pod

import (
	"strings"
	"testing"

	"github.com/opencharly/sdk/deploykit"
	"github.com/opencharly/sdk/kit"
	"github.com/opencharly/spec/spec"
)

// selection_test.go — the W3.1 consumer contract for plugin-pod. Every engine read in this plugin
// goes through resolvePodSelection (selection.go); these tests pin the three behaviours the
// migration exists for, each of which FAILS on the pre-change tree (where the resolver API does
// not exist at all):
//
//  1. a retired/unknown engine word is a hard error, not a silent resolution to the default engine;
//  2. the deploy entity level (engine, rootful, namespace) reaches the resolved target;
//  3. the env/settings fold on rt.RunEngine is honoured when no deploy entry sets a level.
//
// They ALSO run against the pre-change tree as the failing-without control: `resolvePodSelection`
// and `podSelection` are undefined there, so the file does not compile (pasted in the PR).

func runtimeWithEngine(t *testing.T, engine string) *kit.ResolvedRuntime {
	t.Helper()
	// kit.ResolvedRuntime is the hostenv fold; the resolver only reads the fields under test.
	return &kit.ResolvedRuntime{RunEngine: engine}
}

// TestResolvePodSelectionRejectsRetiredEngineWord proves a word outside spec's closed vocabulary is
// an error. The pre-change path returned the entry word verbatim and let the capability table map
// ANY unknown word to the default engine — a `nerdctl` deploy ran podman silently.
func TestResolvePodSelectionRejectsRetiredEngineWord(t *testing.T) {
	_, err := resolvePodSelection(deploykit.Selection{Engine: spec.EngineName("nerdctl")}, nil)
	if err == nil {
		t.Fatal("a retired engine word resolved without an error — the silent-default class is back")
	}
	if !strings.Contains(err.Error(), "engine") {
		t.Errorf("error %q does not name the engine vocabulary", err)
	}
}

// TestDeploySelectionHonoursEntry proves every deploy-entity selector reaches the resolver's
// Selection and then the resolved target: engine, rootful and namespace. On the pre-change tree the
// hand-rolled lookup read ONLY the engine, and the target's namespace/rootful came from the
// capability defaults / rt alone.
func TestDeploySelectionHonoursEntry(t *testing.T) {
	wantEngine := spec.EngineName(spec.EngineNames[0])
	dc := &deploykit.DeployConfig{Deploy: map[string]deploykit.DeployNode{
		"check-pod-selection": {
			Engine:    wantEngine,
			Rootful:   spec.RootfulMode(spec.RootfulModes[len(spec.RootfulModes)-1]),
			Namespace: "selection-test-ns",
		},
	}}
	sel, err := resolvePodSelectionFromConfig(dc, "check-pod-selection", "", nil)
	if err != nil {
		t.Fatalf("resolvePodSelectionFromConfig: %v", err)
	}
	if sel.Engine() != wantEngine {
		t.Errorf("engine = %q, want the deploy entry's %q", sel.Engine(), wantEngine)
	}
	if !sel.Target.Rootful {
		t.Errorf("target.Rootful = false, want true from the deploy entry's rootful: %q", spec.RootfulModes[len(spec.RootfulModes)-1])
	}
	if sel.Target.Namespace != "selection-test-ns" {
		t.Errorf("target.Namespace = %q, want the deploy entry's selection-test-ns", sel.Target.Namespace)
	}
}

// TestResolvePodSelectionFallsBackToRuntime proves the env/settings fold is still a level: with no
// deploy entry, rt.RunEngine answers. The pre-change tree also honoured this level, so the control
// for THIS test is that the API does not exist there — the behaviour must not be lost in the move.
func TestResolvePodSelectionFallsBackToRuntime(t *testing.T) {
	wantEngine := spec.EngineNames[len(spec.EngineNames)-1]
	sel, err := resolvePodSelection(deploykit.Selection{}, runtimeWithEngine(t, wantEngine))
	if err != nil {
		t.Fatalf("resolvePodSelection: %v", err)
	}
	if string(sel.Engine()) != wantEngine {
		t.Errorf("engine = %q, want the runtime fold %q", sel.Engine(), wantEngine)
	}
}

// TestResolvePodSelectionRejectsBadRootfulWord proves the target resolution is REACHABLE, not a
// computed-and-discarded value: a deploy entry whose rootful word is outside the contract's
// vocabulary is a hard error. The pre-change hand-rolled lookup ignored rootful entirely.
func TestResolvePodSelectionRejectsBadRootfulWord(t *testing.T) {
	_, err := resolvePodSelection(deploykit.Selection{Rootful: spec.RootfulMode("bogus-rootful")}, nil)
	if err == nil {
		t.Fatal("a rootful word outside spec.RootfulModes resolved without an error")
	}
}

// TestSelectionBinaryIsACapabilityFact proves the binary is looked up in the engine capability
// table, not derived from the engine word: containerd is driven through nerdctl. A second
// word→binary switch anywhere in the plugin would be the R2 duplication this replaces.
func TestSelectionBinaryIsACapabilityFact(t *testing.T) {
	for _, engine := range spec.EngineNames {
		want, ok := spec.EngineBinaries[engine]
		if !ok {
			t.Fatalf("spec.EngineBinaries has no row for %q", engine)
		}
		got := podSelection{Target: deploykit.ContainerdTarget{Engine: spec.EngineName(engine)}}.binary()
		if got != want {
			t.Errorf("binary(%q) = %q, want the capability fact %q", engine, got, want)
		}
	}
}
