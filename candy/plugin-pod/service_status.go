package pod

import (
	"fmt"
	"strings"

	"github.com/opencharly/sdk"
	"github.com/opencharly/spec/spec"
)

// service_status.go — the verdict `charly service status` applies to the init tool's table.
//
// The tool's aggregate exit code answers a question nobody asked. `supervisorctl status` exits 3
// when ANY process is not RUNNING, and for a deployment that declares a run-to-completion
// (one-shot) service that is the HEALTHY steady state: the service did its job and exited, and the
// image declares it (`restart: no`, `start_sec: 0`, `exit_code: "0"`). Propagating the 3 made a
// healthy deployment report failure (opencharly/charly#1271).
//
// The distinction cannot be drawn from the exit code, and the host cannot draw it at all: only
// this side reads the image's service declarations. So the host returns BOTH the captured table
// and the exit code (`spec.PodLifecycleReply.output` / `exit_code`), and the verdict below decides.
// A reply WITHOUT output is the old host: nothing is judged and the tool's own error stands.

// serviceStates is the closed set of process states the init tool prints, so a diagnostic line is
// never mistaken for a row.
var serviceStates = map[string]bool{
	"RUNNING":  true,
	"STARTING": true,
	"BACKOFF":  true,
	"STOPPING": true,
	"STOPPED":  true,
	"EXITED":   true,
	"FATAL":    true,
	"UNKNOWN":  true,
}

// serviceStatusRow is one table line: `<name> <STATE> <detail…>`.
type serviceStatusRow struct {
	name  string
	state string
}

// parseServiceStatusRows parses the status table. ok is false when the output holds no rows, or
// any non-empty line is not a row — the caller must not read a diagnostic as health.
func parseServiceStatusRows(output string) (rows []serviceStatusRow, ok bool) {
	for _, line := range strings.Split(output, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) < 2 || !serviceStates[fields[1]] {
			return nil, false
		}
		rows = append(rows, serviceStatusRow{name: fields[0], state: fields[1]})
	}
	return rows, len(rows) > 0
}

// declaredRunToCompletion reports whether the image declares name as a service that runs to
// completion: never restarted, no start-up grace period, and an expected exit code. That is the
// three-field idiom the candies use (pod-immich's `immich-db-init` is `restart: no`,
// `start_sec: 0`, `exit_code: "0"`), and it is the only declaration that makes an EXITED row
// healthy.
func declaredRunToCompletion(meta *spec.BoxMetadata, name string) bool {
	if meta == nil {
		return false
	}
	for _, svc := range meta.Service {
		if svc.Name != name {
			continue
		}
		restart := strings.ToLower(strings.TrimSpace(svc.Restart))
		neverRestarted := restart == "no" || restart == "false"
		return neverRestarted && svc.StartSec == 0 && strings.TrimSpace(svc.ExitCode) != ""
	}
	return false
}

// serviceStatusHealthy reports whether an exit-3 status run describes a healthy deployment: every
// long-running service is RUNNING and every other row is a declared run-to-completion service that
// EXITED. Any FATAL/BACKOFF/STOPPED row, any undeclared EXITED row, or output that does not parse
// is unhealthy.
func serviceStatusHealthy(meta *spec.BoxMetadata, code int, output string) bool {
	if code != 3 {
		return false
	}
	rows, ok := parseServiceStatusRows(output)
	if !ok {
		return false
	}
	for _, row := range rows {
		switch row.state {
		case "RUNNING":
		case "EXITED":
			if !declaredRunToCompletion(meta, row.name) {
				return false
			}
		default:
			return false
		}
	}
	return true
}

// serviceRunVerdict is the result of a service-MUTATING op (start/stop/restart). The host reports
// the init tool's exit code in the reply rather than as an error — the same seam that lets the
// status verb judge its table — so the code is turned back into an error here. Printing stays
// here too, because a capturing host no longer prints host-side.
func serviceRunVerdict(reply spec.PodLifecycleReply, err error) error {
	if reply.Output != "" {
		fmt.Print(reply.Output)
	}
	if err != nil {
		return err
	}
	if reply.ExitCode != 0 {
		return &sdk.ExitCodeError{Code: reply.ExitCode}
	}
	return nil
}

// serviceStatusVerdict is the verb's result for an op="service" reply: the tool's error unless the
// table proves the deployment healthy, in which case the run succeeded.
func serviceStatusVerdict(meta *spec.BoxMetadata, reply spec.PodLifecycleReply, err error) error {
	if serviceStatusHealthy(meta, reply.ExitCode, reply.Output) {
		return nil
	}
	if err != nil {
		return err
	}
	if reply.ExitCode != 0 {
		return &sdk.ExitCodeError{Code: reply.ExitCode}
	}
	return nil
}
