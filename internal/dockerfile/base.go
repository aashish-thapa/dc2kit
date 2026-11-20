// Package dockerfile prepares a devcontainer's own Dockerfile so a kit
// recipe can build on it as a named stage.
package dockerfile

import (
	"fmt"
	"regexp"
	"strings"
)

// DefaultStage names the user's final stage when it has no name of its own.
const DefaultStage = "dc2kit-devcontainer"

// Base is a user Dockerfile ready to precede a kit recipe. Header holds its
// parser directives, which are only honored at the very top of a file;
// Body is the rest, with the stage to build on guaranteed a name.
type Base struct {
	Header []string
	Body   string
	Stage  string
}

var (
	directive   = regexp.MustCompile(`^#\s*([A-Za-z]+)\s*=\s*(.*?)\s*$`)
	fromLine    = regexp.MustCompile(`(?i)^\s*FROM\s`)
	stageAlias  = regexp.MustCompile(`(?i)\sAS\s+([A-Za-z0-9][A-Za-z0-9._-]*)\s*$`)
	syntaxValue = regexp.MustCompile(`(?i)^syntax$`)
)

// Prepare parses src and returns it as a Base. With a target, the named
// stage is the one to build on; without one, the last stage is, and it is
// named DefaultStage if it has no alias.
func Prepare(src []byte, target string) (Base, error) {
	lines := strings.Split(strings.ReplaceAll(string(src), "\r\n", "\n"), "\n")

	var header []string
	i := 0
	for ; i < len(lines); i++ {
		m := directive.FindStringSubmatch(lines[i])
		if m == nil {
			break
		}
		if strings.EqualFold(m[1], "escape") && m[2] != `\` {
			return Base{}, fmt.Errorf("the Dockerfile sets escape=%s; dc2kit appends instructions that use the default backslash escape", m[2])
		}
		header = append(header, lines[i])
	}
	body := lines[i:]

	type stage struct {
		lastLine int
		alias    string
	}
	var stages []stage
	for j := 0; j < len(body); j++ {
		if !fromLine.MatchString(body[j]) {
			continue
		}
		// A FROM can continue over several lines; the alias, if any, ends
		// the last of them.
		end := j
		for end < len(body)-1 && strings.HasSuffix(strings.TrimRight(body[end], " \t"), `\`) {
			end++
		}
		alias := ""
		if m := stageAlias.FindStringSubmatch(body[end]); m != nil {
			alias = m[1]
		}
		stages = append(stages, stage{lastLine: end, alias: alias})
		j = end
	}
	if len(stages) == 0 {
		return Base{}, fmt.Errorf("the Dockerfile has no FROM instruction")
	}

	base := Base{Header: header}
	if target != "" {
		for _, s := range stages {
			if strings.EqualFold(s.alias, target) {
				base.Stage = s.alias
			}
		}
		if base.Stage == "" {
			return Base{}, fmt.Errorf("build.target %q names no stage in the Dockerfile", target)
		}
	} else {
		last := stages[len(stages)-1]
		if last.alias == "" {
			body[last.lastLine] = strings.TrimRight(body[last.lastLine], " \t") + " AS " + DefaultStage
			last.alias = DefaultStage
		}
		base.Stage = last.alias
	}
	base.Body = strings.TrimRight(strings.Join(body, "\n"), "\n") + "\n"
	return base, nil
}

// HasSyntax reports whether the header already selects a frontend.
func (b Base) HasSyntax() bool {
	for _, line := range b.Header {
		if m := directive.FindStringSubmatch(line); m != nil && syntaxValue.MatchString(m[1]) {
			return true
		}
	}
	return false
}
