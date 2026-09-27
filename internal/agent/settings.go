package agent

import (
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"

	"github.com/reallongnguyen/babysit/internal/config"
)

type Options struct {
	Agent, Provider, Model, Effort string
	Dir                            string
	// SkipValidate leaves resolved settings unchecked for callers whose
	// command never invokes the agent — an explicit `foreman spawn --command`
	// records the agent for later restarts but runs what the caller wrote.
	SkipValidate bool
}

// ResolveWith resolves independent role settings. Dir is accepted for callers
// that also use it as the launch directory; configuration itself is machine-wide.
func ResolveWith(key string, opts Options) (Profile, error) {
	if key != WorkerKey && key != ForemanKey {
		return Profile{}, fmt.Errorf("unknown agent role %q", key)
	}
	if opts.Dir != "" {
		if _, err := os.Stat(opts.Dir); err != nil {
			return Profile{}, fmt.Errorf("agent settings: %w", err)
		}
	}

	role := strings.TrimSuffix(key, "_agent")
	selectValue := func(field, flag string) (string, string) {
		if flag != "" {
			return flag, "--" + field
		}
		for _, env := range []string{"BABYSIT_" + strings.ToUpper(role+"_"+field), "BABYSIT_" + strings.ToUpper(field)} {
			if v := os.Getenv(env); v != "" {
				return v, env
			}
		}
		setting := role + "_" + field
		if v, _ := config.Get(setting); v != "" {
			return v, config.Path()
		}
		return "", "automatic detection"
	}
	name, source := selectValue("agent", opts.Agent)
	name = Normalize(name)
	if name == "" || name == "auto" {
		name = Detect().Agent
		if name == "unknown" {
			name = Default
			for _, n := range Names() {
				if path, _ := exec.LookPath(profiles[n].Bin); path != "" {
					name = n
					break
				}
			}
		}
	}
	p, err := ByName(name)
	if err != nil {
		return Profile{}, fmt.Errorf("%w (from %s)", err, source)
	}
	p.Provider, _ = selectValue("provider", opts.Provider)
	p.Model, _ = selectValue("model", opts.Model)
	p.Effort, _ = selectValue("effort", opts.Effort)
	if opts.SkipValidate {
		return p, nil
	}
	return p, p.ValidateSettings()
}

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
