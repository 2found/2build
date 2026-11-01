package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/2found/2build/internal/ticket"
)

// Project scope is approved before runtime ticket IDs exist. Seed bindings
// live on children; children and dependency edges remain owned by the ticket DAG.
type projectContract struct {
	Version       int                `json:"version"`
	Audience      string             `json:"audience"`
	Outcome       string             `json:"outcome"`
	NonGoals      []string           `json:"non_goals"`
	Seeds         []string           `json:"seeds"`
	Criteria      []projectCriterion `json:"criteria"`
	FirstJourney  []string           `json:"first_journey"`
	ProductReview bool               `json:"product_review"`
	ArtifactsOnly bool               `json:"artifacts_only,omitempty"`
	Deadline      string             `json:"deadline,omitempty"`
	// Phases split delivery into ordered release increments. Absent keeps the
	// single integrate-and-deliver-at-the-end project.
	Phases []projectPhase `json:"phases,omitempty"`
}

type projectCriterion struct {
	ID     string   `json:"id"`
	Text   string   `json:"text"`
	Owner  string   `json:"owner"`
	Checks []string `json:"checks"`
}

type projectRepo struct {
	Canonical string `json:"canonical"`
	Branch    string `json:"branch"`
	Head      string `json:"head"`
	Base      string `json:"base"`
	BaseHead  string `json:"base_head"`
}

type projectSeal struct {
	Version   int               `json:"version"`
	Ticket    string            `json:"ticket"`
	At        string            `json:"at"`
	Inputs    string            `json:"inputs"`
	Repos     []projectRepo     `json:"repos"`
	Owners    []string          `json:"owners"`
	Artifacts map[string]string `json:"artifacts,omitempty"`
	// PhaseReview: the ticket's own review-pr did not pass (it was left to
	// the phase), so the phase commit carries a review check.
	PhaseReview bool `json:"phase_review,omitempty"`
	// Waiver is how a record written before seal was removed named its
	// deferred gates; read only, so a project in flight keeps its built tickets.
	Waiver *struct {
		Gates       []string          `json:"gates"`
		Reason      string            `json:"reason"`
		At          string            `json:"at"`
		States      map[string]string `json:"states"`
		PhaseReview bool              `json:"phase_review,omitempty"`
	} `json:"waiver,omitempty"`
}

type projectChild struct {
	ID        string           `json:"id"`
	Title     string           `json:"title"`
	Seed      string           `json:"seed"`
	State     string           `json:"state"`
	BlockedBy []string         `json:"blocked_by"`
	Repos     []projectRepo    `json:"repos"`
	Problems  []string         `json:"problems"`
	Progress  *projectProgress `json:"progress,omitempty"`
	// PhaseReview: its work is reviewed on the phase commit.
	PhaseReview bool `json:"phase_review,omitempty"`
	landed      bool
}

// A surface is the phase commit as checked out in its integration worktree
// when verification began; the receipt binds that exact commit.
type projectSurface struct {
	Dir      string `json:"dir"`
	Ref      string `json:"ref"`
	Head     string `json:"head"`
	BaseRef  string `json:"base_ref"`
	BaseHead string `json:"base_head"`
}

type projectCheck struct {
	Criterion string   `json:"criterion"`
	Kind      string   `json:"kind"`
	Command   []string `json:"command"`
	ExitCode  *int     `json:"exit_code"`
	Log       string   `json:"log"`
	Digest    string   `json:"digest,omitempty"`
	// CarriedFrom is the earlier attempt this passed check was carried from.
	CarriedFrom string `json:"carried_from,omitempty"`
}

type projectFinding struct {
	Severity  string `json:"severity"`
	Summary   string `json:"summary"`
	Criterion string `json:"criterion"`
}

type projectEvidence struct {
	State    string           `json:"state,omitempty"`
	Problems []string         `json:"problems,omitempty"`
	Version  int              `json:"version"`
	ID       string           `json:"id"`
	Kind     string           `json:"kind"`
	Subject  string           `json:"subject"`
	Producer string           `json:"producer"`
	Started  string           `json:"started"`
	Finished string           `json:"finished,omitempty"`
	Surfaces []projectSurface `json:"surfaces"`
	Checks   []projectCheck   `json:"checks"`
	Findings []projectFinding `json:"findings"`
	// A preview is only published with a recorded revision probe in Checks.
	Preview string `json:"preview,omitempty"`
	Runtime string `json:"runtime,omitempty"`
	// Phase scopes the receipt to one release increment of a phased project.
	Phase string `json:"phase,omitempty"`
	// Carried is set on a record built by `foreman evidence --carry`.
	Carried *projectCarry `json:"carried,omitempty"`
}

