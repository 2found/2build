package cmd

// runTicketLint ports the retired bbs-ticket-lint script as `bbs ticket lint`: it scans
// markdown for inline `$TH/`, `$TICKET_HOME/`, and
// `$BABYSIT_PROJECT_HOME/tickets/` constructions inside ```bash fenced
// blocks, which should go through `bbs ticket path <kind>` instead.
//
// Modes:
//   --mode discovery  print every hit (file:line\tmatch); always exits 0.
//   --mode enforce    print hits not in the manifest and not marked
//                     `# lint:allow-direct-path`; exits 1 if any remain.

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

var ticketLintBashFence = regexp.MustCompile(`^[[:space:]]*` + "```" + `[[:space:]]*bash[[:space:]]*$`)
var ticketLintFenceClose = regexp.MustCompile(`^[[:space:]]*` + "```" + `[[:space:]]*$`)
var ticketLintHit = regexp.MustCompile(`(\$TH/|\$TICKET_HOME/|\$BABYSIT_PROJECT_HOME/tickets/)`)

const ticketLintMarker = "# lint:allow-direct-path"

func runTicketLint(args []string) {
	mode, scanPath, manifest := "", ".claude", ".babysit/path-broker-manifest.txt"
	verbose := false
	for i := 0; i < len(args); {
		arg := func(k int) string {
			if i+k < len(args) {
				return args[i+k]
			}
			return ""
		}
		switch args[i] {
		case "--mode":
			mode = arg(1)
			i += 2
		case "--path":
			scanPath = arg(1)
			i += 2
		case "--manifest":
			manifest = arg(1)
			i += 2
		case "-v", "--verbose":
			verbose = true
			i++
		case "-h", "--help":
			fmt.Fprintf(os.Stderr, `usage: bbs ticket lint --mode discovery|enforce [--path DIR] [--manifest FILE]

  --mode discovery    list every direct-path hit (seed the manifest)
  --mode enforce      fail if any unapproved direct-path hits remain

Flags a line when, inside a `+"```bash"+` fenced block:
  $TH/<anything>                           (path traversal via local var)
  $TICKET_HOME/<anything>
  $BABYSIT_PROJECT_HOME/tickets/<anything>

Exceptions:
  - Line ends with '# lint:allow-direct-path'
  - Entry 'file:line' appears in the manifest (default: %s)
`, manifest)
			os.Exit(0)
		default:
			fmt.Fprintf(os.Stderr, "bbs ticket lint: unknown arg '%s'\n", args[i])
			os.Exit(2)
		}
	}
	if mode != "discovery" && mode != "enforce" {
		fmt.Fprintln(os.Stderr, "bbs ticket lint: --mode required (discovery|enforce)")
		os.Exit(2)
	}
	if _, err := os.Stat(scanPath); err != nil {
		fmt.Fprintf(os.Stderr, "bbs ticket lint: scan path '%s' does not exist\n", scanPath)
		os.Exit(2)
	}

	// Manifest entries (file:line) approving hits in enforce mode.
	approved := map[string]bool{}
	if mode == "enforce" {
		for _, raw := range strings.Split(readRegular(manifest), "\n") {
			line := strings.TrimSpace(strings.SplitN(raw, "#", 2)[0])
			if line != "" {
				approved[line] = true
			}
		}
	}

	files := ticketLintFiles(scanPath)
	type hit struct {
		file   string
		lineno int
		line   string
	}
	var hits []hit
	total := 0
	for _, f := range files {
		for _, bl := range bashBlockLines(f) {
			if !ticketLintHit.MatchString(bl.text) {
				continue
			}
			if strings.Contains(bl.text, ticketLintMarker) {
				continue
			}
			total++
			entry := f + ":" + strconv.Itoa(bl.no)
			if mode == "enforce" && approved[entry] {
				if verbose {
					fmt.Printf("  ok (approved)  %s\n", entry)
				}
				continue
			}
			hits = append(hits, hit{f, bl.no, bl.text})
		}
	}
	for _, h := range hits {
		fmt.Printf("%s:%d\t%s\n", h.file, h.lineno, h.line)
	}

	if mode == "enforce" {
		if len(hits) > 0 {
			fmt.Fprintln(os.Stderr)
			fmt.Fprintf(os.Stderr, "bbs ticket lint: %d unapproved direct-path hit(s) (total=%d, manifest=%d)\n", len(hits), total, len(approved))
			fmt.Fprintln(os.Stderr, "Fix options:")
			fmt.Fprintln(os.Stderr, "  1. Replace with `bbs ticket path <kind>`")
			fmt.Fprintln(os.Stderr, "  2. Add `# lint:allow-direct-path` inline comment")
			fmt.Fprintf(os.Stderr, "  3. Add `file:line` to %s (with a justification comment)\n", manifest)
			os.Exit(1)
		}
		fmt.Printf("bbs ticket lint: OK — 0 unapproved hits (total=%d, approved=%d)\n", total, len(approved))
		return
	}
	fmt.Fprintf(os.Stderr, "\nbbs ticket lint: discovery found %d hit(s)\n", total)
}

// ticketLintFiles collects the .md files to scan: the dir walked recursively,
// or the file itself.
func ticketLintFiles(scanPath string) []string {
	var files []string
	if !isDir(scanPath) {
		return []string{scanPath}
	}
	_ = filepath.WalkDir(scanPath, func(p string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() || !strings.HasSuffix(p, ".md") {
			return nil
		}
		files = append(files, p)
		return nil
	})
	sort.Strings(files)
	return files
}

// bashBlockLine is one line inside a ```bash fenced block.
type bashBlockLine struct {
	no   int
	text string
}

// bashBlockLines returns every line inside a ```bash fenced block — the awk
// extraction from the bash original.
func bashBlockLines(path string) []bashBlockLine {
	var out []bashBlockLine
	inBlock := false
	for i, ln := range strings.Split(readRegular(path), "\n") {
		if ticketLintBashFence.MatchString(ln) {
			inBlock = true
			continue
		}
		if ticketLintFenceClose.MatchString(ln) {
			inBlock = false
			continue
		}
		if inBlock {
			out = append(out, bashBlockLine{i + 1, ln})
		}
	}
	return out
}
