# dc2kit

Turn a `devcontainer.json` into a [Docker Sandboxes](https://docs.docker.com/ai/sandboxes/) kit, so a coding agent runs in the same environment your editor already uses.

```console
$ dc2kit -o sbx-kit .
fetching feature ghcr.io/devcontainers/features/node:1
info  remoteUser: the sandbox runs as "agent", not "vscode": the image's uid-1000 account is renamed so agent mixins find /home/agent
warn  postAttachCommand: has no sandbox equivalent (it runs per attached editor session) and was dropped
...
Kit written to ./sbx-kit/

$ sbx run ./sbx-kit --kit docker/sbx-kit-claude-mixin:<version> .
```

The output is a v3 **workload** kit, made of a descriptor (`<name>.yaml`) and its companion Dockerfile. It is checked against Docker's own [`sandbox-kit-spec`](https://github.com/docker/sandbox-kit-spec) library before it is written, for both runtime selections of the agent-context capability. Compose it with any published agent mixin (Claude Code, Codex, Gemini, ...).

## Install

```console
git clone https://github.com/aashish-thapa/dc2kit && cd dc2kit
git clone --branch v3.0.0-m.9 https://github.com/docker/sandbox-kit-spec ../sandbox-kit-spec
go install ./cmd/dc2kit
```

The sibling checkout is temporary. Upstream ships a file named `spec/nul.go`, and Go's module system refuses to package it because `nul` is a reserved filename on Windows. Until that is fixed, `go get` cannot fetch the spec module, so `go.mod` uses a `replace` directive.

## What maps to what

| devcontainer.json | Kit |
|---|---|
| `image` | `FROM` in the companion Dockerfile |
| `build.dockerfile` | requires `--base-image <ref>`: build and push it first, because a kit's build context is the kit directory |
| `dockerComposeFile` | refused; a sandbox is one workload |
| `features` (OCI) | fetched at generation time, pinned by manifest digest, vendored under `features/`, and installed in order (`dependsOn`, `installsAfter`, `overrideFeatureInstallOrder`) the way the devcontainer CLI does it |
| `features` (`./local`) | read from beside `devcontainer.json` |
| feature `containerEnv` | `ENV` after that feature's install, with `${PATH}` expanding |
| `containerEnv`, `remoteEnv` | `ENV`. `$` stays literal; only `${containerEnv:X}` expands |
| `remoteUser`, `containerUser` | the image's uid-1000 account becomes `agent` with home `/home/agent`, the path published agent mixins write to. The old home stays as a symlink |
| `onCreateCommand`, `updateContentCommand`, `postCreateCommand` | `lifecycle@1` install hooks, run as `agent` from the workspace |
| `postStartCommand` | `lifecycle@1` startup hook |
| `postAttachCommand` | dropped, with a warning |
| `forwardPorts` | `port@1` (Compose `service:port` entries are skipped) |
| `hostRequirements` | `resources@1` |
| `privileged`, privileged features | `privileged@1` |
| `docker-in-docker` feature | `privileged@1` plus the label `sbx` uses to start the in-sandbox engine |
| `${containerWorkspaceFolder}` | `$WORKSPACE_DIR` in hooks, `/home/agent/workspace` in `ENV` |
| `${localEnv:…}`, `${localWorkspaceFolder}` | the stated default or empty, with a warning; a kit never reads the host |
| `mounts`, `capAdd`, `runArgs`, `customizations`, `initializeCommand` | reported, not translated |

Two sandbox rules shape the output and are easy to miss when writing a kit by hand:

- **Hooks see only declared variables.** `lifecycle@1` hides every env var a hook does not list, except PATH and HOME. A devcontainer command can see the whole container environment, so dc2kit lists every image-level name (a feature's `NVM_DIR`, your `containerEnv`) on each hook.
- **Egress is deny-by-default.** Devcontainers declare no network policy, so the runtime allow-list is built from presets for the installed toolchains (npm, PyPI, Go proxy, crates.io, Maven, Docker Hub, GitHub, distro mirrors), plus any `--allow` hosts. During the install phase, while setup commands run, egress is open by default. That phase closes before the agent starts; use `--install-egress=presets` to narrow it.

## Flags

```
-o DIR               output directory (default sbx-kit)
--name STEM          kit file stem (default: from the devcontainer name)
--base-image REF     build FROM this instead of devcontainer.json's image
--allow HOST         extra runtime egress host (repeatable)
--install-egress M   open (default) or presets
--source-url URL     sourceUrl recorded in the descriptor
--force              regenerate a directory dc2kit created earlier
```

`--force` only replaces a directory that carries dc2kit's `.dc2kit` marker file.

## Testing

```console
go test ./...        # unit tests; every conversion is validated by the upstream spec library
scripts/e2e.sh       # needs Docker: builds each testdata/e2e fixture through the
                     # sandbox-kit:3 frontend, checks it with upstream kit-tck, and
                     # runs platform checks inside the image as the agent user
```

The e2e fixtures cover a devcontainer image with features, plus Debian-slim, Alpine and Fedora bases, so the platform-floor script runs against apt, apk and dnf. CI runs both.

## Status

Experimental, like the kit format itself. v1 covers the common `image` + `features` case. Not yet supported:

- Dockerfile builds without a prebuilt base image
- Compose-based devcontainers
- private feature registries
- tarball-URL features
