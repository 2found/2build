package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"

	"github.com/2found/2build/internal/identity"
	"github.com/2found/2build/internal/ticket"
)

// Symmetric relations (bs-w5p3c6mq). Every relation verb writes its own side
// exactly as before and then the inverse on the peer, under both locks taken
// in id order so two writers linking the same pair cannot deadlock. A peer
// without index.json is skipped: creating a record for an id nobody made would
// invent a ticket. `repair-relations` adds inverses for links written before
// this existed (or by hand).

// inverseRelation is the peer-side type, "" when the relation is one-way.
func inverseRelation(rel string) string {
	switch rel {
	case "blocks":
		return "blocked_by"
	case "blocked_by":
		return "blocks"
	case "related":
		return "related"
	}
	return "" // duplicate_of points one way
}

func peerStore(st *ticket.Store, id string) *ticket.Store {
	env := st.Env
	env.Ticket = id
	return ticket.New(env)
}

// peerExists is true when the peer has a record to update.
func peerExists(st *ticket.Store) bool {
	fi, err := os.Stat(st.IndexPath())
	return err == nil && !fi.IsDir()
}

// withLocks locks each distinct store in ticket-id order, runs fn, and
// releases in reverse.
func withLocks(stores []*ticket.Store, fn func() error) error {
	seen := map[string]bool{}
	var uniq []*ticket.Store
	for _, s := range stores {
		if s != nil && !seen[s.Env.Ticket] {
			seen[s.Env.Ticket] = true
			uniq = append(uniq, s)
		}
	}
	sort.Slice(uniq, func(i, j int) bool { return uniq[i].Env.Ticket < uniq[j].Env.Ticket })
	var held []*ticket.Store
	defer func() {
		for i := len(held) - 1; i >= 0; i-- {
			held[i].ReleaseLock()
		}
	}()
	for _, s := range uniq {
		if err := s.AcquireLock(); err != nil {
			return fmt.Errorf("failed to acquire lock after 5s — remove %s if no writer is active", s.LockPath())
		}
		held = append(held, s)
	}
	return fn()
}

// listAdd / listRemove report whether they changed the doc, so an inverse
// that is already in place writes nothing and logs nothing.
func listAdd(doc ticket.Doc, path, id string) (bool, error) {
	for _, v := range strList(doc.Value(path)) {
		if v == id {
			return false, nil
		}
	}
	return true, appendTicketString(doc, path, id)
}

func listRemove(doc ticket.Doc, path, id string) (bool, error) {
	found := false
	for _, v := range strList(doc.Value(path)) {
		if v == id {
			found = true
		}
	}
	if !found {
		return false, nil
	}
	enc, _ := json.Marshal(id)
	return true, doc.RemoveObj(path, string(enc))
}

// peerEdit loads a peer under its (held) lock, applies edit, and writes plus
// logs only when edit reports a change.
func peerEdit(peer *ticket.Store, event string, extra map[string]string, edit func(ticket.Doc) (bool, error)) error {
	if peer == nil || !peerExists(peer) {
		return nil
	}
	doc, err := ticket.ReadDocStrict(peer.IndexPath())
	if err != nil {
		return err
	}
	changed, err := edit(doc)
	if err != nil || !changed {
		return err
	}
	if err := ticket.WriteDoc(peer.IndexPath(), doc); err != nil {
		return err
	}
	extra["via"] = "inverse"
	raw, _ := json.Marshal(extra)
	peer.HistoryAppendExtra(event, actorRole(), string(raw))
	return nil
}

// isAncestor reports whether anc sits on id's parent chain (or is id), so a
// link that would close a loop is refused before anything is written.
func isAncestor(st *ticket.Store, anc, id string) bool {
	seen := map[string]bool{}
	for cur := id; cur != "" && !seen[cur]; {
		if cur == anc {
			return true
		}
		seen[cur] = true
		cur = ticket.ReadDoc(peerStore(st, cur).IndexPath()).Get("parent")
	}
	return false
}

