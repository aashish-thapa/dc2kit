#!/usr/bin/env bash
# End-to-end check of generated kits. For each fixture under testdata/e2e:
# convert it, build the kit through the docker/sandbox-kit:3 frontend, check
# the built layout with upstream's kit-tck, and run the platform checks plus
# the fixture's own check.sh inside the image as the agent user.
#
# Usage: scripts/e2e.sh [fixture-dir ...]
# Needs Docker with buildx, and Go. kit-tck is a go.mod tool, so it runs at
# the same spec version dc2kit validates against.
set -euo pipefail

root=$(cd "$(dirname "$0")/.." && pwd)
builder=${BUILDER:-dc2kit-e2e}
fixtures_dir=$root/testdata/e2e

work=$(mktemp -d)
trap 'rm -rf "$work"' EXIT

# The docker driver drops the kit annotations on export; kits need the
# container driver.
if ! docker buildx inspect "$builder" >/dev/null 2>&1; then
  docker buildx create --name "$builder" --driver docker-container --bootstrap >/dev/null
fi

(cd "$root" && go build -o "$work/dc2kit" ./cmd/dc2kit)
(cd "$root" && go build -o "$work/kit-tck" github.com/docker/sandbox-kit-spec/v3/cmd/kit-tck)

if [ "$#" -gt 0 ]; then
  fixtures=("$@")
else
  fixtures=("$fixtures_dir"/*/)
fi

run_fixture() {
  local fixture=$1 name=$2 out=$3 digest image checks
  "$work/dc2kit" -o "$out/kit" --name "$name" "$fixture" >"$out/next-steps.txt" 2>"$out/report.txt"

  docker buildx build --builder "$builder" --quiet \
    -f "$out/kit/$name.yaml" \
    --output "type=oci,dest=$out/layout,tar=false" \
    --metadata-file "$out/metadata.json" \
    "$out/kit" >/dev/null
  digest=$(python3 -c 'import json,sys; print(json.load(open(sys.argv[1]))["containerimage.digest"])' "$out/metadata.json")
  "$work/kit-tck" validate --layout "$out/layout" "$digest"

  image=dc2kit-e2e:$name
  docker buildx build --builder "$builder" --quiet -f "$out/kit/$name.yaml" -t "$image" --load "$out/kit" >/dev/null
  checks=("$fixtures_dir/platform-checks.sh")
  [ -f "$fixture/check.sh" ] && checks+=("$fixture/check.sh")
  cat "${checks[@]}" | docker run --rm -i "$image" -s
  docker image rm "$image" >/dev/null
}

failed=()
for fixture in "${fixtures[@]}"; do
  fixture=${fixture%/}
  name=$(basename "$fixture")
  out=$work/$name
  mkdir -p "$out"
  echo "=== $name"

  # A subshell outside any `if` keeps errexit live inside run_fixture; bash
  # suspends it for functions called from a condition.
  set +e
  (set -e; run_fixture "$fixture" "$name" "$out")
  status=$?
  set -e
  if [ "$status" -eq 0 ]; then
    echo "ok   $name"
  else
    echo "FAIL $name (conversion report: $out/report.txt)" >&2
    cat "$out/report.txt" >&2 || true
    failed+=("$name")
  fi
done

if [ "${#failed[@]}" -gt 0 ]; then
  echo "e2e failed: ${failed[*]}" >&2
  exit 1
fi
echo "e2e passed: ${#fixtures[@]} fixture(s)"
