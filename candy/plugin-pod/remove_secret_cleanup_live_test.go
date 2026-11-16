package pod

// remove_secret_cleanup_live_test.go — the LIVE half of the teardown fix (opencharly/plugin-deploy-pod#64).
//
// The unit tests above pin the decision logic with injected seams. This one runs the SAME
// function against a REAL engine: it lists what the engine holds for a box, removes it, and
// asserts the engine no longer has it. It skips visibly unless CHARLY_LIVE_ENGINE=1, because a
// test that needs a container engine must never silently pass without one (R8: live or skip,
// never fake).
//
// Reproduce by hand:
//
//	CHARLY_LIVE_ENGINE=1 CHARLY_LIVE_ENGINE_BIN=podman   go test -run TestLive -v ./...
//
// It is destructive BY DESIGN, on a throwaway engine only: it removes every podman secret whose
// name starts with charly-<box>-. Point it at a disposable engine, or name a box you genuinely
// want cleaned.

import (
	"os"
	"os/exec"
	"strings"
	"testing"
)

func TestLiveRemoveProvisionedSecrets(t *testing.T) {
	if os.Getenv("CHARLY_LIVE_ENGINE") != "1" {
		t.Skip("CHARLY_LIVE_ENGINE not set: the live engine proof is opt-in (R8: live or skip, never fake)")
	}
	engine := os.Getenv("CHARLY_LIVE_ENGINE_BIN")
	if engine == "" {
		engine = "podman"
	}
	box := os.Getenv("CHARLY_LIVE_BOX")
	if box == "" {
		t.Skip("CHARLY_LIVE_BOX not set: name the box whose engine secrets this run may delete")
	}

	before := listProvisionedSecretNames(engine, box)
	if len(before) == 0 {
		t.Skipf("%s holds no charly-%s-* secret to remove; nothing to prove", engine, box)
	}
	t.Logf("engine holds %d provisioned secret(s) for %s: %s", len(before), box, strings.Join(before, ", "))

	removeProvisionedSecrets(engine, box, "")

	after := listProvisionedSecretNames(engine, box)
	if len(after) != 0 {
		t.Fatalf("after the cleanup %s still holds %v (removed %v)", engine, after, before)
	}
	out, err := exec.Command(engine, "secret", "ls", "--format", "{{.Name}}").Output()
	if err != nil {
		t.Fatalf("%s secret ls after the cleanup: %v", engine, err)
	}
	for _, n := range before {
		if strings.Contains(string(out), n) {
			t.Errorf("secret %s is still listed by the engine after the cleanup", n)
		}
	}
	t.Logf("removed %d provisioned secret(s) and the engine lists none of them any more", len(before))
}
