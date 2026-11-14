package cmd

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
)

// The changelog verbs let a release agent write CHANGELOG.md without reading
// it: the file only grows, and an agent that must Read it before an Edit pays
// for every past release on every new one. `add` prepends a notes file,
// `check` reports size, `rotate` moves old sections to an archive.

const (
	changelogDefaultMaxBytes = 40000
	changelogDefaultKeep     = 10
	changelogArchiveName     = "CHANGELOG-ARCHIVE.md"
)

// changelogDoc is a changelog split at its `## ` headings: everything before
// the first heading is the header, each section runs to the next heading.
// Headings inside fenced code blocks do not split.
type changelogDoc struct {
	header   string
	sections []string
}

func parseChangelog(text string) changelogDoc {
	var doc changelogDoc
	var cur strings.Builder
	inSection, fence := false, false
	flush := func() {
		if inSection {
			doc.sections = append(doc.sections, strings.TrimRight(cur.String(), "\n"))
		} else {
			doc.header = cur.String()
		}
		cur.Reset()
	}
	for _, line := range strings.SplitAfter(text, "\n") {
		if strings.HasPrefix(strings.TrimSpace(line), "```") {
			fence = !fence
		}
		if !fence && strings.HasPrefix(line, "## ") {
			flush()
			inSection = true
		}
		cur.WriteString(line)
	}
	flush()
	return doc
}

func (d changelogDoc) render() string {
	var b strings.Builder
	header := strings.TrimRight(d.header, "\n")
	if header != "" {
		b.WriteString(header + "\n\n")
	}
	b.WriteString(strings.Join(d.sections, "\n\n"))
	if len(d.sections) > 0 {
		b.WriteString("\n")
	}
	return b.String()
}

// sectionKey is the first word of a section heading — the version, so a
// re-run on another day still recognises the section it already added.
func sectionKey(section string) string {
	first, _, _ := strings.Cut(section, "\n")
	fields := strings.Fields(strings.TrimPrefix(first, "## "))
	if len(fields) == 0 {
		return ""
	}
	return fields[0]
}

func writeFileAtomic(path, content string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(filepath.Dir(path), ".changelog-*")
	if err != nil {
		return err
	}
	defer os.Remove(tmp.Name())
	if _, err := tmp.WriteString(content); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmp.Name(), path)
}

func readChangelog(path string) (changelogDoc, error) {
	b, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return changelogDoc{header: "# Changelog\n"}, nil
	}
	if err != nil {
		return changelogDoc{}, err
	}
	return parseChangelog(string(b)), nil
}

// changelogReport prints size and, past maxBytes, the rotate suggestion the
// release skill turns into one question for the owner.
func changelogReport(w io.Writer, path string, maxBytes, keep int) error {
	b, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	doc := parseChangelog(string(b))
	fmt.Fprintf(w, "changelog: %s sections=%d bytes=%d\n", path, len(doc.sections), len(b))
	if len(b) > maxBytes && len(doc.sections) > keep {
		fmt.Fprintf(w, "rotate: suggested (%d bytes > %d); keep the newest %d with: bbs changelog rotate --file %s --keep %d\n",
			len(b), maxBytes, keep, path, keep)
	} else {
		fmt.Fprintln(w, "rotate: not needed")
	}
	return nil
}

func changelogAdd(path, notes string) error {
	notes = strings.Trim(notes, "\n")
	if !strings.HasPrefix(notes, "## ") {
		return fmt.Errorf("notes must start with a \"## <version>\" heading")
	}
	added := parseChangelog(notes + "\n")
	doc, err := readChangelog(path)
	if err != nil {
		return err
	}
	have := map[string]bool{}
	for _, s := range doc.sections {
		have[sectionKey(s)] = true
	}
	for _, s := range added.sections {
		if have[sectionKey(s)] {
			return fmt.Errorf("%s already has a %q section; edit that section instead", path, sectionKey(s))
		}
	}
	doc.sections = append(added.sections, doc.sections...)
	return writeFileAtomic(path, doc.render())
}

