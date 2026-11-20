package devcontainer

import (
	"strings"
	"testing"
)

func TestParseAcceptsJSONCAndKeepsFeatureOrder(t *testing.T) {
	cfg, err := Parse([]byte(`{
		// comment
		"image": "ubuntu",
		"features": {
			"ghcr.io/z/last:1": {},
			"ghcr.io/a/first:1": { "version": "20", "flag": true, "n": 3 },
			"ghcr.io/m/shorthand:1": "lts",
		},
	}`))
	if err != nil {
		t.Fatal(err)
	}
	var refs []string
	for _, f := range cfg.Features {
		refs = append(refs, f.Ref)
	}
	if got, want := strings.Join(refs, ","), "ghcr.io/z/last:1,ghcr.io/a/first:1,ghcr.io/m/shorthand:1"; got != want {
		t.Fatalf("feature order = %s, want %s", got, want)
	}
	if got := cfg.Features[1].Options; len(got) != 3 || got[1].Value != "true" || got[2].Value != "3" {
		t.Fatalf("options not stringified as install.sh sees them: %+v", got)
	}
	if got := cfg.Features[2].Options; len(got) != 1 || got[0] != (Option{Name: "version", Value: "lts"}) {
		t.Fatalf("string shorthand should set version: %+v", got)
	}
}

func TestParseCommandForms(t *testing.T) {
	cfg, err := Parse([]byte(`{
		"image": "x",
		"onCreateCommand": "  make setup  ",
		"postCreateCommand": ["echo", "it's", "a b"],
		"postStartCommand": { "db": "start-db", "web": ["serve", "--port", "3000"] }
	}`))
	if err != nil {
		t.Fatal(err)
	}
	if got := cfg.OnCreate.Steps; len(got) != 1 || got[0].Script != "make setup" {
		t.Fatalf("string form: %+v", got)
	}
	if got := cfg.PostCreate.Steps[0].Script; got != `echo 'it'\''s' 'a b'` {
		t.Fatalf("array form must quote each argument, got %s", got)
	}
	steps := cfg.PostStart.Steps
	if len(steps) != 2 || steps[0].Name != "db" || steps[1].Script != "serve --port 3000" {
		t.Fatalf("object form: %+v", steps)
	}
}

func TestParsePortsSkipsComposeServices(t *testing.T) {
	cfg, err := Parse([]byte(`{"image":"x","forwardPorts":[3000,"localhost:9229","db:5432"]}`))
	if err != nil {
		t.Fatal(err)
	}
	if got := cfg.ForwardPorts; len(got) != 2 || got[0] != 3000 || got[1] != 9229 {
		t.Fatalf("ports = %v", got)
	}
}

func TestParseRejects(t *testing.T) {
	for name, doc := range map[string]string{
		"no base":       `{"name":"x"}`,
		"two bases":     `{"image":"x","build":{"dockerfile":"Dockerfile"}}`,
		"bad port":      `{"image":"x","forwardPorts":[70000]}`,
		"build no file": `{"build":{"context":".."}}`,
		"not an object": `[1]`,
		"bad command":   `{"image":"x","postCreateCommand":3}`,
	} {
		t.Run(name, func(t *testing.T) {
			if _, err := Parse([]byte(doc)); err == nil {
				t.Fatal("expected an error")
			}
		})
	}
}

func TestParseRecordsUnmappedKeys(t *testing.T) {
	cfg, err := Parse([]byte(`{"$schema":"s","image":"x","customizations":{},"initializeCommand":"x"}`))
	if err != nil {
		t.Fatal(err)
	}
	if got := strings.Join(cfg.Unmapped, ","); got != "customizations,initializeCommand" {
		t.Fatalf("unmapped = %s", got)
	}
}
