package cmd

import (
	"path/filepath"
	"testing"
	"time"
)

// Waits are sized to the current agent's prompt cache: claude's one hour
// leaves the 540s foreground block alone, a five-minute cache shortens it to
// return inside the lifetime, and foreman_cache_ttl overrides both.
func TestBlockingWaitFitsThePromptCache(t *testing.T) {
	state := t.TempDir()
	t.Setenv("BABYSIT_STATE_DIR", state)
	t.Setenv("BABYSIT_CURRENT_AGENT", "claude")
	if got := blockingWaitSeconds(); got != 540 {
		t.Fatalf("claude wait = %d", got)
	}
	t.Setenv("BABYSIT_CURRENT_AGENT", "codex")
	if got := blockingWaitSeconds(); got != 240 {
		t.Fatalf("codex wait = %d", got)
	}
	if got := promptCacheTTL("no-such-agent"); got != 5*time.Minute {
		t.Fatalf("unknown agent ttl = %s", got)
	}
	write(t, filepath.Join(state, "config.yaml"), "foreman_cache_ttl: 3600\n")
	if got := blockingWaitSeconds(); got != 540 {
		t.Fatalf("overridden codex wait = %d", got)
	}
	write(t, filepath.Join(state, "config.yaml"), "foreman_cache_ttl: auto\n")
	if got := promptCacheTTL("claude"); got != time.Hour {
		t.Fatalf("auto claude ttl = %s", got)
	}
}
