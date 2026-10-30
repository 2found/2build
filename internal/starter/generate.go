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

// Runner executes one argv in dir; bootstrap supplies it for scaffold commands.
type Runner func(dir string, argv []string) error

// Generate builds in a sibling temporary directory and refuses existing targets.
// Scaffold commands run first in the empty stage; common/ and the template then
// overlay their output. Verification runs before publication; failed generation
// leaves no project behind.
func Generate(root, target, id, revision string, run Runner, verify func(string, Template) error) (Lock, error) {
	var lock Lock
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
	// The stage is named after the project: scaffolders such as create-next-app
	// derive the package name from the directory they generate into.
	staging, err := os.MkdirTemp(filepath.Dir(target), ".bbs-starter-")
	if err != nil {
		return lock, err
	}
	defer os.RemoveAll(staging)
	stage := filepath.Join(staging, name)
	if err := os.Mkdir(stage, 0o755); err != nil {
		return lock, err
	}
	if len(template.Scaffold) > 0 {
		if run == nil {
			return lock, fmt.Errorf("starter template %q needs a scaffold runner", id)
		}
		for _, argv := range template.Scaffold {
			substituted := make([]string, len(argv))
			for i, arg := range argv {
				substituted[i] = strings.ReplaceAll(arg, "__PROJECT_NAME__", name)
			}
			if err := run(stage, substituted); err != nil {
				return lock, fmt.Errorf("starter scaffold failed; target was not created: %s: %w", strings.Join(substituted, " "), err)
			}
		}
		// Bootstrap never initializes Git; the caller owns repository creation.
		if err := os.RemoveAll(filepath.Join(stage, ".git")); err != nil {
			return lock, err
		}
	}
	// Starter files may replace scaffold output, never each other.
	copied := map[string]bool{}
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
			// Older starter releases ship a git-flow profile 2build no longer reads.
			if rel == filepath.Join(".babysit", "git-flow.yaml") {
				return nil
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
			b = []byte(strings.ReplaceAll(string(b), "__PROJECT_NAME__", name))
			if copied[rel] {
				return fmt.Errorf("starter file collision: %s", rel)
			}
			copied[rel] = true
			if info, err := os.Lstat(destination); err == nil {
				if !info.Mode().IsRegular() {
					return fmt.Errorf("starter file collision: %s", rel)
				}
				if err := os.Remove(destination); err != nil {
					return err
				}
			}
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
	if err := os.MkdirAll(filepath.Dir(filepath.Join(stage, LockPath)), 0o755); err != nil {
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
