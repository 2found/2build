package cmd

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"

	"github.com/2found/2build/internal/ticket"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
)

// Contracts are the seams parallel work builds against: an API doc, a schema,
// a types file, an event list — files the repo already has. The index
// .babysit/contracts.yaml names them; nothing else about a contract is
// configured. Its version is the digest of its files' content, so editing the
// files IS publishing a new version, and the history is git's.
//
// A ticket pins the versions it builds against. When a pinned contract changes
// under it, its gates go stale until it re-reads the contract, re-verifies and
// pins again. A contract with an owner may only be changed by a phase of that
// Foreman; its checks are the consumers' tests, run before the provider lands.

const contractIndexPath = ".babysit/contracts.yaml"

type contractEntry struct {
	Owner  string   `yaml:"owner,omitempty" json:"owner,omitempty"`
	About  string   `yaml:"about,omitempty" json:"about,omitempty"`
	Files  []string `yaml:"files" json:"files"`
	Checks []string `yaml:"checks,omitempty" json:"checks,omitempty"`
}

type contractPin struct {
	Version string `json:"version"`
	Dir     string `json:"dir"`
	At      string `json:"at"`
}

type contractStatus struct {
	ID     string `json:"id"`
	Owner  string `json:"owner,omitempty"`
	Pinned string `json:"pinned"`
	Here   string `json:"here"`
	Base   string `json:"base"`
	State  string `json:"state"` // current | behind | changed | gone | unchecked
	Next   string `json:"next,omitempty"`
}

// readAt returns path's content in dir's working tree (ref "") or at ref.
func readAt(dir, ref, path string) ([]byte, bool) {
	if ref == "" {
		b, err := os.ReadFile(filepath.Join(dir, path))
		return b, err == nil
	}
	b, err := exec.Command("git", "-C", dir, "show", ref+":"+path).Output()
	return b, err == nil
}

// filesAt expands a contract's entries: a path ending in "/" is every file
// under that directory.
func filesAt(dir, ref string, entries []string) []string {
	var out []string
	for _, e := range entries {
		if !strings.HasSuffix(e, "/") {
			out = append(out, e)
			continue
		}
		if ref != "" {
			for _, f := range strings.Split(gitOutIn(dir, "ls-tree", "-r", "--name-only", ref, "--", e), "\n") {
				if f != "" {
					out = append(out, f)
				}
			}
			continue
		}
		_ = filepath.WalkDir(filepath.Join(dir, e), func(p string, d fs.DirEntry, err error) error {
			if err == nil && !d.IsDir() {
				if rel, err := filepath.Rel(dir, p); err == nil {
					out = append(out, filepath.ToSlash(rel))
				}
			}
			return nil
		})
	}
	slices.Sort(out)
	return slices.Compact(out)
}

func readContracts(dir, ref string) (map[string]contractEntry, error) {
	out := map[string]contractEntry{}
	b, ok := readAt(dir, ref, contractIndexPath)
	if !ok {
		return out, nil
	}
	if err := yaml.Unmarshal(b, &out); err != nil {
		return nil, fmt.Errorf("%s: %v", contractIndexPath, err)
	}
	for id, e := range out {
		if len(e.Files) == 0 {
			return nil, fmt.Errorf("%s: contract %s lists no files", contractIndexPath, id)
		}
		for _, f := range e.Files {
			if filepath.IsAbs(f) || strings.HasPrefix(filepath.Clean(f), "..") {
				return nil, fmt.Errorf("%s: contract %s: %s is outside the repo", contractIndexPath, id, f)
			}
		}
	}
	return out, nil
}

// contractDigest digests the content of a contract's files. A missing file is
// part of the version, so deleting one is a change too.
func contractDigest(dir, ref string, e contractEntry) string {
	h := sha256.New()
	for _, f := range filesAt(dir, ref, e.Files) {
		b, ok := readAt(dir, ref, f)
		fmt.Fprintf(h, "%s\x00%t\x00%d\x00", f, ok, len(b))
		h.Write(b)
	}
	return hex.EncodeToString(h.Sum(nil))[:16]
}

