package cmd

import (
	"time"

	"github.com/2found/2build/internal/agent"
	"github.com/2found/2build/internal/config"
)

// A session that resumes inside its provider's prompt-cache lifetime reads
// its whole context at a tenth of the input price; one that resumes after it
// rewrites the context at up to twice the price. Waits that end a turn's tool
// call, and the watcher's prompts to an idle Foreman, are sized against that
// lifetime so the resume lands warm.

// promptCacheTTL is the cache lifetime for an agent: config foreman_cache_ttl
// when set, else the agent profile's own. An unknown agent gets the common
// five-minute lifetime, the safe side for every wait sized from it.
func promptCacheTTL(agentName string) time.Duration {
	if n, err := config.ForemanCacheTTLSeconds(); err == nil && n > 0 {
		return time.Duration(n) * time.Second
	}
	if p, err := agent.ByName(agentName); err == nil {
		return time.Duration(p.PromptCacheTTL()) * time.Second
	}
	return agent.DefaultCacheTTL * time.Second
}

// blockingWaitSeconds is the default block for a worker's await or ask: 540s,
// shortened to return a minute inside the current agent's cache lifetime so
// the re-run that follows a timeout reads the cache rather than rewriting it.
func blockingWaitSeconds() int {
	ttl := int(promptCacheTTL(agent.Detect().Agent).Seconds()) - 60
	return max(30, min(540, ttl))
}
