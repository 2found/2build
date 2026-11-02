package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/2found/2build/internal/identity"
	"github.com/2found/2build/internal/ticket"
)

// findRow is a plan row with its project, since --all-projects mixes them.
type findRow struct {
	Project string `json:"project"`
	ticket.PlanRow
}

// runFind is `bbs ticket find` — the queryable view of the planning fields and
// the agent pick-up queue (`find --role swe --ready --json`).
func runFind(args []string) {
	var f ticket.PlanFilter
	all, asJSON, project := false, false, ""
	for i := 0; i < len(args); i++ {
		a := args[i]
		str := map[string]*string{"--product": &f.Product, "--kind": &f.Kind, "--milestone": &f.Milestone,
			"--phase": &f.Phase, "--role": &f.Role, "--person": &f.Person, "--status": &f.Status,
			"--label": &f.Label, "--project": &project}
		switch {
		case str[a] != nil:
			*str[a], i = valueOf(args, i, a), i+1
		case a == "--ready":
			f.Ready = true
		case a == "--check-due":
			f.CheckDue = true
		case a == "--all-projects":
			all = true
		case a == "--json":
			asJSON = true
		default:
			fmt.Fprintf(os.Stderr, "find: unknown argument %s\n", a)
			fmt.Fprintln(os.Stderr, "usage: find [--product P] [--kind K] [--milestone ID] [--phase ID] [--role R] [--person P] [--status S] [--label L] [--ready] [--check-due] [--project SLUG | --all-projects] [--json]")
			os.Exit(2)
		}
	}
	if f.Role != "" {
		if err := ticket.ValidRole(f.Role); err != nil {
			fmt.Fprintln(os.Stderr, "find:", err)
			os.Exit(2)
		}
	}
	var homes []string
	switch {
	case all:
		ents, _ := os.ReadDir(filepath.Join(identity.BabysitHome(), "projects"))
		for _, e := range ents {
			if e.IsDir() {
				homes = append(homes, filepath.Join(identity.BabysitHome(), "projects", e.Name()))
			}
		}
	case project != "":
		if !plainSlug(project) {
			fmt.Fprintf(os.Stderr, "find: invalid project %q\n", project)
			os.Exit(2)
		}
		homes = []string{filepath.Join(identity.BabysitHome(), "projects", project)}
	default:
		homes = []string{resolveProject().ProjectHome}
	}
	rows := []findRow{}
	today := ticket.Today()
	for _, h := range homes {
		for _, r := range ticket.LoadPlan(h, today) {
			if f.Match(r) {
				rows = append(rows, findRow{filepath.Base(h), r})
			}
		}
	}
	if asJSON {
		b, _ := json.MarshalIndent(rows, "", "  ")
		fmt.Println(string(b))
		os.Exit(0)
	}
	for _, r := range rows {
		who := r.Role
		if r.Person != "" {
			who += ":" + r.Person
		}
		extra := ""
		if len(r.BlockedBy) > 0 {
			extra = "  waits on " + strings.Join(r.BlockedBy, ",")
		} else if r.Ready {
			extra = "  ready"
		}
		if r.Overdue {
			extra += "  check due " + r.CheckDue
		}
		pfx := ""
		if all {
			pfx = r.Project + "  "
		}
		fmt.Printf("%s%-12s %-9s %-11s %-10s %-14s %s%s\n", pfx, r.ID, r.Kind, r.Status, r.Product, dashIfEmpty(who), r.Title, extra)
	}
	os.Exit(0)
}

func plainSlug(s string) bool {
	return s != "" && s != "." && s != ".." && !strings.ContainsAny(s, `/\`)
}

func dashIfEmpty(s string) string {
	if s == "" {
		return "-"
	}
	return s
}
