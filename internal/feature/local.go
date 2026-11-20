package feature

import (
	"context"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/aashish-thapa/dc2kit/internal/devcontainer"
)

// LocalResolver reads features stored beside devcontainer.json, referenced
// as "./name" or "../name" relative to the directory that holds it.
type LocalResolver struct {
	ConfigDir string
}

// Resolve loads the feature folder. Symlinks are refused for the same
// reason archive links are: the payload is copied into a build context.
func (l LocalResolver) Resolve(_ context.Context, req devcontainer.FeatureRequest) (Feature, error) {
	root := filepath.Join(l.ConfigDir, filepath.FromSlash(req.Ref))
	var files []File
	err := filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		if !d.Type().IsRegular() {
			return fmt.Errorf("%s: only regular files are allowed in a feature", p)
		}
		rel, err := filepath.Rel(root, p)
		if err != nil {
			return err
		}
		info, err := d.Info()
		if err != nil {
			return err
		}
		data, err := os.ReadFile(p)
		if err != nil {
			return err
		}
		files = append(files, File{Path: filepath.ToSlash(rel), Mode: info.Mode().Perm(), Data: data})
		return nil
	})
	if err != nil {
		return Feature{}, fmt.Errorf("local feature %q: %w", req.Ref, err)
	}
	return fromFiles(req, "local:"+filepath.ToSlash(filepath.Clean(req.Ref)), "local", files)
}

// IsLocalRef reports whether a feature reference names a folder beside
// devcontainer.json rather than a registry artifact.
func IsLocalRef(ref string) bool {
	return strings.HasPrefix(ref, "./") || strings.HasPrefix(ref, "../")
}

// SourceResolver dispatches each reference to the resolver for its form.
// Deprecated forms (GitHub release shorthand, tarball URLs) are refused by
// name instead of being guessed at.
type SourceResolver struct {
	Local    Resolver
	Registry Resolver
}

func (s SourceResolver) Resolve(ctx context.Context, req devcontainer.FeatureRequest) (Feature, error) {
	switch {
	case IsLocalRef(req.Ref):
		return s.Local.Resolve(ctx, req)
	case strings.HasPrefix(req.Ref, "http://"), strings.HasPrefix(req.Ref, "https://"):
		return Feature{}, fmt.Errorf("feature %q: tarball URL features are not supported; publish it to an OCI registry", req.Ref)
	case !looksLikeRegistryHost(strings.SplitN(req.Ref, "/", 2)[0]):
		return Feature{}, fmt.Errorf("feature %q: the GitHub-release shorthand is deprecated by the Features spec; use the ghcr.io reference", req.Ref)
	default:
		return s.Registry.Resolve(ctx, req)
	}
}

// looksLikeRegistryHost applies the same test image references use for
// their first path component: a dot, a port, or the name localhost.
func looksLikeRegistryHost(component string) bool {
	return strings.ContainsAny(component, ".:") || component == "localhost"
}
