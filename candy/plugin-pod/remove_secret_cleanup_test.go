package pod

// remove_secret_cleanup_test.go — pins the teardown gap filed as
// opencharly/plugin-deploy-pod#64: `charly remove` tore down the container, the quadlets and
// the deploy entry, but never the podman secrets the same deploy provisioned, so they stayed in
// the engine store forever — indistinguishable from a live deployment's, and reused
// ("kept (already provisioned)") by the next run of the same bed, which can mask a provisioning
// defect by carrying a stale value across runs.
//
// Pinned here:
//  1. the LAST instance of a box removes exactly what the engine lists for that box;
//  2. a sibling deployment of the SAME box (podman secret names are box-scoped, not
//     instance-scoped) makes the cleanup refuse rather than break it;
//  3. an engine that cannot be enumerated is treated as "a sibling may exist" — conservative,
//     because deleting a shared secret is not recoverable;
//  4. no secrets listed is a silent no-op (no engine delete call);
//  5. teardown never touches the credential STORE (only engine objects die), since that
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

func installSeams(t *testing.T, names []string, engineNames []string, engineErr error) *secretSeams {
	t.Helper()
	rec := &secretSeams{}
	origList, origRm, origPs := listProvisionedSecretNames, removePodmanSecrets, engineContainerNames
	t.Cleanup(func() {
		listProvisionedSecretNames, removePodmanSecrets, engineContainerNames = origList, origRm, origPs
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
	engineContainerNames = func(string) ([]string, error) { return engineNames, engineErr }
	return rec
}

func TestRemoveProvisionedSecrets_LastInstanceRemovesThem(t *testing.T) {
	rec := installSeams(t,
		[]string{"charly-app-token", "charly-app-webui-secret-key"},
		[]string{"charly-unrelated"}, nil)
	removeProvisionedSecrets("podman", "app", "")
	want := [][2]string{{"podman", "charly-app-token"}, {"podman", "charly-app-webui-secret-key"}}
	if !reflect.DeepEqual(rec.removed, want) {
		t.Errorf("removed %v, want %v", rec.removed, want)
	}
	if len(rec.listed) != 1 || rec.listed[0] != [2]string{"podman", "app"} {
		t.Errorf("listed %v, want one lookup for (podman, app)", rec.listed)
	}
}

func TestRemoveProvisionedSecrets_SiblingDeployKeepsThem(t *testing.T) {
	// A second instance of the same box is still deployed: its container is still present.
	rec := installSeams(t, []string{"charly-app-token"}, []string{"charly-app-blue"}, nil)
	removeProvisionedSecrets("podman", "app", "")
	if len(rec.removed) != 0 {
		t.Errorf("removed %v although another instance of the box is still deployed", rec.removed)
	}
	if len(rec.listed) != 0 {
		t.Error("the cleanup listed secrets before proving it is the last instance")
	}
}

func TestRemoveProvisionedSecrets_UnlistableEngineIsConservative(t *testing.T) {
	rec := installSeams(t, []string{"charly-app-token"}, nil, errors.New("engine unavailable"))
	removeProvisionedSecrets("podman", "app", "")
	if len(rec.removed) != 0 || len(rec.listed) != 0 {
		t.Errorf("acted on an unlistable engine: removed=%v listed=%v", rec.removed, rec.listed)
	}
}

func TestRemoveProvisionedSecrets_NoSecretsIsASilentNoop(t *testing.T) {
	rec := installSeams(t, nil, nil, nil)
	removeProvisionedSecrets("podman", "app", "")
	if len(rec.removed) != 0 {
		t.Errorf("removed %v with nothing listed", rec.removed)
	}
}

// TestRemoveProvisionedSecrets_OwnContainerIsNotASibling: the deploy's OWN container (when it is
// still present, e.g. removed by a mode that does not delete it first) must not be mistaken for
// a sibling — otherwise the cleanup would never fire on the last instance.
func TestRemoveProvisionedSecrets_OwnContainerIsNotASibling(t *testing.T) {
	rec := installSeams(t, []string{"charly-app-token"}, []string{"charly-app"}, nil)
	removeProvisionedSecrets("podman", "app", "")
	if len(rec.removed) != 1 {
		t.Errorf("removed %v, want the one secret (own container is not a sibling)", rec.removed)
	}
}
