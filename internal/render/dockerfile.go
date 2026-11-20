// Package render writes a conversion plan to disk as a kit directory: the
// descriptor, its companion Dockerfile, the platform-floor script, and the
// vendored feature payloads.
package render

import (
	"fmt"
	"strings"

	"github.com/aashish-thapa/dc2kit/internal/convert"
)

const (
	floorScript     = "sbx-floor.sh"
	floorInImage    = "/usr/local/share/dc2kit/" + floorScript
	featuresDir     = "features"
	featureEnvFile  = "devcontainer-features.env"
	featureStageDir = "/tmp/dc2kit-features"
	persistentEnv   = "/etc/sandbox-persistent.sh"
)

// Dockerfile renders the companion recipe for a plan. With a Prelude, the
// project's own Dockerfile comes first, verbatim apart from naming its
// stage, and the recipe builds on that stage.
func Dockerfile(plan convert.ImagePlan, layout Layout, provenance string) string {
	var b strings.Builder
	if plan.Prelude != nil {
		for _, line := range plan.Prelude.Header {
			b.WriteString(line + "\n")
		}
	}
	if plan.Prelude == nil || !plan.Prelude.HasSyntax() {
		b.WriteString("# syntax=docker/dockerfile:1\n")
	}
	for _, line := range strings.Split(provenance, "\n") {
		b.WriteString("# " + line + "\n")
	}
	if plan.Prelude != nil {
		b.WriteString("\n# --- the project's devcontainer Dockerfile ---\n")
		b.WriteString(plan.Prelude.Body)
		b.WriteString("# --- end of the project's Dockerfile ---\n\n")
	}
	fmt.Fprintf(&b, "FROM %s\n\n", plan.BaseImage)

	b.WriteString("USER root\n")
	fmt.Fprintf(&b, "COPY --chmod=0755 %s %s\n", layout.contextPath(floorScript), floorInImage)
	fmt.Fprintf(&b, "RUN %s\n", floorInImage)

	for _, step := range plan.Features {
		stage := featureStageDir + "/" + step.Dir
		fmt.Fprintf(&b, "\n# Feature %s (%s)\n", step.Ref, step.Pin)
		fmt.Fprintf(&b, "COPY %s %s\n", layout.contextPath(featuresDir+"/"+step.Dir), stage)
		fmt.Fprintf(&b, "RUN cd %s \\\n", stage)
		b.WriteString(" && chmod +x install.sh \\\n")
		fmt.Fprintf(&b, " && set -a && . ./%s && set +a \\\n", featureEnvFile)
		b.WriteString(" && ./install.sh \\\n")
		fmt.Fprintf(&b, " && rm -rf %s\n", stage)
		writeEnv(&b, step.ContainerEnv)
	}

	if len(plan.Env) > 0 {
		b.WriteString("\n")
		writeEnv(&b, plan.Env)
	}

	b.WriteString("\n")
	fmt.Fprintf(&b, "ENV BASH_ENV=%s\n", persistentEnv)
	if plan.StartsDocker {
		// sbx releases before dockerd@1 start the inner engine on this label.
		b.WriteString("LABEL com.docker.sandboxes.start-docker=\"true\"\n")
	}
	fmt.Fprintf(&b, "USER %s\n", convert.AgentUser)
	fmt.Fprintf(&b, "WORKDIR %s\n", convert.WorkspaceDir)
	b.WriteString("ENTRYPOINT [\"bash\"]\n")
	b.WriteString("CMD [\"-l\"]\n")
	return b.String()
}

func writeEnv(b *strings.Builder, env []convert.ImageEnv) {
	for _, v := range env {
		fmt.Fprintf(b, "ENV %s=\"%s\"\n", v.Name, dockerfileValue(v.Value))
	}
}

// dockerfileValue renders an ENV value for a double-quoted Dockerfile word:
// literal text has its quote, backslash and dollar escaped, references are
// emitted for the builder to expand.
func dockerfileValue(value convert.EnvValue) string {
	var b strings.Builder
	for _, part := range value {
		if part.IsRef() {
			b.WriteString("${" + part.Text() + "}")
			continue
		}
		for _, r := range part.Text() {
			if r == '\\' || r == '"' || r == '$' {
				b.WriteRune('\\')
			}
			b.WriteRune(r)
		}
	}
	return b.String()
}
