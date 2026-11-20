package convert

import "strings"

// Egress presets: the hosts a toolchain's ordinary use reaches. Keyed by
// devcontainer feature id, and by a substring of the base image reference
// for the official language images. Extending coverage is a data change.
var (
	npmHosts    = []string{"registry.npmjs.org", "registry.yarnpkg.com", "nodejs.org"}
	pythonHosts = []string{"pypi.org", "files.pythonhosted.org"}
	goHosts     = []string{"proxy.golang.org", "sum.golang.org", "storage.googleapis.com"}
	rustHosts   = []string{"crates.io", "index.crates.io", "static.crates.io", "static.rust-lang.org"}
	javaHosts   = []string{"repo.maven.apache.org", "repo1.maven.org", "services.gradle.org", "plugins.gradle.org"}
	dockerHosts = []string{"registry-1.docker.io", "auth.docker.io", "index.docker.io", "production.cloudflare.docker.com"}
	githubHosts = []string{"github.com", "api.github.com", "uploads.github.com"}
	debianHosts = []string{"deb.debian.org"}
	ubuntuHosts = []string{"archive.ubuntu.com", "security.ubuntu.com", "ports.ubuntu.com"}
	alpineHosts = []string{"dl-cdn.alpinelinux.org"}
)

var featurePresets = map[string][]string{
	"node":             npmHosts,
	"python":           pythonHosts,
	"go":               goHosts,
	"rust":             rustHosts,
	"java":             javaHosts,
	"docker-in-docker": dockerHosts,
	"github-cli":       githubHosts,
}

var imagePresets = []struct {
	substring string
	hosts     []string
}{
	{"node", npmHosts},
	{"python", pythonHosts},
	{"/go:", goHosts},
	{"golang", goHosts},
	{"rust", rustHosts},
	{"/java:", javaHosts},
	{"openjdk", javaHosts},
	{"temurin", javaHosts},
	{"ubuntu", ubuntuHosts},
	{"debian", debianHosts},
	{"bookworm", debianHosts},
	{"bullseye", debianHosts},
	{"trixie", debianHosts},
	{"alpine", alpineHosts},
}

func presetsForFeature(id string) []string { return featurePresets[id] }

func presetsForImage(ref string) []string {
	ref = strings.ToLower(ref)
	var hosts []string
	for _, p := range imagePresets {
		if strings.Contains(ref, p.substring) {
			hosts = append(hosts, p.hosts...)
		}
	}
	return hosts
}
