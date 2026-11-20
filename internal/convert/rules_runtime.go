package convert

import (
	"fmt"
	"strings"

	"github.com/aashish-thapa/dc2kit/internal/devcontainer"
	"github.com/aashish-thapa/dc2kit/internal/kit"
)

// lifecycleRule maps devcontainer lifecycle commands onto lifecycle@1.
// onCreate, updateContent and postCreate all run once when the container is
// created, in that order, so they become install hooks; postStart runs on
// every start, which is exactly a startup hook.
type lifecycleRule struct{}

func (lifecycleRule) apply(in Input, d *draft, report *Report) error {
	env := d.hookEnv()
	cfg := in.Config
	slots := []struct {
		field     string
		cmd       devcontainer.Command
		isStartup bool
	}{
		{"onCreateCommand", cfg.OnCreate, false},
		{"updateContentCommand", cfg.UpdateContent, false},
		{"postCreateCommand", cfg.PostCreate, false},
		{"postStartCommand", cfg.PostStart, true},
	}
	for _, slot := range slots {
		hooks := hooksFor(slot.field, slot.cmd, env, report)
		if slot.isStartup {
			d.lifecycle.Startup = append(d.lifecycle.Startup, hooks...)
		} else {
			d.lifecycle.Install = append(d.lifecycle.Install, hooks...)
		}
		if len(slot.cmd.Steps) > 1 {
			report.info(slot.field, "its %d named commands run one after another; the devcontainer CLI runs them in parallel", len(slot.cmd.Steps))
		}
	}
	if !cfg.PostAttach.IsEmpty() {
		report.warn("postAttachCommand", "has no sandbox equivalent (it runs per attached editor session) and was dropped; move it to postStartCommand if it must run")
	}
	return nil
}

func hooksFor(field string, cmd devcontainer.Command, env []string, report *Report) []kit.Hook {
	hooks := make([]kit.Hook, 0, len(cmd.Steps))
	for _, step := range cmd.Steps {
		script, unresolved := substituteShell(step.Script)
		for _, u := range unresolved {
			report.warn(field, "%s has no value inside a sandbox and was replaced with its default or an empty string", u)
		}
		label := field
		if step.Name != "" {
			label = field + ": " + step.Name
		}
		hooks = append(hooks, kit.Hook{
			// Devcontainer commands run from the workspace folder; a
			// sandbox may have no workspace, so fall back to home.
			Command:     "cd \"${WORKSPACE_DIR:-$HOME}\" || exit 1\n" + script,
			User:        AgentUser,
			Env:         env,
			Description: label,
		})
	}
	return hooks
}

// portRule publishes forwardPorts with port@1.
type portRule struct{}

func (portRule) apply(in Input, d *draft, _ *Report) error {
	seen := map[int]bool{}
	for _, port := range in.Config.ForwardPorts {
		if !seen[port] {
			seen[port] = true
			d.ports = append(d.ports, port)
		}
	}
	return nil
}

// resourceRule turns hostRequirements into resources@1 defaults.
type resourceRule struct{}

func (resourceRule) apply(in Input, d *draft, _ *Report) error {
	hr := in.Config.HostRequirements
	if hr.CPUs == 0 && hr.Memory == "" {
		return nil
	}
	d.resources = &kit.ResourcesConfig{CPU: float64(hr.CPUs), Memory: hr.Memory}
	return nil
}

// privilegeRule carries an explicit privileged request and reports the
// container-level knobs a kit has no field for.
type privilegeRule struct{}

func (privilegeRule) apply(in Input, d *draft, report *Report) error {
	cfg := in.Config
	if cfg.Privileged {
		d.privileged = append(d.privileged, "devcontainer.json sets \"privileged\": true.")
	}
	if d.image.StartsDocker && len(d.privileged) == 0 {
		d.privileged = append(d.privileged, "Runs nested containers via the in-sandbox Docker engine.")
	}
	if len(cfg.CapAdd) > 0 {
		report.warn("capAdd", "%s not granted: kits cannot request individual Linux capabilities", strings.Join(cfg.CapAdd, ", "))
	}
	for _, m := range cfg.Mounts {
		report.warn("mounts", "%q not translated: sandboxes manage the workspace mount themselves", m)
	}
	if len(cfg.RunArgs) > 0 {
		report.warn("runArgs", "%q not translated: there is no docker run command line to pass them to", strings.Join(cfg.RunArgs, " "))
	}
	return nil
}

