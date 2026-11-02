package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"

	"github.com/2found/2build/internal/ticket"
)

// `bbs ticket import <plan.yaml|json>` (bs-w5p3c6mq) creates or updates a set
// of tickets from one plan file. Each entry is keyed by {plan, key} — or bound
// to an existing ticket with `id:` — so re-importing the same file changes
// nothing, and an edited file changes only what it edits. Import never deletes
// a ticket, a relation or a label, and never rewrites requirement.md: the
// requirement is seeded once, on create.
//
//	plan: lohi-h1            # required: the key namespace
//	product: lohi            # default for entries
//	tickets:
//	  - key: m
//	    kind: milestone
//	    title: LoHi revenue 45 → 70m ₫/month
//	    window: {start: 2026-11-01, due: 2027-01-31}
//	    loop: {stage: product, target_outcome: wallet revenue, expected_delta: +25m, check_after_days: 90}
//	  - key: p1
//	    kind: phase
//	    parent: m              # a key in this plan, or an existing ticket id
//	  - key: attribution
//	    parent: p1
//	    role: swe
//	    labels: [analytics]
//	    blocked_by: [bs-v938tpde]
//	    requirement: |
//	      # First-touch attribution
//	      ...

type importLoop struct {
	Stage          string   `yaml:"stage"`
	TargetOutcome  string   `yaml:"target_outcome"`
	ExpectedDelta  string   `yaml:"expected_delta"`
	CheckAfterDays *int     `yaml:"check_after_days"`
	CheckFrom      string   `yaml:"check_from"`
	EvidenceRefs   []string `yaml:"evidence_refs"`
}

type importEntry struct {
	Key         string                       `yaml:"key"`
	ID          string                       `yaml:"id"`
	Title       string                       `yaml:"title"`
	Kind        string                       `yaml:"kind"`
	Product     string                       `yaml:"product"`
	Parent      string                       `yaml:"parent"`
	Role        string                       `yaml:"role"`
	Person      string                       `yaml:"person"`
	Status      string                       `yaml:"status"`
	Labels      []string                     `yaml:"labels"`
	BlockedBy   []string                     `yaml:"blocked_by"`
	Window      *struct{ Start, Due string } `yaml:"window"`
	Loop        *importLoop                  `yaml:"loop"`
	Requirement string                       `yaml:"requirement"`
}

type importPlan struct {
	Plan    string        `yaml:"plan"`
	Product string        `yaml:"product"`
	Tickets []importEntry `yaml:"tickets"`
}

type importResult struct {
	Key    string   `json:"key"`
	ID     string   `json:"id"`
	Action string   `json:"action"` // create | update | unchanged
	Fields []string `json:"fields"`
}

var importStatuses = map[string]bool{"triage": true, "backlog": true, "planned": true}

func loadImportPlan(path string) (importPlan, error) {
	var p importPlan
	raw, err := os.ReadFile(path)
	if err != nil {
		return p, err
	}
	dec := yaml.NewDecoder(strings.NewReader(string(raw)))
	dec.KnownFields(true) // a misspelled field must not silently import as nothing
	if err := dec.Decode(&p); err != nil {
		return p, fmt.Errorf("%s: %w", path, err)
	}
	return p, nil
}

// entryWrites validates one entry into the field writes it states. Fields the
// entry leaves out are not written; labels and evidence merge into what is
// there.
func entryWrites(e importEntry, plan, defProduct string, cur ticket.Doc) ([]fieldWrite, error) {
	var w []fieldWrite
	add := func(fw fieldWrite, err error) error {
		if err == nil {
			w = append(w, fw)
		}
		return err
	}
	if e.Title != "" {
		if err := add(titleWrite(e.Title)); err != nil {
			return nil, err
		}
	}
	if e.Kind != "" {
		if err := add(kindWrite(e.Kind)); err != nil {
			return nil, err
		}
	}
	prod := e.Product
	if prod == "" {
		prod = defProduct
	}
	if prod != "" {
		if err := add(productWrite(prod)); err != nil {
			return nil, err
		}
	}
	if e.Role != "" || e.Person != "" {
		if err := add(assignedToWrite(e.Role, e.Person)); err != nil {
			return nil, err
		}
	}
	if len(e.Labels) > 0 {
		l, err := labelsWith(cur, e.Labels, nil)
		if err != nil {
			return nil, err
		}
		w = append(w, fieldWrite{"labels", l})
	}
	if e.Window != nil {
		if err := add(windowWrite(e.Window.Start, e.Window.Due)); err != nil {
			return nil, err
		}
	}
	if l := e.Loop; l != nil {
		s := loopSpec{CheckAfter: l.CheckAfterDays, Evidence: l.EvidenceRefs}
		for _, f := range []struct {
			v   string
			dst **string
		}{{l.Stage, &s.Stage}, {l.TargetOutcome, &s.Target}, {l.ExpectedDelta, &s.Expected}, {l.CheckFrom, &s.CheckFrom}} {
			if f.v != "" {
				v := f.v
				*f.dst = &v
			}
		}
		lw, err := loopWrites(cur, s)
		if err != nil {
			return nil, err
		}
		w = append(w, lw...)
	}
	w = append(w, fieldWrite{"import", map[string]interface{}{"plan": plan, "key": e.Key}})
	return w, nil
}

