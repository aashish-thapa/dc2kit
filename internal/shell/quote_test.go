package shell

import (
	"os/exec"
	"testing"
)

// The only property that matters: a real shell turns Join(argv) back into
// exactly argv.
func TestJoinRoundTripsThroughSh(t *testing.T) {
	sh, err := exec.LookPath("sh")
	if err != nil {
		t.Skip("no sh on PATH")
	}
	argv := []string{"plain", "", "with space", "it's", `back\slash`, "$HOME", "`date`", "a\nb", "*", "--flag=v", "FOO=bar", `"dq"`, "!"}
	script := `for a in "$@"; do printf '%s\0' "$a"; done`
	out, err := exec.Command(sh, "-c", "eval \"set -- $0\"; "+script, Join(argv)).Output()
	if err != nil {
		t.Fatal(err)
	}
	var got []string
	start := 0
	for i, b := range out {
		if b == 0 {
			got = append(got, string(out[start:i]))
			start = i + 1
		}
	}
	if len(got) != len(argv) {
		t.Fatalf("got %d args, want %d: %q", len(got), len(argv), got)
	}
	for i := range argv {
		if got[i] != argv[i] {
			t.Errorf("arg %d: got %q, want %q", i, got[i], argv[i])
		}
	}
}

func TestAssignmentLikeWordIsQuoted(t *testing.T) {
	if got := Quote("FOO=bar"); got != "'FOO=bar'" {
		t.Fatalf("a bare FOO=bar would be parsed as an assignment, got %s", got)
	}
}
