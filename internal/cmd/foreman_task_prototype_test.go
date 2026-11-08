package cmd

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/2found/2build/internal/foreman"
)

func TestPrototypeSectionListsOwnedStatesWithLinks(t *testing.T) {
	home := t.TempDir()
	parent := filepath.Join(home, "tickets", "bs-parent")
	write := func(path, body string) {
		t.Helper()
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	design := filepath.Join(parent, "design.md")
	proto := filepath.Join(parent, "prototype.html")
	write(design, "# Design\n`#p2-other` in §1\n\n## rail\n- **Screens:** `#p1-rail`, `#p1-narrow`, `#p1-gone`, `#p1-rail`.\n\n## queue\n- **Screens:** `#p2-queue`.\n")
	write(proto, `<script>add("p1","p1-rail");add("p1",'p1-narrow');add("p2","p2-queue")</script>`)
	ticketDoc := func(id, body string) { write(filepath.Join(home, "tickets", id, "index.json"), body) }
	ticketDoc("bs-rail", `{"origin":{"design_doc":"`+design+`#rail"}}`)
	ticketDoc("bs-queue", `{"origin":{"design_doc":"`+design+`#queue"}}`)
	ticketDoc("bs-queue-fix", `{"origin":{"design_doc":"`+design+`#queue"}}`)
	ticketDoc("bs-phase", `{"children":["bs-rail","bs-queue","bs-queue-fix"]}`)
	ticketDoc("bs-api", `{"origin":{"design_doc":null}}`)

	own := prototypeSection(foreman.Task{Ticket: "bs-rail", Kind: "ticket", ProjectHome: home})
	for _, want := range []string{
		"## Prototype (added by bbs)",
		"- `#p1-rail` — file://" + proto + "#p1-rail\n",
		"- `#p1-narrow` — file://" + proto + "#p1-narrow\n",
		"- `#p1-gone` — not in " + proto + ": ask the lead",
	} {
		if !strings.Contains(own, want) {
			t.Fatalf("ticket section lacks %q:\n%s", want, own)
		}
	}
	if strings.Count(own, "#p1-rail`") != 1 || strings.Contains(own, "p2-") {
		t.Fatalf("ticket section must list its own brief's states once each:\n%s", own)
	}

	phase := prototypeSection(foreman.Task{Ticket: "bs-phase", Kind: "phase-qa", ProjectHome: home})
	if !strings.Contains(phase, "#p1-rail") || strings.Count(phase, "- `#p2-queue`") != 1 {
		t.Fatalf("phase section must union its children's briefs, each brief once:\n%s", phase)
	}

	for _, task := range []foreman.Task{
		{Ticket: "bs-phase", Kind: "release", ProjectHome: home},
		{Ticket: "bs-api", Kind: "ticket", ProjectHome: home},
		{Ticket: "bs-rail", Kind: "ticket"},
	} {
		if got := prototypeSection(task); got != "" {
			t.Fatalf("%+v: want no section, got:\n%s", task, got)
		}
	}
}
