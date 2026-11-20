package convert_test

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/aashish-thapa/dc2kit/internal/convert"
	"github.com/aashish-thapa/dc2kit/internal/devcontainer"
	"github.com/aashish-thapa/dc2kit/internal/feature"
	"github.com/aashish-thapa/dc2kit/internal/kit"
	"github.com/aashish-thapa/dc2kit/internal/render"
)

func parse(t *testing.T, doc string) devcontainer.Config {
	t.Helper()
	cfg, err := devcontainer.Parse([]byte(doc))
	if err != nil {
		t.Fatal(err)
	}
	return cfg
}

func nodeFeature() feature.Feature {
	return feature.Feature{
		Request: devcontainer.FeatureRequest{Ref: "ghcr.io/devcontainers/features/node:1"},
		Key:     "ghcr.io/devcontainers/features/node",
		Pin:     "sha256:0000",
		Metadata: feature.Metadata{
			ID:           "node",
			Options:      map[string]feature.OptionSpec{"version": {Default: json.RawMessage(`"lts"`)}},
			ContainerEnv: map[string]string{"NVM_DIR": "/usr/local/share/nvm", "PATH": "/usr/local/share/nvm/current/bin:${PATH}"},
		},
		Files: []feature.File{{Path: "install.sh", Mode: 0o755, Data: []byte("#!/bin/sh\n")}},
	}
}

// The invariant the tool exists for: whatever devcontainer goes in, the
// descriptor that comes out is one the upstream spec library accepts.
func TestEveryConversionPassesTheKitSpec(t *testing.T) {
	cases := map[string]struct {
		doc      string
		features []feature.Feature
		opts     convert.Options
	}{
		"bare image": {doc: `{"image":"ubuntu:24.04"}`},
		"everything": {
			doc: `{
				"name": "Full",
				"image": "mcr.microsoft.com/devcontainers/typescript-node:22",
				"containerEnv": {"A": "1", "P": "${containerEnv:PATH}:/x"},
				"remoteEnv": {"B": "${localEnv:SECRET:fallback}"},
				"forwardPorts": [3000, 3000, 5173],
				"onCreateCommand": "make deps",
				"postCreateCommand": {"npm": "npm ci", "py": ["pip", "install", "-r", "req.txt"]},
				"postStartCommand": "make watch",
				"hostRequirements": {"cpus": 2, "memory": "4gb"},
				"privileged": true
			}`,
			features: []feature.Feature{nodeFeature()},
		},
		"presets install egress": {
			doc:  `{"image":"python:3.12","postCreateCommand":"pip install -e ."}`,
			opts: convert.Options{InstallEgress: convert.InstallEgressPresets, RuntimeAllow: []string{"api.example.com:443"}},
		},
		"build with base image": {
			doc:  `{"build":{"dockerfile":"Dockerfile"}}`,
			opts: convert.Options{BaseImage: "ghcr.io/me/dev@sha256:" + strings.Repeat("a", 64)},
		},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			plan, _, err := convert.Convert(convert.Input{Config: parse(t, tc.doc), Features: tc.features, Options: tc.opts})
			if err != nil {
				t.Fatal(err)
			}
			raw, err := kit.Marshal(plan.Descriptor, "test")
			if err != nil {
				t.Fatal(err)
			}
			if _, err := kit.Validate(t.Context(), raw); err != nil {
				t.Fatalf("spec rejected the descriptor: %v\n%s", err, raw)
			}
		})
	}
}

func TestUnsupportedBasesFailWithGuidance(t *testing.T) {
	for name, doc := range map[string]string{
		"compose":           `{"dockerComposeFile":"compose.yml","service":"app"}`,
		"build without ref": `{"build":{"dockerfile":"Dockerfile"}}`,
	} {
		t.Run(name, func(t *testing.T) {
			_, _, err := convert.Convert(convert.Input{Config: parse(t, doc)})
			if err == nil || !strings.Contains(err.Error(), "--base-image") {
				t.Fatalf("want an error that points at --base-image, got %v", err)
			}
		})
	}
}

