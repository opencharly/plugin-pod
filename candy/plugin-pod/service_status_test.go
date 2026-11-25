package pod

import (
	"errors"
	"testing"

	"github.com/opencharly/sdk"
	"github.com/opencharly/spec/spec"
)

// oneShotMeta is the shape pod-immich declares for its migration service: a long-running server
// plus a run-to-completion init that exits 0 and is never restarted.
func oneShotMeta() *spec.BoxMetadata {
	return &spec.BoxMetadata{Service: []spec.CapabilityService{
		{Name: "immich-server", Restart: "always"},
		{Name: "immich-db-init", Restart: "no", StartSec: 0, ExitCode: "0"},
	}}
}

func TestServiceStatusHealthy(t *testing.T) {
	const healthyTable = "dbus                             RUNNING   pid 51, uptime 0:23:35\n" +
		"immich-db-init                   EXITED    Nov 24 03:13 PM\n" +
		"immich-server                    RUNNING   pid 56, uptime 0:23:35\n"

	cases := []struct {
		name string
		code int
		out  string
		want bool
	}{
		{"a declared one-shot that exited is healthy", 3, healthyTable, true},
		{"exit 0 is not this verdict's question", 0, healthyTable, false},
		{"a long-running service that exited is not healthy", 3,
			"immich-server                    EXITED    Nov 24 03:13 PM\n", false},
		{"an EXITED service the image does not declare is not healthy", 3,
			"mystery                          EXITED    Nov 24 03:13 PM\n", false},
		{"a FATAL row is not healthy", 3,
			"immich-server                    FATAL     Nov 24 03:13 PM\n", false},
		{"a STOPPED row is not healthy", 3,
			"immich-db-init                   STOPPED   Nov 24 03:13 PM\n", false},
		{"output that is not a table proves nothing", 3,
			"unix:///var/run/supervisor.sock no such file\n", false},
		{"an empty table proves nothing", 3, "", false},
	}
	for _, tc := range cases {
		if got := serviceStatusHealthy(oneShotMeta(), tc.code, tc.out); got != tc.want {
			t.Errorf("%s: serviceStatusHealthy = %v, want %v", tc.name, got, tc.want)
		}
	}
}

func TestDeclaredRunToCompletion(t *testing.T) {
	meta := oneShotMeta()
	cases := []struct {
		name string
		svc  spec.CapabilityService
		want bool
	}{
		{"restart no + start_sec 0 + exit_code", spec.CapabilityService{Name: "a", Restart: "no", ExitCode: "0"}, true},
		{"restart false is the same statement", spec.CapabilityService{Name: "a", Restart: "false", ExitCode: "0"}, true},
		{"a restarting service is not one-shot", spec.CapabilityService{Name: "a", Restart: "always", ExitCode: "0"}, false},
		{"a start-up grace period means the tool waits for it", spec.CapabilityService{Name: "a", Restart: "no", StartSec: 5, ExitCode: "0"}, false},
		{"no expected exit code means nothing to judge", spec.CapabilityService{Name: "a", Restart: "no"}, false},
	}
	for _, tc := range cases {
		m := &spec.BoxMetadata{Service: []spec.CapabilityService{tc.svc}}
		if got := declaredRunToCompletion(m, tc.svc.Name); got != tc.want {
			t.Errorf("%s: declaredRunToCompletion = %v, want %v", tc.name, got, tc.want)
		}
	}
	if declaredRunToCompletion(meta, "absent") {
		t.Error("a service the image does not declare cannot be one-shot")
	}
	if declaredRunToCompletion(nil, "immich-db-init") {
		t.Error("no metadata means no declaration")
	}
}

func TestServiceStatusVerdict(t *testing.T) {
	const healthyTable = "immich-db-init                   EXITED    Nov 24 03:13 PM\n" +
		"immich-server                    RUNNING   pid 56, uptime 0:23:35\n"
	toolErr := errors.New(`command "service": exit code 3`)

	// The healthy one-shot: the host reports exit 3 with the rows, and the verb succeeds.
	if err := serviceStatusVerdict(oneShotMeta(), spec.PodLifecycleReply{ExitCode: 3, Output: healthyTable}, toolErr); err != nil {
		t.Errorf("healthy one-shot deploy still failed: %v", err)
	}

	// A dead long-running service keeps the tool's own error.
	if err := serviceStatusVerdict(oneShotMeta(), spec.PodLifecycleReply{ExitCode: 3, Output: "immich-server FATAL Nov 24\n"}, toolErr); !errors.Is(err, toolErr) {
		t.Errorf("a FATAL row must keep the tool's error, got %v", err)
	}

	// An older host sends no rows and no error code: today's behaviour stands.
	if err := serviceStatusVerdict(oneShotMeta(), spec.PodLifecycleReply{}, toolErr); !errors.Is(err, toolErr) {
		t.Errorf("a reply without output must keep the tool's error, got %v", err)
	}

	// A host that reports a code without an error still fails the verb.
	var ece *sdk.ExitCodeError
	if err := serviceStatusVerdict(oneShotMeta(), spec.PodLifecycleReply{ExitCode: 2, Output: "boom\n"}, nil); !errors.As(err, &ece) {
		t.Errorf("a non-zero code with no error must surface as an exit code, got %v", err)
	}
}
