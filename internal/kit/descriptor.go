// Package kit models the sandbox-kit v3 descriptor dc2kit emits and checks
// it against the upstream specification library. The model covers only
// what dc2kit writes; validity is decided by the spec library, not here.
package kit

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"

	"gopkg.in/yaml.v3"

	"github.com/docker/sandbox-kit-spec/v3/spec"
)

// Capability type names, one home for each string the descriptor uses.
const (
	TypeSbx           = "com.docker.sandbox/sbx@1"
	TypeNetworkPolicy = "com.docker.sandbox/network-policy@1"
	TypeLifecycle     = "com.docker.sandbox/lifecycle@1"
	TypePort          = "com.docker.sandbox/port@1"
	TypeResources     = "com.docker.sandbox/resources@1"
	TypePrivileged    = "com.docker.sandbox/privileged@1"
	TypeDockerd       = "com.docker.sandbox/dockerd@1"
	TypeAgentContext1 = "com.docker.sandbox/agent-context@1"
	TypeAgentContext2 = "com.docker.sandbox/agent-context@2"
)

// Descriptor is a workload kit descriptor.
type Descriptor struct {
	SchemaVersion string       `yaml:"schemaVersion"`
	Kind          string       `yaml:"kind"`
	DisplayName   string       `yaml:"displayName,omitempty"`
	Description   string       `yaml:"description,omitempty"`
	SourceURL     string       `yaml:"sourceUrl,omitempty"`
	Version       string       `yaml:"version,omitempty"`
	Capabilities  []Capability `yaml:"capabilities,omitempty"`
}

// NewWorkload returns an empty workload descriptor targeting the sandbox
// agent platform; sbx@1 is what makes the image's user and workdir binding.
func NewWorkload() Descriptor {
	return Descriptor{
		SchemaVersion: spec.SchemaVersion,
		Kind:          "workload",
		Capabilities:  []Capability{{Type: TypeSbx}},
	}
}

// Capability is one typed request. Config is one of the *Config types in
// this package, or nil for config-less types.
type Capability struct {
	Type        string `yaml:"type"`
	Optional    bool   `yaml:"optional,omitempty"`
	Description string `yaml:"description,omitempty"`
	Config      any    `yaml:"config,omitempty"`
}

// NetworkPolicyConfig is network-policy@1's config.
type NetworkPolicyConfig struct {
	Install *EgressRules `yaml:"install,omitempty"`
	Runtime *EgressRules `yaml:"runtime,omitempty"`
}

// EgressRules is one phase's allow and deny lists.
type EgressRules struct {
	Allow []string `yaml:"allow,omitempty"`
	Deny  []string `yaml:"deny,omitempty"`
}

// LifecycleConfig is lifecycle@1's config.
type LifecycleConfig struct {
	Install []Hook `yaml:"install,omitempty"`
	Startup []Hook `yaml:"startup,omitempty"`
}

// Hook is one install or startup command.
type Hook struct {
	Command     string   `yaml:"command"`
	User        string   `yaml:"user,omitempty"`
	Env         []string `yaml:"env,omitempty"`
	Background  bool     `yaml:"background,omitempty"`
	Description string   `yaml:"description,omitempty"`
}

// PortConfig is port@1's config.
type PortConfig struct {
	Name      string `yaml:"name,omitempty"`
	Container int    `yaml:"container"`
}

// ResourcesConfig is resources@1's config.
type ResourcesConfig struct {
	CPU    float64 `yaml:"cpu,omitempty"`
	Memory string  `yaml:"memory,omitempty"`
}

// AgentContextConfig is the config shared by agent-context@1 and @2.
type AgentContextConfig struct {
	Filename string `yaml:"filename,omitempty"`
	Content  string `yaml:"content"`
}

const header = "# syntax=docker/sandbox-kit:3\n"

// Marshal renders the descriptor with the frontend syntax line BuildKit
// dispatches on, followed by a provenance comment.
func Marshal(d Descriptor, provenance string) ([]byte, error) {
	var buf bytes.Buffer
	buf.WriteString(header)
	for _, line := range splitLines(provenance) {
		buf.WriteString("# " + line + "\n")
	}
	enc := yaml.NewEncoder(&buf)
	enc.SetIndent(2)
	if err := enc.Encode(d); err != nil {
		return nil, fmt.Errorf("encode descriptor: %w", err)
	}
	if err := enc.Close(); err != nil {
		return nil, fmt.Errorf("encode descriptor: %w", err)
	}
	return buf.Bytes(), nil
}

// Validate checks raw descriptor bytes with the upstream spec library, the
// way upstream checks its own examples: strict decode, declaration-level
// validation of the authored form, then effective validation of what each
// kind of runtime would select from it. The descriptor offers both
// agent-context versions as optional alternatives, so "every runtime"
// means one that knows only @1 and one that knows only @2. Any error means
// the kit would be refused.
func Validate(ctx context.Context, raw []byte) ([]string, error) {
	d, err := spec.Decode(raw)
	if err != nil {
		return nil, err
	}
	warnings, err := spec.ValidateDeclarations(raw, d)
	if err != nil {
		return warnings, err
	}
	for _, excluded := range []string{spec.CapabilityAgentContextV2, spec.CapabilityAgentContext} {
		if err := validateSelection(ctx, d, excluded); err != nil {
			return warnings, err
		}
	}
	return warnings, nil
}

func validateSelection(ctx context.Context, d *spec.Descriptor, excluded string) error {
	var supported []string
	for _, typ := range spec.KnownCapabilities() {
		if typ != excluded {
			supported = append(supported, typ)
		}
	}
	selection, err := spec.SelectCapabilities(ctx, "dc2kit", d, spec.Supported(supported...))
	if err != nil {
		return fmt.Errorf("select capabilities for a runtime without %s: %w", excluded, err)
	}
	selected := *d
	selected.Capabilities = selection.Capabilities
	raw, err := json.Marshal(&selected)
	if err != nil {
		return err
	}
	if _, err := spec.ValidateEffective(raw, &selected); err != nil {
		return fmt.Errorf("as selected by a runtime without %s: %w", excluded, err)
	}
	return nil
}

func splitLines(s string) []string {
	if s == "" {
		return nil
	}
	var lines []string
	for _, line := range bytes.Split([]byte(s), []byte("\n")) {
		lines = append(lines, string(line))
	}
	return lines
}