func TestHooksDeclareTheImageEnvironment(t *testing.T) {
	cfg := parse(t, `{"image":"x","containerEnv":{"APP":"1"},"postCreateCommand":"echo ${containerWorkspaceFolder}"}`)
	plan, _, err := convert.Convert(convert.Input{Config: cfg, Features: []feature.Feature{nodeFeature()}})
	if err != nil {
		t.Fatal(err)
	}
	var hook kit.Hook
	for _, c := range plan.Descriptor.Capabilities {
		if c.Type == kit.TypeLifecycle {
			hook = c.Config.(*kit.LifecycleConfig).Install[0]
		}
	}
	env := strings.Join(hook.Env, ",")
	// lifecycle@1 hides undeclared variables from hooks; a feature's
	// NVM_DIR must be declared or `npm` setup scripts break.
	if env != "WORKSPACE_DIR,NVM_DIR,APP" {
		t.Fatalf("hook env = %s", env)
	}
	if hook.User != convert.AgentUser || !strings.Contains(hook.Command, "echo ${WORKSPACE_DIR}") {
		t.Fatalf("hook = %+v", hook)
	}
}

func TestDockerfileEnvEscaping(t *testing.T) {
	cfg := parse(t, `{"image":"x","containerEnv":{
		"LITERAL": "pa$$w\"o\\rd",
		"EXTENDS": "${containerEnv:PATH}:/opt/bin",
		"HOMEDIR": "${containerEnv:HOME}/tools",
		"WS": "${containerWorkspaceFolder}/src"
	}}`)
	plan, _, err := convert.Convert(convert.Input{Config: cfg, Features: []feature.Feature{nodeFeature()}})
	if err != nil {
		t.Fatal(err)
	}
	dockerfile := render.Dockerfile(plan.Image, "test")
	for _, want := range []string{
		`ENV LITERAL="pa\$\$w\"o\\rd"`,
		`ENV EXTENDS="${PATH}:/opt/bin"`,
		`ENV HOMEDIR="/home/agent/tools"`,
		`ENV WS="/home/agent/workspace/src"`,
		`ENV PATH="/usr/local/share/nvm/current/bin:${PATH}"`,
		"USER agent\n",
		`ENTRYPOINT ["bash"]`,
	} {
		if !strings.Contains(dockerfile, want) {
			t.Errorf("Dockerfile lacks %s\n%s", want, dockerfile)
		}
	}
}

func TestHostOnlyVariablesAreReported(t *testing.T) {
	cfg := parse(t, `{"image":"x","remoteEnv":{"T":"${localEnv:TOKEN}"},"postCreateCommand":"cd ${localWorkspaceFolder}"}`)
	_, report, err := convert.Convert(convert.Input{Config: cfg})
	if err != nil {
		t.Fatal(err)
	}
	var warned []string
	for _, f := range report.Findings {
		if f.Level == convert.Warn {
			warned = append(warned, f.Field)
		}
	}
	if got := strings.Join(warned, ","); got != "remoteEnv.T,postCreateCommand" {
		t.Fatalf("warnings on %s", got)
	}
}

func TestEgressPresetsAndInstallPhase(t *testing.T) {
	cfg := parse(t, `{"image":"mcr.microsoft.com/devcontainers/javascript-node:22","postCreateCommand":"npm ci"}`)
	plan, _, err := convert.Convert(convert.Input{Config: cfg})
	if err != nil {
		t.Fatal(err)
	}
	var policy *kit.NetworkPolicyConfig
	for _, c := range plan.Descriptor.Capabilities {
		if c.Type == kit.TypeNetworkPolicy {
			policy = c.Config.(*kit.NetworkPolicyConfig)
		}
	}
	runtime := strings.Join(policy.Runtime.Allow, ",")
	if !strings.Contains(runtime, "registry.npmjs.org") || strings.Contains(runtime, "maven") {
		t.Fatalf("javascript-node must get npm presets and not Java ones: %s", runtime)
	}
	if got := strings.Join(policy.Install.Allow, ","); got != "**" {
		t.Fatalf("default install egress = %s", got)
	}
}
