package pod

import (
	"encoding/json"
	"errors"
	"fmt"

	"github.com/opencharly/sdk"
	"github.com/opencharly/sdk/deploykit"
	"github.com/opencharly/spec/container"
	"github.com/opencharly/spec/spec"
)

// remove_tunnel.go — the `charly remove` tunnel-teardown leg (Cutover B unit 2 remove-verb
// completion). Mirrors candy/plugin-deploy-pod/lifecycle.go's podTunnelOp exactly (R3): both reach
// the SAME compiled-in, placement-agnostic verb:tunnel provider over the SAME
// {plugin_input:{method,config}} envelope.
// The tunnel CONFIG is resolved PLUGIN-SIDE (#55 coneC-dsh — the former
// pod-config-container-tunnel host seam is DELETED): the running container's baked image ref, its
// metadata and the charly.yml overlay merge are all plugin-reachable, and the engine that answers
// the container/image probes comes from the ONE sdk selection model rather than a hand-picked
// engine word.

// resolveContainerTunnel resolves the running container's tunnel config PLUGIN-SIDE (#55 coneC-dsh
// — the pod-config-container-tunnel host seam is DELETED). The engine selection is the CALLER's
// (the `charly remove` teardown selection), so this read can never re-resolve — or silently
// swallow — a selection error. It only reads, and returns nil when there is no tunnel to stop.
func resolveContainerTunnel(box, instance string, sel podSelection) *spec.TunnelConfig {
	ctrName := spec.ContainerNameInstance(box, instance)
	// The overlay merge is best-effort: a tunnel config is an enhancement to the teardown, and an
	// unreadable overlay means "no per-host tunnel overrides", never a missing teardown.
	dc, _ := loadPodDeployConfig()
	// The probes take the resolved engine WORD: each resolves the binary through the capability
	// table itself, so handing them a BINARY would make that resolve fall back to the default
	// engine (the control in TestProbeBoundaryTakesTheEngineWord logs the exact values).
	imageRef := container.ContainerImage(string(sel.Engine()), ctrName)
	if imageRef == "" {
		return nil
	}
	meta, err := deploykit.ExtractMetadata(string(sel.Engine()), imageRef)
	if err != nil || meta == nil {
		return nil
	}
	deploykit.MergeDeployOntoMetadata(meta, dc, box, instance)
	if meta.Tunnel == nil {
		return nil
	}
	return deploykit.TunnelConfigFromMetadata(meta)
}

// podTunnelStop composes verb:tunnel over InvokeProvider — byte-identical in shape to
// candy/plugin-deploy-pod/lifecycle.go's podTunnelOp(ctx, exec, "stop", cfg), including decoding
// the reply's business-level Error field (R1/R3: InvokeProvider only surfaces RPC-transport
// failures, so a healthy-RPC exec failure must be pulled out of the reply payload explicitly — the
// SAME fix applied to podTunnelOp so neither call site silently swallows a real teardown failure).
func podTunnelStop(cfg *spec.TunnelConfig) error {
	if cmdExec == nil {
		return fmt.Errorf("pod remove: no host reverse channel (command not compiled-in?)")
	}
	body, err := json.Marshal(map[string]any{"plugin_input": map[string]any{"method": "stop", "config": cfg}})
	if err != nil {
		return err
	}
	resJSON, err := cmdExec.InvokeProvider(cmdCtx, "verb", "tunnel", sdk.OpRun, body, nil, sdk.InvokeProviderOpts{})
	if err != nil {
		return err
	}
	if len(resJSON) == 0 {
		return nil
	}
	var rep struct {
		Error string `json:"error,omitempty"`
	}
	if json.Unmarshal(resJSON, &rep) == nil && rep.Error != "" {
		return errors.New(rep.Error)
	}
	return nil
}
