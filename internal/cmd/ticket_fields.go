package cmd

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"reflect"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/2found/2build/internal/ticket"
)

// Planning-field verbs (bs-w5p3c6mq): title, kind, product, role, labels,
// window and the company-loop block. Each verb is a thin argv parser over one
// locked function the dashboard calls too, so a field moved from a browser and
// a field moved from a terminal leave the same history row.

// fieldWrite is one dotted-path assignment. Value is any JSON value; nil
// writes null.
type fieldWrite struct {
	Path  string
	Value interface{}
}

// fieldsApply writes a batch to one ticket under its lock and appends one
// `field_set` history event per path whose value actually changed. A batch
// that changes nothing writes nothing — re-running an import or a migration
// is silent.
func fieldsApply(st *ticket.Store, writes []fieldWrite, actor string) (changed []string, err error) {
	return fieldsApplyFn(st, func(ticket.Doc) ([]fieldWrite, error) { return writes, nil }, actor)
}

// buildErr marks a validation failure from a fieldsApplyFn builder, so a verb
// can exit 2 (usage) rather than 1 (I/O).
type buildErr struct{ error }

// fieldsApplyFn computes the writes from the record read under the lock, so a
// read-modify-write field (labels, evidence refs) cannot lose a concurrent
// writer's change.
func fieldsApplyFn(st *ticket.Store, build func(ticket.Doc) ([]fieldWrite, error), actor string) (changed []string, err error) {
	err = withLock(st, func() error {
		doc := st.LoadForMutate()
		writes, err := build(doc)
		if err != nil {
			return buildErr{err}
		}
		type change struct {
			path     string
			from, to interface{}
		}
		var changes []change
		for _, w := range writes {
			to, err := jsonNormalize(w.Value)
			if err != nil {
				return err
			}
			from := doc.Value(w.Path)
			if reflect.DeepEqual(from, to) || (from == nil && reflect.DeepEqual(to, []interface{}{})) {
				continue
			}
			raw, _ := json.Marshal(to)
			if err := doc.SetObj(w.Path, string(raw)); err != nil {
				return err
			}
			changes = append(changes, change{w.Path, from, to})
		}
		if len(changes) == 0 {
			return nil
		}
		if err := ticket.WriteDoc(st.IndexPath(), doc); err != nil {
			return err
		}
		for _, c := range changes {
			extra, _ := json.Marshal(map[string]interface{}{"field": c.path, "from": c.from, "to": c.to})
			st.HistoryAppendExtra("field_set", actor, string(extra))
			changed = append(changed, c.path)
		}
		return nil
	})
	return changed, err
}

// jsonNormalize round-trips a Go value through JSON with UseNumber, so it
// compares equal to what ReadDoc decoded from disk (json.Number, []interface{},
// map[string]interface{}).
func jsonNormalize(v interface{}) (interface{}, error) {
	raw, err := json.Marshal(v)
	if err != nil {
		return nil, err
	}
	dec := json.NewDecoder(strings.NewReader(string(raw)))
	dec.UseNumber()
	var out interface{}
	if err := dec.Decode(&out); err != nil {
		return nil, err
	}
	return out, nil
}

// ─── validated setters (shared with the dashboard and the importer) ──────────

func titleWrite(title string) (fieldWrite, error) {
	title = strings.TrimSpace(title)
	if title == "" || len(title) > 300 || strings.ContainsAny(title, "\n\r") {
		return fieldWrite{}, fmt.Errorf("title must be one line, 1–300 chars")
	}
	return fieldWrite{"title", title}, nil
}

func kindWrite(kind string) (fieldWrite, error) {
	if err := ticket.ValidKind(kind); err != nil {
		return fieldWrite{}, err
	}
	return fieldWrite{"kind", kind}, nil
}

func productWrite(product string) (fieldWrite, error) {
	if product == "" {
		return fieldWrite{"product", nil}, nil
	}
	if err := ticket.ValidProduct(product); err != nil {
		return fieldWrite{}, err
	}
	return fieldWrite{"product", product}, nil
}

