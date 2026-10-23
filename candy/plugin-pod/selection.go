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

// Engine is the resolved container-engine word (never `auto`: the resolver resolves it).
func (s podSelection) Engine() spec.EngineName { return s.Target.Engine }

// binary is the engine's CLI binary. It is a capability fact (containerd is driven through
// nerdctl), never an engine-word spelling.
func (s podSelection) binary() string { return kit.EngineBinary(string(s.Target.Engine)) }

// deploySelection maps the deploy entity level of the precedence chain onto the resolver's
// Selection. An absent entry contributes nothing, so every lower level is consulted.
func deploySelection(dc *deploykit.DeployConfig, box, instance string) deploykit.Selection {
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

// resolvePodSelectionFor loads the per-host overlay once and resolves against it. Callers that
// already hold the config use resolvePodSelectionFromConfig instead.
func resolvePodSelectionFor(box, instance string, rt *kit.ResolvedRuntime) (podSelection, error) {
	dc, _ := loadPodDeployConfig()
	return resolvePodSelectionFromConfig(dc, box, instance, rt)
}
