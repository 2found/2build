package cmd

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"

	"github.com/2found/2build/internal/ticket"
)

// `bbs ticket migrate` (bs-w5p3c6mq) backfills the planning fields of a whole
// project from what its records already say: the requirement H1 and header
// bullets, the role words parked in `assignee`, the done transition in
// history, and the one-sided relation links. It only ever fills a field that is
// empty — never overwrites one — so it is safe to re-run, and a second run
// changes nothing. The one exception to "add only" is documented: an
// `assignee` holding a role word (swe|human|marketing|ops) is a role, not a
// Team lead claim, so it moves to `assigned_to.role` and `assignee` becomes
// null, which is what keeps `claim` from treating "swe" as a foreman.

type migrateTicket struct {
	ID      string       `json:"id"`
	Changes []fieldWrite `json:"-"`
	Fields  []string     `json:"fields"`
	Notes   []string     `json:"notes,omitempty"`
}

type migrateReport struct {
	Project     string          `json:"project"`
	DryRun      bool            `json:"dry_run"`
	Records     int             `json:"records"`
	NoIndex     []string        `json:"no_index"`
	Changed     int             `json:"changed_records"`
	FieldCounts map[string]int  `json:"field_counts"`
	Tickets     []migrateTicket `json:"tickets"`
	Relations   []relationFix   `json:"relations"`
}

// isEmpty is "nothing is stored there yet".
func isEmpty(v interface{}) bool {
	switch x := v.(type) {
	case nil:
		return true
	case string:
		return strings.TrimSpace(x) == ""
	case []interface{}:
		return len(x) == 0
	case map[string]interface{}:
		return len(x) == 0
	}
	return false
}

// migrateWrites derives the backfill for one record. doc is not modified.
func migrateWrites(home string, doc ticket.Doc) (writes []fieldWrite, notes []string) {
	fill := func(path string, v interface{}) {
		if v != nil && isEmpty(doc.Value(path)) {
			writes = append(writes, fieldWrite{path, v})
		}
	}
	h := ticket.ReadHeader(filepath.Join(home, "requirement.md"))
	b := h.Bullets

	if h.H1 != "" {
		title, ws, wd := ticket.CleanTitle(h.H1)
		if title != "" {
			fill("title", title)
		}
		if ws != "" && isEmpty(doc.Value("window")) && b["window"] == "" {
			fill("window", map[string]interface{}{"start": ws, "due": wd})
		}
	}
	if k := ticket.FirstWord(b["kind"]); k != "" {
		if ticket.ValidKind(k) == nil {
			fill("kind", k)
		} else {
			notes = append(notes, fmt.Sprintf("kind %q not recognised", b["kind"]))
		}
	}
	if p := ticket.FirstWord(b["product"]); p != "" {
		if ticket.ValidProduct(p) == nil {
			fill("product", p)
		} else {
			notes = append(notes, fmt.Sprintf("product %q not a slug", b["product"]))
		}
	}
	if b["window"] != "" {
		if s, d, ok := ticket.ParseWindow(b["window"]); ok {
			fill("window", map[string]interface{}{"start": s, "due": d})
		} else {
			notes = append(notes, fmt.Sprintf("window %q not parsed", b["window"]))
		}
	}

	// The role: a role word in `assignee` moves; otherwise the header's.
	role := ""
	if a := doc.Get("assignee"); ticket.ValidRole(a) == nil {
		role = a
		writes = append(writes, fieldWrite{"assignee", nil})
	} else if r := ticket.FirstWord(b["assignee"]); ticket.ValidRole(r) == nil {
		role = r
	}
	if role != "" && isEmpty(doc.Value("assigned_to")) {
		writes = append(writes, fieldWrite{"assigned_to", map[string]interface{}{"role": role, "person": nil}})
	}

	if s := strings.ToUpper(ticket.FirstWord(b["size"])); s != "" {
		switch s {
		case "XS", "S", "M", "L":
			fill("pointers.ticket_size", s)
		}
	}
	if b["target"] != "" {
		fill("loop.target_outcome", b["target"])
	}
	if b["expected"] != "" {
		fill("loop.expected_delta", b["expected"])
	}
	if b["check_after"] != "" {
		if n, ok := ticket.LeadingInt(b["check_after"]); ok {
			fill("loop.check_after_days", n)
		} else {
			notes = append(notes, fmt.Sprintf("check after %q not a number", b["check_after"]))
		}
	}
	if st := ticket.FirstWord(b["stage"]); ticket.ValidStage(st) == nil {
		fill("loop.stage", st)
	}
	if refs := ticket.Refs(b["evidence"]); len(refs) > 0 {
		fill("loop.evidence_refs", refs)
	}
	if v := b["decision"]; v != "" {
		// A decision the founder already wrote down is transcribed, with its
		// text as the note and the file as its provenance — not re-decided.
		if d := ticket.FirstWord(v); ticket.ValidDecision(d) == nil {
			at := ticket.ISODate(v)
			if at != "" {
				at += "T00:00:00Z"
			}
			fill("loop.decision", map[string]interface{}{"value": d, "at": nilIfEmpty(at), "by": "requirement.md", "note": v})
		} else {
			notes = append(notes, fmt.Sprintf("decision %q not go|later|drop", v))
		}
	}
	if k := strings.Trim(b["key"], "` "); k != "" {
		plan := ""
		if ids := ticket.IDsIn(b["milestone"]); len(ids) > 0 {
			plan = ids[0]
		}
		fill("import", map[string]interface{}{"plan": nilIfEmpty(plan), "key": k})
	}
	// Blocked by: the list only grows. Existing ids keep their order.
	if ids := ticket.IDsIn(b["blocked_by"]); len(ids) > 0 {
		cur := strList(doc.Value("relations.blocked_by"))
		merged := append([]string{}, cur...)
		for _, id := range ids {
			if id != doc.Get("id") && !contains(merged, id) {
				merged = append(merged, id)
			}
		}
		if len(merged) > len(cur) {
			writes = append(writes, fieldWrite{"relations.blocked_by", merged})
		}
	}
	if doc.Get("status") == "done" && isEmpty(doc.Value("done_at")) {
		if at := lastDoneAt(filepath.Join(home, "history.jsonl")); at != "" {
			fill("done_at", at)
		} else {
			notes = append(notes, "done with no status_changed→done in history; done_at left empty")
		}
	}
	return writes, notes
}