// assignedToWrite sets the role queue. It never touches `assignee`, which is
// the Team lead's compare-and-set claim.
func assignedToWrite(role, person string) (fieldWrite, error) {
	if role == "" {
		if person != "" {
			return fieldWrite{}, fmt.Errorf("--person needs a role")
		}
		return fieldWrite{"assigned_to", nil}, nil
	}
	if err := ticket.ValidRole(role); err != nil {
		return fieldWrite{}, err
	}
	var p interface{}
	if person != "" {
		if err := ticket.ValidPerson(person); err != nil {
			return fieldWrite{}, err
		}
		p = person
	}
	return fieldWrite{"assigned_to", map[string]interface{}{"role": role, "person": p}}, nil
}

// windowWrite validates a start/due pair; "" leaves that end unset.
func windowWrite(start, due string) (fieldWrite, error) {
	for _, d := range []string{start, due} {
		if d != "" {
			if err := ticket.ValidDate(d); err != nil {
				return fieldWrite{}, err
			}
		}
	}
	if start != "" && due != "" && due < start {
		return fieldWrite{}, fmt.Errorf("window due %s is before start %s", due, start)
	}
	if start == "" && due == "" {
		return fieldWrite{"window", nil}, nil
	}
	return fieldWrite{"window", map[string]interface{}{"start": nilIfEmpty(start), "due": nilIfEmpty(due)}}, nil
}

func nilIfEmpty(s string) interface{} {
	if s == "" {
		return nil
	}
	return s
}

// labelsWith returns the doc's labels with adds applied and removes taken out,
// sorted and deduped so the stored order never depends on who wrote last.
func labelsWith(doc ticket.Doc, add, remove []string) ([]string, error) {
	set := map[string]bool{}
	for _, l := range strList(doc.Value("labels")) {
		set[l] = true
	}
	for _, l := range add {
		if err := ticket.ValidLabel(l); err != nil {
			return nil, err
		}
		set[l] = true
	}
	for _, l := range remove {
		delete(set, l)
	}
	out := make([]string, 0, len(set))
	for l := range set {
		out = append(out, l)
	}
	sort.Strings(out)
	return out, nil
}

func strList(v interface{}) []string {
	var out []string
	switch x := v.(type) {
	case []interface{}:
		for _, e := range x {
			if s, ok := e.(string); ok && s != "" {
				out = append(out, s)
			}
		}
	case []string:
		out = append(out, x...)
	case string:
		if x != "" {
			out = append(out, x)
		}
	}
	return out
}

// loopSpec is the editable half of the loop block. A nil pointer leaves that
// field alone; evidence refs are appended (deduped), never replaced.
type loopSpec struct {
	Stage, Target, Expected, CheckFrom *string
	CheckAfter                         *int
	Evidence                           []string
}

func loopWrites(doc ticket.Doc, s loopSpec) ([]fieldWrite, error) {
	var w []fieldWrite
	if s.Stage != nil {
		if *s.Stage == "" {
			w = append(w, fieldWrite{"loop.stage", nil})
		} else if err := ticket.ValidStage(*s.Stage); err != nil {
			return nil, err
		} else {
			w = append(w, fieldWrite{"loop.stage", *s.Stage})
		}
	}
	for _, f := range []struct {
		p *string
		k string
	}{{s.Target, "loop.target_outcome"}, {s.Expected, "loop.expected_delta"}} {
		if f.p == nil {
			continue
		}
		v := strings.TrimSpace(*f.p)
		if len(v) > 500 || strings.ContainsAny(v, "\n\r") {
			return nil, fmt.Errorf("%s must be one line, ≤500 chars", f.k)
		}
		w = append(w, fieldWrite{f.k, nilIfEmpty(v)})
	}
	if s.CheckAfter != nil {
		if *s.CheckAfter < 0 || *s.CheckAfter > 3650 {
			return nil, fmt.Errorf("check_after_days must be 0–3650")
		}
		w = append(w, fieldWrite{"loop.check_after_days", *s.CheckAfter})
	}
	if s.CheckFrom != nil {
		if *s.CheckFrom != "" {
			if err := ticket.ValidDate(*s.CheckFrom); err != nil {
				return nil, err
			}
		}
		w = append(w, fieldWrite{"loop.check_from", nilIfEmpty(*s.CheckFrom)})
	}
	if len(s.Evidence) > 0 {
		refs, err := evidenceWith(doc, s.Evidence)
		if err != nil {
			return nil, err
		}
		w = append(w, fieldWrite{"loop.evidence_refs", refs})
	}
	return w, nil
}

