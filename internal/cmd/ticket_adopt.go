package cmd

import (
	"fmt"
	"os"

	"github.com/2found/2build/internal/identity"
)

// runAdopt merges other tickets' snapshots into the current worktree as
// uncommitted changes — how a ticket session repairs a phase that does not
// compose: adopt the ticket it collided with, resolve, reseal.
func runAdopt(args []string) {
	if len(args) == 0 {
		fmt.Fprintln(os.Stderr, "usage: bbs ticket adopt <ticket>... (run in the adopting ticket's worktree)")
		os.Exit(2)
	}
	dir := gitOutIn("", "rev-parse", "--show-toplevel")
	if dir == "" {
		fmt.Fprintln(os.Stderr, "adopt: not inside a git worktree")
		os.Exit(2)
	}
	var heads []string
	for _, t := range args {
		if !idRe.MatchString(t) {
			fmt.Fprintf(os.Stderr, "adopt: invalid ticket id %q\n", t)
			os.Exit(2)
		}
		h := gitOutIn(dir, "rev-parse", "--verify", "-q", ticketSnapshotRef(t)+"^{commit}")
		if h == "" {
			fmt.Fprintf(os.Stderr, "adopt: ticket %s has no snapshot (it is sealed, or awaited, before it can be adopted)\n", t)
			os.Exit(1)
		}
		heads = append(heads, h)
	}
	conflicts, err := adoptSnapshots(dir, heads)
	if err != nil {
		fmt.Fprintln(os.Stderr, "adopt:", err)
		os.Exit(1)
	}
	if env := identity.Resolve(); env.Ticket != "" {
		if err := recordAdopted(storeForTicket(env, env.Ticket).Home(), heads); err != nil {
			fmt.Fprintln(os.Stderr, "adopt:", err)
			os.Exit(1)
		}
	}
	for i, t := range args {
		fmt.Printf("ADOPTED=%s SNAPSHOT=%s\n", t, heads[i])
	}
	for _, f := range conflicts {
		fmt.Println("CONFLICT=" + f)
	}
	if len(conflicts) > 0 {
		fmt.Println("NEXT=resolve the conflict markers in each CONFLICT file, rerun the affected checks; never commit")
		os.Exit(5)
	}
}
