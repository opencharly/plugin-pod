# AGENTS.md — plugin-pod

Standalone plugin repo owning the pod-lifecycle CLI (`command:start` / `stop` /
`restart` / `config` / `shell` / `service` / `logs` / `remove` / `cp` / `volume` /
`update`). The plugin is a Go module at `candy/plugin-pod/` (module path
`github.com/opencharly/plugin-pod/candy/plugin-pod`); the root `charly.yml`
declares `discover: candy` + `discover: box` (so the repo is a project, its candy
is scanned, and the `check-pod` R10 bed is declared) and the candy carries three
embedded `skill:` entities.

Canonical files:

- `candy/plugin-pod/charly.yml` — the `plugin-pod:` candy entity (`plugin:`
  block, `plan:` check) and the `restart-skill:` / `volume-skill:` /
  `cp-skill:` entities.
- `candy/plugin-pod/` — the Go source: `plugin.go`, `command.go`, `pod_cmd.go`,
  `enc_cmd.go`, `host_seams.go`, `remove_orchestration.go`, `remove_tunnel.go`,
  `service_resolve.go`, `schema/pod.cue`, `cmd/serve/main.go`.
- `charly.yml` — the root manifest (`discover: candy` + `discover: box`) + the
  `check-pod` disposable R10 bed.
- `.github/workflows/tag-on-merge.yml` — CalVer tag + `CHANGELOG/` on merge.
- `README.md` — user overview only; never agent guidance.

## Load these skills first (R0)

- `/charly-pod-verbs:restart`, `/charly-pod-verbs:volume`, `/charly-pod-verbs:cp`
  — the projected owning skills for three of this candy's command words.
- `/charly-core:start` / `/charly-core:stop` / `/charly-core:shell` — the
  command reference for the registry-bound words (owned by `pod-charly-core`).
- `/charly-internals:plugin` — the plugin authoring reference: the `plugin:`
  block, the `command` provider class, the host-build seam pattern, the
  per-plugin CUE-schema contract.
- `/charly-internals:git-workflow` — before any git/PR action.

## Build / validate / test

- `go build ./...` in `candy/plugin-pod/` — compile the plugin module.
- `go test ./...` in `candy/plugin-pod/` — the plugin's Go tests (the remove
  orchestration/ordering, enc, service-resolve, and update seams).
- `charly box validate` at the repo root — the structural check (the candy +
  `plugin:` block, CUE schema).
- The merge gate is the **org-wide** `charly/pr-validator` (required check
  `validate / validate`, defined in `opencharly/.github`); this repo has **no**
  per-repo candy gate.
- The live R10 witness is `charly check run check-pod` (declared in the root
  `charly.yml`).

## Modify this repo

- Edit the `plugin-pod:` candy entity, the Go source, and `schema/pod.cue`
  **together** — the schema is the single source for the command surface.
- `charly restart` stays pure `sdk/kit` + `sdk/deploykit` with zero host
  coupling; the registry-bound commands keep their per-command host-build seam
  (the host runs the original orchestration `Run()` verbatim).
- Keep each embedded `*-skill:` entity in step with its command word — they are
  the projected sources for `/charly-pod-verbs:restart|volume|cp`.
- Placement of this candy is independent of `candy/plugin-deploy-pod`'s.

## Landing

Load `/charly-internals:git-workflow` before any git/PR action; it owns the
landing mechanics. The authoritative rulebook is the umbrella `AGENTS.md` in
`opencharly/opencharly` and `charly/AGENTS.md` in the charly repo.