func evidenceWith(doc ticket.Doc, add []string) ([]string, error) {
	refs := strList(doc.Value("loop.evidence_refs"))
	seen := map[string]bool{}
	for _, r := range refs {
		seen[r] = true
	}
	for _, r := range add {
		r = strings.TrimSpace(r)
		if err := ticket.ValidEvidence(r); err != nil {
			return nil, err
		}
		if !seen[r] {
			seen[r] = true
			refs = append(refs, r)
		}
	}
	return refs, nil
}

// readForWrites reads the current record so a writer can compute a merge
// (labels, evidence) before taking the lock. fieldsApply re-reads under the
// lock and compares, so a lost race only ever costs a no-op, never a stale
// overwrite of an unrelated field.
func readForWrites(st *ticket.Store) ticket.Doc { return ticket.ReadDoc(st.IndexPath()) }

// ─── company-loop decisions ──────────────────────────────────────────────────

// humanGate refuses the human-only verbs from an agent context. It is a
// policy guard, not authentication: it stops a dispatched worker, a Team lead
// or an orchestrator role from recording a founder's go, which is the rule the
// company loop rests on ("Soot never publishes, spends or ships without a
// human go"). The dashboard calls the locked functions directly as the human.
func humanGate(verb string) error {
	role := actorRole()
	if role != "developer" && role != "dashboard" {
		return fmt.Errorf("%s is a human decision; refused for AGENT_ROLE=%s", verb, role)
	}
	for _, k := range []string{"BABYSIT_FOREMAN", "OPENCLAW_SESSION"} {
		if os.Getenv(k) != "" {
			return fmt.Errorf("%s is a human decision; refused inside an agent run (%s is set)", verb, k)
		}
	}
	return nil
}

// personName names who decided: BBS_PERSON, else the login name.
func personName() string {
	for _, k := range []string{"BBS_PERSON", "USER", "USERNAME"} {
		if v := os.Getenv(k); v != "" {
			return v
		}
	}
	return "human"
}

// loopRecord is the CL4.3 decision-log row, written whole into the history
// event so the log reads from history alone, even after later edits.
func loopRecord(st *ticket.Store, doc ticket.Doc, at string) map[string]interface{} {
	days, _ := strconv.Atoi(doc.Get("loop.check_after_days"))
	return map[string]interface{}{
		"at": at, "stage": nilIfEmpty(doc.Get("loop.stage")), "opportunity_ref": st.Env.Ticket,
		"product": nilIfEmpty(doc.Get("product")), "title": ticket.Title(st.Home(), doc),
		"target_outcome":   nilIfEmpty(doc.Get("loop.target_outcome")),
		"expected_delta":   nilIfEmpty(doc.Get("loop.expected_delta")),
		"check_after_days": days, "evidence_refs": strList(doc.Value("loop.evidence_refs")),
	}
}