// changelogRotate keeps the newest `keep` sections and prepends the rest to
// the archive (newest first, like the changelog). The archive is written
// before the changelog, so an interruption duplicates sections, never loses them.
func changelogRotate(w io.Writer, path, archive string, keep int) error {
	if keep < 1 {
		return fmt.Errorf("--keep must be at least 1")
	}
	doc, err := readChangelog(path)
	if err != nil {
		return err
	}
	if len(doc.sections) <= keep {
		fmt.Fprintf(w, "rotate: nothing to move (%d sections <= %d)\n", len(doc.sections), keep)
		return nil
	}
	moved := doc.sections[keep:]
	doc.sections = doc.sections[:keep]

	arch := changelogDoc{header: fmt.Sprintf("# Changelog archive\n\nOlder releases moved out of [%[1]s](%[1]s) by `bbs changelog rotate`.\n", relLink(archive, path))}
	if b, err := os.ReadFile(archive); err == nil {
		arch = parseChangelog(string(b))
	} else if !os.IsNotExist(err) {
		return err
	}
	arch.sections = append(append([]string{}, moved...), arch.sections...)
	if err := writeFileAtomic(archive, arch.render()); err != nil {
		return err
	}

	link := relLink(path, archive)
	if !strings.Contains(doc.header, "("+link+")") {
		doc.header = strings.TrimRight(doc.header, "\n") + fmt.Sprintf("\n\nOlder releases: [%[1]s](%[1]s).\n", link)
	}
	if err := writeFileAtomic(path, doc.render()); err != nil {
		return err
	}
	fmt.Fprintf(w, "rotate: moved %d sections to %s; %s keeps %d\n", len(moved), archive, path, keep)
	return nil
}

// relLink is target's path relative to the directory holding from.
func relLink(from, target string) string {
	rel, err := filepath.Rel(filepath.Dir(from), target)
	if err != nil {
		return filepath.ToSlash(target)
	}
	return filepath.ToSlash(rel)
}

func newChangelogCmd() *cobra.Command {
	root := &cobra.Command{
		Use:   "changelog",
		Short: "Prepend, size-check and rotate CHANGELOG.md without an agent reading it",
	}
	var file string
	var maxBytes, keep int
	addFileFlag := func(c *cobra.Command) {
		c.Flags().StringVar(&file, "file", "CHANGELOG.md", "changelog path")
	}
	addCheckFlags := func(c *cobra.Command) {
		c.Flags().IntVar(&maxBytes, "max-bytes", changelogDefaultMaxBytes, "size above which rotation is suggested")
		c.Flags().IntVar(&keep, "keep", changelogDefaultKeep, "sections a suggested rotation keeps")
	}

	var notes string
	add := &cobra.Command{
		Use:   "add --notes <file|->",
		Short: "Prepend a release section (notes start with \"## <version>\"), then report size",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			var b []byte
			var err error
			if notes == "-" {
				b, err = io.ReadAll(cmd.InOrStdin())
			} else if notes != "" {
				b, err = os.ReadFile(notes)
			} else {
				return fmt.Errorf("--notes is required")
			}
			if err != nil {
				return err
			}
			if err := changelogAdd(file, string(b)); err != nil {
				return err
			}
			return changelogReport(cmd.OutOrStdout(), file, maxBytes, keep)
		},
	}
	add.Flags().StringVar(&notes, "notes", "", "notes file, or - for stdin")
	addFileFlag(add)
	addCheckFlags(add)

	check := &cobra.Command{
		Use:   "check",
		Short: "Report size and whether rotation is suggested (read-only)",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return changelogReport(cmd.OutOrStdout(), file, maxBytes, keep)
		},
	}
	addFileFlag(check)
	addCheckFlags(check)

	var archive string
	rotate := &cobra.Command{
		Use:   "rotate [--keep N] [--archive path]",
		Short: "Keep the newest N sections; move the rest to the top of the archive",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			if archive == "" {
				archive = filepath.Join(filepath.Dir(file), changelogArchiveName)
			}
			return changelogRotate(cmd.OutOrStdout(), file, archive, keep)
		},
	}
	addFileFlag(rotate)
	rotate.Flags().IntVar(&keep, "keep", changelogDefaultKeep, "sections to keep")
	rotate.Flags().StringVar(&archive, "archive", "", "archive path (default "+changelogArchiveName+" beside the changelog)")

	root.AddCommand(add, check, rotate)
	return root
}
