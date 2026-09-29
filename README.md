# plugin-pod

The pod-lifecycle CLI for OpenCharly — `charly start` / `stop` / `restart` /
`config` / `shell` / `service` / `logs` / `remove` / `cp` / `volume` / `update`.

The plugin is a command-class plugin on the charly SDK. It is the CLI sibling of
`candy/plugin-deploy-pod` (which keeps `deploy:pod` out-of-process, untouched) —
mirroring the `candy/plugin-vm` / `candy/plugin-deploy-vm` split. Each command
word is independent (no shared parent).

## What it provides

| Capability | Surface |
|---|---|
| `command:start` / `stop` / `restart` | pod lifecycle |
| `command:config` / `shell` / `service` / `logs` | pod access and inspection |
| `command:remove` / `cp` / `volume` / `update` | pod maintenance |

`charly restart` is pure `sdk/kit` + `sdk/deploykit` logic
(`deploykit.RestartPodService`) with zero host coupling. The registry-bound
commands (`start`/`stop`/…) forward their authored flags, as `sdk/spec` wire
requests, to a per-command host-build seam — the host reconstructs the original
core orchestration struct and runs its `Run()` logic verbatim, because the
provider registry is a core mechanism a plugin cannot import or hold. Compiled-in,
it dispatches in-proc via `Invoke(OpRun)` and inherits real stdio/TTY.

## How to use it

```bash
charly start my-app
charly restart my-app -i second
charly shell my-app
charly cp my-app ./config.toml :/etc/app/config.toml
charly volume list my-app
```

## Layout

- `candy/plugin-pod/` — the plugin module: `plugin.go`, `command.go`,
  `pod_cmd.go`, `enc_cmd.go`, `host_seams.go`, `remove_orchestration.go`,
  `remove_tunnel.go`, `service_resolve.go`, `schema/pod.cue`, and
  `cmd/serve/main.go`.
- `charly.yml` — the root project manifest (`discover: candy` + `discover: box`)
  plus the `check-pod` disposable R10 bed.
- `.github/workflows/tag-on-merge.yml` — CalVer tag + `CHANGELOG/` on merge.
- The candy embeds three skill entities — `restart-skill:`, `volume-skill:`, and
  `cp-skill:` — the projected sources for `/charly-pod-verbs:restart`,
  `/charly-pod-verbs:volume`, and `/charly-pod-verbs:cp`.

## R10 witness


`charly check run check-pod` — the canonical fast pod-runtime bed, proving
`kind:box` build, `kind:candy` composition order, `kind:pod` runtime, and
deploy-target rendering in one run.

## Related

- Owning skills: `/charly-pod-verbs:restart`, `/charly-pod-verbs:volume`,
  `/charly-pod-verbs:cp` (projected from this candy's own `skill:` entities).
- `/charly-core:start` / `/charly-core:stop` / `/charly-core:shell` — the
  command reference (owned by `pod-charly-core`).
- `/charly-internals:plugin` — the plugin/provider model.
- [`opencharly/charly`](https://github.com/opencharly/charly) — the charly CLI.