// parentSet links child st under parent: the child-side fields exactly as the
// pre-symmetric verb wrote them, the child id on the new parent's children,
// and the child id off a previous parent's children (a reparent).
func parentSet(st *ticket.Store, parent, actor string) error {
	self := st.Env.Ticket
	if parent != self && isAncestor(st, self, parent) {
		return &usageErr{fmt.Sprintf("set-parent: %s is under %s; making it the parent would close a loop", parent, self)}
	}
	cur := ticket.ReadDoc(st.IndexPath())
	old := cur.Get("parent")
	np := peerStore(st, parent)
	var op *ticket.Store
	if old != "" && old != parent && old != self {
		op = peerStore(st, old)
	}
	lockable := []*ticket.Store{st}
	if parent == self || !peerExists(np) {
		np = nil
	} else {
		lockable = append(lockable, np)
	}
	if op != nil && !peerExists(op) {
		op = nil
	} else if op != nil {
		lockable = append(lockable, op)
	}
	return withLocks(lockable, func() error {
		doc := st.LoadForMutate()
		doc.Set("parent", parent)
		doc.Set("origin.parent", parent)
		if err := ticket.WriteDoc(st.IndexPath(), doc); err != nil {
			return err
		}
		st.HistoryAppendExtra("parent_set", actor, fmt.Sprintf(`{"parent":"%s"}`, parent))
		if np != nil {
			if err := peerEdit(np, "child_added", map[string]string{"child": self}, func(d ticket.Doc) (bool, error) {
				return listAdd(d, "children", self)
			}); err != nil {
				return err
			}
		}
		if op != nil {
			return peerEdit(op, "child_removed", map[string]string{"child": self}, func(d ticket.Doc) (bool, error) {
				return listRemove(d, "children", self)
			})
		}
		return nil
	})
}

// childAdd appends child to st's children and points the child at st. A child
// that already has a different parent is a conflict, not a silent reparent:
// the caller runs `set-parent` on the child to move it.
func childAdd(st *ticket.Store, child, actor string) error {
	self := st.Env.Ticket
	if child != self && isAncestor(st, child, self) {
		return &usageErr{fmt.Sprintf("add-child: %s is above %s; adding it as a child would close a loop", child, self)}
	}
	cs := peerStore(st, child)
	lockable := []*ticket.Store{st}
	if child == self || !peerExists(cs) {
		cs = nil
	} else {
		if p := ticket.ReadDoc(cs.IndexPath()).Get("parent"); p != "" && p != self {
			return &usageErr{fmt.Sprintf("add-child: %s already has parent %s; move it with `BABYSIT_TICKET=%s bbs ticket set-parent %s`", child, p, child, self)}
		}
		lockable = append(lockable, cs)
	}
	return withLocks(lockable, func() error {
		doc, err := loadRelationshipDoc(st)
		if err != nil {
			return err
		}
		extra, _ := json.Marshal(map[string]string{"child": child})
		changed, err := listAdd(doc, "children", child)
		if err != nil {
			return err
		}
		// Already linked (e.g. by the child's set-parent inverse): nothing to
		// write, nothing to log — linking twice is one link.
		if changed {
			if err := ticket.WriteDoc(st.IndexPath(), doc); err != nil {
				return err
			}
			st.HistoryAppendExtra("child_added", actor, string(extra))
		}
		if cs == nil {
			return nil
		}
		return peerEdit(cs, "parent_set", map[string]string{"parent": self}, func(d ticket.Doc) (bool, error) {
			if p := d.Get("parent"); p == self {
				return false, nil
			} else if p != "" {
				return false, fmt.Errorf("add-child: %s gained parent %s while linking", child, p)
			}
			d.Set("parent", self)
			d.Set("origin.parent", self)
			return true, nil
		})
	})
}

// relationEdit adds or removes one typed relation on st and its inverse on
// the target.
func relationEdit(st *ticket.Store, relation, target string, add bool, actor string) error {
	self := st.Env.Ticket
	inv := inverseRelation(relation)
	ps := peerStore(st, target)
	lockable := []*ticket.Store{st}
	if inv != "" && target != self && peerExists(ps) {
		lockable = append(lockable, ps)
	} else {
		ps = nil
	}
	return withLocks(lockable, func() error {
		doc, err := loadRelationshipDoc(st)
		if err != nil {
			return err
		}
		extra, _ := json.Marshal(map[string]string{"type": relation, "target": target})
		path := "relations." + relation
		changed := true
		switch {
		case add && relation == "duplicate_of":
			changed = doc.Get(path) != target
			doc.Set(path, target)
		case add:
			if changed, err = listAdd(doc, path, target); err != nil {
				return err
			}
		case relation == "duplicate_of":
			changed = false
			if value, ok := doc.Value(path).(string); ok && value == target {
				doc.Set(path, "null")
				changed = true
			}
		default:
			if changed, err = listRemove(doc, path, target); err != nil {
				return err
			}
		}
		event := "relation_removed"
		if add {
			event = "relation_added"
		}
		// An edge already in place (or already gone) — typically written by the
		// peer's inverse — is a no-op on this side; the peer still gets checked.
		if changed {
			if err := ticket.WriteDoc(st.IndexPath(), doc); err != nil {
				return err
			}
			st.HistoryAppendExtra(event, actor, string(extra))
		}
		if ps == nil {
			return nil
		}
		return peerEdit(ps, event, map[string]string{"type": inv, "target": self}, func(d ticket.Doc) (bool, error) {
			if add {
				return listAdd(d, "relations."+inv, self)
			}
			return listRemove(d, "relations."+inv, self)
		})
	})
}

