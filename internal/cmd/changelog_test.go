package cmd

import (
	"bytes"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

func writeTestFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func readTestFile(t *testing.T, path string) string {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

func runChangelog(t *testing.T, args ...string) (string, error) {
	t.Helper()
	c := newChangelogCmd()
	var out bytes.Buffer
	c.SetOut(&out)
	c.SetErr(&out)
	c.SetArgs(args)
	err := c.Execute()
	return out.String(), err
}

// add prepends above the newest section, keeps the header, and a heading
// inside a code fence does not split a section.
func TestChangelogAddPrepends(t *testing.T) {
	dir := t.TempDir()
	log := filepath.Join(dir, "CHANGELOG.md")
	notes := filepath.Join(dir, "notes.md")
	writeTestFile(t, log, "# Changelog\n## 1.0.0 — 2026-01-01\n\n- first\n\n```md\n## not a heading\n```\n")
	writeTestFile(t, notes, "\n## 1.1.0 — 2026-02-01\n\n- second\n")

	out, err := runChangelog(t, "add", "--file", log, "--notes", notes)
	if err != nil {
		t.Fatal(err)
	}
	got := readTestFile(t, log)
	want := "# Changelog\n\n## 1.1.0 — 2026-02-01\n\n- second\n\n## 1.0.0 — 2026-01-01\n\n- first\n\n```md\n## not a heading\n```\n"
	if got != want {
		t.Fatalf("changelog:\n%s\nwant:\n%s", got, want)
	}
	if !strings.Contains(out, "sections=2") || !strings.Contains(out, "rotate: not needed") {
		t.Fatalf("report: %s", out)
	}
}

// A version already present is refused and the file is left untouched;
// notes without a heading are refused.
func TestChangelogAddRefusesDuplicateAndHeadless(t *testing.T) {
	dir := t.TempDir()
	log := filepath.Join(dir, "CHANGELOG.md")
	notes := filepath.Join(dir, "notes.md")
	before := "# Changelog\n\n## 1.0.0 — 2026-01-01\n\n- first\n"
	writeTestFile(t, log, before)

	writeTestFile(t, notes, "## 1.0.0 — 2026-03-03\n\n- again\n")
	if _, err := runChangelog(t, "add", "--file", log, "--notes", notes); err == nil || !strings.Contains(err.Error(), "already has") {
		t.Fatalf("duplicate version: err=%v", err)
	}
	writeTestFile(t, notes, "- no heading\n")
	if _, err := runChangelog(t, "add", "--file", log, "--notes", notes); err == nil {
		t.Fatal("headless notes accepted")
	}
	if got := readTestFile(t, log); got != before {
		t.Fatalf("file changed:\n%s", got)
	}
}

// add creates a missing changelog.
func TestChangelogAddCreates(t *testing.T) {
	dir := t.TempDir()
	log := filepath.Join(dir, "CHANGELOG.md")
	notes := filepath.Join(dir, "notes.md")
	writeTestFile(t, notes, "## 0.1.0\n\n- hello\n")
	if _, err := runChangelog(t, "add", "--file", log, "--notes", notes); err != nil {
		t.Fatal(err)
	}
	if got := readTestFile(t, log); got != "# Changelog\n\n## 0.1.0\n\n- hello\n" {
		t.Fatalf("new changelog:\n%q", got)
	}
}

func sampleChangelog(n int) string {
	var b strings.Builder
	b.WriteString("# Changelog\n")
	for i := n; i >= 1; i-- {
		b.WriteString("\n## 1." + strconv.Itoa(i) + ".0\n\n- change " + strings.Repeat("x", 100) + "\n")
	}
	return b.String()
}

// check suggests rotation only past the byte limit and with more sections
// than it would keep; it never writes.
func TestChangelogCheckSuggestsRotation(t *testing.T) {
	dir := t.TempDir()
	log := filepath.Join(dir, "CHANGELOG.md")
	content := sampleChangelog(30)
	writeTestFile(t, log, content)

	out, err := runChangelog(t, "check", "--file", log, "--max-bytes", "1000", "--keep", "10")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "rotate: suggested") || !strings.Contains(out, "--keep 10") {
		t.Fatalf("expected a suggestion: %s", out)
	}
	out, _ = runChangelog(t, "check", "--file", log)
	if !strings.Contains(out, "rotate: not needed") {
		t.Fatalf("under the default limit: %s", out)
	}
	if readTestFile(t, log) != content {
		t.Fatal("check wrote the file")
	}
	if _, err := runChangelog(t, "check", "--file", filepath.Join(dir, "missing.md")); err == nil {
		t.Fatal("check on a missing file should fail")
	}
}

// rotate keeps the newest N, prepends the rest to the archive newest-first,
// links the archive once, and loses no section across two rotations.
func TestChangelogRotate(t *testing.T) {
	dir := t.TempDir()
	log := filepath.Join(dir, "CHANGELOG.md")
	archive := filepath.Join(dir, changelogArchiveName)
	writeTestFile(t, log, sampleChangelog(30))

	out, err := runChangelog(t, "rotate", "--file", log, "--keep", "10")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "moved 20 sections") {
		t.Fatalf("report: %s", out)
	}
	kept := parseChangelog(readTestFile(t, log))
	arch := parseChangelog(readTestFile(t, archive))
	if len(kept.sections) != 10 || sectionKey(kept.sections[0]) != "1.30.0" || sectionKey(kept.sections[9]) != "1.21.0" {
		t.Fatalf("kept: %d sections, first %q", len(kept.sections), sectionKey(kept.sections[0]))
	}
	if len(arch.sections) != 20 || sectionKey(arch.sections[0]) != "1.20.0" || sectionKey(arch.sections[19]) != "1.1.0" {
		t.Fatalf("archive: %d sections", len(arch.sections))
	}
	if !strings.Contains(kept.header, "Older releases: [CHANGELOG-ARCHIVE.md](CHANGELOG-ARCHIVE.md).") {
		t.Fatalf("missing archive link in header:\n%s", kept.header)
	}

	if _, err := runChangelog(t, "rotate", "--file", log, "--keep", "4"); err != nil {
		t.Fatal(err)
	}
	kept = parseChangelog(readTestFile(t, log))
	arch = parseChangelog(readTestFile(t, archive))
	if len(kept.sections) != 4 || len(arch.sections) != 26 || sectionKey(arch.sections[0]) != "1.26.0" || sectionKey(arch.sections[25]) != "1.1.0" {
		t.Fatalf("second rotation: kept %d, archive %d first %q", len(kept.sections), len(arch.sections), sectionKey(arch.sections[0]))
	}
	if strings.Count(kept.header, "Older releases") != 1 {
		t.Fatalf("archive link repeated:\n%s", kept.header)
	}

	out, err = runChangelog(t, "rotate", "--file", log, "--keep", "10")
	if err != nil || !strings.Contains(out, "nothing to move") {
		t.Fatalf("short changelog: out=%s err=%v", out, err)
	}
}
