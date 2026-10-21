package host

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// TestOpenIDEAppendsDir checks the command keeps its own arguments and gets
// the directory (spaces included) as one final argument.
func TestOpenIDEAppendsDir(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("uses sh")
	}
	tmp := t.TempDir()
	out := filepath.Join(tmp, "args")
	dir := filepath.Join(tmp, "my clone")
	if err := (Host{}).OpenIDE("printf '%s\\n' --flag >"+out, dir); err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(out)
	if err != nil {
		t.Fatal(err)
	}
	if want := "--flag\n" + dir + "\n"; string(got) != want {
		t.Fatalf("args = %q, want %q", got, want)
	}
}

func TestOpenIDEFindsToolboxScripts(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("uses sh")
	}
	home := t.TempDir()
	t.Setenv("HOME", home)
	scripts := toolboxScripts()
	if !strings.HasPrefix(scripts, home) {
		t.Fatalf("toolboxScripts() = %s, want under HOME", scripts)
	}
	if err := os.MkdirAll(scripts, 0o755); err != nil {
		t.Fatal(err)
	}
	marker := filepath.Join(home, "ran")
	script := "#!/bin/sh\necho \"$1\" > " + marker + "\n"
	if err := os.WriteFile(filepath.Join(scripts, "fakeide"), []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", "/usr/bin:/bin")
	if err := (Host{}).OpenIDE("fakeide", "/some/clone"); err != nil {
		t.Fatal(err)
	}
	if got, _ := os.ReadFile(marker); strings.TrimSpace(string(got)) != "/some/clone" {
		t.Fatalf("fakeide got %q", got)
	}
}

func TestIDECandidates(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	bin := filepath.Join(home, "bin")
	for dir, name := range map[string]string{toolboxScripts(): "pycharm", bin: "goland"} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, name), []byte("#!/bin/sh\n"), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	// Not executable: must be ignored.
	if err := os.WriteFile(filepath.Join(bin, "code"), nil, 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", bin)
	if got := strings.Join((Host{}).IDECandidates(), ","); got != "goland,pycharm" {
		t.Fatalf("IDECandidates() = %s", got)
	}
}
