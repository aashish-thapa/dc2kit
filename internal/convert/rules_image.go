package convert

import (
	"fmt"
	"regexp"
	"sort"
	"strings"

	"github.com/aashish-thapa/dc2kit/internal/devcontainer"
	"github.com/aashish-thapa/dc2kit/internal/kit"
)

// baseImageRule picks the image the workload builds FROM.
type baseImageRule struct{}

func (baseImageRule) apply(in Input, d *draft, report *Report) error {
	cfg := in.Config
	switch {
	case len(cfg.ComposeFiles) > 0:
		return fmt.Errorf("dockerComposeFile devcontainers are not supported: a sandbox is one workload, not a Compose project. " +
			"Point dc2kit at a devcontainer.json that uses \"image\" or \"build\", or pass --base-image with the service image")
	case in.Options.BaseImage != "":
		d.image.BaseImage = in.Options.BaseImage
		if cfg.Build != nil {
			report.info("build", "using --base-image %s in place of building %s", in.Options.BaseImage, cfg.Build.Dockerfile)
		}
	case cfg.Build != nil && in.Options.UserDockerfile != nil:
		d.image.Prelude = in.Options.UserDockerfile
		d.image.BaseImage = in.Options.UserDockerfile.Stage
		if err := declareBuildArgs(cfg.Build.Args, d, report); err != nil {
			return err
		}
		return nil
	case cfg.Build != nil:
		return fmt.Errorf("devcontainer.json builds from %s, which was not supplied; "+
			"or build and push it yourself and rerun with --base-image <ref>", cfg.Build.Dockerfile)
	default:
		d.image.BaseImage = cfg.Image
	}
	if !strings.Contains(d.image.BaseImage, "@sha256:") {
		report.info("image", "%s is a tag, so rebuilding the kit can pick up a different image; pin a digest for reproducible kits", d.image.BaseImage)
	}
	return nil
}