// decideSet records go/later/drop. `go` needs a target outcome first — CL1.2:
// without one, hit rate cannot be measured.
func decideSet(st *ticket.Store, decision, note string, evidence []string, by, actor string) error {
	if err := ticket.ValidDecision(decision); err != nil {
		return err
	}
	return withLock(st, func() error {
		doc := st.LoadForMutate()
		if decision == "go" && doc.Get("loop.target_outcome") == "" {
			return fmt.Errorf("go needs a target outcome first: bbs ticket loop set --target <outcome> --expected <delta> --check-after <days>")
		}
		if len(evidence) > 0 {
			refs, err := evidenceWith(doc, evidence)
			if err != nil {
				return err
			}
			if err := setJSON(doc, "loop.evidence_refs", refs); err != nil {
				return err
			}
		}
		at := time.Now().UTC().Format(time.RFC3339)
		rec := map[string]interface{}{"value": decision, "at": at, "by": by, "note": nilIfEmpty(note)}
		if err := setJSON(doc, "loop.decision", rec); err != nil {
			return err
		}
		if err := ticket.WriteDoc(st.IndexPath(), doc); err != nil {
			return err
		}
		row := loopRecord(st, doc, at)
		row["decision"], row["note"], row["by"] = decision, nilIfEmpty(note), by
		extra, _ := json.Marshal(row)
		st.HistoryAppendExtra("loop_decision", actor, string(extra))
		return nil
	})
}

// actualSet records what the target outcome actually did. A later record
// supersedes it in index.json; history keeps every one.
func actualSet(st *ticket.Store, delta, result, note string, evidence []string, by, actor string) error {
	delta = strings.TrimSpace(delta)
	if delta == "" || len(delta) > 500 || strings.ContainsAny(delta, "\n\r") {
		return fmt.Errorf("--delta must be one line, 1–500 chars")
	}
	if err := ticket.ValidResult(result); err != nil {
		return err
	}
	for _, e := range evidence {
		if err := ticket.ValidEvidence(e); err != nil {
			return err
		}
	}
	return withLock(st, func() error {
		doc := st.LoadForMutate()
		at := time.Now().UTC().Format(time.RFC3339)
		if evidence == nil {
			evidence = []string{}
		}
		rec := map[string]interface{}{"delta": delta, "result": result, "at": at, "by": by, "note": nilIfEmpty(note), "evidence_refs": evidence}
		if err := setJSON(doc, "loop.actual", rec); err != nil {
			return err
		}
		if err := ticket.WriteDoc(st.IndexPath(), doc); err != nil {
			return err
		}
		row := loopRecord(st, doc, at)
		row["decision"] = nilIfEmpty(doc.Get("loop.decision.value"))
		row["actual_delta"], row["result"], row["note"], row["by"] = delta, result, nilIfEmpty(note), by
		row["actual_evidence_refs"] = evidence
		extra, _ := json.Marshal(row)
		st.HistoryAppendExtra("loop_actual", actor, string(extra))
		return nil
	})
}

func setJSON(doc ticket.Doc, path string, v interface{}) error {
	raw, err := json.Marshal(v)
	if err != nil {
		return err
	}
	return doc.SetObj(path, string(raw))
}

// ─── argv verbs ──────────────────────────────────────────────────────────────

func fieldsVerb(verb string, build func(st *ticket.Store) ([]fieldWrite, error)) {
	env := resolveEnv()
	needTicket(env)
	st := ticket.New(env)
	// The builder reads the record itself (readForWrites); running it under the
	// lock makes that read current.
	changed, err := fieldsApplyFn(st, func(ticket.Doc) ([]fieldWrite, error) { return build(st) }, actorRole())
	if err != nil {
		fmt.Fprintf(os.Stderr, "%s: %v\n", verb, err)
		var be buildErr
		if errors.As(err, &be) {
			os.Exit(2)
		}
		os.Exit(1)
	}
	if len(changed) == 0 {
		fmt.Printf("%s: unchanged\n", env.Ticket)
	} else {
		fmt.Printf("%s: set %s\n", env.Ticket, strings.Join(changed, ", "))
	}
	os.Exit(0)
}

