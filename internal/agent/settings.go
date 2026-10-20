package agent

import (
	"fmt"
	"strconv"
	"strings"
)

// ValidateSettings rejects options a CLI cannot honor. Provider credentials and
// arbitrary model identifiers remain owned by that CLI, never by babysit.
func (p Profile) ValidateSettings() error {
	switch p.Name {
	case "claude":
		switch p.Provider {
		case "", "anthropic", "bedrock", "vertex", "foundry":
		default:
			return fmt.Errorf("agent claude: provider %q is unsupported; use anthropic, bedrock, vertex or foundry", p.Provider)
		}
	case "grok", "cursor":
		provider := "xai"
		if p.Name == "cursor" {
			provider = "cursor"
		}
		if p.Provider != "" && p.Provider != provider {
			return fmt.Errorf("agent %s cannot select provider %q; its native provider is %s", p.Name, p.Provider, provider)
		}
	case "omp":
		// omp resolves @-prefixed model names as role aliases, and only three
		// exist. An alias it does not know is not a soft miss: the launch prints
		// `Model "@x" not found`, falls back to "No model selected" and exits,
		// so an unattended worker dies before it reads its prompt. Catching it
		// here turns that silent death into one line the foreman can act on.
		if strings.HasPrefix(p.Model, "@") {
			switch strings.ToLower(p.Model) {
			case "@smol", "@slow", "@plan":
			default:
				return fmt.Errorf("agent omp has no model role %q; use @smol, @slow or @plan, or omit the model to use omp's default", p.Model)
			}
		}
	}
	if p.Name == "cursor" && p.Effort != "" {
		return fmt.Errorf("agent cursor does not expose an effort flag; select a model with the desired reasoning setting")
	}
	return nil
}

func (p Profile) launchCommand() string {
	command := p.Bin + " " + p.Yolo
	if p.Provider != "" {
		switch p.Name {
		case "omp":
			command += " --provider " + shellQuote(p.Provider)
		case "codex":
			command += " -c " + shellQuote("model_provider="+strconv.Quote(p.Provider))
		case "claude":
			// Clear the other selectors so an inherited cloud setting cannot
			// override the provider explicitly selected for this launch.
			for _, provider := range []string{"bedrock", "vertex", "foundry"} {
				value := "0"
				if provider == p.Provider {
					value = "1"
				}
				command = "CLAUDE_CODE_USE_" + strings.ToUpper(provider) + "=" + value + " " + command
			}
		}
	}
	if p.Model != "" {
		command += " --model " + shellQuote(p.Model)
	}
	if p.Effort != "" {
		switch p.Name {
		case "codex":
			command += " -c " + shellQuote("model_reasoning_effort="+strconv.Quote(p.Effort))
		case "omp":
			command += " --thinking " + shellQuote(p.Effort)
		case "claude":
			command += " --effort " + shellQuote(p.Effort)
		case "grok":
			command += " --reasoning-effort " + shellQuote(p.Effort)
		}
	}
	return command
}
