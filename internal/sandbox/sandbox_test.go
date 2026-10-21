package sandbox

import (
	"os/exec"
	"reflect"
	"strings"
	"testing"
)

func TestName(t *testing.T) {
	if got := New().Name("/home/u/.task/clones/repo/fix-bug"); got != "claude-fix-bug" {
		t.Fatalf("Name = %q", got)
	}
}

func TestParseList(t *testing.T) {
	out := "claude-one\nclaude-two\n\n"
	if got, want := ParseList(out), []string{"claude-one", "claude-two"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("got %v want %v", got, want)
	}
	if got := ParseList(""); len(got) != 0 {
		t.Fatalf("expected empty, got %v", got)
	}
}

func TestIsSSHURL(t *testing.T) {
	for u, want := range map[string]bool{
		"git@github.com:o/r.git":       true,
		"ssh://git@github.com/o/r.git": true,
		"https://github.com/o/r.git":   false,
	} {
		if got := isSSHURL(u); got != want {
			t.Errorf("isSSHURL(%q) = %v", u, got)
		}
	}
}

func TestSSHAgentProblem(t *testing.T) {
	if _, err := exec.LookPath("ssh-add"); err != nil {
		t.Skip("ssh-add not installed")
	}
	t.Setenv("SSH_AUTH_SOCK", "/nonexistent/agent.sock")
	if got := sshAgentProblem(); !strings.Contains(got, "no SSH agent") {
		t.Fatalf("sshAgentProblem() = %q", got)
	}
}

func TestParseHasSecret(t *testing.T) {
	out := []byte(`{"secrets":[{"scope":"global","type":"service","name":"github","secret":"(stored)"}],"custom_secrets":[]}`)
	if ok, err := parseHasSecret(out, "github"); err != nil || !ok {
		t.Fatalf("got %v, %v; want true", ok, err)
	}
	if ok, err := parseHasSecret([]byte(`{"secrets":[]}`), "github"); err != nil || ok {
		t.Fatalf("empty: got %v, %v; want false", ok, err)
	}
	scoped := []byte(`{"secrets":[{"scope":"sandbox:x","name":"github"}]}`)
	if ok, _ := parseHasSecret(scoped, "github"); ok {
		t.Fatal("sandbox-scoped secret should not count")
	}
}