func runSetTitle(args []string) {
	fieldsVerb("set-title", func(*ticket.Store) ([]fieldWrite, error) {
		w, err := titleWrite(strings.Join(args, " "))
		return []fieldWrite{w}, err
	})
}

func runSetKind(args []string) {
	fieldsVerb("set-kind", func(*ticket.Store) ([]fieldWrite, error) {
		if len(args) != 1 {
			return nil, fmt.Errorf("usage: set-kind %s", strings.Join(ticket.Kinds, "|"))
		}
		w, err := kindWrite(args[0])
		return []fieldWrite{w}, err
	})
}

func runSetProduct(args []string) {
	fieldsVerb("set-product", func(*ticket.Store) ([]fieldWrite, error) {
		if len(args) != 1 {
			return nil, fmt.Errorf("usage: set-product <product>|--none")
		}
		p := args[0]
		if p == "--none" {
			p = ""
		}
		w, err := productWrite(p)
		return []fieldWrite{w}, err
	})
}

func runAssignTo(args []string) {
	fieldsVerb("assign-to", func(*ticket.Store) ([]fieldWrite, error) {
		role, person := "", ""
		for i := 0; i < len(args); i++ {
			switch a := args[i]; {
			case a == "--person":
				person, i = valueOf(args, i, a), i+1
			case a == "--none":
				role = ""
			case strings.HasPrefix(a, "-"):
				return nil, fmt.Errorf("unknown flag %s", a)
			default:
				role = a
			}
		}
		if role == "" && !contains(args, "--none") {
			return nil, fmt.Errorf("usage: assign-to %s [--person P] | --none", strings.Join(ticket.Roles, "|"))
		}
		w, err := assignedToWrite(role, person)
		return []fieldWrite{w}, err
	})
}

func runLabel(args []string) {
	fieldsVerb("label", func(st *ticket.Store) ([]fieldWrite, error) {
		if len(args) < 2 || (args[0] != "add" && args[0] != "remove") {
			return nil, fmt.Errorf("usage: label add|remove <label>...")
		}
		var add, rm []string
		if args[0] == "add" {
			add = args[1:]
		} else {
			rm = args[1:]
		}
		labels, err := labelsWith(readForWrites(st), add, rm)
		return []fieldWrite{{"labels", labels}}, err
	})
}

func runSetWindow(args []string) {
	fieldsVerb("set-window", func(st *ticket.Store) ([]fieldWrite, error) {
		doc := readForWrites(st)
		start, due := doc.Get("window.start"), doc.Get("window.due")
		seen := false
		for i := 0; i < len(args); i++ {
			switch a := args[i]; a {
			case "--start":
				start, i, seen = valueOf(args, i, a), i+1, true
			case "--due":
				due, i, seen = valueOf(args, i, a), i+1, true
			case "--none":
				start, due, seen = "", "", true
			default:
				return nil, fmt.Errorf("unknown argument %s", a)
			}
		}
		if !seen {
			return nil, fmt.Errorf("usage: set-window [--start YYYY-MM-DD] [--due YYYY-MM-DD] | --none")
		}
		w, err := windowWrite(start, due)
		return []fieldWrite{w}, err
	})
}

