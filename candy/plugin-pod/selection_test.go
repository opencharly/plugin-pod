package pod

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/opencharly/sdk/deploykit"
	"github.com/opencharly/sdk/kit"
	"github.com/opencharly/spec/container"
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
	if sel.Target.Engine != wantEngine {
		t.Errorf("engine = %q, want the deploy entry's %q", sel.Target.Engine, wantEngine)
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
	if string(sel.Target.Engine) != wantEngine {
		t.Errorf("engine = %q, want the runtime fold %q", sel.Target.Engine, wantEngine)
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

// TestResolvePodSelectionForHandlesNilAndUnreadableOverlay pins the two overlay states the
// selection must survive: an ABSENT overlay (nil config, nil error) contributes no deploy level and
// resolves from the runtime fold, and an UNREADABLE overlay is an explicit error — never a silent
// fallback to the runtime engine (the silent-fallback class this migration removes).
func TestResolvePodSelectionForHandlesNilAndUnreadableOverlay(t *testing.T) {
	wantEngine := spec.EngineNames[0]
	orig := loadPodDeployConfig
	t.Cleanup(func() { loadPodDeployConfig = orig })

	// Absent overlay: nil config, no error -> the runtime fold answers.
	loadPodDeployConfig = func() (*deploykit.DeployConfig, error) { return nil, nil }
	sel, err := resolvePodSelectionFor("probe", "", runtimeWithEngine(t, wantEngine))
	if err != nil {
		t.Fatalf("nil config resolved with an error: %v", err)
	}
	if string(sel.Target.Engine) != wantEngine {
		t.Errorf("engine = %q, want the runtime fold %q", sel.Target.Engine, wantEngine)
	}

	// Unreadable overlay: an error, not a silent runtime fallback.
	loadPodDeployConfig = func() (*deploykit.DeployConfig, error) { return nil, errors.New("unreadable overlay") }
	if _, err := resolvePodSelectionFor("probe", "", runtimeWithEngine(t, wantEngine)); err == nil {
		t.Fatal("an unreadable overlay resolved silently; the loader error was discarded")
	} else if !strings.Contains(err.Error(), "unreadable overlay") {
		t.Errorf("error %q does not wrap the loader error", err)
	}
}

// TestProbeBoundaryTakesTheEngineWord pins the form every probe receives and WHY. The probes
// resolve the binary through the capability table themselves, so the engine WORD is correct
// (containerd -> nerdctl), while handing them the BINARY would make their own resolve fall back to
// the default engine (kit.EngineBinary("nerdctl") == "podman"). A later "pass the binary" cleanup
// would silently move containerd reads onto podman; this fails first.
func TestProbeBoundaryTakesTheEngineWord(t *testing.T) {
	for _, engine := range spec.EngineNames {
		want, ok := spec.EngineBinaries[engine]
		if !ok {
			t.Fatalf("spec.EngineBinaries has no row for %q", engine)
		}
		sel := podSelection{Target: deploykit.ContainerdTarget{Engine: spec.EngineName(engine)}}
		if got := sel.binary(); got != want {
			t.Errorf("binary(%q) = %q, want the capability fact %q", engine, got, want)
		}
		// What a probe does with the value it is handed: EngineBinary(value).
		if got := kit.EngineBinary(string(sel.Engine())); got != want {
			t.Errorf("a probe handed the WORD %q resolves %q, want %q", engine, got, want)
		}
		if want != engine {
			// The discriminating case: a probe handed the BINARY instead is rewritten away from it.
			if got := kit.EngineBinary(want); got == want {
				t.Errorf("kit.EngineBinary(%q) = %q; the fixture no longer proves a BINARY would be rewritten", want, got)
			} else {
				t.Logf("control: a probe handed the BINARY %q would invoke %q — the word is required", want, got)
			}
		}
	}
}

// TestResolveTeardownSelection pins the teardown contract: an UNREADABLE overlay degrades to the
// runtime fold (an orphaned deploy must still be torn down), while a READABLE overlay with a
// retired engine word stays a HARD error — never a silent default-engine teardown.
func TestResolveTeardownSelection(t *testing.T) {
	wantEngine := spec.EngineNames[0]
	orig := loadPodDeployConfig
	t.Cleanup(func() { loadPodDeployConfig = orig })

	// Unreadable overlay: teardown still resolves, from the runtime fold.
	loadPodDeployConfig = func() (*deploykit.DeployConfig, error) { return nil, errors.New("unreadable overlay") }
	sel, err := resolveTeardownSelection("probe", "", runtimeWithEngine(t, wantEngine))
	if err != nil {
		t.Fatalf("teardown refused to resolve on an unreadable overlay: %v", err)
	}
	if string(sel.Target.Engine) != wantEngine {
		t.Errorf("teardown engine = %q, want the runtime fold %q", sel.Target.Engine, wantEngine)
	}

	// Readable overlay, retired engine word: a hard error, not a silent default.
	loadPodDeployConfig = func() (*deploykit.DeployConfig, error) {
		return &deploykit.DeployConfig{Deploy: map[string]deploykit.DeployNode{
			"probe": {Engine: spec.EngineName("nerdctl")},
		}}, nil
	}
	if _, err := resolveTeardownSelection("probe", "", runtimeWithEngine(t, wantEngine)); err == nil {
		t.Fatal("a retired engine word in a readable overlay resolved silently on the teardown path")
	}
}

// TestProbesResolveTheEngineWordForContainerd proves the PROBE boundary itself, not just the
// capability lookup: with a fake nerdctl (and a fake podman that answers WRONG-ENGINE) first on
// PATH, the probes handed the WORD "containerd" must run nerdctl. The BINARY form is the trap the
// capability table hides — its own resolve falls back to the default engine — so it is asserted as
// the negative control too.
func TestProbesResolveTheEngineWordForContainerd(t *testing.T) {
	// Isolate the persistent image-label cache so this test's fake answers are the ones read.
	t.Setenv("XDG_CACHE_HOME", t.TempDir())
	dir := t.TempDir()
	script := `#!/bin/sh
for a in "$@"; do
	case "$a" in
		*"{{.Config.Image}}"*) echo fake-image-ref; exit 0 ;;
		*"{{.State.Running}}"*) echo true; exit 0 ;;
		*"{{json .Config.Labels}}"*) echo '{"ai.opencharly.box":"probe"}'; exit 0 ;;
	esac
done
exit 0
`
	if err := os.WriteFile(filepath.Join(dir, "nerdctl"), []byte(script), 0o755); err != nil {
		t.Fatalf("write fake nerdctl: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "podman"), []byte("#!/bin/sh\necho WRONG-ENGINE\n"), 0o755); err != nil {
		t.Fatalf("write fake podman: %v", err)
	}
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))

	if got := container.ContainerImage("containerd", "probe-ctr"); got != "fake-image-ref" {
		t.Fatalf("ContainerImage(containerd) = %q; the probe did not resolve nerdctl", got)
	}
	if !kit.ContainerRunning("containerd", "probe-ctr") {
		t.Fatal("ContainerRunning(containerd) = false; the probe did not resolve nerdctl")
	}
	if meta, err := deploykit.ExtractMetadata("containerd", "probe-img-"+filepath.Base(dir)); err != nil || meta == nil {
		t.Fatalf("ExtractMetadata(containerd) = (%v, %v); the probe did not resolve nerdctl", meta, err)
	}

	// The binary form resolves through the capability table to the default engine: that is why
	// every probe call site must pass the WORD. This is the round-2 control.
	if got := container.ContainerImage(kit.EngineBinary("nerdctl"), "probe-ctr"); got != "WRONG-ENGINE" {
		t.Errorf("the binary form no longer discriminates: got %q, want the fake podman answer", got)
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
