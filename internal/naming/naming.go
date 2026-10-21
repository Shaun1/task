// Package naming derives branch/directory/sandbox-safe names from free text.
package naming

import (
	"errors"
	"strings"
)

// MaxSlugLen caps the slug length so branch, directory and sandbox names stay readable.
const MaxSlugLen = 40

// ErrEmptySlug is returned when a description contains no usable characters.
var ErrEmptySlug = errors.New("description has no letters or digits to build a name from")

// Slugify turns a task description into a lowercase, hyphen-separated slug:
// "Fix login redirect bug!" -> "fix-login-redirect-bug".
func Slugify(desc string) (string, error) {
	var b strings.Builder
	lastDash := true // suppresses a leading dash
	for _, r := range strings.ToLower(desc) {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
			b.WriteRune(r)
			lastDash = false
			continue
		}
		if !lastDash {
			b.WriteByte('-')
			lastDash = true
		}
	}
	s := strings.TrimRight(b.String(), "-")
	if len(s) > MaxSlugLen {
		s = strings.TrimRight(s[:MaxSlugLen], "-")
	}
	if s == "" {
		return "", ErrEmptySlug
	}
	return s, nil
}

// RepoName extracts the repository name from a git remote URL or local path.
//
//	git@github.com:org/repo.git       -> repo
//	https://github.com/org/repo.git/  -> repo
//	/srv/git/repo.git                 -> repo
func RepoName(remoteURL string) string {
	s := strings.TrimSpace(remoteURL)
	s = strings.TrimRight(s, "/")
	s = strings.TrimSuffix(s, ".git")
	if i := strings.LastIndexAny(s, "/:"); i >= 0 {
		s = s[i+1:]
	}
	if s == "" {
		return "repo"
	}
	return s
}
