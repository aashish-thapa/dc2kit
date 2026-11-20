# The project Dockerfile ran with its context, target and build args.
[ "$(greet)" = hello-from-build-args ] || fail "greet printed '$(greet)'; build.args did not reach the Dockerfile ARG"
gh --version >/dev/null || fail "feature installed on the project image is missing"
