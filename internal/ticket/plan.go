package ticket

import (
	"os"
	"path/filepath"
	"sort"
)

// PlanRow is one ticket as the planning views read it: the stored fields plus
// what is derived from the whole project — the milestone and phase it sits
// under, its effective blockers and whether it is ready. `bbs ticket find`, the
// dashboard and the agent pick-up queue all read this one projection.
type PlanRow struct {
	ID        string   `json:"id"`
	Title     string   `json:"title"`
	Kind      string   `json:"kind"`
	Status    string   `json:"status"`
	Product   string   `json:"product"`
	Role      string   `json:"role"`
	Person    string   `json:"person"`
	Assignee  string   `json:"assignee"` // the Team lead claim, unchanged
	Labels    []string `json:"labels"`
	Parent    string   `json:"parent"`
	Children  []string `json:"children"`
	Milestone string   `json:"milestone"` // nearest milestone ancestor (or self)
	Phase     string   `json:"phase"`     // nearest phase ancestor (or self)
	Start     string   `json:"start"`
	Due       string   `json:"due"`
	DoneAt    string   `json:"done_at"`
	Control   string   `json:"control"`
	// BlockedBy is every effective blocker not yet settled: the ticket's own,
	// those inherited from its ancestors, a container blocker expanded to its
	// open work. Empty means nothing upstream is in the way.
	BlockedBy []string `json:"blocked_by"`
	Ready     bool     `json:"ready"`
	CheckDue  string   `json:"check_due"`
	Overdue   bool     `json:"check_overdue"`
	Loop      any      `json:"loop"`
	UpdatedAt string   `json:"updated_at"`
}

// openStatuses can be picked up: nothing has started, nothing is settled.
var openStatuses = map[string]bool{"triage": true, "backlog": true, "planned": true}

// LoadPlan reads every record under projectHome/tickets and derives the plan
// rows. A directory without index.json is not a record; one that fails to parse
// is skipped the same way (the store survey lists them separately), so one bad
// file cannot blank the board.
func LoadPlan(projectHome, today string) []PlanRow {
	dir := filepath.Join(projectHome, "tickets")
	ents, _ := os.ReadDir(dir)
	docs := map[string]Doc{}
	for _, e := range ents {
		if !e.IsDir() {
			continue
		}
		d, err := ReadDocStrict(filepath.Join(dir, e.Name(), "index.json"))
		if err == nil {
			docs[e.Name()] = d
		}
	}
	return DerivePlan(dir, docs, today)
}