// usageErr marks a refusal the CLI reports with exit 2 (a conflict the caller
// must resolve), as opposed to an I/O failure (exit 1).
type usageErr struct{ msg string }

func (e *usageErr) Error() string { return e.msg }

func exitRelation(err error) {
	if err == nil {
		os.Exit(0)
	}
	fmt.Fprintln(os.Stderr, err)
	if _, ok := err.(*usageErr); ok {
		os.Exit(2)
	}
	os.Exit(1)
}

// ─── repair-relations ────────────────────────────────────────────────────────

// relationFix is one missing inverse found by repair.
type relationFix struct {
	Ticket   string `json:"ticket"`  // the record that gains the entry
	Path     string `json:"path"`    // children | parent | relations.<type>
	Value    string `json:"value"`   // the id it gains
	Because  string `json:"because"` // the record whose link implies it
	Conflict string `json:"conflict,omitempty"`
}

// planRepairs scans every record in the project and lists the inverses that
// are missing. A child whose parent field names someone else is a conflict,
// reported and never rewritten: which side is right is a human call.
func planRepairs(projectHome string, only map[string]bool) ([]relationFix, error) {
	docs, err := loadProjectDocs(projectHome)
	if err != nil {
		return nil, err
	}
	return planRepairsDocs(docs, only), nil
}

func planRepairsDocs(docs map[string]ticket.Doc, only map[string]bool) []relationFix {
	var fixes []relationFix
	has := func(id, path, v string) bool {
		d, ok := docs[id]
		if !ok {
			return true // no record: nothing to repair
		}
		if path == "parent" {
			return d.Get("parent") == v
		}
		for _, x := range strList(d.Value(path)) {
			if x == v {
				return true
			}
		}
		return false
	}
	ids := make([]string, 0, len(docs))
	for id := range docs {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	for _, id := range ids {
		if only != nil && !only[id] {
			continue
		}
		d := docs[id]
		if p := d.Get("parent"); p != "" && p != id && !has(p, "children", id) {
			fixes = append(fixes, relationFix{Ticket: p, Path: "children", Value: id, Because: id})
		}
		for _, c := range strList(d.Value("children")) {
			if c == id {
				continue
			}
			cd, ok := docs[c]
			if !ok {
				continue
			}
			switch p := cd.Get("parent"); {
			case p == id:
			case p == "":
				fixes = append(fixes, relationFix{Ticket: c, Path: "parent", Value: id, Because: id})
			default:
				fixes = append(fixes, relationFix{Ticket: c, Path: "parent", Value: id, Because: id,
					Conflict: fmt.Sprintf("%s lists %s as a child but its parent is %s", id, c, p)})
			}
		}
		for _, rel := range []string{"blocks", "blocked_by", "related"} {
			inv := inverseRelation(rel)
			for _, t := range strList(d.Value("relations." + rel)) {
				if t != id && !has(t, "relations."+inv, id) {
					fixes = append(fixes, relationFix{Ticket: t, Path: "relations." + inv, Value: id, Because: id})
				}
			}
		}
	}
	return dedupeFixes(fixes)
}

// identityFor scopes a store to a project home without resolving git state —
// for whole-project passes (migrate, import) that address tickets by id.
func identityFor(projectHome, id string) identity.Env {
	return identity.Env{ProjectHome: projectHome, Ticket: id, Slug: filepath.Base(projectHome)}
}

func dedupeFixes(in []relationFix) []relationFix {
	seen := map[string]bool{}
	var out []relationFix
	for _, f := range in {
		k := f.Ticket + "\x00" + f.Path + "\x00" + f.Value
		if !seen[k] {
			seen[k] = true
			out = append(out, f)
		}
	}
	return out
}

// loadProjectDocs reads every tickets/<id>/index.json under a project home.
// A directory without one is not a record and is skipped. An unreadable record
// fails the load: import and repair must not write from a partial view.
func loadProjectDocs(projectHome string) (map[string]ticket.Doc, error) {
	return readProjectDocs(projectHome, false)
}

// loadProjectDocsSkipping is loadProjectDocs for read-only reports: an
// unreadable record is left out instead of failing the whole project.
func loadProjectDocsSkipping(projectHome string) (map[string]ticket.Doc, error) {
	return readProjectDocs(projectHome, true)
}

func readProjectDocs(projectHome string, skipBad bool) (map[string]ticket.Doc, error) {
	dir := projectHome + "/tickets"
	ents, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return map[string]ticket.Doc{}, nil
		}
		return nil, err
	}
	docs := map[string]ticket.Doc{}
	for _, e := range ents {
		if !e.IsDir() {
			continue
		}
		p := dir + "/" + e.Name() + "/index.json"
		if _, err := os.Stat(p); err != nil {
			continue
		}
		d, err := ticket.ReadDocStrict(p)
		if err != nil {
			if skipBad {
				continue
			}
			return nil, fmt.Errorf("%s: %w", p, err)
		}
		docs[e.Name()] = d
	}
	return docs, nil
}

