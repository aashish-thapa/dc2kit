package devcontainer

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/tailscale/hujson"

	"github.com/aashish-thapa/dc2kit/internal/shell"
)

// Parse reads devcontainer.json (JSON with comments and trailing commas)
// and returns its typed form. Errors name the offending key.
func Parse(data []byte) (Config, error) {
	standard, err := hujson.Standardize(data)
	if err != nil {
		return Config{}, fmt.Errorf("devcontainer.json is not valid JSONC: %w", err)
	}
	root, err := decodeOrdered(standard)
	if err != nil {
		return Config{}, fmt.Errorf("devcontainer.json: %w", err)
	}
	if root.kind != kindObject {
		return Config{}, fmt.Errorf("devcontainer.json: top level must be an object, got %s", root.kind)
	}

	var cfg Config
	for _, m := range root.members {
		handle, isMapped := fieldHandlers[m.key]
		if !isMapped {
			if m.key != "$schema" {
				cfg.Unmapped = append(cfg.Unmapped, m.key)
			}
			continue
		}
		if err := handle(&cfg, m.value); err != nil {
			return Config{}, fmt.Errorf("devcontainer.json %q: %w", m.key, err)
		}
	}
	if err := checkSingleBase(cfg); err != nil {
		return Config{}, err
	}
	return cfg, nil
}

type fieldHandler func(cfg *Config, value node) error

var fieldHandlers = map[string]fieldHandler{
	"name":  stringInto(func(c *Config) *string { return &c.Name }),
	"image": stringInto(func(c *Config) *string { return &c.Image }),
	"build": func(c *Config, v node) error {
		b, err := parseBuild(v)
		c.Build = b
		return err
	},
	"dockerComposeFile": func(c *Config, v node) error {
		files, err := stringOrList(v)
		c.ComposeFiles = files
		return err
	},
	"features": func(c *Config, v node) error {
		features, err := parseFeatures(v)
		c.Features = features
		return err
	},
	"overrideFeatureInstallOrder": func(c *Config, v node) error {
		order, err := stringList(v)
		c.OverrideFeatureInstallOrder = order
		return err
	},
	"containerEnv": func(c *Config, v node) error {
		env, err := stringMap(v)
		c.ContainerEnv = env
		return err
	},
	"remoteEnv": func(c *Config, v node) error {
		env, err := stringMap(v)
		c.RemoteEnv = env
		return err
	},
	"containerUser": stringInto(func(c *Config) *string { return &c.ContainerUser }),
	"remoteUser":    stringInto(func(c *Config) *string { return &c.RemoteUser }),
	"forwardPorts": func(c *Config, v node) error {
		ports, err := parsePorts(v)
		c.ForwardPorts = ports
		return err
	},
	"onCreateCommand":      commandInto(func(c *Config) *Command { return &c.OnCreate }),
	"updateContentCommand": commandInto(func(c *Config) *Command { return &c.UpdateContent }),
	"postCreateCommand":    commandInto(func(c *Config) *Command { return &c.PostCreate }),
	"postStartCommand":     commandInto(func(c *Config) *Command { return &c.PostStart }),
	"postAttachCommand":    commandInto(func(c *Config) *Command { return &c.PostAttach }),
	"hostRequirements": func(c *Config, v node) error {
		hr, err := parseHostRequirements(v)
		c.HostRequirements = hr
		return err
	},
	"privileged": func(c *Config, v node) error {
		if v.kind != kindBool {
			return fmt.Errorf("must be a boolean, got %s", v.kind)
		}
		c.Privileged = v.boolean
		return nil
	},
	"capAdd": func(c *Config, v node) error {
		caps, err := stringList(v)
		c.CapAdd = caps
		return err
	},
	"mounts": func(c *Config, v node) error {
		mounts, err := parseMounts(v)
		c.Mounts = mounts
		return err
	},
	"runArgs": func(c *Config, v node) error {
		args, err := stringList(v)
		c.RunArgs = args
		return err
	},
}

