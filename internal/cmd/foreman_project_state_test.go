package cmd

import (
	"os"
	"path/filepath"
	"testing"
)

// A record written before seal was removed still names its deferred gates in
// "waiver"; a project in flight must keep reading its tickets as built.
func TestReadsRecordedWorkWithLegacyWaiver(t *testing.T) {
	path := filepath.Join(t.TempDir(), "project-seal.json")
	legacy := `{"version":1,"ticket":"bs-a","at":"2026-10-31T19:09:42Z","inputs":"sha256:x","repos":[],"owners":[],` +
		`"waiver":{"gates":["qa","review-pr"],"reason":"S ticket","at":"2026-10-31T19:09:42Z","states":{"qa":"missing"},"phase_review":true}}`
	if err := os.WriteFile(path, []byte(legacy), 0o644); err != nil {
		t.Fatal(err)
	}
	var seal projectSeal
	if err := readProjectJSON(path, &seal); err != nil {
		t.Fatalf("legacy record refused: %v", err)
	}
	if seal.Waiver == nil || !seal.Waiver.PhaseReview {
		t.Fatalf("waiver not read: %+v", seal.Waiver)
	}
}
