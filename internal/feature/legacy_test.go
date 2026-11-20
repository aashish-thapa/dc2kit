package feature

import "testing"

func TestLegacyRef(t *testing.T) {
	for in, want := range map[string]string{
		"sshd":   "ghcr.io/devcontainers/features/sshd:1",
		"golang": "ghcr.io/devcontainers/features/go:1",
		"common": "ghcr.io/devcontainers/features/common-utils:1",
	} {
		if got, ok := LegacyRef(in); !ok || got != want {
			t.Errorf("LegacyRef(%q) = %q, %v", in, got, ok)
		}
	}
	for _, in := range []string{"fish", "ghcr.io/devcontainers/features/sshd:1", "./sshd"} {
		if _, ok := LegacyRef(in); ok {
			t.Errorf("LegacyRef(%q) must not map", in)
		}
	}
}
