package pod

import (
	"fmt"
	"strings"

	"github.com/opencharly/sdk/deploykit"
	"github.com/opencharly/sdk/loaderkit"
	"github.com/opencharly/spec/spec"
)

// resolveDeployAddress turns a `charly <verb> <address>` argument into the (box, instance) pair the
// pod lifecycle dispatch keys on, applying DESIGN D-REF-1 to a dotted address: "a dotted name's first
// segment is a namespace if one with that name exists, otherwise a node of this project".
//
// The deploy command's ADD dispatch receives the deploy KEY (`ns/entity`), while the lifecycle
// commands receive the user's ADDRESS — the same identity written `ns.entity`. Both reach the
// substrate as (box, instance), and the container and systemd names are derived from that pair
// (`charly-<box>[-<instance>]`), so reading a namespaced address as ONE BARE BOX produced a name
// start never created. Measured on opencharly/spec#450: `charly update
// kde-vnc-cdp.check-kde-vnc-cdp-pod` looked for `charly-kde-vnc-cdp.check-kde-vnc-cdp-pod`, while
// the unit start had created was `charly-kde-vnc-cdp-check-kde-vnc-cdp-pod`.
//
// The namespace TABLE decides, never the shape of the string, and a load failure is reported rather
// than guessed through — so a dotted name whose head a project does not mount keeps walking as a
// nested path, and a project that cannot be read says so instead of silently splitting.
func resolveDeployAddress(image, instance string) (string, string, error) {
	box, inst := deploykit.CanonicalizeDeployArg(image, instance)
	if inst != "" || !strings.Contains(box, ".") {
		return box, inst, nil
	}
	var pre spec.DeployPluginsConnectReply
	if err := hostPodSeamReply("deploy-plugins-connect", spec.DeployPluginsConnectRequest{Path: image}, &pre); err != nil {
		return "", "", fmt.Errorf("resolving deploy address %q: %w", image, err)
	}
	uf, _, err := loaderkit.LoadUnifiedViaExecutor(cmdCtx, cmdExec, pre.Dir)
	if err != nil {
		return "", "", fmt.Errorf("resolving deploy address %q: load project: %w", image, err)
	}
	box, inst = splitOnNamespace(uf, box, inst)
	return box, inst, nil
}

// splitOnNamespace is the pure half: D-REF-1's test against the loaded project's namespace table. A
// `-i <instance>` argument already carries the split, and a name without a dot is a node of this
// project; only a dotted, instance-less name asks whether its head mounts a namespace.
func splitOnNamespace(uf *spec.UnifiedFile, box, inst string) (string, string) {
	if uf == nil || inst != "" {
		return box, inst
	}
	head, rest, ok := strings.Cut(box, ".")
	if !ok || head == "" || rest == "" {
		return box, inst
	}
	if _, isNamespace := uf.Namespaces[head]; isNamespace {
		return head, rest
	}
	return box, inst
}
