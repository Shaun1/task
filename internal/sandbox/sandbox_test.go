package sandbox

import (
	"reflect"
	"testing"
)

func TestName(t *testing.T) {
	if got := New().Name("/home/u/.task/clones/repo/fix-bug"); got != "claude-fix-bug" {
		t.Fatalf("Name = %q", got)
	}
}

func TestParseList(t *testing.T) {
	out := "NAME              STATUS\nclaude-one        running\nclaude-two        stopped\n\n"
	if got, want := ParseList(out), []string{"claude-one", "claude-two"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("got %v want %v", got, want)
	}
	if got := ParseList("No sandboxes found\n"); len(got) != 0 {
		t.Fatalf("expected empty, got %v", got)
	}
}