// applyRepairs writes each non-conflict fix under the target's lock.
func applyRepairs(st *ticket.Store, fixes []relationFix, actor string) (int, error) {
	n := 0
	for _, f := range fixes {
		if f.Conflict != "" {
			continue
		}
		ps := peerStore(st, f.Ticket)
		err := withLock(ps, func() error {
			return peerEdit(ps, "relation_repaired", map[string]string{"path": f.Path, "value": f.Value, "because": f.Because},
				func(d ticket.Doc) (bool, error) {
					if f.Path == "parent" {
						if d.Get("parent") != "" {
							return false, nil
						}
						d.Set("parent", f.Value)
						d.Set("origin.parent", f.Value)
						return true, nil
					}
					return listAdd(d, f.Path, f.Value)
				})
		})
		if err != nil {
			return n, err
		}
		n++
	}
	return n, nil
}

func runRepairRelations(args []string) {
	env := resolveEnv()
	all, dry, asJSON := false, false, false
	for _, a := range args {
		switch a {
		case "--all":
			all = true
		case "--dry-run":
			dry = true
		case "--json":
			asJSON = true
		default:
			fmt.Fprintln(os.Stderr, "usage: repair-relations [--all] [--dry-run] [--json]")
			os.Exit(2)
		}
	}
	var only map[string]bool
	if !all {
		needTicket(env)
		only = map[string]bool{env.Ticket: true}
	}
	fixes, err := planRepairs(env.ProjectHome, only)
	if err != nil {
		fmt.Fprintln(os.Stderr, "repair-relations:", err)
		os.Exit(1)
	}
	applied := 0
	if !dry {
		if applied, err = applyRepairs(ticket.New(env), fixes, actorRole()); err != nil {
			fmt.Fprintln(os.Stderr, "repair-relations:", err)
			os.Exit(1)
		}
	}
	if asJSON {
		b, _ := json.MarshalIndent(map[string]interface{}{"dry_run": dry, "fixes": nonNil(fixes), "applied": applied}, "", "  ")
		fmt.Println(string(b))
	} else {
		conflicts := 0
		for _, f := range fixes {
			if f.Conflict != "" {
				conflicts++
				fmt.Printf("conflict  %s\n", f.Conflict)
			} else {
				fmt.Printf("%-8s %s %s += %s  (from %s)\n", map[bool]string{true: "would", false: "fixed"}[dry], f.Ticket, f.Path, f.Value, f.Because)
			}
		}
		fmt.Printf("%d inverse(s) %s, %d conflict(s)\n", len(fixes)-conflicts, map[bool]string{true: "missing", false: "added"}[dry], conflicts)
	}
	os.Exit(0)
}

func nonNil(f []relationFix) []relationFix {
	if f == nil {
		return []relationFix{}
	}
	return f
}
