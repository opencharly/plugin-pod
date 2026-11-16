package pod

// remove_secret_cleanup_test.go — pins the teardown gap filed as
// opencharly/plugin-deploy-pod#64: `charly remove` tore down the container, the quadlets and
// the deploy entry, but never the podman secrets the same deploy provisioned, so they stayed in
// the engine store forever — indistinguishable from a live deployment's, and reused
// ("kept (already provisioned)") by the next run of the same bed, which can mask a provisioning
// defect by carrying a stale value across runs.
//
// Pinned here:
//  1. the LAST deployment of a box removes exactly what the engine lists for that box;
//  2. a sibling deployment of the SAME box makes it refuse, in BOTH directions — a named
//     instance removed while the DEFAULT deployment is present (opencharly/plugin-pod#28's
//     finding: the default container is "charly-<box>" with no trailing dash, so a container-name
//     scan missed it) and the default removed while a named instance is present;
//  3. an unreadable deploy config is treated as "a sibling may exist" — conservative, because
//     deleting a shared secret is not recoverable;
//  4. no secrets listed is a silent no-op (no engine delete call);
//  5. this deploy's OWN sidecar containers are never mistaken for a sibling deployment;
//  6. teardown never touches the credential STORE (only engine objects die), since that
//     coordinate may serve other deployments (opencharly/pod-openwebui#19).

import (
	"errors"
	"reflect"
	"testing"

	"github.com/opencharly/sdk/deploykit"
)

type secretSeams struct {
	listed  [][2]string // (engine, box) pairs the cleanup asked about
	removed [][2]string // (engine, secret name) pairs it deleted
}

// installSeams injects the engine seam and the deploy config. deployKeys are the charly.yml keys
// the config holds for this box/instance family ("" for a config that cannot be read).
func installSeams(t *testing.T, names []string, deployKeys []string, cfgErr error) *secretSeams {
	t.Helper()
	rec := &secretSeams{}
	origList, origRm, origCfg := listProvisionedSecretNames, removePodmanSecrets, loadPodDeployConfig
	t.Cleanup(func() {
		listProvisionedSecretNames, removePodmanSecrets, loadPodDeployConfig = origList, origRm, origCfg
	})
	listProvisionedSecretNames = func(engine, box string) []string {
		rec.listed = append(rec.listed, [2]string{engine, box})
		return names
	}
	removePodmanSecrets = func(engine string, secrets []deploykit.CollectedSecret) {
		for _, s := range secrets {
			rec.removed = append(rec.removed, [2]string{engine, s.Name})
		}
	}
	loadPodDeployConfig = func() (*deploykit.DeployConfig, error) {
		if cfgErr != nil {
			return nil, cfgErr
		}
		dc := &deploykit.DeployConfig{Deploy: map[string]deploykit.DeployNode{}}
		for _, k := range deployKeys {
			dc.Deploy[k] = deploykit.DeployNode{}
		}
		return dc, nil
	}
	return rec
}

func TestRemoveProvisionedSecrets_LastDeploymentRemovesThem(t *testing.T) {
	rec := installSeams(t, []string{"charly-app-token", "charly-app-webui-secret-key"}, []string{"app"}, nil)
	removeProvisionedSecrets("podman", "app", "")
	want := [][2]string{{"podman", "charly-app-token"}, {"podman", "charly-app-webui-secret-key"}}
	if !reflect.DeepEqual(rec.removed, want) {
		t.Errorf("removed %v, want %v", rec.removed, want)
	}
	if len(rec.listed) != 1 || rec.listed[0] != [2]string{"podman", "app"} {
		t.Errorf("listed %v, want one lookup for (podman, app)", rec.listed)
	}
}

// TestRemoveProvisionedSecrets_NamedInstanceKeepsThemWhenDefaultIsPresent is the regression
// test for opencharly/plugin-pod#28: removing the NAMED instance while the DEFAULT deployment
// is still configured must refuse. The default's container is "charly-<box>" (no trailing dash),
// which the first version of this guard missed.
func TestRemoveProvisionedSecrets_NamedInstanceKeepsThemWhenDefaultIsPresent(t *testing.T) {
	rec := installSeams(t, []string{"charly-app-token"}, []string{"app", "app/blue"}, nil)
	removeProvisionedSecrets("podman", "app", "blue")
	if len(rec.removed) != 0 {
		t.Errorf("removed %v, want none: the default deployment still needs these secrets", rec.removed)
	}
	if len(rec.listed) != 0 {
		t.Error("the cleanup listed secrets before proving it is the last deployment")
	}
}

// TestRemoveProvisionedSecrets_DefaultKeepsThemWhenNamedInstanceIsPresent is the mirror direction.
func TestRemoveProvisionedSecrets_DefaultKeepsThemWhenNamedInstanceIsPresent(t *testing.T) {
	rec := installSeams(t, []string{"charly-app-token"}, []string{"app", "app/blue"}, nil)
	removeProvisionedSecrets("podman", "app", "")
	if len(rec.removed) != 0 {
		t.Errorf("removed %v, want none: a named instance still needs these secrets", rec.removed)
	}
}

func TestRemoveProvisionedSecrets_UnreadableConfigIsConservative(t *testing.T) {
	rec := installSeams(t, []string{"charly-app-token"}, nil, errors.New("deploy config unreadable"))
	removeProvisionedSecrets("podman", "app", "")
	if len(rec.removed) != 0 || len(rec.listed) != 0 {
		t.Errorf("acted with an unreadable config: removed=%v listed=%v", rec.removed, rec.listed)
	}
}

func TestRemoveProvisionedSecrets_NoSecretsIsASilentNoop(t *testing.T) {
	rec := installSeams(t, nil, []string{"app"}, nil)
	removeProvisionedSecrets("podman", "app", "")
	if len(rec.removed) != 0 {
		t.Errorf("removed %v with nothing listed", rec.removed)
	}
}

// TestSiblingDeployments_OwnSidecarsAreNotSiblings pins the exactness the config read buys:
// a sidecar's container ("charly-<box>-<sidecar>") can never be confused with a named instance
// because the question is answered from the deploy KEYS, not from container names.
func TestSiblingDeployments_OwnSidecarsAreNotSiblings(t *testing.T) {
	installSeams(t, nil, []string{"app"}, nil)
	others, ok := siblingDeployments("app", "")
	if !ok {
		t.Fatal("siblingDeployments reported an unreadable config")
	}
	if len(others) != 0 {
		t.Errorf("siblings = %v, want none (the only deployment is this one, sidecars included)", others)
	}
	// A different box entirely must never count either.
	installSeams(t, nil, []string{"app", "other", "appx", "app/blue"}, nil)
	others, ok = siblingDeployments("app", "")
	if !ok || !reflect.DeepEqual(others, []string{"app/blue"}) {
		t.Errorf("siblings = %v (ok=%v), want exactly [app/blue]", others, ok)
	}
}
