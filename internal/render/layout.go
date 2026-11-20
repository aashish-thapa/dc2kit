package render

import (
	"path"
	"path/filepath"
)

// Layout says where a kit's files go. A standalone kit keeps everything in
// one directory. An in-context kit lives inside the devcontainer's build
// context, because a kit's build context is its descriptor's directory and
// the user's Dockerfile COPYs paths relative to that context: the
// descriptor sits at the context root and dc2kit's own files go under
// AuxDir, out of the way of the project's files.
type Layout struct {
	Root   string
	AuxDir string
}

// InContextDir is where an in-context kit keeps everything but its
// descriptor.
const InContextDir = ".sbx-kit"

// Standalone lays a kit out in dir.
func Standalone(dir string) Layout { return Layout{Root: dir} }

// InContext lays a kit out inside a build context directory.
func InContext(contextDir string) Layout {
	return Layout{Root: contextDir, AuxDir: InContextDir}
}

// IsInContext reports whether the kit shares its directory with a project.
func (l Layout) IsInContext() bool { return l.AuxDir != "" }

// DockerfileRef is the descriptor's `dockerfile:` value: empty when the
// companion sits beside the descriptor under its stem.
func (l Layout) DockerfileRef(stem string) string {
	if !l.IsInContext() {
		return ""
	}
	return l.contextPath(stem + ".dockerfile")
}

func (l Layout) kitDir() string { return filepath.Join(l.Root, l.AuxDir) }

func (l Layout) descriptorPath(stem string) string {
	return filepath.Join(l.Root, stem+".yaml")
}

// contextPath is a dc2kit file's path relative to the build context, as a
// COPY source or descriptor reference spells it.
func (l Layout) contextPath(p string) string {
	if !l.IsInContext() {
		return p
	}
	return path.Join(l.AuxDir, p)
}