var identifier = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`)

// declareBuildArgs turns build.args into kit args delivered as build args,
// so the values devcontainer.json sets stay the defaults and an installer
// can still override them. The kit spec admits only identifier build-arg
// names, narrower than Dockerfile ARG; there is no way to pass any other
// name through a kit, so such a name is refused rather than renamed.
func declareBuildArgs(args []devcontainer.EnvVar, d *draft, report *Report) error {
	for _, a := range args {
		if !identifier.MatchString(a.Name) {
			return fmt.Errorf("build.args %q: kits pass only build args named like identifiers ([A-Za-z_][A-Za-z0-9_]*); rename the ARG in the Dockerfile", a.Name)
		}
		value, unresolved := substituteShell(a.Value)
		for _, u := range unresolved {
			report.warn("build.args."+a.Name, "%s has no value inside a sandbox and was replaced with its default or an empty string", u)
		}
		if d.args == nil {
			d.args = map[string]kit.Arg{}
		}
		d.args[a.Name] = kit.Arg{Default: value, BuildArg: a.Name, Description: "devcontainer build arg " + a.Name}
	}
	return nil
}

// featureRule turns each resolved feature into an install step.
type featureRule struct{}

var unsafeDirChars = regexp.MustCompile(`[^a-z0-9._-]+`)

func (featureRule) apply(in Input, d *draft, report *Report) error {
	for i, f := range in.Features {
		installEnv, err := f.InstallEnv()
		if err != nil {
			return err
		}
		installEnv = append(installEnv,
			devcontainer.EnvVar{Name: "_REMOTE_USER", Value: AgentUser},
			devcontainer.EnvVar{Name: "_REMOTE_USER_HOME", Value: AgentHome},
			devcontainer.EnvVar{Name: "_CONTAINER_USER", Value: AgentUser},
			devcontainer.EnvVar{Name: "_CONTAINER_USER_HOME", Value: AgentHome},
		)

		containerEnv := make([]ImageEnv, 0, len(f.Metadata.ContainerEnv))
		for _, name := range sortedKeys(f.Metadata.ContainerEnv) {
			value := featureEnvValue(f.Metadata.ContainerEnv[name])
			if value.containsNewline() {
				return fmt.Errorf("feature %s containerEnv %s contains a newline, which a Dockerfile ENV cannot carry", f.Metadata.ID, name)
			}
			containerEnv = append(containerEnv, ImageEnv{Name: name, Value: value})
		}

		id := unsafeDirChars.ReplaceAllString(strings.ToLower(f.Metadata.ID), "-")
		d.image.Features = append(d.image.Features, FeatureStep{
			Dir:          fmt.Sprintf("%02d-%s", i+1, id),
			Ref:          f.Request.Ref,
			Pin:          f.Pin,
			InstallEnv:   installEnv,
			ContainerEnv: containerEnv,
			Files:        f.Files,
		})

		field := "features." + f.Request.Ref
		if f.Metadata.Entrypoint != "" {
			report.warn(field, "declares entrypoint %s; sandboxes launch the agent themselves, so it never runs", f.Metadata.Entrypoint)
		}
		if len(f.Metadata.Mounts) > 0 {
			report.warn(field, "declares %d mount(s), which are not translated", len(f.Metadata.Mounts))
		}
		if len(f.Metadata.CapAdd) > 0 {
			report.warn(field, "asks for capabilities %s, which sandboxes do not grant per kit", strings.Join(f.Metadata.CapAdd, ", "))
		}
		if f.Metadata.Privileged {
			d.privileged = append(d.privileged, fmt.Sprintf("Feature %s requires a privileged container.", f.Metadata.ID))
		}
		if isDockerInDocker(f.Metadata.ID) {
			d.image.StartsDocker = true
			report.info(field, "the sandbox runtime starts its own Docker engine; the feature's dockerd entrypoint is not used")
		}
	}
	return nil
}

func isDockerInDocker(id string) bool {
	return id == "docker-in-docker"
}

// envRule carries containerEnv and remoteEnv into the image config. The
// sandbox runtime has no per-session env layer to map remoteEnv onto, and
// the image config is what every hook and the agent start from.
type envRule struct{}

func (envRule) apply(in Input, d *draft, report *Report) error {
	for _, source := range []struct {
		field string
		vars  []devcontainer.EnvVar
	}{
		{"containerEnv", in.Config.ContainerEnv},
		{"remoteEnv", in.Config.RemoteEnv},
	} {
		for _, v := range source.vars {
			value, unresolved := userEnvValue(v.Value)
			if value.containsNewline() {
				return fmt.Errorf("%s.%s contains a newline, which a Dockerfile ENV cannot carry", source.field, v.Name)
			}
			for _, u := range unresolved {
				report.warn(source.field+"."+v.Name, "%s has no value inside a sandbox and was replaced with its default or an empty string", u)
			}
			d.image.Env = append(d.image.Env, ImageEnv{Name: v.Name, Value: value})
		}
	}
	if len(in.Config.RemoteEnv) > 0 {
		report.info("remoteEnv", "set in the image environment, so install hooks see it too (the devcontainer CLI applies it only to attached tools)")
	}
	return nil
}

// userRule explains the identity change: whatever user the devcontainer
// ran as, the sandbox runs `agent`.
type userRule struct{}

func (userRule) apply(in Input, _ *draft, report *Report) error {
	for _, u := range []struct{ field, name string }{
		{"remoteUser", in.Config.RemoteUser},
		{"containerUser", in.Config.ContainerUser},
	} {
		switch u.name {
		case "", AgentUser:
		case "root", "0":
			report.warn(u.field, "the devcontainer runs as root; the sandbox runs as %q (uid 1000) with passwordless sudo", AgentUser)
		default:
			report.info(u.field, "the sandbox runs as %q, not %q: the image's uid-1000 account is renamed so agent mixins find %s", AgentUser, u.name, AgentHome)
		}
	}
	return nil
}

func sortedKeys(m map[string]string) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}