// importProject resolves, validates and (unless dry) applies a plan. All
// validation runs before the first write, so a bad entry leaves the store as
// it was.
func importProject(projectHome string, p importPlan, dry bool, actor string) ([]importResult, error) {
	if p.Plan == "" || !plainSlug(p.Plan) || strings.ContainsAny(p.Plan, " \t\n") {
		return nil, fmt.Errorf("plan: a key namespace is required (e.g. plan: lohi-h1)")
	}
	if len(p.Tickets) == 0 {
		return nil, fmt.Errorf("tickets: nothing to import")
	}
	docs, err := loadProjectDocs(projectHome)
	if err != nil {
		return nil, err
	}
	// key -> id: explicit id, else the ticket an earlier import made, else new.
	byKey := map[string]string{}
	for id, d := range docs {
		if d.Get("import.plan") == p.Plan && d.Get("import.key") != "" {
			byKey[d.Get("import.key")] = id
		}
	}
	ids := map[string]string{}
	created := map[string]bool{}
	for i, e := range p.Tickets {
		if e.Key == "" || strings.ContainsAny(e.Key, " \t\n") {
			return nil, fmt.Errorf("tickets[%d]: key is required (one word)", i)
		}
		if _, dup := ids[e.Key]; dup {
			return nil, fmt.Errorf("tickets[%d]: duplicate key %q", i, e.Key)
		}
		switch {
		case e.ID != "":
			if _, ok := docs[e.ID]; !ok {
				return nil, fmt.Errorf("%s: id %s has no record in this project", e.Key, e.ID)
			}
			if prev, ok := byKey[e.Key]; ok && prev != e.ID {
				return nil, fmt.Errorf("%s: id %s, but an earlier import bound this key to %s", e.Key, e.ID, prev)
			}
			ids[e.Key] = e.ID
		case byKey[e.Key] != "":
			ids[e.Key] = byKey[e.Key]
		default:
			id := newTicketID()
			for docs[id] != nil {
				id = newTicketID()
			}
			ids[e.Key], created[e.Key] = id, true
		}
		if e.Status != "" && !importStatuses[e.Status] {
			return nil, fmt.Errorf("%s: status %q — import only seeds triage|backlog|planned; lifecycle moves through set-status", e.Key, e.Status)
		}
	}
	ref := func(key, r string) (string, error) {
		if id, ok := ids[r]; ok {
			return id, nil
		}
		if _, ok := docs[r]; ok {
			return r, nil
		}
		return "", fmt.Errorf("%s: %q is neither a key in this plan nor a ticket in this project", key, r)
	}
	type planned struct {
		e       importEntry
		id      string
		writes  []fieldWrite
		parent  string
		blocked []string
	}
	var work []planned
	for _, e := range p.Tickets {
		id := ids[e.Key]
		cur := docs[id]
		if cur == nil {
			cur = ticket.Doc{}
		}
		w, err := entryWrites(e, p.Plan, p.Product, cur)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", e.Key, err)
		}
		pl := planned{e: e, id: id, writes: w}
		if e.Parent != "" {
			if pl.parent, err = ref(e.Key, e.Parent); err != nil {
				return nil, err
			}
		}
		for _, b := range e.BlockedBy {
			bid, err := ref(e.Key, b)
			if err != nil {
				return nil, err
			}
			pl.blocked = append(pl.blocked, bid)
		}
		work = append(work, pl)
	}
	// Loops in the plan's own parent links are refused before writing.
	parentOf := map[string]string{}
	for id, d := range docs {
		parentOf[id] = d.Get("parent")
	}
	for _, pl := range work {
		if pl.parent != "" {
			parentOf[pl.id] = pl.parent
		}
	}
	for _, pl := range work {
		seen := map[string]bool{}
		for cur := pl.id; cur != ""; cur = parentOf[cur] {
			if seen[cur] {
				return nil, fmt.Errorf("%s: parent links form a loop", pl.e.Key)
			}
			seen[cur] = true
		}
	}
	// So are blocker cycles, through the plan's links and the store's.
	blockedBy := map[string][]string{}
	for id, d := range docs {
		blockedBy[id] = strList(d.Value("relations.blocked_by"))
	}
	for _, pl := range work {
		blockedBy[pl.id] = append(blockedBy[pl.id], pl.blocked...)
	}
	state := map[string]int{} // 1 on the DFS stack, 2 done
	var visit func(id string) bool
	visit = func(id string) bool {
		switch state[id] {
		case 1:
			return true
		case 2:
			return false
		}
		state[id] = 1
		for _, b := range blockedBy[id] {
			if visit(b) {
				return true
			}
		}
		state[id] = 2
		return false
	}
	for _, pl := range work {
		if visit(pl.id) {
			return nil, fmt.Errorf("%s: blocked_by links form a cycle", pl.e.Key)
		}
	}

	base := ticket.New(identityFor(projectHome, ""))
	var out []importResult
	for _, pl := range work {
		r := importResult{Key: pl.e.Key, ID: pl.id, Action: "update", Fields: []string{}}
		cur := docs[pl.id]
		if created[pl.e.Key] {
			r.Action = "create"
			cur = ticket.Doc{}
		}
		for _, w := range effective(cur, pl.writes) {
			r.Fields = append(r.Fields, w.Path)
		}
		if pl.parent != "" && cur.Get("parent") != pl.parent {
			r.Fields = append(r.Fields, "parent")
		}
		for _, b := range pl.blocked {
			if !contains(strList(cur.Value("relations.blocked_by")), b) {
				r.Fields = append(r.Fields, "relations.blocked_by")
				break
			}
		}
		if r.Action == "update" && len(r.Fields) == 0 {
			r.Action = "unchanged"
		}
		out = append(out, r)
	}
	if dry {
		return out, nil
	}

	// Create every new record first, so parent and blocker links between new
	// tickets have both ends on disk when they are written.
	for _, pl := range work {
		if !created[pl.e.Key] {
			continue
		}
		st := peerStore(base, pl.id)
		st.EnsureDirs()
		status := pl.e.Status
		if status == "" {
			status = "backlog"
		}
		if err := withLock(st, func() error {
			d := st.LoadForMutate()
			d.Set("id", pl.id)
			d.Set("status", status)
			d.Set("origin.type", "import")
			if pl.e.Requirement != "" {
				if err := os.WriteFile(filepath.Join(st.Home(), "requirement.md"), []byte(strings.TrimRight(pl.e.Requirement, "\n")+"\n"), 0o644); err != nil {
					return err
				}
				d.Set("pointers.requirement", "requirement.md")
			}
			if err := ticket.WriteDoc(st.IndexPath(), d); err != nil {
				return err
			}
			extra, _ := json.Marshal(map[string]string{"plan": p.Plan, "key": pl.e.Key})
			st.HistoryAppendExtra("ticket_initialized", actor, string(extra))
			return nil
		}); err != nil {
			return out, fmt.Errorf("%s: %w", pl.e.Key, err)
		}
	}
	for _, pl := range work {
		st := peerStore(base, pl.id)
		if _, err := fieldsApply(st, pl.writes, actor); err != nil {
			return out, fmt.Errorf("%s: %w", pl.e.Key, err)
		}
		if pl.parent != "" && ticket.ReadDoc(st.IndexPath()).Get("parent") != pl.parent {
			if err := parentSet(st, pl.parent, actor); err != nil {
				return out, fmt.Errorf("%s: %w", pl.e.Key, err)
			}
		}
		for _, b := range pl.blocked {
			if err := relationEdit(st, "blocked_by", b, true, actor); err != nil {
				return out, fmt.Errorf("%s: %w", pl.e.Key, err)
			}
		}
	}
	return out, nil
}

