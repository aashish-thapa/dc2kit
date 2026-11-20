# Compatibility

dc2kit v0.1.0-2-g405044a against the devcontainers of 28 public repos: **22 converted** to a kit that passes Docker's spec library.
Warnings name the devcontainer properties a sandbox cannot honor; dc2kit reports them instead of dropping them silently.
Regenerate with `scripts/compat.sh`.

| Repo | Result | Warnings | Warned on, or why refused |
|---|---|---|---|
| [Azure/azure-sdk-for-python](https://github.com/Azure/azure-sdk-for-python/tree/7730a9b/.devcontainer) | refused | 0 | devcontainer.json names no "image", "build", or "dockerComposeFile" (Codespaces fills in its default image); pass --base-image, for example --base-image mcr.mic |
| [apache/airflow](https://github.com/apache/airflow/tree/7055657/.devcontainer) | refused | 0 | dockerComposeFile devcontainers are not supported: a sandbox is one workload, not a Compose project. Point dc2kit at a devcontainer.json that uses "image" or "b |
| [apache/superset](https://github.com/apache/superset/tree/0f516a5/.devcontainer) | converted | 3 | features.ghcr.io/devcontainers/features/docker-in-docker, features.ghcr.io/devcontainers/features/sshd |
| [astral-sh/ruff](https://github.com/astral-sh/ruff/tree/36f6404/.devcontainer) | converted | 1 | mounts |
| [cli/cli](https://github.com/cli/cli/tree/de06af2/.devcontainer) | converted | 2 | features.ghcr.io/devcontainers/features/sshd, runArgs |
| [containerd/containerd](https://github.com/containerd/containerd/tree/8abc4c7/.devcontainer) | converted | 6 | features.ghcr.io/devcontainers/features/docker-in-docker, features.ghcr.io/devcontainers/features/go, postAttachCommand, remoteUser, runArgs |
| [denoland/deno](https://github.com/denoland/deno/tree/d8c551b/.devcontainer) | converted | 1 | runArgs |
| [discourse/discourse](https://github.com/discourse/discourse/tree/2f9615c/.devcontainer) | converted | 3 | mounts |
| [dotnet/aspnetcore](https://github.com/dotnet/aspnetcore/tree/d348398/.devcontainer) | converted | 1 | features.ghcr.io/devcontainers/features/sshd |
| [dotnet/runtime](https://github.com/dotnet/runtime/tree/be2de60/.devcontainer) | converted | 1 | features.ghcr.io/devcontainers/features/sshd |
| [github/docs](https://github.com/github/docs/tree/7b5450f/.devcontainer) | converted | 5 | features.ghcr.io/devcontainers/features/docker-in-docker, features.ghcr.io/devcontainers/features/sshd, features.sshd, postAttachCommand |
| [github/linguist](https://github.com/github/linguist/tree/cfd7a01/.devcontainer) | converted | 4 | features.ghcr.io/devcontainers/features/docker-in-docker, features.ghcr.io/devcontainers/features/go, features.ghcr.io/devcontainers/features/rust |
| [home-assistant/core](https://github.com/home-assistant/core/tree/f98ff1e/.devcontainer) | converted | 1 | runArgs |
| [jupyterlab/jupyterlab](https://github.com/jupyterlab/jupyterlab/tree/adcb6f8/.devcontainer) | converted | 0 |  |
| [langchain-ai/langchain](https://github.com/langchain-ai/langchain/tree/e8a78d5/.devcontainer) | refused | 0 | dockerComposeFile devcontainers are not supported: a sandbox is one workload, not a Compose project. Point dc2kit at a devcontainer.json that uses "image" or "b |
| [mastodon/mastodon](https://github.com/mastodon/mastodon/tree/9bd7031/.devcontainer) | refused | 0 | dockerComposeFile devcontainers are not supported: a sandbox is one workload, not a Compose project. Point dc2kit at a devcontainer.json that uses "image" or "b |
| [microsoft/TypeScript](https://github.com/microsoft/TypeScript/tree/f5a247c/.devcontainer) | converted | 0 |  |
| [microsoft/fluentui](https://github.com/microsoft/fluentui/tree/a568b4d/.devcontainer) | converted | 1 | containerEnv.DISPLAY |
| [microsoft/semantic-kernel](https://github.com/microsoft/semantic-kernel/tree/740c94b/.devcontainer) | converted | 0 |  |
| [microsoft/vscode](https://github.com/microsoft/vscode/tree/0d15d4d/.devcontainer) | converted | 4 | features.ghcr.io/devcontainers/features/desktop-lite, features.ghcr.io/devcontainers/features/rust, features.ghcr.io/devcontainers/features/sshd, mounts |
| [microsoft/vscode-python](https://github.com/microsoft/vscode-python/tree/4ddb282/.devcontainer) | converted | 0 |  |
| [moby/moby](https://github.com/moby/moby/tree/53a2a23/.devcontainer) | converted | 2 | remoteUser, runArgs |
| [n8n-io/n8n](https://github.com/n8n-io/n8n/tree/a3fbb2e/.devcontainer) | refused | 0 | dockerComposeFile devcontainers are not supported: a sandbox is one workload, not a Compose project. Point dc2kit at a devcontainer.json that uses "image" or "b |
| [python/cpython](https://github.com/python/cpython/tree/e0d0f2e/.devcontainer) | converted | 0 |  |
| [rails/rails](https://github.com/rails/rails/tree/655d4ad/.devcontainer) | refused | 0 | dockerComposeFile devcontainers are not supported: a sandbox is one workload, not a Compose project. Point dc2kit at a devcontainer.json that uses "image" or "b |
| [scikit-learn/scikit-learn](https://github.com/scikit-learn/scikit-learn/tree/5e9184e/.devcontainer) | converted | 0 |  |
| [streamlit/streamlit](https://github.com/streamlit/streamlit/tree/4837b52/.devcontainer) | converted | 0 |  |
| [vercel/next.js](https://github.com/vercel/next.js/tree/fe94c9de/.devcontainer) | converted | 1 | features.ghcr.io/devcontainers/features/rust |
