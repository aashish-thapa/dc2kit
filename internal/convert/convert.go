// Package convert maps a parsed devcontainer and its resolved features onto
// a sandbox-kit workload. Each devcontainer concern is one rule; a rule
// reads the input and writes into a shared draft, and the draft assembles
// the descriptor once every rule has run.
package convert

import (
	"fmt"

	"github.com/aashish-thapa/dc2kit/internal/devcontainer"
	"github.com/aashish-thapa/dc2kit/internal/feature"
	"github.com/aashish-thapa/dc2kit/internal/kit"
)

// The identity every sandbox agent mixin assumes: published mixins write
// into /home/agent and run as `agent`, so the workload must provide it.
const (
	AgentUser    = "agent"
	AgentHome    = "/home/agent"
	WorkspaceDir = AgentHome + "/workspace"
)

// InstallEgress chooses what the install phase (where postCreateCommand and
// friends run) may reach.
type InstallEgress int

const (
	// InstallEgressOpen grants all egress while install hooks run; the phase
	// closes before the agent starts. Setup commands such as `npm install`
	// reach hosts no preset can predict.
	InstallEgressOpen InstallEgress = iota
	// InstallEgressPresets grants the install phase the same hosts as the
	// runtime phase.
	InstallEgressPresets
)

// Options are the conversion choices that do not come from the file.
type Options struct {
	BaseImage     string
	RuntimeAllow  []string
	InstallEgress InstallEgress
	SourceURL     string
}

// Input is everything a conversion reads. Features must already be in
// install order.
type Input struct {
	Config   devcontainer.Config
	Features []feature.Feature
	Options  Options
}

// Plan is the conversion result: the descriptor and the image recipe the
// renderer turns into a Dockerfile.
type Plan struct {
	Descriptor kit.Descriptor
	Image      ImagePlan
}

// ImagePlan is what the companion Dockerfile must build.
type ImagePlan struct {
	BaseImage    string
	Features     []FeatureStep
	Env          []ImageEnv
	StartsDocker bool
}

// FeatureStep is one feature install, in order.
type FeatureStep struct {
	Dir          string
	Ref          string
	Pin          string
	InstallEnv   []devcontainer.EnvVar
	ContainerEnv []ImageEnv
	Files        []feature.File
}

// Convert runs every rule and returns the plan and what the user should
// know about it. An error means the devcontainer cannot become a kit.
func Convert(in Input) (Plan, Report, error) {
	d := newDraft()
	var report Report
	for _, r := range rules {
		if err := r.apply(in, d, &report); err != nil {
			return Plan{}, report, err
		}
	}
	descriptor, err := d.descriptor(in)
	if err != nil {
		return Plan{}, report, err
	}
	return Plan{Descriptor: descriptor, Image: d.image}, report, nil
}

// rule maps one devcontainer concern. Rules run in the order listed: later
// rules may read what earlier ones put in the draft (egress needs to know
// whether any install hook exists).
type rule interface {
	apply(in Input, d *draft, report *Report) error
}

var rules = []rule{
	baseImageRule{},
	featureRule{},
	envRule{},
	userRule{},
	lifecycleRule{},
	portRule{},
	resourceRule{},
	privilegeRule{},
	egressRule{},
	agentContextRule{},
	unmappedRule{},
}

// draft accumulates capability contributions until every rule has run.
type draft struct {
	image        ImagePlan
	lifecycle    kit.LifecycleConfig
	ports        []int
	resources    *kit.ResourcesConfig
	privileged   []string
	runtimeAllow orderedSet
	installAllow orderedSet
	context      []string
}

func newDraft() *draft { return &draft{} }

// hookEnv names every variable a hook may read. lifecycle@1 hooks see only
// the names they declare, while devcontainer commands see the whole
// container environment; declaring every image-level name keeps the two
// equivalent.
func (d *draft) hookEnv() []string {
	names := orderedSet{}
	names.add("WORKSPACE_DIR")
	for _, step := range d.image.Features {
		for _, v := range step.ContainerEnv {
			names.add(v.Name)
		}
	}
	for _, v := range d.image.Env {
		names.add(v.Name)
	}
	names.remove("PATH")
	names.remove("HOME")
	return names.items()
}

func (d *draft) descriptor(in Input) (kit.Descriptor, error) {
	desc := kit.NewWorkload()
	desc.DisplayName = in.Config.Name
	if desc.DisplayName == "" {
		desc.DisplayName = "Dev container"
	}
	desc.Description = fmt.Sprintf("Sandbox workload converted from devcontainer.json (base %s).", d.image.BaseImage)
	desc.SourceURL = in.Options.SourceURL
	desc.Version = "0.1.0"

	if len(d.privileged) > 0 {
		desc.Capabilities = append(desc.Capabilities, kit.Capability{
			Type:        kit.TypePrivileged,
			Description: joinSentences(d.privileged),
		})
	}
	if policy := d.networkPolicy(); policy != nil {
		desc.Capabilities = append(desc.Capabilities, kit.Capability{Type: kit.TypeNetworkPolicy, Config: policy})
	}
	if len(d.lifecycle.Install) > 0 || len(d.lifecycle.Startup) > 0 {
		lifecycle := d.lifecycle
		desc.Capabilities = append(desc.Capabilities, kit.Capability{Type: kit.TypeLifecycle, Config: &lifecycle})
	}
	for _, port := range d.ports {
		desc.Capabilities = append(desc.Capabilities, kit.Capability{
			Type:   kit.TypePort,
			Config: &kit.PortConfig{Container: port},
		})
	}
	if d.resources != nil {
		desc.Capabilities = append(desc.Capabilities, kit.Capability{Type: kit.TypeResources, Config: d.resources})
	}
	if len(d.context) > 0 {
		// Both versions, optional, as the upstream examples do: a runtime
		// accepts the one it knows and skips the other.
		ctx := &kit.AgentContextConfig{Filename: "AGENTS.md", Content: joinParagraphs(d.context)}
		desc.Capabilities = append(desc.Capabilities,
			kit.Capability{Type: kit.TypeAgentContext1, Optional: true, Config: ctx},
			kit.Capability{Type: kit.TypeAgentContext2, Optional: true, Config: ctx},
		)
	}
	return desc, nil
}

func (d *draft) networkPolicy() *kit.NetworkPolicyConfig {
	policy := &kit.NetworkPolicyConfig{}
	if allow := d.runtimeAllow.items(); len(allow) > 0 {
		policy.Runtime = &kit.EgressRules{Allow: allow}
	}
	if allow := d.installAllow.items(); len(allow) > 0 {
		policy.Install = &kit.EgressRules{Allow: allow}
	}
	if policy.Runtime == nil && policy.Install == nil {
		return nil
	}
	return policy
}
