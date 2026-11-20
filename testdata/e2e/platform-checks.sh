# Checks every generated workload must pass inside its image, whatever the
# base: the identity and floor that sbx and published agent mixins assume.
# Fixture check.sh files are appended after this and may use `fail`.
set -euo pipefail
fail() { echo "FAIL: $*" >&2; exit 1; }

[ "$(id -u)" = 1000 ] || fail "uid is $(id -u), want 1000"
[ "$(id -un)" = agent ] || fail "user is $(id -un), want agent"
[ "$HOME" = /home/agent ] || fail "HOME is $HOME"
[ "$(pwd)" = /home/agent/workspace ] || fail "workdir is $(pwd)"
[ -O /home/agent ] && [ -O /home/agent/workspace ] || fail "agent does not own its home and workspace"
[ "$(awk -F: '$1 == "agent" { print $7 }' /etc/passwd)" = /bin/bash ] || fail "agent's login shell is not bash"
command -v git >/dev/null || fail "git missing"
sudo -n true || fail "passwordless sudo missing"
[ -s /etc/ssl/certs/ca-certificates.crt ] || [ -s /etc/pki/tls/certs/ca-bundle.crt ] || fail "CA store missing"
[ "$BASH_ENV" = /etc/sandbox-persistent.sh ] && [ -f "$BASH_ENV" ] || fail "BASH_ENV does not name a shipped file"
