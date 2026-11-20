// Package devcontainer reads devcontainer.json into typed values. Parsing
// happens once, here: everything downstream receives shapes that already
// rule out the spec's many loose encodings.
package devcontainer

// Config is the subset of devcontainer.json that has a sandbox-kit meaning.
// Keys outside that subset are listed in Unmapped so a caller can tell the
// user what the conversion left behind instead of dropping it silently.
type Config struct {
	Name string

	// Exactly one of Image, Build, or ComposeFiles describes the base. The
	// parser rejects documents naming none or several.
	Image        string
	Build        *Build
	ComposeFiles []string

	Features                    []FeatureRequest
	OverrideFeatureInstallOrder []string

	ContainerEnv []EnvVar
	RemoteEnv    []EnvVar

	ContainerUser string
	RemoteUser    string

	ForwardPorts []int

	OnCreate      Command
	UpdateContent Command
	PostCreate    Command
	PostStart     Command
	PostAttach    Command

	HostRequirements HostRequirements

	Privileged bool
	CapAdd     []string
	Mounts     []string
	RunArgs    []string

	// Unmapped lists top-level keys present in the file that dc2kit does not
	// translate, in document order.
	Unmapped []string
}

// Build is the devcontainer `build` block for Dockerfile-based configs.
type Build struct {
	Dockerfile string
	Context    string
	Target     string
	Args       []EnvVar
}

// FeatureRequest is one entry of the `features` object, in document order.
type FeatureRequest struct {
	Ref     string
	Options []Option
}

// Option is a feature option as the user wrote it. Value is already the
// string the feature's install.sh will see in its environment.
type Option struct {
	Name  string
	Value string
}

// EnvVar is an ordered name/value pair; ordering keeps generated output
// stable across runs.
type EnvVar struct {
	Name  string
	Value string
}

// HostRequirements carries the resource floor a devcontainer asks for. Zero
// values mean the file stated nothing.
type HostRequirements struct {
	CPUs   int
	Memory string
}

// Command is a devcontainer lifecycle command. The spec allows a shell
// string, an exec array, or an object of named commands run in parallel;
// all three normalize to an ordered list of named shell scripts.
type Command struct {
	Steps []Step
}

// IsEmpty reports whether the lifecycle slot was unset or empty.
func (c Command) IsEmpty() bool { return len(c.Steps) == 0 }

// Step is one command to run. Script is POSIX shell text: exec-array forms
// are quoted into a single equivalent line at parse time, so there is only
// one representation to render.
type Step struct {
	Name   string
	Script string
}