// lastDoneAt is the ts of the last status_changed→done event.
func lastDoneAt(path string) string {
	f, err := os.Open(path)
	if err != nil {
		return ""
	}
	defer f.Close()
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 64*1024), 4*1024*1024)
	at := ""
	for sc.Scan() {
		line := sc.Bytes()
		if !strings.Contains(string(line), `"status_changed"`) {
			continue
		}
		var ev struct {
			Event, To, Ts string
		}
		if json.Unmarshal(line, &ev) == nil && ev.Event == "status_changed" && ev.To == "done" {
			at = ev.Ts
		}
	}
	return at
}

// effective filters writes to those that change the doc.
func effective(doc ticket.Doc, writes []fieldWrite) []fieldWrite {
	var out []fieldWrite
	for _, w := range writes {
		to, err := jsonNormalize(w.Value)
		if err == nil && !reflect.DeepEqual(doc.Value(w.Path), to) {
			out = append(out, w)
		}
	}
	return out
}

func runMigrate(args []string) {
	dry, report := false, ""
	for i := 0; i < len(args); i++ {
		switch a := args[i]; a {
		case "--dry-run":
			dry = true
		case "--report":
			report, i = valueOf(args, i, a), i+1
		default:
			fmt.Fprintln(os.Stderr, "usage: migrate [--dry-run] [--report FILE.json]")
			os.Exit(2)
		}
	}
	env := resolveProject()
	rep, err := migrateProject(env.ProjectHome, dry, "migrate")
	if err != nil {
		fmt.Fprintln(os.Stderr, "migrate:", err)
		os.Exit(1)
	}
	rep.Project = filepath.Base(env.ProjectHome)
	if report != "" {
		b, _ := json.MarshalIndent(rep, "", "  ")
		if err := os.WriteFile(report, append(b, '\n'), 0o644); err != nil {
			fmt.Fprintln(os.Stderr, "migrate:", err)
			os.Exit(1)
		}
	}
	verb := "changed"
	if dry {
		verb = "would change"
	}
	fmt.Printf("migrate %s: %d records, %d %s, %d without index.json (untouched)\n", rep.Project, rep.Records, rep.Changed, verb, len(rep.NoIndex))
	keys := make([]string, 0, len(rep.FieldCounts))
	for k := range rep.FieldCounts {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		fmt.Printf("  %-24s %d\n", k, rep.FieldCounts[k])
	}
	conflicts := 0
	for _, f := range rep.Relations {
		if f.Conflict != "" {
			conflicts++
		}
	}
	fmt.Printf("  %-24s %d (%d conflicts reported, not rewritten)\n", "relation inverses", len(rep.Relations)-conflicts, conflicts)
	os.Exit(0)
}

// migrateProject runs (or dry-runs) the backfill over every record, then the
// relation repair over the result.
func migrateProject(projectHome string, dry bool, actor string) (migrateReport, error) {
	rep := migrateReport{DryRun: dry, FieldCounts: map[string]int{}, Tickets: []migrateTicket{}, NoIndex: []string{}}
	dir := filepath.Join(projectHome, "tickets")
	ents, err := os.ReadDir(dir)
	if err != nil {
		return rep, err
	}
	docs := map[string]ticket.Doc{}
	for _, e := range ents {
		if !e.IsDir() {
			continue
		}
		p := filepath.Join(dir, e.Name(), "index.json")
		if _, err := os.Stat(p); err != nil {
			rep.NoIndex = append(rep.NoIndex, e.Name())
			continue
		}
		d, err := ticket.ReadDocStrict(p)
		if err != nil {
			return rep, fmt.Errorf("%s: %w (fix or move it first; nothing was written)", p, err)
		}
		docs[e.Name()] = d
	}
	rep.Records = len(docs)
	ids := make([]string, 0, len(docs))
	for id := range docs {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	base := ticket.New(identityFor(projectHome, ""))
	for _, id := range ids {
		home := filepath.Join(dir, id)
		writes, notes := migrateWrites(home, docs[id])
		writes = effective(docs[id], writes)
		if len(writes) == 0 && len(notes) == 0 {
			continue
		}
		mt := migrateTicket{ID: id, Notes: notes, Fields: []string{}}
		for _, w := range writes {
			mt.Fields = append(mt.Fields, w.Path)
			rep.FieldCounts[w.Path]++
		}
		if len(writes) > 0 {
			rep.Changed++
			if dry {
				// Apply in memory so the relation pass sees the new edges.
				for _, w := range writes {
					_ = setJSON(docs[id], w.Path, w.Value)
				}
			} else {
				if _, err := fieldsApply(peerStore(base, id), writes, actor); err != nil {
					return rep, fmt.Errorf("%s: %w", id, err)
				}
				docs[id] = ticket.ReadDoc(filepath.Join(home, "index.json"))
			}
		}
		rep.Tickets = append(rep.Tickets, mt)
	}
	rep.Relations = nonNil(planRepairsDocs(docs, nil))
	if !dry {
		if _, err := applyRepairs(base, rep.Relations, actor); err != nil {
			return rep, err
		}
	}
	return rep, nil
}
