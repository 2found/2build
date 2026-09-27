package workspace

import (
	"path/filepath"
	"runtime"
	"strings"
)

// Resolver answers topology questions for one checkout from the unified config.
// It resolves membership once because ticket serving asks for several sibling
// roles against the same checkout.
type Resolver struct {
	repo  Repo
	found bool
	ws    Workspace
	err   error
}

// NewResolver matches the checkout by its registered path or git URL.
func NewResolver(toplevel, gitURL string) *Resolver {
	r := &Resolver{}
	r.ws, r.repo, r.found, r.err = Find(toplevel, gitURL)
	return r
}

func (r *Resolver) Err() error { return r.err }

func (r *Resolver) Registered() bool { return r.found && r.err == nil }

func (r *Resolver) Name() string {
	if !r.Registered() {
		return ""
	}
	return r.ws.Name
}

// Repo returns the matched repository entry.
func (r *Resolver) Repo() (Repo, bool) { return r.repo, r.Registered() }

// FanOut is disabled only for a registered monorepo.
func (r *Resolver) FanOut() bool {
	return !(r.Registered() && r.repo.RepoType == TypeMonorepo)
}

// RolePath resolves a sibling role to its local path through the registry.
func (r *Resolver) RolePath(role string) (string, bool) {
	if !r.Registered() || role == "" {
		return "", false
	}
	for _, e := range r.ws.Repos {
		if e.Role == role && e.Path != "" {
			return e.Path, true
		}
	}
	return "", false
}

// SamePath reports whether two paths name the same directory. It is exported
// because the same question decides both workspace membership and whether the
// registry and .babysit/.env disagree about a sibling — and a string compare
// there would BLOCK on `~/src/api` vs `/Users/x/src/api`, which is not a
// disagreement.
func SamePath(a, b string) bool { return samePath(a, b) }

func samePath(a, b string) bool {
	if a == b {
		return true
	}
	return pathEqual(canonPath(a), canonPath(b))
}

// fsCaseInsensitive mirrors the platform filesystem's default case handling:
// NTFS compares case-insensitively, so `C:\Repo` and `c:\repo` are the same
// directory and a case-sensitive compare would false-negative workspace
// matching. Tests flip it to exercise the Windows branch on any host.
var fsCaseInsensitive = runtime.GOOS == "windows"

func pathEqual(a, b string) bool {
	if fsCaseInsensitive {
		return strings.EqualFold(a, b)
	}
	return a == b
}

// canonPath is absolute + symlinks resolved where possible. On macOS /tmp is a
// symlink to /private/tmp, so two spellings of one directory are routine; when
// the path does not exist yet (a repo listed but not cloned here) EvalSymlinks
// fails and the absolute form is the best available answer.
func canonPath(p string) string {
	abs, err := filepath.Abs(p)
	if err != nil {
		return filepath.Clean(p)
	}
	if resolved, err := filepath.EvalSymlinks(abs); err == nil {
		return resolved
	}
	return filepath.Clean(abs)
}
