// Package feature resolves devcontainer Features into their install payload
// and metadata, and orders them for installation.
package feature

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io/fs"
	"regexp"
	"sort"
	"strings"

	"github.com/tailscale/hujson"

	"github.com/aashish-thapa/dc2kit/internal/devcontainer"
)

// Resolver turns a feature reference from devcontainer.json into a Feature.
type Resolver interface {
	Resolve(ctx context.Context, req devcontainer.FeatureRequest) (Feature, error)
}

// Feature is one resolved devcontainer Feature: what the user asked for,
// what the feature declares about itself, and the files install.sh needs.
type Feature struct {
	Request  devcontainer.FeatureRequest
	Key      string
	Pin      string
	Metadata Metadata
	Files    []File
}

// File is one regular file of a feature payload, path relative to the
// payload root and already checked not to escape it.
type File struct {
	Path string
	Mode fs.FileMode
	Data []byte
}

// Metadata is the subset of devcontainer-feature.json dc2kit acts on.
type Metadata struct {
	ID            string                     `json:"id"`
	Version       string                     `json:"version"`
	Name          string                     `json:"name"`
	Options       map[string]OptionSpec      `json:"options"`
	ContainerEnv  map[string]string          `json:"containerEnv"`
	InstallsAfter []string                   `json:"installsAfter"`
	DependsOn     map[string]json.RawMessage `json:"dependsOn"`
	Privileged    bool                       `json:"privileged"`
	CapAdd        []string                   `json:"capAdd"`
	Mounts        []json.RawMessage          `json:"mounts"`
	Entrypoint    string                     `json:"entrypoint"`
}

// OptionSpec is one declared option; only its default matters here.
type OptionSpec struct {
	Default json.RawMessage `json:"default"`
}

const metadataFile = "devcontainer-feature.json"

func parseMetadata(data []byte) (Metadata, error) {
	// The devcontainer CLI reads this file as JSONC, and local Features in
	// the wild carry comments.
	data, err := hujson.Standardize(data)
	if err != nil {
		return Metadata{}, fmt.Errorf("%s: %w", metadataFile, err)
	}
	var m Metadata
	if err := json.Unmarshal(data, &m); err != nil {
		return Metadata{}, fmt.Errorf("%s: %w", metadataFile, err)
	}
	if m.ID == "" {
		return Metadata{}, fmt.Errorf("%s: \"id\" is required", metadataFile)
	}
	return m, nil
}

func metadataFrom(files []File) (Metadata, error) {
	for _, f := range files {
		if f.Path == metadataFile {
			return parseMetadata(f.Data)
		}
	}
	return Metadata{}, fmt.Errorf("payload has no %s", metadataFile)
}

func hasInstallScript(files []File) bool {
	for _, f := range files {
		if f.Path == "install.sh" {
			return true
		}
	}
	return false
}

// InstallEnv is the environment install.sh receives: every declared option
// at its default, overridden by what the user set, under the env-var name
// the devcontainer Features spec derives from the option id.
func (f Feature) InstallEnv() ([]devcontainer.EnvVar, error) {
	values := map[string]string{}
	for name, spec := range f.Metadata.Options {
		if len(spec.Default) == 0 {
			continue
		}
		value, err := scalarJSON(spec.Default)
		if err != nil {
			return nil, fmt.Errorf("feature %s option %q default: %w", f.Metadata.ID, name, err)
		}
		values[name] = value
	}
	for _, opt := range f.Request.Options {
		values[opt.Name] = opt.Value
	}

	names := make([]string, 0, len(values))
	for name := range values {
		names = append(names, name)
	}
	sort.Strings(names)

	env := make([]devcontainer.EnvVar, 0, len(names))
	seen := map[string]string{}
	for _, name := range names {
		envName := OptionEnvName(name)
		if envName == "" {
			return nil, fmt.Errorf("feature %s option %q has no valid env-var name", f.Metadata.ID, name)
		}
		if other, clash := seen[envName]; clash {
			return nil, fmt.Errorf("feature %s options %q and %q both map to %s", f.Metadata.ID, other, name, envName)
		}
		seen[envName] = name
		env = append(env, devcontainer.EnvVar{Name: envName, Value: values[name]})
	}
	return env, nil
}

var (
	nonEnvChars    = regexp.MustCompile(`[^A-Za-z0-9_]`)
	leadingNonName = regexp.MustCompile(`^[0-9_]+`)
)

// OptionEnvName applies the Features spec rule: uppercase, replace anything
// outside [A-Z0-9_] with '_', strip leading digits and underscores.
func OptionEnvName(option string) string {
	name := nonEnvChars.ReplaceAllString(option, "_")
	name = leadingNonName.ReplaceAllString(name, "")
	return strings.ToUpper(name)
}

func scalarJSON(raw json.RawMessage) (string, error) {
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.UseNumber()
	var v any
	if err := dec.Decode(&v); err != nil {
		return "", err
	}
	switch t := v.(type) {
	case string:
		return t, nil
	case bool:
		if t {
			return "true", nil
		}
		return "false", nil
	case json.Number:
		return t.String(), nil
	default:
		return "", fmt.Errorf("must be a string, number, or boolean")
	}
}

// NormalizeKey reduces a feature reference to its identity for ordering:
// lowercase, without tag or digest, so `node:1` and `node@sha256:…` match
// an `installsAfter` entry naming `node`.
func NormalizeKey(ref string) string {
	ref = strings.ToLower(ref)
	if at := strings.Index(ref, "@"); at >= 0 {
		ref = ref[:at]
	}
	lastSlash := strings.LastIndex(ref, "/")
	if colon := strings.LastIndex(ref, ":"); colon > lastSlash {
		ref = ref[:colon]
	}
	return ref
}
