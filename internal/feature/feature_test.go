package feature

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"encoding/json"
	"strings"
	"testing"

	"github.com/aashish-thapa/dc2kit/internal/devcontainer"
)

func feat(key string, installsAfter []string, dependsOn ...string) Feature {
	deps := map[string]json.RawMessage{}
	for _, d := range dependsOn {
		deps[d] = json.RawMessage(`{}`)
	}
	return Feature{
		Request:  devcontainer.FeatureRequest{Ref: key + ":1"},
		Key:      key,
		Metadata: Metadata{ID: key, InstallsAfter: installsAfter, DependsOn: deps},
	}
}

func keys(fs []Feature) string {
	var out []string
	for _, f := range fs {
		out = append(out, f.Key)
	}
	return strings.Join(out, ",")
}

func TestOrder(t *testing.T) {
	cases := []struct {
		name     string
		in       []Feature
		override []string
		want     string
	}{
		{"document order by default", []Feature{feat("r/a", nil), feat("r/b", nil)}, nil, "r/a,r/b"},
		{"installsAfter moves a feature later", []Feature{feat("r/a", []string{"r/b"}), feat("r/b", nil)}, nil, "r/b,r/a"},
		{"installsAfter on an absent feature is ignored", []Feature{feat("r/a", []string{"r/zz"}), feat("r/b", nil)}, nil, "r/a,r/b"},
		{"dependsOn is honored", []Feature{feat("r/a", nil, "r/b:2"), feat("r/b", nil)}, nil, "r/b,r/a"},
		{"override wins among free features", []Feature{feat("r/a", nil), feat("r/b", nil), feat("r/c", nil)}, []string{"r/c:1"}, "r/c,r/a,r/b"},
		{"override cannot break an edge", []Feature{feat("r/a", []string{"r/b"}), feat("r/b", nil)}, []string{"r/a"}, "r/b,r/a"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := Order(tc.in, tc.override)
			if err != nil {
				t.Fatal(err)
			}
			if keys(got) != tc.want {
				t.Fatalf("got %s, want %s", keys(got), tc.want)
			}
		})
	}
}

func TestOrderErrors(t *testing.T) {
	if _, err := Order([]Feature{feat("r/a", nil, "r/missing")}, nil); err == nil {
		t.Error("a missing dependsOn target must fail, not be silently skipped")
	}
	if _, err := Order([]Feature{feat("r/a", []string{"r/b"}), feat("r/b", []string{"r/a"})}, nil); err == nil {
		t.Error("a cycle must fail")
	}
	if _, err := Order([]Feature{feat("r/a", nil), feat("r/a", nil)}, nil); err == nil {
		t.Error("the same feature twice must fail")
	}
}

func TestNormalizeKey(t *testing.T) {
	for in, want := range map[string]string{
		"ghcr.io/devcontainers/features/node:1":       "ghcr.io/devcontainers/features/node",
		"GHCR.io/x/node@sha256:abc":                   "ghcr.io/x/node",
		"localhost:5000/features/node":                "localhost:5000/features/node",
		"localhost:5000/features/node:2":              "localhost:5000/features/node",
		"ghcr.io/devcontainers/features/common-utils": "ghcr.io/devcontainers/features/common-utils",
	} {
		if got := NormalizeKey(in); got != want {
			t.Errorf("NormalizeKey(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestOptionEnvName(t *testing.T) {
	for in, want := range map[string]string{"version": "VERSION", "nvm-install.path": "NVM_INSTALL_PATH", "2fa_mode": "FA_MODE"} {
		if got := OptionEnvName(in); got != want {
			t.Errorf("OptionEnvName(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestInstallEnvAppliesDefaultsThenUserValues(t *testing.T) {
	f := Feature{
		Request: devcontainer.FeatureRequest{Options: []devcontainer.Option{{Name: "version", Value: "22"}}},
		Metadata: Metadata{ID: "node", Options: map[string]OptionSpec{
			"version":   {Default: json.RawMessage(`"lts"`)},
			"gyp":       {Default: json.RawMessage(`true`)},
			"retries":   {Default: json.RawMessage(`20`)},
			"noDefault": {},
		}},
	}
	env, err := f.InstallEnv()
	if err != nil {
		t.Fatal(err)
	}
	got := map[string]string{}
	for _, v := range env {
		got[v.Name] = v.Value
	}
	if got["VERSION"] != "22" || got["GYP"] != "true" || got["RETRIES"] != "20" {
		t.Fatalf("env = %v", got)
	}
	if _, present := got["NODEFAULT"]; present {
		t.Fatal("an option with no default and no user value must stay unset")
	}
}

func tarball(t *testing.T, gz bool, entries ...tar.Header) []byte {
	t.Helper()
	var buf bytes.Buffer
	var w *tar.Writer
	var zw *gzip.Writer
	if gz {
		zw = gzip.NewWriter(&buf)
		w = tar.NewWriter(zw)
	} else {
		w = tar.NewWriter(&buf)
	}
	for _, h := range entries {
		h := h
		body := []byte("x")
		if h.Typeflag == tar.TypeReg {
			h.Size = int64(len(body))
		}
		if err := w.WriteHeader(&h); err != nil {
			t.Fatal(err)
		}
		if h.Typeflag == tar.TypeReg {
			_, _ = w.Write(body)
		}
	}
	_ = w.Close()
	if zw != nil {
		_ = zw.Close()
	}
	return buf.Bytes()
}

func TestExtractArchive(t *testing.T) {
	ok := tarball(t, true,
		tar.Header{Name: "./", Typeflag: tar.TypeDir, Mode: 0o755},
		tar.Header{Name: "./install.sh", Typeflag: tar.TypeReg, Mode: 0o755},
	)
	files, err := extractArchive(ok)
	if err != nil {
		t.Fatal(err)
	}
	if len(files) != 1 || files[0].Path != "install.sh" || files[0].Mode != 0o755 {
		t.Fatalf("files = %+v", files)
	}

	for name, hdr := range map[string]tar.Header{
		"parent escape": {Name: "../evil.sh", Typeflag: tar.TypeReg, Mode: 0o644},
		"nested escape": {Name: "a/../../evil.sh", Typeflag: tar.TypeReg, Mode: 0o644},
		"absolute":      {Name: "/etc/passwd", Typeflag: tar.TypeReg, Mode: 0o644},
		"symlink":       {Name: "link", Typeflag: tar.TypeSymlink, Linkname: "/etc/passwd"},
		"hardlink":      {Name: "link", Typeflag: tar.TypeLink, Linkname: "install.sh"},
	} {
		t.Run(name, func(t *testing.T) {
			if _, err := extractArchive(tarball(t, false, hdr)); err == nil {
				t.Fatal("hostile entry accepted")
			}
		})
	}
}

func TestSourceResolverRefusesDeprecatedForms(t *testing.T) {
	r := SourceResolver{}
	for _, ref := range []string{"devcontainers/features/node", "https://example.com/f.tgz"} {
		if _, err := r.Resolve(t.Context(), devcontainer.FeatureRequest{Ref: ref}); err == nil {
			t.Errorf("%s: expected a refusal", ref)
		}
	}
}
