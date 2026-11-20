# The renamed devcontainer user and its features, as the agent sees them.
[ "$(readlink /home/vscode)" = /home/agent ] || fail "old home /home/vscode does not resolve to /home/agent"
case "$(node --version)" in v22.*) ;; *) fail "node $(node --version), want v22 from the feature option" ;; esac
gh --version >/dev/null || fail "gh missing"
docker --version >/dev/null || fail "docker CLI missing"
id -nG | tr ' ' '\n' | grep -qx docker || fail "agent not in the docker group the feature configures"
[ "$LITERAL" = 'pa$$word' ] || fail "LITERAL is $LITERAL; a literal \$ was expanded"
[ "$TOOLS" = /home/agent/tools ] || fail "TOOLS is $TOOLS"
[ -n "${NVM_DIR:-}" ] || fail "feature containerEnv NVM_DIR missing"