func runImport(args []string) {
	file, dry, asJSON := "", false, false
	for _, a := range args {
		switch a {
		case "--dry-run":
			dry = true
		case "--json":
			asJSON = true
		default:
			if strings.HasPrefix(a, "-") || file != "" {
				fmt.Fprintln(os.Stderr, "usage: import <plan.yaml|plan.json> [--dry-run] [--json]")
				os.Exit(2)
			}
			file = a
		}
	}
	if file == "" {
		fmt.Fprintln(os.Stderr, "usage: import <plan.yaml|plan.json> [--dry-run] [--json]")
		os.Exit(2)
	}
	p, err := loadImportPlan(file)
	if err != nil {
		fmt.Fprintln(os.Stderr, "import:", err)
		os.Exit(2)
	}
	env := resolveProject()
	res, err := importProject(env.ProjectHome, p, dry, actorRole())
	if err != nil {
		fmt.Fprintln(os.Stderr, "import:", err)
		os.Exit(2)
	}
	if asJSON {
		b, _ := json.MarshalIndent(map[string]interface{}{"plan": p.Plan, "dry_run": dry, "tickets": res}, "", "  ")
		fmt.Println(string(b))
		os.Exit(0)
	}
	n := map[string]int{}
	for _, r := range res {
		n[r.Action]++
		if r.Action != "unchanged" {
			sort.Strings(r.Fields)
			fmt.Printf("%-9s %-12s %-20s %s\n", r.Action, r.ID, r.Key, strings.Join(r.Fields, ", "))
		}
	}
	verb := ""
	if dry {
		verb = " (dry run — nothing written)"
	}
	fmt.Printf("import %s: %d create, %d update, %d unchanged%s\n", p.Plan, n["create"], n["update"], n["unchanged"], verb)
	os.Exit(0)
}
