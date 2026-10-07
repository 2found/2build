// Package ticket reads and writes the per-ticket Layout C directory that
// the retired bbs-ticket.bash owns: index.json, manifest.yaml, verdicts/, history.jsonl,
// and the session files under ~/.babysit/sessions. Only the pieces the ported
// (native) subcommands need live here — everything else still runs in bash.
package ticket

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/2found/2build/internal/identity"
)

// Store is a ticket's on-disk home, derived from the resolved identity.
type Store struct {
	Env identity.Env
}

func New(env identity.Env) *Store { return &Store{Env: env} }

// safe mirrors bash `safe()`: keep [a-zA-Z0-9._/:-], cap at 256 bytes.
func safe(s string) string {
	var b strings.Builder
	for _, r := range s {
		if r < 128 && (r == '.' || r == '_' || r == '/' || r == ':' || r == '-' ||
			(r >= '0' && r <= '9') || (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z')) {
			b.WriteRune(r)
		}
	}
	out := b.String()
	if len(out) > 256 {
		out = out[:256]
	}
	return out
}

// Home is $PROJECT_HOME/tickets/<safe ticket>.
func (s *Store) Home() string {
	return filepath.Join(s.Env.ProjectHome, "tickets", safe(s.Env.Ticket))
}

func (s *Store) IndexPath() string { return filepath.Join(s.Home(), "index.json") }

// EnsureDirs mirrors bash ensure_dirs() — best-effort, errors ignored.
func (s *Store) EnsureDirs() {
	h := s.Home()
	for _, d := range []string{"handoffs", "verdicts", "reviews", "evidence", "sub-tickets"} {
		_ = os.MkdirAll(filepath.Join(h, d), 0o755)
	}
}

// LoadForMutate reads index.json for a mutation, seeding defaults only when
// the file is absent — matching bash
// `[ -f "$F" ] || json_mutate "$F" ensure_defaults "$TICKET"`. The record
// layer owns this contract so no caller re-implements the defaults rule.
func (s *Store) LoadForMutate() Doc {
	p := s.IndexPath()
	doc := ReadDoc(p)
	if fi, err := os.Stat(p); err != nil || fi.IsDir() {
		doc.EnsureDefaults(s.Env.Ticket)
	}
	return doc
}

func (s *Store) VerdictPath(skill string) string {
	return filepath.Join(s.Home(), "verdicts", skill+".md")
}

// isoNow matches bash iso_now(): UTC, second precision, Z suffix.
func isoNow() string { return time.Now().UTC().Format("2006-01-02T15:04:05Z") }

// HistoryAppend appends an event through the same writer as enriched history.
func (s *Store) HistoryAppend(event, actor string) {
	s.HistoryAppendExtra(event, actor, "")
}

// Index is the slice of index.json the native subcommands read.
type Index struct {
	Status   string `json:"status"`
	Pointers struct {
		PR string `json:"pr"`
	} `json:"pointers"`
	Siblings []Sibling `json:"siblings"`
	Control  *Control  `json:"control"`
}

// Control is the human override axis (see internal/cmd/ticket_control.go). A
// pointer because `null` — no override — is the common case and has to stay
// distinguishable from a zero-valued record.
type Control struct {
	State       string `json:"state"`
	PriorStatus string `json:"prior_status"`
	Note        string `json:"note"`
	Actor       string `json:"actor"`
	At          string `json:"at"`
}

type Sibling struct {
	Role   string `json:"role"`
	Repo   string `json:"repo"`
	Ticket string `json:"ticket"`
}

// ReadIndex returns the typed view of the canonical record: a zero Index when
// the file is missing or malformed — matching bash json_read, which prints
// empty and exits 0 either way.
func ReadIndex(path string) Index {
	var idx Index
	b, err := os.ReadFile(path)
	if err != nil {
		return idx
	}
	_ = json.Unmarshal(b, &idx)
	return idx
}
