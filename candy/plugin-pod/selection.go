package pod

// selection.go — this plugin's ONE consumer of the sdk selection model. Every site that needs to
// know which engine a deploy runs on calls resolvePodSelection; none reads rt.RunEngine, re-derives
// the deploy-entry-over-runtime precedence by hand, or carries an engine word literal.
//
// The precedence is the sdk's (flag > deploy entity > env > settings file > built-in default).
// This plugin has no engine FLAG, so the flag level is always empty; the deploy entity comes from
// the per-host overlay entry, and the env/settings fold arrives already resolved on
// rt.RunEngine / rt.Namespace (hostenv.ResolveRuntime owns it). `auto` is a selector the resolver
// handles at its own level.
//
// The candy `engine:` constraint has no source on this side of the plugin boundary: the command
// plugin receives a deploy key, not the resolved candy graph. It is therefore left empty here,
// exactly as the sibling deploy plugin does; the constraint stays enforced where the graph exists
// (`charly deploy add`).

import (
	"fmt"
	"os"

	"github.com/opencharly/sdk/deploykit"
	"github.com/opencharly/sdk/kit"
	"github.com/opencharly/spec/spec"
)

// podSelection is the resolved selection for one deploy. It IS the engine target the resolver
// produced: the engine word plus the capability facts (address, namespace, rootful, privilege
// prefix). Keeping only the target means the word this plugin launches and the word the capability
// facts were derived from cannot disagree — there is no second copy to drift.
type podSelection struct {
	Target deploykit.ContainerdTarget
}

// Engine is the resolved container-engine WORD (never `auto`: the resolver resolves it). The
// container/image probes take the WORD, because each resolves the binary through the capability
// table itself; handing them a BINARY makes that resolve fall back to the default engine. Pass the
// word to a probe; pass binary() only to an argv. TestProbeBoundaryTakesTheEngineWord pins this and
// logs the exact fallback.
func (s podSelection) Engine() spec.EngineName { return s.Target.Engine }

// binary is the engine's CLI binary for argv construction. It is a capability fact (containerd is
// driven through nerdctl), never an engine-word spelling.
func (s podSelection) binary() string { return kit.EngineBinary(string(s.Target.Engine)) }

// deploySelection maps the deploy entity level of the precedence chain onto the resolver's
// Selection. A nil config (no readable overlay) contributes nothing, exactly like an absent entry,
// so every lower level is consulted. The explicit guard is load-bearing even though
// spec.DeployConfig.Lookup is nil-receiver-safe: it makes the absent-overlay case visible here
// instead of depending on a nil-receiver method a future refactor could change.
func deploySelection(dc *deploykit.DeployConfig, box, instance string) deploykit.Selection {
	if dc == nil {
		return deploykit.Selection{}
	}
	entry, ok := dc.Lookup(box, instance)
	if !ok {
		return deploykit.Selection{}
	}
	return deploykit.Selection{
		Engine:    entry.Engine,
		Rootful:   entry.Rootful,
		Namespace: entry.Namespace,
	}
}

// resolvePodSelection runs the ONE selection model: the engine word first, then the target facts
// derived from that word. ResolveContainerdTarget receives the already-resolved engine, so the
// address/namespace/rootful facts can never disagree with the engine that launches, and a deploy
// entry whose rootful word is outside the contract's vocabulary is a hard error here instead of
// being silently ignored.
func resolvePodSelection(deploy deploykit.Selection, rt *kit.ResolvedRuntime) (podSelection, error) {
	engine, err := deploykit.ResolvePodEngine("", deploy, rt)
	if err != nil {
		return podSelection{}, err
	}
	target, err := deploykit.ResolveContainerdTarget(engine, deploy, rt)
	if err != nil {
		return podSelection{}, err
	}
	return podSelection{Target: target}, nil
}

// resolvePodSelectionFromConfig resolves against an already-loaded overlay.
func resolvePodSelectionFromConfig(dc *deploykit.DeployConfig, box, instance string, rt *kit.ResolvedRuntime) (podSelection, error) {
	return resolvePodSelection(deploySelection(dc, box, instance), rt)
}

// resolvePodSelectionFor loads the per-host overlay once and resolves against it. An UNREADABLE
// overlay is an ERROR here: only an absent entry may fall through to the env/settings level, so a
// broken config cannot silently resolve a deploy to the wrong engine. Callers with a documented
// best-effort contract (teardown) handle the error explicitly instead of discarding it.
func resolvePodSelectionFor(box, instance string, rt *kit.ResolvedRuntime) (podSelection, error) {
	dc, err := loadPodDeployConfig()
	if err != nil {
		return podSelection{}, fmt.Errorf("load the deploy overlay for %q: %w", spec.DeployKey(box, instance), err)
	}
	return resolvePodSelectionFromConfig(dc, box, instance, rt)
}

// resolveTeardownSelection resolves the engine for `charly remove`. Its contract is narrower than
// resolvePodSelectionFor's: an UNREADABLE overlay must not block tearing down an orphaned deploy
// (the pre-existing best-effort contract shared by removeVolumes/resolveSidecarNames), so it
// degrades to the env/settings fold, VISIBLY. A READABLE overlay is still resolved strictly, so a
// retired/unknown engine word is a hard error here too and never silently runs the default engine.
func resolveTeardownSelection(box, instance string, rt *kit.ResolvedRuntime) (podSelection, error) {
	dc, err := loadPodDeployConfig()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Warning: %v — resolving the teardown engine from the runtime settings\n", err)
		return resolvePodSelection(deploykit.Selection{}, rt)
	}
	return resolvePodSelectionFromConfig(dc, box, instance, rt)
}
