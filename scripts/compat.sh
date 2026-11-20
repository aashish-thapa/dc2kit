#!/usr/bin/env bash
# Convert the devcontainer of each repo in scripts/compat-repos.txt and write
# the results to docs/compatibility.md. Only .devcontainer/, Dockerfiles and
# .dockerignore files are checked out: what dc2kit reads.
# Every kit dc2kit writes has already passed Docker's spec library, so a
# "converted" row means a spec-valid kit; it does not build or run the kit.
#
# Usage: scripts/compat.sh
# Needs Go and git; fetches Features from their registries.
set -euo pipefail

root=$(cd "$(dirname "$0")/.." && pwd)
work=$(mktemp -d)
trap 'rm -rf "$work"' EXIT

(cd "$root" && go build -o "$work/dc2kit" ./cmd/dc2kit)
version=$(git -C "$root" describe --tags --always)

rows=()
converted=0 total=0
while read -r repo; do
  case "$repo" in ''|'#'*) continue ;; esac
  total=$((total + 1))
  dir=$work/repos/$repo
  echo "=== $repo" >&2
  git clone --quiet --depth 1 --filter=blob:none --sparse "https://github.com/$repo" "$dir"
  git -C "$dir" sparse-checkout set --no-cone /.devcontainer/ 'Dockerfile*' '*.Dockerfile' '*.dockerfile' .dockerignore
  sha=$(git -C "$dir" rev-parse --short HEAD)

  set +e
  (cd "$dir" && "$work/dc2kit" --name kit .) >/dev/null 2>"$work/report.txt"
  status=$?
  set -e
  warnings=$(grep -c '^warn' "$work/report.txt" || true)
  if [ "$status" -eq 0 ]; then
    converted=$((converted + 1))
    result="converted"
    notes=$(grep '^warn' "$work/report.txt" | sed -E 's/^warn +([^:]+):.*/\1/' | sort -u | paste -sd, - | sed 's/,/, /g' || true)
  else
    result="refused"
    notes=$(grep '^dc2kit:' "$work/report.txt" | head -1 | sed -E 's/^dc2kit: //' | cut -c1-160 || true)
  fi
  notes=${notes//|/\\|}
  rows+=("| [$repo](https://github.com/$repo/tree/$sha/.devcontainer) | $result | $warnings | ${notes:-} |")
done <"$root/scripts/compat-repos.txt"

{
  echo "# Compatibility"
  echo
  echo "dc2kit $version against the devcontainers of $total public repos: **$converted converted** to a kit that passes Docker's spec library."
  echo "Warnings name the devcontainer properties a sandbox cannot honor; dc2kit reports them instead of dropping them silently."
  echo "Regenerate with \`scripts/compat.sh\`."
  echo
  echo "| Repo | Result | Warnings | Warned on, or why refused |"
  echo "|---|---|---|---|"
  printf '%s\n' "${rows[@]}"
} >"$root/docs/compatibility.md"
echo "converted $converted of $total" >&2
