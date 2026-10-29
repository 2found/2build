package cmd

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/2found/2build/internal/identity"
	"github.com/2found/2build/internal/ticket"
)

// A gate's subject binds the ticket's own change, not the whole checkout. The
// first checkpoint written from the work records a baseline in checkpoint.json:
// HEAD then, and the paths already dirty then (another session's or the
// owner's work in a shared checkout). The ticket's scope is every path that
// differs from baseline.head, minus those baseline paths, plus any the ticket
// claims (`verification claim`) because it edits them too. A change inside the
// scope, or a path joining it, changes tree_digest; a change outside does not.
// A checkpoint without a baseline keeps the whole-tree digest.

type ticketBaseline struct {
	Head    string
	Paths   map[string]bool
	Claimed map[string]bool
}

func stringSet(v interface{}) map[string]bool {
	set := map[string]bool{}
	for _, x := range interfaceSlice(v) {
		if s, ok := x.(string); ok {
			set[s] = true
		}
	}
	return set
}

func sortedKeys(set map[string]bool) []string {
	keys := make([]string, 0, len(set))
	for k := range set {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

func baselineFromCheckpoint(cp map[string]interface{}) *ticketBaseline {
	raw, ok := cp["baseline"].(map[string]interface{})
	if !ok || stringValue(raw["head"]) == "" {
		return nil
	}
	return &ticketBaseline{Head: stringValue(raw["head"]), Paths: stringSet(raw["paths"]), Claimed: stringSet(raw["claimed"])}
}

// dirtyPathsSince lists paths whose working content differs from commit
// (staged or not) plus untracked, non-ignored paths.
func dirtyPathsSince(dir, commit string) (map[string]bool, error) {
	set := map[string]bool{}
	for _, args := range [][]string{
		{"diff", "--name-only", "-z", "--no-renames", commit},
		{"ls-files", "-z", "--others", "--exclude-standard"},
	} {
		raw, err := gitOutputBytes(dir, args...)
		if err != nil {
			return nil, err
		}
		for _, rel := range strings.Split(string(raw), "\x00") {
			if rel != "" {
				set[rel] = true
			}
		}
	}
	return set, nil
}

// captureBaseline returns the checkpoint "baseline" value for the checkout in
// the current directory, or nil when it has no commit to compare against.
func captureBaseline() map[string]interface{} {
	top, head := gitOut("rev-parse", "--show-toplevel"), gitOut("rev-parse", "HEAD")
	if top == "" || head == "" {
		return nil
	}
	dirty, err := dirtyPathsSince(top, head)
	if err != nil {
		return nil
	}
	return map[string]interface{}{"head": head, "paths": sortedKeys(dirty), "claimed": []string{}}
}

// scopedTreeDigest hashes the content of the ticket's scope; a path deleted
// from the tree is part of the digest. Any git failure falls back to the
// whole-tree digest, which is stricter.
func scopedTreeDigest(dir string, b ticketBaseline) (string, error) {
	dirty, err := dirtyPathsSince(dir, b.Head)
	if err != nil {
		return workingTreeDigest(dir)
	}
	var scope []string
	for rel := range dirty {
		if !b.Paths[rel] || b.Claimed[rel] {
			scope = append(scope, rel)
		}
	}
	sort.Strings(scope)
	h := sha256.New()
	_, _ = io.WriteString(h, "ticket-scope\x00")
	for _, rel := range scope {
		_, _ = io.WriteString(h, "\x00"+rel+"\x00")
		path := filepath.Join(dir, filepath.FromSlash(rel))
		fi, statErr := os.Lstat(path)
		if statErr != nil {
			_, _ = io.WriteString(h, "deleted")
			continue
		}
		_, _ = io.WriteString(h, fi.Mode().String()+"\x00")
		switch {
		case fi.Mode()&os.ModeSymlink != 0:
			target, readErr := os.Readlink(path)
			if readErr != nil {
				return "", readErr
			}
			_, _ = io.WriteString(h, target)
		case fi.Mode().IsRegular():
			f, openErr := os.Open(path)
			if openErr != nil {
				return "", openErr
			}
			_, copyErr := io.Copy(h, f)
			_ = f.Close()
			if copyErr != nil {
				return "", copyErr
			}
		}
	}
	return "sha256:" + hex.EncodeToString(h.Sum(nil)), nil
}

// ticketTreeDigestFunc picks the digest a ticket's gates bind: its scope when
// its checkpoint recorded a baseline, otherwise the whole tree.
func ticketTreeDigestFunc(ticketHome string) func(string) (string, error) {
	if ticketHome != "" {
		if cp, err := readStrictObject(filepath.Join(ticketHome, "checkpoint.json"), false); err == nil {
			if b := baselineFromCheckpoint(cp); b != nil {
				return func(dir string) (string, error) { return scopedTreeDigest(dir, *b) }
			}
		}
	}
	return workingTreeDigest
}

// claimPaths adds baseline paths the ticket edits to its scope. It changes the
// subject, so a running gate must begin again.
func (a *apState) claimPaths(env identity.Env, paths []string) error {
	if len(paths) == 0 {
		return fmt.Errorf("claim needs one or more repo-relative paths")
	}
	st := ticket.New(env)
	return withLock(st, func() error {
		path := filepath.Join(st.Home(), "checkpoint.json")
		cp, err := readStrictObject(path, true)
		if err != nil {
			return err
		}
		b := baselineFromCheckpoint(cp)
		if b == nil {
			return fmt.Errorf("checkpoint has no baseline; the whole tree is already bound")
		}
		for _, p := range paths {
			p = filepath.ToSlash(filepath.Clean(p))
			if filepath.IsAbs(p) || p == "." || strings.HasPrefix(p, "../") {
				return fmt.Errorf("claim path %q must be repo-relative", p)
			}
			b.Claimed[p] = true
		}
		cp["baseline"] = map[string]interface{}{"head": b.Head, "paths": sortedKeys(b.Paths), "claimed": sortedKeys(b.Claimed)}
		cp["revision"] = int64Value(cp["revision"]) + 1
		cp["updated_at"] = isoNow()
		return writeJSONAtomic(path, cp)
	})
}
