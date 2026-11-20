package convert

import (
	"regexp"
)

var devcontainerVar = regexp.MustCompile(`\$\{(localEnv|containerEnv|localWorkspaceFolder|localWorkspaceFolderBasename|containerWorkspaceFolder|containerWorkspaceFolderBasename|devcontainerId)(?::([A-Za-z_][A-Za-z0-9_]*))?(?::([^}]*))?\}`)

// substituteShell rewrites devcontainer `${…}` variables in a hook script
// and returns the ones it could not carry over. Host-side variables
// (localEnv, localWorkspaceFolder, devcontainerId) have no sandbox value: a
// kit is built once and run anywhere, so they become their stated default
// or empty, and the caller reports them.
func substituteShell(s string) (string, []string) {
	var unresolved []string
	out := devcontainerVar.ReplaceAllStringFunc(s, func(match string) string {
		parts := devcontainerVar.FindStringSubmatch(match)
		kind, name, fallback := parts[1], parts[2], parts[3]
		switch {
		case kind == "containerEnv" && name != "":
			return "${" + name + "}"
		case kind == "containerWorkspaceFolder":
			return "${WORKSPACE_DIR}"
		case kind == "containerWorkspaceFolderBasename":
			return `$(basename "${WORKSPACE_DIR}")`
		default:
			unresolved = append(unresolved, match)
			return fallback
		}
	})
	return out, unresolved
}