// DerivePlan is LoadPlan over records the caller already read (the dashboard
// composer reads every index.json once and derives from that).
func DerivePlan(dir string, docs map[string]Doc, today string) []PlanRow {
	// Parent: the record's own field, else whoever lists it as a child — real
	// stores carry both half-links (see dag.go).
	parent := map[string]string{}
	for id, d := range docs {
		for _, c := range d.Children() {
			if _, ok := docs[c]; ok && parent[c] == "" && c != id {
				parent[c] = id
			}
		}
	}
	for id, d := range docs {
		if p := d.Get("parent"); p != "" && p != id {
			parent[id] = p
		}
	}
	kids := map[string][]string{}
	for c, p := range parent {
		kids[p] = append(kids[p], c)
	}
	ancestors := func(id string) []string {
		var out []string
		seen := map[string]bool{id: true}
		for p := parent[id]; p != "" && !seen[p]; p = parent[p] {
			seen[p] = true
			out = append(out, p)
		}
		return out
	}
	settled := func(id string) bool {
		d, ok := docs[id]
		if !ok {
			return true // a dangling blocker blocks nothing it can be checked against
		}
		return StatusSettled(d.Get("status")) || d.Get("control.state") == ControlCancelled
	}
	// open leaves of a blocker: a container stands for its unfinished work.
	var openUnder func(id string, seen map[string]bool, out map[string]bool)
	openUnder = func(id string, seen map[string]bool, out map[string]bool) {
		if seen[id] {
			return
		}
		seen[id] = true
		// A container stands for its leaves even once marked settled, as in
		// dag.go: a phase closed by hand over open work still holds dependents.
		if len(kids[id]) == 0 {
			if !settled(id) {
				out[id] = true
			}
			return
		}
		for _, k := range kids[id] {
			openUnder(k, seen, out)
		}
	}

	// Blockers from both half-links, as in dag.go: a's blocked_by, and every
	// x whose blocks lists a (a store not yet repaired carries only one side).
	blockers := map[string][]string{}
	for id, d := range docs {
		blockers[id] = append(blockers[id], strList(d.Value("relations.blocked_by"))...)
		for _, down := range strList(d.Value("relations.blocks")) {
			blockers[down] = append(blockers[down], id)
		}
	}

	rows := make([]PlanRow, 0, len(docs))
	for id, d := range docs {
		r := PlanRow{
			ID: id, Title: Title(filepath.Join(dir, id), d), Kind: KindOf(d), Status: d.Get("status"),
			Product: d.Get("product"), Role: d.Get("assigned_to.role"), Person: d.Get("assigned_to.person"),
			Assignee: d.Get("assignee"), Labels: nonNilStrs(strList(d.Value("labels"))), Parent: parent[id],
			Children: nonNilStrs(d.Children()), Start: d.Get("window.start"), Due: d.Get("window.due"),
			DoneAt: d.Get("done_at"), Control: d.Get("control.state"), Loop: d.Value("loop"),
			UpdatedAt: d.Get("updated_at"), BlockedBy: []string{},
		}
		chain := append([]string{id}, ancestors(id)...)
		for _, a := range chain {
			switch KindOf(docs[a]) {
			case KindMilestone:
				if r.Milestone == "" {
					r.Milestone = a
				}
			case KindPhase:
				if r.Phase == "" {
					r.Phase = a
				}
			}
			if r.Product == "" {
				r.Product = docs[a].Get("product") // work inherits its milestone's product
			}
		}
		open := map[string]bool{}
		inChain := map[string]bool{}
		for _, a := range chain {
			inChain[a] = true
		}
		for _, a := range chain {
			for _, b := range blockers[a] {
				if inChain[b] {
					continue // itself or its own ancestor: finishes with it, orders nothing
				}
				openUnder(b, map[string]bool{}, open)
			}
		}
		delete(open, id)
		for b := range open {
			r.BlockedBy = append(r.BlockedBy, b)
		}
		sort.Strings(r.BlockedBy)
		r.Ready = r.Kind == KindWork && openStatuses[r.Status] && r.Control == "" && len(r.BlockedBy) == 0
		r.CheckDue, _ = CheckDue(d)
		r.Overdue = CheckOverdue(d, today)
		rows = append(rows, r)
	}
	sort.Slice(rows, func(i, j int) bool { return rows[i].ID < rows[j].ID })
	return rows
}

func nonNilStrs(s []string) []string {
	if s == nil {
		return []string{}
	}
	return s
}

// PlanFilter selects rows; an empty field matches everything.
type PlanFilter struct {
	Product, Kind, Milestone, Phase, Role, Person, Status, Label string
	Ready, CheckDue                                              bool
}

func (f PlanFilter) Match(r PlanRow) bool {
	eq := func(want, got string) bool { return want == "" || want == got }
	if !eq(f.Product, r.Product) || !eq(f.Kind, r.Kind) || !eq(f.Milestone, r.Milestone) ||
		!eq(f.Phase, r.Phase) || !eq(f.Role, r.Role) || !eq(f.Person, r.Person) || !eq(f.Status, r.Status) {
		return false
	}
	if f.Label != "" && !oneOf(f.Label, r.Labels) {
		return false
	}
	if f.Ready && !r.Ready {
		return false
	}
	if f.CheckDue && !r.Overdue {
		return false
	}
	return true
}
