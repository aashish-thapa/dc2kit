package convert

import (
	"regexp"
	"strings"
)

// ImageEnv is one ENV instruction. Its value is split into literal text and
// variable references because the two need different treatment in a
// Dockerfile: literals must have `$` escaped, references must not.
type ImageEnv struct {
	Name  string
	Value EnvValue
}

// EnvValue is an ENV value as an ordered list of parts.
type EnvValue []EnvPart

// EnvPart is either literal text or a reference to another variable,
// never both; build parts with literal and ref.
type EnvPart struct {
	text  string
	isRef bool
}

func literal(s string) EnvPart { return EnvPart{text: s} }
func ref(name string) EnvPart  { return EnvPart{text: name, isRef: true} }

// Text returns the literal text or the referenced variable name.
func (p EnvPart) Text() string { return p.text }

// IsRef reports whether the part names a variable to expand.
func (p EnvPart) IsRef() bool { return p.isRef }

// userEnvValue parses a devcontainer.json containerEnv/remoteEnv value. The
// devcontainer CLI passes these to `docker run -e` without shell expansion,
// so only `${containerEnv:NAME}` is a reference; every other `$` is literal.
func userEnvValue(raw string) (EnvValue, []string) {
	var value EnvValue
	var unresolved []string
	last := 0
	for _, loc := range devcontainerVar.FindAllStringSubmatchIndex(raw, -1) {
		value = appendLiteral(value, raw[last:loc[0]])
		match := raw[loc[0]:loc[1]]
		kind := raw[loc[2]:loc[3]]
		name := submatch(raw, loc, 2)
		fallback := submatch(raw, loc, 3)
		switch {
		case kind == "containerEnv" && name != "":
			// The devcontainer CLI resolves these against the running
			// container, but a Dockerfile ENV expands only variables the
			// image declared, and HOME and USER usually are not. Their
			// sandbox values are fixed, so they resolve here.
			if fixed, ok := fixedAccountEnv[name]; ok {
				value = appendLiteral(value, fixed)
			} else {
				value = append(value, ref(name))
			}
		case kind == "containerWorkspaceFolder":
			value = appendLiteral(value, WorkspaceDir)
		case kind == "containerWorkspaceFolderBasename":
			value = appendLiteral(value, "workspace")
		default:
			unresolved = append(unresolved, match)
			value = appendLiteral(value, fallback)
		}
		last = loc[1]
	}
	return appendLiteral(value, raw[last:]), unresolved
}

var fixedAccountEnv = map[string]string{"HOME": AgentHome, "USER": AgentUser}

var dockerfileVarRef = regexp.MustCompile(`\$\{([A-Za-z_][A-Za-z0-9_]*)\}|\$([A-Za-z_][A-Za-z0-9_]*)`)

// featureEnvValue parses a feature's containerEnv value. The devcontainer
// CLI writes these as Dockerfile ENV lines, so `$NAME` and `${NAME}` expand
// (that is how features prepend to PATH).
func featureEnvValue(raw string) EnvValue {
	var value EnvValue
	last := 0
	for _, loc := range dockerfileVarRef.FindAllStringSubmatchIndex(raw, -1) {
		value = appendLiteral(value, raw[last:loc[0]])
		name := submatch(raw, loc, 1)
		if name == "" {
			name = submatch(raw, loc, 2)
		}
		value = append(value, ref(name))
		last = loc[1]
	}
	return appendLiteral(value, raw[last:])
}

func appendLiteral(value EnvValue, s string) EnvValue {
	if s == "" {
		return value
	}
	if n := len(value); n > 0 && !value[n-1].isRef {
		value[n-1].text += s
		return value
	}
	return append(value, literal(s))
}

func submatch(s string, loc []int, group int) string {
	start, end := loc[2*group], loc[2*group+1]
	if start < 0 {
		return ""
	}
	return s[start:end]
}

// containsNewline reports whether any literal part would break an ENV line.
func (v EnvValue) containsNewline() bool {
	for _, p := range v {
		if strings.ContainsAny(p.text, "\r\n") {
			return true
		}
	}
	return false
}