// contractsChanged lists the contracts whose version or entry differs between
// two commits, with the owner each had before.
func contractsChanged(dir, from, to string) (map[string]contractEntry, error) {
	before, err := readContracts(dir, from)
	if err != nil {
		return nil, err
	}
	after, err := readContracts(dir, to)
	if err != nil {
		return nil, err
	}
	changed := map[string]contractEntry{}
	for id, a := range after {
		b, existed := before[id]
		if !existed {
			changed[id] = a
			continue
		}
		if contractDigest(dir, from, b) != contractDigest(dir, to, a) || fmt.Sprint(b) != fmt.Sprint(a) {
			changed[id] = b
		}
	}
	for id, b := range before {
		if _, kept := after[id]; !kept {
			changed[id] = b
		}
	}
	return changed, nil
}

// contractOwnerConflicts names the contracts a change between two commits
// touches that belong to a Foreman other than me.
func contractOwnerConflicts(changed map[string]contractEntry, me string) []string {
	var out []string
	for id, e := range changed {
		if e.Owner != "" && e.Owner != me {
			out = append(out, id+" (owner "+e.Owner+")")
		}
	}
	slices.Sort(out)
	return out
}

func contractPinsPath(st *ticket.Store) string { return filepath.Join(st.Home(), "contracts.json") }

func readContractPins(st *ticket.Store) map[string]contractPin {
	pins := map[string]contractPin{}
	if b, err := os.ReadFile(contractPinsPath(st)); err == nil {
		_ = json.Unmarshal(b, &pins)
	}
	return pins
}

// contractStatuses compares each pin with the contract in the checkout it was
// pinned in (fallback: dir) and on local base.
func contractStatuses(st *ticket.Store, dir string) []contractStatus {
	pins := readContractPins(st)
	ids := make([]string, 0, len(pins))
	for id := range pins {
		ids = append(ids, id)
	}
	slices.Sort(ids)
	var out []contractStatus
	for _, id := range ids {
		pin := pins[id]
		here := dir
		if fi, err := os.Stat(pin.Dir); err == nil && fi.IsDir() {
			here = pin.Dir
		}
		top := gitOutIn(here, "rev-parse", "--show-toplevel")
		row := contractStatus{ID: id, Pinned: pin.Version, State: "current"}
		if top == "" {
			row.State = "unchecked"
			out = append(out, row)
			continue
		}
		local, _ := readContracts(top, "")
		e, ok := local[id]
		if !ok {
			row.State, row.Next = "gone", "the contract left this checkout; find what replaced it, adapt, then drop or re-pin"
			out = append(out, row)
			continue
		}
		row.Owner, row.Here = e.Owner, contractDigest(top, "", e)
		base := "refs/heads/" + baseBranchIn(top)
		if onBase, err := readContracts(top, base); err == nil {
			if be, ok := onBase[id]; ok {
				row.Base = contractDigest(top, base, be)
			}
		}
		switch {
		case row.Here != row.Pinned:
			row.State = "changed"
			row.Next = "re-read " + strings.Join(e.Files, ", ") + ", adapt and re-verify, then bbs contract pin " + id
		case row.Base != "" && row.Base != row.Here:
			row.State = "behind"
			row.Next = "base has a newer version; read it there — your phase integrates on it"
		}
		out = append(out, row)
	}
	return out
}

// contractReadinessReasons: building against a version that is no longer the
// one in the checkout makes the ticket's gates stale.
func contractReadinessReasons(st *ticket.Store, dir string) []string {
	var out []string
	for _, s := range contractStatuses(st, dir) {
		if s.State == "changed" || s.State == "gone" {
			out = append(out, "contract:"+s.ID+":"+s.State)
		}
	}
	return out
}

func newContractCmd() *cobra.Command {
	return &cobra.Command{
		Use:                "contract",
		Short:              "shared seams parallel work builds against (.babysit/contracts.yaml)",
		DisableFlagParsing: true,
		RunE: func(_ *cobra.Command, args []string) error {
			if err := runContract(args); err != nil {
				fmt.Fprintln(os.Stderr, err)
				os.Exit(1)
			}
			return nil
		},
	}
}

const contractUsage = `Usage:
  bbs contract list [--json]       contracts in this checkout: owner, version, files
  bbs contract pin <id>...         record the version this ticket builds against
  bbs contract status [--json]     this ticket's pins: current | behind | changed | gone
  bbs contract check [<id>...]     run the contracts' checks here; exit 1 on a failure

A contract is any set of repo files named in .babysit/contracts.yaml:

  api.search:
    owner: fm-catalog            # optional: only this Foreman's phases change it
    about: novel search API      # optional
    files: [docs/api/search.yaml, api/src/search/types.ts]   # "dir/" = whole dir
    checks: ["bun test api/test/search.contract.test.ts"]   # consumers' tests

Its version is the digest of those files: editing them publishes a new one.`