// checkSingleBase rejects configs naming several bases. Naming none is
// legal: Codespaces then uses its default image, so the converter asks for
// one instead.
func checkSingleBase(cfg Config) error {
	declared := 0
	for _, isSet := range []bool{cfg.Image != "", cfg.Build != nil, len(cfg.ComposeFiles) > 0} {
		if isSet {
			declared++
		}
	}
	if declared > 1 {
		return fmt.Errorf("devcontainer.json names more than one of \"image\", \"build\", and \"dockerComposeFile\"")
	}
	return nil
}

func stringInto(field func(*Config) *string) fieldHandler {
	return func(c *Config, v node) error {
		if v.kind != kindString {
			return fmt.Errorf("must be a string, got %s", v.kind)
		}
		*field(c) = v.str
		return nil
	}
}

func commandInto(field func(*Config) *Command) fieldHandler {
	return func(c *Config, v node) error {
		cmd, err := parseCommand(v)
		*field(c) = cmd
		return err
	}
}

func parseBuild(v node) (*Build, error) {
	if v.kind != kindObject {
		return nil, fmt.Errorf("must be an object, got %s", v.kind)
	}
	b := &Build{}
	for _, m := range v.members {
		var err error
		switch m.key {
		case "dockerfile", "dockerFile":
			b.Dockerfile, err = asString(m.value)
		case "context":
			b.Context, err = asString(m.value)
		case "target":
			b.Target, err = asString(m.value)
		case "args":
			b.Args, err = stringMap(m.value)
		}
		if err != nil {
			return nil, fmt.Errorf("%s: %w", m.key, err)
		}
	}
	if b.Dockerfile == "" {
		return nil, fmt.Errorf("\"dockerfile\" is required")
	}
	return b, nil
}

// ParseFeatureRequests reads a JSON object shaped like "features", such as
// a Feature's dependsOn, with the same option rules as devcontainer.json.
func ParseFeatureRequests(data []byte) ([]FeatureRequest, error) {
	v, err := decodeOrdered(data)
	if err != nil {
		return nil, err
	}
	return parseFeatures(v)
}

func parseFeatures(v node) ([]FeatureRequest, error) {
	if v.kind != kindObject {
		return nil, fmt.Errorf("must be an object, got %s", v.kind)
	}
	requests := make([]FeatureRequest, 0, len(v.members))
	for _, m := range v.members {
		req := FeatureRequest{Ref: m.key}
		switch m.value.kind {
		case kindObject:
			for _, opt := range m.value.members {
				value, ok := opt.value.scalarString()
				if !ok {
					return nil, fmt.Errorf("%s option %q must be a string, number, or boolean", m.key, opt.key)
				}
				req.Options = append(req.Options, Option{Name: opt.key, Value: value})
			}
		default:
			// The shorthand `"feature": "1.2"` sets the conventional
			// `version` option; `true` and `{}` mean defaults.
			if value, ok := m.value.scalarString(); ok && m.value.kind != kindBool {
				req.Options = []Option{{Name: "version", Value: value}}
			} else if m.value.kind != kindBool {
				return nil, fmt.Errorf("%s must be an object, string, or boolean", m.key)
			}
		}
		requests = append(requests, req)
	}
	return requests, nil
}

// parseCommand normalizes the three lifecycle command forms. An object's
// named entries run in parallel under the devcontainer CLI; their names
// are kept so a sequential runtime can still label each one.
func parseCommand(v node) (Command, error) {
	switch v.kind {
	case kindNull:
		return Command{}, nil
	case kindString, kindArray:
		script, err := commandScript(v)
		if err != nil || script == "" {
			return Command{}, err
		}
		return Command{Steps: []Step{{Script: script}}}, nil
	case kindObject:
		var cmd Command
		for _, m := range v.members {
			script, err := commandScript(m.value)
			if err != nil {
				return Command{}, fmt.Errorf("%s: %w", m.key, err)
			}
			if script != "" {
				cmd.Steps = append(cmd.Steps, Step{Name: m.key, Script: script})
			}
		}
		return cmd, nil
	default:
		return Command{}, fmt.Errorf("must be a string, array, or object, got %s", v.kind)
	}
}

