// Package shell quotes values for POSIX sh. Every place dc2kit turns data
// into shell text goes through here, so there is one quoting rule to audit.
package shell

import "strings"

// Quote returns s as a single POSIX shell word that expands to exactly s.
func Quote(s string) string {
	if s == "" {
		return "''"
	}
	if isPlainWord(s) {
		return s
	}
	return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
}

// Join quotes each argument and joins them into one command line whose
// word splitting reproduces argv exactly.
func Join(argv []string) string {
	words := make([]string, len(argv))
	for i, arg := range argv {
		words[i] = Quote(arg)
	}
	return strings.Join(words, " ")
}

func isPlainWord(s string) bool {
	for _, r := range s {
		isSafe := r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9' ||
			strings.ContainsRune("-_./:@%+,", r)
		if !isSafe {
			return false
		}
	}
	return true
}
