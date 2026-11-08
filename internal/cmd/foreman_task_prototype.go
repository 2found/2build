package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/2found/2build/internal/foreman"
	"github.com/2found/2build/internal/ticket"
)

// briefStateRef matches a prototype state a ticket brief cites: `#p1-rail`.
var briefStateRef = regexp.MustCompile("`#([A-Za-z][A-Za-z0-9_-]*)`")

// prototypeSection lists the accepted prototype states a Task's ticket owns,
// each with a link that opens it, so the worker gets the right states from the
// accepted design instead of from the lead's retyping. A ticket owns the states
// its brief (origin.design_doc = <design.md>#<seed>) cites; a phase ticket owns
// those of its children. The prototype is the prototype.html beside design.md.
func prototypeSection(t foreman.Task) string {
	if t.ProjectHome == "" || t.Kind == "release" {
		return ""
	}
	tickets := filepath.Join(t.ProjectHome, "tickets")
	doc := ticket.ReadDoc(filepath.Join(tickets, t.Ticket, "index.json"))
	refs := []string{doc.Get("origin.design_doc")}
	if kids, ok := doc.Value("children").([]interface{}); ok {
		for _, k := range kids {
			if id, ok := k.(string); ok {
				refs = append(refs, ticket.ReadDoc(filepath.Join(tickets, id, "index.json")).Get("origin.design_doc"))
			}
		}
	}
	var b strings.Builder
	done := map[string]bool{}
	for _, ref := range refs {
		design, seed, ok := strings.Cut(ref, "#")
		if !ok || design == "" || seed == "" || done[ref] {
			continue
		}
		done[ref] = true
		ids := briefStates(design, seed)
		if len(ids) == 0 {
			continue
		}
		proto := filepath.Join(filepath.Dir(design), "prototype.html")
		src, err := os.ReadFile(proto)
		fmt.Fprintf(&b, "\nBrief `%s` owns:\n", ref)
		if err != nil {
			fmt.Fprintf(&b, "- prototype %s is unreadable (%v): ask the lead before building or checking UI.\n", proto, err)
			continue
		}
		for _, id := range ids {
			if strings.Contains(string(src), `"`+id+`"`) || strings.Contains(string(src), `'`+id+`'`) {
				fmt.Fprintf(&b, "- `#%s` — file://%s#%s\n", id, proto, id)
			} else {
				fmt.Fprintf(&b, "- `#%s` — not in %s: ask the lead (the brief cites a state the prototype lacks).\n", id, proto)
			}
		}
	}
	if b.Len() == 0 {
		return ""
	}
	return "\n\n## Prototype (added by bbs)\n" +
		"The accepted prototype is the standard: build and check against it, never edit it to match the build. " +
		"Each link opens the page scrolled to that state's element; capture it with the browse engine's element screenshot " +
		"(`screenshot \"#<id>\" <path>`) at the viewport you use for the build. Every state below is a required conformance case " +
		"for whoever QAs this ticket (qa § prototype conformance)." + b.String()
}

// briefStates returns, in order and once each, the state ids cited inside the
// `## <seed>` section of design.md.
func briefStates(design, seed string) []string {
	data, err := os.ReadFile(design)
	if err != nil {
		return nil
	}
	var ids []string
	seen := map[string]bool{}
	in := false
	for _, line := range strings.Split(string(data), "\n") {
		if strings.HasPrefix(line, "## ") {
			in = strings.TrimSpace(strings.TrimPrefix(line, "## ")) == seed
			continue
		}
		if !in {
			continue
		}
		for _, m := range briefStateRef.FindAllStringSubmatch(line, -1) {
			if !seen[m[1]] {
				seen[m[1]] = true
				ids = append(ids, m[1])
			}
		}
	}
	return ids
}