// runLoop is `loop set …`; `loop show` prints the block as JSON.
func runLoop(args []string) {
	if len(args) > 0 && args[0] == "show" {
		env := resolveEnv()
		needTicket(env)
		doc := ticket.ReadDoc(ticket.New(env).IndexPath())
		out := map[string]interface{}{"loop": doc.Value("loop")}
		if due, ok := ticket.CheckDue(doc); ok {
			out["check_due"] = due
			out["check_overdue"] = ticket.CheckOverdue(doc, ticket.Today())
		}
		b, _ := json.MarshalIndent(out, "", "  ")
		fmt.Println(string(b))
		os.Exit(0)
	}
	if len(args) == 0 || args[0] != "set" {
		fmt.Fprintln(os.Stderr, "usage: loop set [--stage S] [--target T] [--expected E] [--check-after N] [--check-from D] [--evidence REF]... | loop show")
		os.Exit(2)
	}
	rest := args[1:]
	fieldsVerb("loop set", func(st *ticket.Store) ([]fieldWrite, error) {
		var s loopSpec
		str := func(i int, f string) *string { v := valueOf(rest, i, f); return &v }
		for i := 0; i < len(rest); i++ {
			switch a := rest[i]; a {
			case "--stage":
				s.Stage, i = str(i, a), i+1
			case "--target":
				s.Target, i = str(i, a), i+1
			case "--expected":
				s.Expected, i = str(i, a), i+1
			case "--check-from":
				s.CheckFrom, i = str(i, a), i+1
			case "--check-after":
				n, err := strconv.Atoi(valueOf(rest, i, a))
				if err != nil {
					return nil, fmt.Errorf("--check-after must be a whole number of days")
				}
				s.CheckAfter, i = &n, i+1
			case "--evidence":
				s.Evidence, i = append(s.Evidence, valueOf(rest, i, a)), i+1
			default:
				return nil, fmt.Errorf("unknown argument %s", a)
			}
		}
		return loopWrites(readForWrites(st), s)
	})
}

func parseNoteEvidence(args []string, extra map[string]*string) (note string, evidence []string, err error) {
	for i := 0; i < len(args); i++ {
		a := args[i]
		switch {
		case a == "--note":
			note, i = valueOf(args, i, a), i+1
		case a == "--evidence":
			evidence, i = append(evidence, valueOf(args, i, a)), i+1
		case extra[a] != nil:
			*extra[a], i = valueOf(args, i, a), i+1
		default:
			return "", nil, fmt.Errorf("unknown argument %s", a)
		}
	}
	return note, evidence, nil
}

func runDecide(args []string) {
	env := resolveEnv()
	needTicket(env)
	if len(args) == 0 {
		fmt.Fprintln(os.Stderr, "usage: decide go|later|drop [--note M] [--evidence REF]...")
		os.Exit(2)
	}
	if err := humanGate("decide"); err != nil {
		fmt.Fprintln(os.Stderr, "decide:", err)
		os.Exit(3)
	}
	by := personName()
	note, ev, err := parseNoteEvidence(args[1:], map[string]*string{"--by": &by})
	if err != nil {
		fmt.Fprintln(os.Stderr, "decide:", err)
		os.Exit(2)
	}
	if err := decideSet(ticket.New(env), args[0], note, ev, by, actorRole()); err != nil {
		fmt.Fprintln(os.Stderr, "decide:", err)
		os.Exit(2)
	}
	fmt.Printf("%s: decision %s by %s\n", env.Ticket, args[0], by)
	os.Exit(0)
}

func runRecordActual(args []string) {
	env := resolveEnv()
	needTicket(env)
	var delta, result string
	by := personName()
	if r := actorRole(); r != "developer" && r != "dashboard" {
		by = r // an agent records as itself (CL4.3: the review job fills it)
	}
	note, ev, err := parseNoteEvidence(args, map[string]*string{"--delta": &delta, "--result": &result, "--by": &by})
	if err != nil || delta == "" || result == "" {
		if err == nil {
			err = fmt.Errorf("--delta and --result are required")
		}
		fmt.Fprintln(os.Stderr, "record-actual:", err)
		fmt.Fprintln(os.Stderr, "usage: record-actual --delta TEXT --result hit|partial|miss [--note M] [--evidence REF]...")
		os.Exit(2)
	}
	if err := actualSet(ticket.New(env), delta, result, note, ev, by, actorRole()); err != nil {
		fmt.Fprintln(os.Stderr, "record-actual:", err)
		os.Exit(2)
	}
	fmt.Printf("%s: actual recorded (%s)\n", env.Ticket, result)
	os.Exit(0)
}

func contains(list []string, v string) bool {
	for _, s := range list {
		if s == v {
			return true
		}
	}
	return false
}
