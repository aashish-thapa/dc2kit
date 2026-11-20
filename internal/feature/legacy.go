package feature

// legacyIDs maps the bare Feature IDs of microsoft/vscode-dev-containers to
// their ghcr.io/devcontainers/features names. The devcontainer CLI still
// accepts these (getBackwardCompatibleFeatureId) and pins them to major
// version 1, so configs in the wild keep using them.
var legacyIDs = map[string]string{
	"aws-cli":               "aws-cli",
	"azure-cli":             "azure-cli",
	"desktop-lite":          "desktop-lite",
	"docker-in-docker":      "docker-in-docker",
	"docker-from-docker":    "docker-from-docker",
	"dotnet":                "dotnet",
	"git":                   "git",
	"git-lfs":               "git-lfs",
	"github-cli":            "github-cli",
	"java":                  "java",
	"kubectl-helm-minikube": "kubectl-helm-minikube",
	"node":                  "node",
	"powershell":            "powershell",
	"python":                "python",
	"ruby":                  "ruby",
	"rust":                  "rust",
	"sshd":                  "sshd",
	"terraform":             "terraform",
	"golang":                "go",
	"common":                "common-utils",
}

// LegacyRef returns the OCI reference the devcontainer CLI installs for a
// legacy bare Feature ID, and whether ref was one.
func LegacyRef(ref string) (string, bool) {
	id, ok := legacyIDs[ref]
	if !ok {
		return "", false
	}
	return "ghcr.io/devcontainers/features/" + id + ":1", true
}