func commandScript(v node) (string, error) {
	switch v.kind {
	case kindString:
		return strings.TrimSpace(v.str), nil
	case kindArray:
		argv, err := stringList(v)
		if err != nil {
			return "", err
		}
		return shell.Join(argv), nil
	default:
		return "", fmt.Errorf("command must be a string or array, got %s", v.kind)
	}
}

// parsePorts keeps numeric ports and `localhost:N`. Other `host:port`
// strings name Compose services, which a single sandbox cannot publish.
func parsePorts(v node) ([]int, error) {
	if v.kind != kindArray {
		return nil, fmt.Errorf("must be an array, got %s", v.kind)
	}
	var ports []int
	for _, item := range v.items {
		raw, ok := item.scalarString()
		if !ok {
			return nil, fmt.Errorf("entries must be numbers or strings")
		}
		host, portText, hasHost := strings.Cut(raw, ":")
		if !hasHost {
			portText = host
		} else if host != "localhost" && host != "127.0.0.1" {
			continue
		}
		port, err := strconv.Atoi(portText)
		if err != nil || port < 1 || port > 65535 {
			return nil, fmt.Errorf("%q is not a port in 1..65535", raw)
		}
		ports = append(ports, port)
	}
	return ports, nil
}

func parseHostRequirements(v node) (HostRequirements, error) {
	if v.kind != kindObject {
		return HostRequirements{}, fmt.Errorf("must be an object, got %s", v.kind)
	}
	var hr HostRequirements
	for _, m := range v.members {
		switch m.key {
		case "cpus":
			// The schema says integer, but Codespaces also accepts "4", and
			// real configs use it.
			var text string
			switch m.value.kind {
			case kindNumber:
				text = m.value.num.String()
			case kindString:
				text = m.value.str
			default:
				return HostRequirements{}, fmt.Errorf("cpus must be a number")
			}
			cpus, err := strconv.Atoi(text)
			if err != nil || cpus < 1 {
				return HostRequirements{}, fmt.Errorf("cpus must be a positive integer")
			}
			hr.CPUs = cpus
		case "memory":
			memory, err := asString(m.value)
			if err != nil {
				return HostRequirements{}, fmt.Errorf("memory: %w", err)
			}
			hr.Memory = memory
		}
	}
	return hr, nil
}

// parseMounts accepts both the string and object mount forms and keeps a
// display string for each; mounts are reported, never translated.
func parseMounts(v node) ([]string, error) {
	if v.kind != kindArray {
		return nil, fmt.Errorf("must be an array, got %s", v.kind)
	}
	mounts := make([]string, 0, len(v.items))
	for _, item := range v.items {
		switch item.kind {
		case kindString:
			mounts = append(mounts, item.str)
		case kindObject:
			parts := make([]string, 0, len(item.members))
			for _, m := range item.members {
				value, _ := m.value.scalarString()
				parts = append(parts, m.key+"="+value)
			}
			mounts = append(mounts, strings.Join(parts, ","))
		default:
			return nil, fmt.Errorf("entries must be strings or objects")
		}
	}
	return mounts, nil
}

func asString(v node) (string, error) {
	if v.kind != kindString {
		return "", fmt.Errorf("must be a string, got %s", v.kind)
	}
	return v.str, nil
}

func stringList(v node) ([]string, error) {
	if v.kind != kindArray {
		return nil, fmt.Errorf("must be an array, got %s", v.kind)
	}
	out := make([]string, 0, len(v.items))
	for _, item := range v.items {
		s, err := asString(item)
		if err != nil {
			return nil, err
		}
		out = append(out, s)
	}
	return out, nil
}

func stringOrList(v node) ([]string, error) {
	if v.kind == kindString {
		return []string{v.str}, nil
	}
	return stringList(v)
}

func stringMap(v node) ([]EnvVar, error) {
	if v.kind != kindObject {
		return nil, fmt.Errorf("must be an object, got %s", v.kind)
	}
	out := make([]EnvVar, 0, len(v.members))
	for _, m := range v.members {
		value, ok := m.value.scalarString()
		if !ok {
			return nil, fmt.Errorf("%s must be a string", m.key)
		}
		out = append(out, EnvVar{Name: m.key, Value: value})
	}
	return out, nil
}