// egressRule builds network-policy@1. Devcontainers declare no egress at
// all, so the runtime phase is assembled from presets for what is known
// to be installed, plus hosts the user names.
type egressRule struct{}

func (egressRule) apply(in Input, d *draft, report *Report) error {
	for _, f := range in.Features {
		d.runtimeAllow.add(presetsForFeature(f.Metadata.ID)...)
	}
	if d.image.Prelude == nil {
		d.runtimeAllow.add(presetsForImage(d.image.BaseImage)...)
	}
	d.runtimeAllow.add(in.Options.RuntimeAllow...)

	if len(d.lifecycle.Install) == 0 {
		return nil
	}
	switch in.Options.InstallEgress {
	case InstallEgressOpen:
		d.installAllow.add("**")
		report.info("network", "install hooks get unrestricted egress; the phase closes before the agent starts (use --install-egress=presets to narrow it)")
	case InstallEgressPresets:
		d.installAllow.add(d.runtimeAllow.items()...)
	default:
		return fmt.Errorf("unknown install egress mode %d", in.Options.InstallEgress)
	}
	return nil
}

// agentContextRule tells the agent what kind of environment it is in.
type agentContextRule struct{}

func (agentContextRule) apply(in Input, d *draft, _ *Report) error {
	name := in.Config.Name
	if name == "" {
		name = "this project's"
	}
	parts := []string{fmt.Sprintf("This sandbox was generated by dc2kit from %s devcontainer, %s.", name, d.baseSummary())}
	if len(d.image.Features) > 0 {
		var lines []string
		for _, step := range d.image.Features {
			lines = append(lines, "- `"+step.Ref+"`")
		}
		parts = append(parts, "Installed devcontainer features:\n"+strings.Join(lines, "\n"))
	}
	if len(d.lifecycle.Install) > 0 {
		parts = append(parts, "The project's setup commands (postCreateCommand and related) already ran when the sandbox was created.")
	}
	if len(d.ports) > 0 {
		ports := make([]string, len(d.ports))
		for i, p := range d.ports {
			ports[i] = fmt.Sprint(p)
		}
		parts = append(parts, "Ports published to the host: "+strings.Join(ports, ", ")+".")
	}
	if allow := d.runtimeAllow.items(); len(allow) > 0 {
		parts = append(parts, "Network egress is limited to declared hosts. If a command fails on a refused connection, report the host instead of working around the policy.")
	}
	d.context = parts
	return nil
}

// unmappedRule reports top-level keys no rule consumed.
type unmappedRule struct{}

var unmappedReasons = map[string]string{
	"customizations":       "editor settings and extensions have no sandbox equivalent",
	"initializeCommand":    "runs on the host before the container exists; a kit never runs host commands",
	"workspaceFolder":      "the sandbox places the workspace at " + WorkspaceDir,
	"workspaceMount":       "the sandbox manages the workspace mount",
	"appPort":              "deprecated; use forwardPorts",
	"portsAttributes":      "port labels and behaviors are editor features",
	"otherPortsAttributes": "port labels and behaviors are editor features",
	"shutdownAction":       "sandbox lifecycle is managed by sbx",
	"overrideCommand":      "the sandbox launches the agent itself",
	"securityOpt":          "kits cannot set container security options",
}

func (unmappedRule) apply(in Input, _ *draft, report *Report) error {
	for _, key := range in.Config.Unmapped {
		reason, known := unmappedReasons[key]
		if !known {
			reason = "not translated"
		}
		report.info(key, "%s", reason)
	}
	return nil
}
