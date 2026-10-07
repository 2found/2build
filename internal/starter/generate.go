package starter

import (
	"crypto/sha256"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

// Generate builds in a sibling temporary directory and refuses existing targets.
// Verification runs before publication; failed generation leaves no project behind.
func Generate(root, target, id, profile, revision string, verify func(string, Template) error) (Lock, error) {
	var lock Lock
	if profile != "pet" && profile != "startup" && profile != "enterprise" {
		return lock, fmt.Errorf("profile must be pet, startup, or enterprise")
	}
	target, err := filepath.Abs(target)
	if err != nil {
		return lock, err
	}
	name := filepath.Base(target)
	if !ValidName(name) {
		return lock, fmt.Errorf("project name must be lowercase kebab-case (max 64 characters)")
	}
	if _, err := os.Lstat(target); !os.IsNotExist(err) {
		return lock, fmt.Errorf("target already exists or cannot be accessed: %s", target)
	}
	catalog, err := ReadCatalog(root)
	if err != nil {
		return lock, err
	}
	root, err = filepath.EvalSymlinks(root)
	if err != nil {
		return lock, err
	}
	root, err = filepath.Abs(root)
	if err != nil {
		return lock, err
	}
	template, ok := catalog.Templates[id]
	if !ok {
		return lock, fmt.Errorf("unknown starter template %q", id)
	}
	stage, err := os.MkdirTemp(filepath.Dir(target), ".bbs-starter-")
	if err != nil {
		return lock, err
	}
	defer os.RemoveAll(stage)
	digest := sha256.New()
	// Include catalog bytes and source files in local snapshot provenance.
	body, err := os.ReadFile(filepath.Join(root, "catalog.json"))
	if err != nil {
		return lock, err
	}
	digest.Write(body)
	for _, folder := range []string{"common", template.Path} {
		base := filepath.Join(root, filepath.FromSlash(folder))
		// WalkDir does not follow symlinks. Check ancestors too, including root.
		resolved, err := filepath.EvalSymlinks(base)
		if err != nil {
			return lock, err
		}
		absolute, err := filepath.Abs(base)
		if err != nil {
			return lock, err
		}
		if resolved != absolute {
			return lock, fmt.Errorf("starter source must not contain symlink paths: %s", folder)
		}
		err = filepath.WalkDir(base, func(path string, entry fs.DirEntry, walkErr error) error {
			if walkErr != nil {
				return walkErr
			}
			rel, err := filepath.Rel(base, path)
			if err != nil {
				return err
			}
			if entry.Type()&os.ModeSymlink != 0 {
				return fmt.Errorf("starter source contains symlink: %s", path)
			}
			if entry.IsDir() && (entry.Name() == "node_modules" || entry.Name() == "dist" || entry.Name() == ".git") {
				return filepath.SkipDir
			}
			destination := filepath.Join(stage, rel)
			if entry.IsDir() {
				return os.MkdirAll(destination, 0o755)
			}
			if !entry.Type().IsRegular() {
				return fmt.Errorf("starter source contains non-regular file: %s", path)
			}
			if (entry.Name() == ".env" || strings.HasPrefix(entry.Name(), ".env.") && entry.Name() != ".env.example") || rel == filepath.FromSlash(LockPath) {
				return fmt.Errorf("starter contains reserved file: %s", rel)
			}
			b, err := os.ReadFile(path)
			if err != nil {
				return err
			}
			fmt.Fprintf(digest, "%s/%s\x00%d\x00", folder, filepath.ToSlash(rel), len(b))
			digest.Write(b)
			b = []byte(strings.ReplaceAll(strings.ReplaceAll(string(b), "__PROJECT_NAME__", name), "__GIT_PROFILE__", profile))
			f, err := os.OpenFile(destination, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o644)
			if err != nil {
				return fmt.Errorf("starter file collision: %s: %w", rel, err)
			}
			_, writeErr := f.Write(b)
			closeErr := f.Close()
			if writeErr != nil {
				return writeErr
			}
			return closeErr
		})
		if err != nil {
			return lock, err
		}
	}
	if revision == "" {
		revision = fmt.Sprintf("sha256:%x", digest.Sum(nil))
	}
	lock = Lock{SchemaVersion: 1, Source: Source, Release: catalog.Version, Template: id, TemplateRevision: template.Revision, Revision: revision}
	data, err := yaml.Marshal(lock)
	if err != nil {
		return lock, err
	}
	if err := os.WriteFile(filepath.Join(stage, LockPath), data, 0o644); err != nil {
		return lock, err
	}
	if verify != nil {
		if err := verify(stage, template); err != nil {
			return lock, fmt.Errorf("starter verification failed; target was not created: %w", err)
		}
	}
	// Reserve the name with mkdir: rename alone can replace a concurrently
	// created empty directory on some platforms. Move children into our own
	// reservation; renaming a directory over a reservation is not portable.
	if err := os.Mkdir(target, 0o755); err != nil {
		return lock, err
	}
	entries, err := os.ReadDir(stage)
	if err != nil {
		_ = os.Remove(target)
		return lock, err
	}
	for _, entry := range entries {
		if err := os.Rename(filepath.Join(stage, entry.Name()), filepath.Join(target, entry.Name())); err != nil {
			_ = os.RemoveAll(target)
			return lock, err
		}
	}
	return lock, nil
}
