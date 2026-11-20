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

Download a binary for Linux, macOS or Windows from [Releases](https://github.com/aashish-thapa/dc2kit/releases), or:

```console
go install github.com/aashish-thapa/dc2kit/cmd/dc2kit@latest
```

dc2kit tracks the kit spec release named in `go.mod` (currently `v3.0.0-m.13`).

## What maps to what

| devcontainer.json | Kit |
|---|---|
| `image` | `FROM` in the companion Dockerfile |
| `build.dockerfile`, `build.context` | the kit is written **into the build context**: the descriptor at its root, dc2kit's files under `.sbx-kit/`. Your Dockerfile becomes the first stage, so its `COPY` paths keep working. `--base-image <ref>` builds on a prebuilt image instead |
| `build.target` | the stage the kit builds on (default: the last stage) |
| `build.args` | kit args with those defaults, passed as build args; names must be identifiers, a kit-spec rule |
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
| `docker-in-docker` feature | `dockerd@1` (optional), with `privileged@1` and the `start-docker` label as the fallback older `sbx` releases use |
| `${containerWorkspaceFolder}` | `$WORKSPACE_DIR` in hooks, `/home/agent/workspace` in `ENV` |
| `${localEnv:…}`, `${localWorkspaceFolder}` | the stated default or empty, with a warning; a kit never reads the host |
| `mounts`, `capAdd`, `runArgs`, `customizations`, `initializeCommand` | reported, not translated |

Two sandbox rules shape the output and are easy to miss when writing a kit by hand:

- **Hooks see only declared variables.** `lifecycle@1` hides every env var a hook does not list, except PATH and HOME. A devcontainer command can see the whole container environment, so dc2kit lists every image-level name (a feature's `NVM_DIR`, your `containerEnv`) on each hook.
- **Egress is deny-by-default.** Devcontainers declare no network policy, so the runtime allow-list is built from presets for the installed toolchains (npm, PyPI, Go proxy, crates.io, Maven, Docker Hub, GitHub, distro mirrors), plus any `--allow` hosts. During the install phase, while setup commands run, egress is open by default. That phase closes before the agent starts; use `--install-egress=presets` to narrow it.

## GitHub Action

Keep a committed kit in step with `devcontainer.json`. With `check: true` the job fails when regenerating would change the kit, for example after someone edits the devcontainer or a feature publishes a new digest:

```yaml
- uses: actions/checkout@v7
- uses: aashish-thapa/dc2kit@v0
  with:
    args: -o sbx-kit
    check: true
```

Inputs: `path` (default `.`), `args`, `check` (default `false`), `version` (a release tag, default `latest`). Runs on Linux and macOS runners.

## Flags

```
-o DIR               output directory for image-based configs (default sbx-kit)
--name STEM          kit file stem (default: from the devcontainer name)
--base-image REF     build FROM this instead of devcontainer.json's image
--allow HOST         extra runtime egress host (repeatable)
--install-egress M   open (default) or presets
--source-url URL     sourceUrl recorded in the descriptor
--force              regenerate a directory dc2kit created earlier
--version            print the version
```

`--force` only replaces a directory that carries dc2kit's `.dc2kit` marker file.

## Testing

```console
go test ./...        # unit tests; every conversion is validated by the upstream spec library
scripts/e2e.sh       # needs Docker: builds each testdata/e2e fixture through the
                     # sandbox-kit:3 frontend, checks it with upstream kit-tck, and
                     # runs platform checks inside the image as the agent user
```

The e2e fixtures cover a devcontainer image with features; a project Dockerfile with a parent build context, a build target, and build args; and Debian-slim, Alpine and Fedora bases, so the platform-floor script runs against apt, apk and dnf. CI runs both.

## Status

Experimental, like the kit format itself. Not yet supported:

- Compose-based devcontainers
- private feature registries
- tarball-URL features