func runContract(args []string) error {
	verb := ""
	if len(args) > 0 {
		verb, args = args[0], args[1:]
	}
	jsonOut := slices.Contains(args, "--json")
	args = slices.DeleteFunc(slices.Clone(args), func(a string) bool { return a == "--json" })
	cwd, _ := os.Getwd()
	top := gitOutIn(cwd, "rev-parse", "--show-toplevel")
	switch verb {
	case "list":
		if top == "" {
			return fmt.Errorf("contract list: not in a git checkout")
		}
		all, err := readContracts(top, "")
		if err != nil {
			return err
		}
		type row struct {
			ID      string `json:"id"`
			Version string `json:"version"`
			contractEntry
		}
		var rows []row
		for id, e := range all {
			rows = append(rows, row{id, contractDigest(top, "", e), e})
		}
		slices.SortFunc(rows, func(a, b row) int { return strings.Compare(a.ID, b.ID) })
		if jsonOut {
			printV2Envelope(map[string]interface{}{"contracts": rows})
			return nil
		}
		if len(rows) == 0 {
			fmt.Println("no contracts (" + contractIndexPath + ")")
		}
		for _, r := range rows {
			fmt.Printf("%-24s %s  owner=%s  %s\n", r.ID, r.Version, orDefault(r.Owner, "-"), strings.Join(r.Files, " "))
		}
		return nil
	case "pin":
		env := resolveEnv()
		if env.Ticket == "" {
			return fmt.Errorf("contract pin: needs a ticket (BABYSIT_TICKET)")
		}
		if len(args) == 0 {
			return fmt.Errorf("contract pin: name at least one contract\n%s", contractUsage)
		}
		all, err := readContracts(top, "")
		if err != nil {
			return err
		}
		st := ticket.New(env)
		return withLock(st, func() error {
			pins := readContractPins(st)
			for _, id := range args {
				e, ok := all[id]
				if !ok {
					return fmt.Errorf("contract pin: no contract %s in %s", id, contractIndexPath)
				}
				pins[id] = contractPin{Version: contractDigest(top, "", e), Dir: top, At: isoNow()}
				fmt.Printf("PINNED=%s@%s\n", id, pins[id].Version)
			}
			return writeJSONAtomic(contractPinsPath(st), pins)
		})
	case "status":
		env := resolveEnv()
		if env.Ticket == "" {
			return fmt.Errorf("contract status: needs a ticket (BABYSIT_TICKET)")
		}
		rows := contractStatuses(ticket.New(env), cwd)
		if jsonOut {
			printV2Envelope(map[string]interface{}{"contracts": rows})
			return nil
		}
		if len(rows) == 0 {
			fmt.Println("no pinned contracts")
		}
		for _, r := range rows {
			fmt.Printf("%-24s %-8s pinned=%s here=%s base=%s\n", r.ID, r.State, r.Pinned, orDefault(r.Here, "-"), orDefault(r.Base, "-"))
			if r.Next != "" {
				fmt.Println("  next: " + r.Next)
			}
		}
		return nil
	case "check":
		if top == "" {
			return fmt.Errorf("contract check: not in a git checkout")
		}
		all, err := readContracts(top, "")
		if err != nil {
			return err
		}
		ids := args
		if len(ids) == 0 {
			for id := range all {
				ids = append(ids, id)
			}
			slices.Sort(ids)
		}
		failed := 0
		for _, id := range ids {
			e, ok := all[id]
			if !ok {
				return fmt.Errorf("contract check: no contract %s in %s", id, contractIndexPath)
			}
			if len(e.Checks) == 0 {
				fmt.Printf("NONE %s: no checks\n", id)
			}
			for _, check := range e.Checks {
				c := exec.Command("sh", "-c", check)
				c.Dir, c.Stdout, c.Stderr = top, os.Stderr, os.Stderr
				if err := c.Run(); err != nil {
					failed++
					fmt.Printf("FAIL %s: %s (%v)\n", id, check, err)
				} else {
					fmt.Printf("PASS %s: %s\n", id, check)
				}
			}
		}
		if failed > 0 {
			os.Exit(1)
		}
		return nil
	case "", "help", "--help", "-h":
		fmt.Println(contractUsage)
		return nil
	}
	return fmt.Errorf("contract: unknown verb %q\n%s", verb, contractUsage)
}