// projectCarry is the provenance of checks carried forward from earlier
// attempts of the same phase (a different, older subject).
type projectCarry struct {
	From        []string `json:"from"`
	FromSubject []string `json:"from_subjects"`
	Reason      string   `json:"reason"`
	Except      []string `json:"except,omitempty"`
}

type projectCoverage struct {
	projectCriterion
	State    string   `json:"state"`
	Evidence []string `json:"evidence"`
}

type projectSnapshot struct {
	Version     int                 `json:"version"`
	Ticket      string              `json:"ticket"`
	Observed    string              `json:"observed_at"`
	Subject     string              `json:"subject"`
	Contract    *projectContract    `json:"contract"`
	Approval    string              `json:"approval"`
	Children    []projectChild      `json:"children"`
	Coverage    []projectCoverage   `json:"coverage"`
	Evidence    []projectEvidence   `json:"evidence"`
	Dispatch    []string            `json:"dispatch_blockers"`
	Blockers    []string            `json:"blockers"`
	Ready       bool                `json:"ready"`
	Completion  string              `json:"completion"`
	Coordinator map[string]string   `json:"coordinator"`
	Delivery    []projectDelivery   `json:"delivery"`
	Phases      []projectPhaseState `json:"phases,omitempty"`
	Usage       skillUsage          `json:"provider_usage"`
}

func readProjectJSON(path string, into interface{}) error {
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()
	d := json.NewDecoder(f)
	d.DisallowUnknownFields()
	if err := d.Decode(into); err != nil {
		return fmt.Errorf("%s: %w", path, err)
	}
	return requireJSONEOF(d)
}

func projectContractPath(st *ticket.Store) string { return filepath.Join(st.Home(), "project.json") }

func validateProjectContract(c projectContract) error {
	if c.Version != 1 || strings.TrimSpace(c.Audience) == "" || strings.TrimSpace(c.Outcome) == "" || len(c.Seeds) == 0 || len(c.Criteria) == 0 || len(c.FirstJourney) == 0 {
		return fmt.Errorf("project contract needs version 1, audience, outcome, seeds, criteria and first_journey")
	}
	seeds, ids := map[string]bool{}, map[string]bool{}
	for _, seed := range c.Seeds {
		if !idRe.MatchString(seed) || seeds[seed] {
			return fmt.Errorf("invalid or duplicate seed %q", seed)
		}
		seeds[seed] = true
	}
	for _, ac := range c.Criteria {
		if !idRe.MatchString(ac.ID) || ids[ac.ID] || !seeds[ac.Owner] || strings.TrimSpace(ac.Text) == "" || len(ac.Checks) == 0 {
			return fmt.Errorf("criterion %q needs a unique ID, text, seed owner and checks", ac.ID)
		}
		ids[ac.ID] = true
		seen := map[string]bool{}
		for _, check := range ac.Checks {
			if !idRe.MatchString(check) || seen[check] {
				return fmt.Errorf("invalid or duplicate check for %s", ac.ID)
			}
			seen[check] = true
		}
	}
	for _, ac := range c.FirstJourney {
		if !ids[ac] {
			return fmt.Errorf("first_journey references unknown criterion %s", ac)
		}
	}
	if c.Deadline != "" {
		if _, err := parseProjectTime(c.Deadline); err != nil {
			return err
		}
	}
	return validateProjectPhases(c)
}

// projectChildOwners names the workers whose recorded gates built the
// ticket, so phase evidence is never produced by one of them.
func projectChildOwners(st *ticket.Store) []string {
	owners := []string{}
	cp, err := readCheckpointV2(st.Home(), st.Env.Ticket)
	if err != nil {
		return owners
	}
	accepted, _ := cp["accepted_evidence"].(map[string]interface{})
	for _, gate := range []string{"review-pr", "qa"} {
		id := stringValue(accepted[gate])
		if !idRe.MatchString(id) {
			continue
		}
		ev, err := readStrictObject(filepath.Join(st.Home(), "evidence", "verification", "attempts", id+".json"), true)
		if err != nil {
			continue
		}
		producer, _ := ev["producer"].(map[string]interface{})
		if owner := stringValue(producer["owner"]); owner != "" && !slices.Contains(owners, owner) {
			owners = append(owners, owner)
		}
	}
	return owners
}
