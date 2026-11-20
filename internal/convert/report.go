package convert

import (
	"fmt"
	"strings"
)

// Level ranks a finding by whether the user must act on it.
type Level int

const (
	// Info records a translation the user may want to know about.
	Info Level = iota
	// Warn records a setting dropped or approximated in a way that can
	// change behavior inside the sandbox.
	Warn
)

func (l Level) String() string {
	if l == Warn {
		return "warn"
	}
	return "info"
}

// Finding is one note about the conversion, tied to the devcontainer key it
// concerns.
type Finding struct {
	Level   Level
	Field   string
	Message string
}

// Report collects findings in the order rules produced them.
type Report struct {
	Findings []Finding
}

func (r *Report) info(field, format string, args ...any) {
	r.Findings = append(r.Findings, Finding{Level: Info, Field: field, Message: fmt.Sprintf(format, args...)})
}

func (r *Report) warn(field, format string, args ...any) {
	r.Findings = append(r.Findings, Finding{Level: Warn, Field: field, Message: fmt.Sprintf(format, args...)})
}

// HasWarnings reports whether any finding may change behavior.
func (r Report) HasWarnings() bool {
	for _, f := range r.Findings {
		if f.Level == Warn {
			return true
		}
	}
	return false
}

// orderedSet is an insertion-ordered set of strings; generated output must
// not reorder between runs.
type orderedSet struct {
	index map[string]int
	list  []string
}

func (s *orderedSet) add(values ...string) {
	if s.index == nil {
		s.index = map[string]int{}
	}
	for _, v := range values {
		if _, seen := s.index[v]; !seen {
			s.index[v] = len(s.list)
			s.list = append(s.list, v)
		}
	}
}

func (s *orderedSet) remove(v string) {
	i, ok := s.index[v]
	if !ok {
		return
	}
	s.list = append(s.list[:i], s.list[i+1:]...)
	delete(s.index, v)
	for j := i; j < len(s.list); j++ {
		s.index[s.list[j]] = j
	}
}

func (s *orderedSet) items() []string {
	return append([]string(nil), s.list...)
}

func joinSentences(parts []string) string { return strings.Join(parts, " ") }

func joinParagraphs(parts []string) string { return strings.Join(parts, "\n\n") + "\n" }
