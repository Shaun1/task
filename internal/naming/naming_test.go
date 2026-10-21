package naming

import (
	"errors"
	"strings"
	"testing"
)

func TestSlugify(t *testing.T) {
	cases := map[string]string{
		"fix login redirect bug":    "fix-login-redirect-bug",
		"  Fix: the BIG bug!!  ":    "fix-the-big-bug",
		"a---b___c":                 "a-b-c",
		"Add OAuth2 support (v2)":   "add-oauth2-support-v2",
		"ünïcode café":              "n-code-caf",
		strings.Repeat("abcd ", 20): "abcd-abcd-abcd-abcd-abcd-abcd-abcd-abcd",
	}
	for in, want := range cases {
		got, err := Slugify(in)
		if err != nil {
			t.Errorf("Slugify(%q) error: %v", in, err)
			continue
		}
		if got != want {
			t.Errorf("Slugify(%q) = %q, want %q", in, got, want)
		}
		if len(got) > MaxSlugLen || strings.HasSuffix(got, "-") {
			t.Errorf("Slugify(%q) = %q violates length/trailing-dash rules", in, got)
		}
	}
}

func TestSlugifyEmpty(t *testing.T) {
	for _, in := range []string{"", "   ", "!!!", "---"} {
		if _, err := Slugify(in); !errors.Is(err, ErrEmptySlug) {
			t.Errorf("Slugify(%q) err = %v, want ErrEmptySlug", in, err)
		}
	}
}

func TestRepoName(t *testing.T) {
	cases := map[string]string{
		"git@github.com:org/repo.git":      "repo",
		"https://github.com/org/repo.git":  "repo",
		"https://github.com/org/repo.git/": "repo",
		"https://github.com/org/repo":      "repo",
		"/srv/git/my-repo.git":             "my-repo",
		"ssh://git@host:2222/team/x.git":   "x",
		"":                                 "repo",
	}
	for in, want := range cases {
		if got := RepoName(in); got != want {
			t.Errorf("RepoName(%q) = %q, want %q", in, got, want)
		}
	}
}
