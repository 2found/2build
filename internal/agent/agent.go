// Package agent describes coding-agent CLIs and detects the current harness.
//
// A profile is data, not a template language: the CLIs differ in the binary
// name, the flag that turns off permission prompts, how (or whether) a
// conversation can be given a durable handle, and how they namespace babysit's
// skills. Everything else a spawn needs — the prompt, the Orca terminal around
// it — is identical.
//
// Two of those axes are where the registry stopped being a pure widening
// exercise, and both are load-bearing:
//
//   - Not every agent can mint a session id. claude and grok take
//     `--session-id <uuid>`; omp and codex have no such flag. An agent without
//     one must never render a flagless command line, and must not have a uuid
//     recorded against it that it has never heard of — a later resume would
//     hand it an id it cannot find. See MintsSessionID and SessionDir.
//   - Not every agent namespaces skills the same way. claude and grok read
//     babysit's plugin manifest and expose `bbs:autopilot`; omp finds skills
//     through `skills.customDirectories`, which is a flat list, so the same
//     skill is bare `autopilot` there. Codex keeps the `bbs:` namespace but
//     invokes skills with `$` instead of `/`. See SkillSigil, SkillPrefix and
//     SkillRef.
//
// New dispatches use explicit launch intent or Orca discovery. A recovered
// session always uses the agent recorded when it was created.
package agent

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
)

// Default is the legacy agent for records written before agent selection, and
// the fallback when no current harness can be detected.
const Default = "claude"

// Profile is one coding-agent CLI's spawn shape.
type Profile struct {
	// Name identifies the selected or recorded agent.
	Name string
	// Bin is the executable looked up on PATH.
	Bin string
	// Provider, Model and Effort are effective launch selectors. New worker
	// routes provide model/effort explicitly; provider is retained for recovery.
	Provider string
	Model    string
	Effort   string
	// Yolo is the flag that stops the agent asking for tool approval. Workers
	// and autonomous foremen run unattended in Orca terminals, so without it a
	// multi-day run stalls on the first mutation. Skill policy still fences
	// money, auth, irreversible data, and unauthorized finish actions.
	Yolo string
	// Session is the flag binding a NEW conversation to a uuid we minted, and
	// Resume re-opens one by that uuid. claude and grok happen to spell these
	// the same; they are fields rather than constants so an agent that spells
	// them differently stays a registry entry instead of a code path. Empty
	// Session means the agent cannot be told which conversation to start —
	// see MintsSessionID, and never render Session unset.
	Session string
	// Resume re-opens a conversation by the token recorded for it. It is not
	// necessarily a flag: codex spells it as the subcommand `codex resume
	// <id>`, which renders identically because the token follows the word.
	Resume string
	// SessionDir is the flag pointing an agent at a private conversation store
	// (omp: --session-dir). It is the weaker durable handle for an agent that
	// cannot mint an id: give one foreman its own directory and "the most
	// recent conversation in there" is unambiguously that foreman's, which is
	// the property Continue alone does not have in a shared checkout.
	SessionDir string
	// Continue re-opens the most recent conversation without naming it (omp:
	// --continue). It is only trustworthy when paired with SessionDir. A shared
	// repo may host several foremen, so an unscoped "last" is never safe.
	Continue string
	// SkillPrefix is how this agent namespaces babysit's skills in a prompt.
	// "bbs:" for agents that read the plugin manifest; "" for agents that
	// discover skills through a flat directory list and expose them bare.
	// Getting this wrong is silent: the agent starts fine and then resolves
	// the prompt to no skill at all.
	SkillSigil  string
	SkillPrefix string
	// Install is the hint printed when Bin is not on PATH. It names what to
	// install, and for agents with their own plugin store, what else they need
	// before a babysit skill prompt resolves.
	Install string
	// TrustFile, when non-empty, is a path under $HOME recording the directories
	// the agent has been told to trust. Agents that keep one refuse to touch an
	// unlisted directory until a human answers a prompt, which is fatal for an
	// unattended worker: the pane sits on a question nobody reads. Empty means
	// the agent has no such gate.
	TrustFile string
	// TrustHint says how to grant that trust, named in the preflight failure.
	TrustHint string
}

// profiles holds the supported CLI protocols. Provider/model/effort translation
// lives in settings.go; model identifiers never belong in this registry.
var profiles = map[string]Profile{
	"claude": {
		Name: "claude", Bin: "claude",
		Yolo:    "--dangerously-skip-permissions",
		Session: "--session-id", Resume: "--resume", Continue: "--continue",
		SkillSigil: "/", SkillPrefix: "bbs:",
		Install: "install Claude Code: https://claude.com/product/claude-code",
		// --dangerously-skip-permissions answers the *tool* prompts, not the
		// folder-trust dialog: a first run in a directory Claude Code has not
		// been trusted in stops on "Is this a project you trust?" with no log
		// line, no verdict and a checkpoint that never advances — the quietest
		// way an unattended worker can die.
		TrustFile: ".claude.json",
		TrustHint: "run `claude` there once and accept the trust prompt, " +
			"or dispatch the worker from a directory you have already trusted",
	},
	"grok": {
		Name: "grok", Bin: "grok",
		Yolo:    "--always-approve",
		Session: "--session-id", Resume: "--resume",
		SkillSigil: "/", SkillPrefix: "bbs:",
		// The second half is the failure this hint exists to prevent: grok finds
		// babysit's skills in the babysit repo itself (they are project skills
		// there) and nowhere else, so a worker dispatched in a product repo
		// comes up fine and then cannot resolve its own prompt.
		Install: "install grok, then give it babysit's skills: " +
			"grok plugin install https://github.com/lohi-ai/babysit " +
			"(grok has its own plugin store — without that install, /bbs:autopilot is not a skill grok can see)",
		// grok's directory trust is separate from its permission mode: with
		// `permission_mode = "always-approve"` already set, a first run in an
		// unlisted directory still stops on "Do you trust the contents of this
		// directory?" and --always-approve does not answer it.
		TrustFile: ".grok/trusted_folders.toml",
		TrustHint: "run `grok` there once and answer the trust prompt, " +
			"or add a `[folders.\"<dir>\"]` stanza with `trusted = true`",
	},
	"omp": {
		Name: "omp", Bin: "omp",
		Yolo: "--auto-approve",
		// No --session-id: omp resumes by id-prefix or path only, so a uuid we
		// minted would name a conversation it has never heard of. --session-dir
		// is the handle that does work — one directory per foreman makes
		// --continue unambiguous. Verified by round-trip on omp v18.0.6.
		Session: "", Resume: "--resume",
		SessionDir: "--session-dir", Continue: "--continue",
		// omp discovers Claude *user* and *project* skills but not Claude
		// *plugin* skills, and skills reached through customDirectories are a
		// FLAT list — they come out bare, not namespaced. So a `/bbs:autopilot`
		// prompt resolves to nothing here while `/autopilot` works.
		SkillSigil: "/", SkillPrefix: "",
		// `omp plugin install <git-url>` looks like the fix and is not: it is an
		// npm-shaped installer and fails with "package.json not found" on a
		// Claude plugin repo (--dry-run reports success anyway, which is what
		// makes it a trap). customDirectories is the verified fix, and it points
		// at the marketplace checkout rather than plugins/cache/<version>
		// because that path is stable across upgrades.
		Install: "install omp, then point it at babysit's skills: " +
			`omp config set skills.customDirectories '["$HOME/.claude/plugins/marketplaces/babysit/.claude/skills"]' ` +
			"(omp does not scan ~/.claude/plugins, so without this the worker comes up and " +
			"then cannot resolve its own prompt; note omp exposes them bare — /autopilot, not /bbs:autopilot)",
	},
	"codex": {
		Name: "codex", Bin: "codex",
		Yolo: "--dangerously-bypass-approvals-and-sandbox",
		// codex has no mint flag either, and its resume is a SUBCOMMAND taking a
		// positional id (`codex resume <id>`) rather than a flag — which renders
		// identically, because the token follows the word either way.
		Session: "", Resume: "resume", Continue: "",
		SkillSigil: "$", SkillPrefix: "bbs:",
		Install: "install codex: https://developers.openai.com/codex/cli, then install babysit: " +
			"codex plugin marketplace add lohi-ai/babysit && codex plugin add bbs@babysit",
	},
	"cursor": {
		Name: "cursor", Bin: "cursor-agent", Yolo: "--yolo",
		Resume: "--resume", SkillSigil: "/", SkillPrefix: "",
		Install: "install Cursor CLI: https://cursor.com/docs/cli/installation; make babysit's skills available in .cursor/skills or .agents/skills",
	},
}

// Names lists the registered agents, sorted, for error messages and docs.
func Names() []string {
	out := make([]string, 0, len(profiles))
	for n := range profiles {
		out = append(out, n)
	}
	sort.Strings(out)
	return out
}

// ByName maps an explicit or recorded agent identifier without consulting
// global settings. Callers provide any launch route separately.
func ByName(name string) (Profile, error) {
	name = Normalize(name)
	p, ok := profiles[name]
	if !ok {
		return Profile{}, fmt.Errorf("unknown agent %q — known agents: %s",
			name, strings.Join(Names(), ", "))
	}
	return p, nil
}

// Preflight reports whether this agent can actually be spawned. It runs before
// the terminal is created, because Orca happily opens a terminal on a command
// that does not exist: the human gets a pane containing "command not found"
// and a session that never reports, which reads as a hung ticket rather than a
// missing binary.
func (p Profile) Preflight() error {
	if _, err := exec.LookPath(p.Bin); err != nil {
		return fmt.Errorf("agent %q needs %q on PATH — %s", p.Name, p.Bin, p.Install)
	}
	return nil
}

// PreflightDir reports whether this agent will start in dir without first
// asking a question no unattended worker can answer. It is the second half of
// Preflight: a binary on PATH still hangs on the trust prompt, and a hung pane
// is the failure that costs a whole overnight batch.
//
// Trust is checked for the exact directory only. A trusted parent is not read as
// covering a child: guessing permissively re-creates the hang this exists to
// prevent, while guessing strictly costs one refusal that names a one-time fix.
func (p Profile) PreflightDir(dir string) error {
	if p.TrustFile == "" || dir == "" {
		return nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return nil // cannot locate the record; let the agent speak for itself
	}
	b, err := os.ReadFile(filepath.Join(home, p.TrustFile))
	if err != nil {
		// No trust file at all means the agent has never run anywhere, which is
		// exactly the state that hangs — report it rather than hoping.
		return p.untrusted(dir)
	}
	// grok resolves symlinks before recording, so compare on the physical path
	// (on macOS /tmp and /var are symlinked, and a worktree may be too).
	if real, err := filepath.EvalSymlinks(dir); err == nil {
		dir = real
	}
	trusted := trustedIn
	if strings.HasSuffix(p.TrustFile, ".json") {
		trusted = trustedInClaudeJSON
	}
	if trusted(string(b), dir) {
		return nil
	}
	return p.untrusted(dir)
}

func (p Profile) untrusted(dir string) error {
	return fmt.Errorf("agent %q has not been told to trust %s — it would stop there on a "+
		"trust prompt that no unattended worker can answer (%s is separate from %s). Fix: %s",
		p.Name, dir, p.TrustFile, p.Yolo, p.TrustHint)
}

// trustedIn scans the trust file for dir's stanza. The file is machine-written
// TOML in a fixed shape, so it is scanned rather than parsed — a TOML dependency
// for one lookup of one key would cost more than it explains.
func trustedIn(body, dir string) bool {
	header := `[folders."` + dir + `"]`
	lines := strings.Split(body, "\n")
	for i, ln := range lines {
		if strings.TrimSpace(ln) != header {
			continue
		}
		// Read this stanza's keys, stopping at the next table header.
		for _, kv := range lines[i+1:] {
			kv = strings.TrimSpace(kv)
			if strings.HasPrefix(kv, "[") {
				break
			}
			if strings.HasPrefix(kv, "trusted") {
				return strings.HasSuffix(kv, "true")
			}
		}
		return false
	}
	return false
}

// trustedInClaudeJSON reads ~/.claude.json, where Claude Code records one
// entry per directory it has opened. The entry existing is not the answer:
// it is written on first sight and the flag only flips once a human accepts
// the dialog, so most recorded projects are untrusted. Read the flag.
func trustedInClaudeJSON(body, dir string) bool {
	var doc struct {
		Projects map[string]struct {
			HasTrustDialogAccepted bool `json:"hasTrustDialogAccepted"`
		} `json:"projects"`
	}
	if err := json.Unmarshal([]byte(body), &doc); err != nil {
		return false
	}
	return doc.Projects[dir].HasTrustDialogAccepted
}

// StartupCommand renders the native CLI launch without task input, so Orca can
// inject its authoritative Dispatch after attaching supervision to the terminal.
func (p Profile) StartupCommand() string {
	return p.launchCommand()
}

// WorkerCommand renders the shell command line that runs one worker on the
// given prompt, e.g. `/bbs:autopilot ship the settings page`.
func (p Profile) WorkerCommand(prompt string) string {
	return p.launchCommand() + " " + shellQuote(prompt)
}

// MintsSessionID reports whether a NEW conversation can be bound to a handle
// this side chooses. When it is false the caller must not mint a uuid and
// record it: the agent has never heard of that id and the next resume would
// hand it one it cannot find. SessionToken says what to record instead.
func (p Profile) MintsSessionID() bool { return p.Session != "" }

// CanResume reports whether session identifies exactly one conversation.
// Falling back to a repo-wide "most recent" chat can attach one foreman to
// another foreman's goal, so agents without an id or private store cold-start
// and recover from the project's durable state instead.
func (p Profile) CanResume(session string) bool {
	return session != "" && (p.MintsSessionID() || p.SessionDir != "")
}

// SessionToken is the durable handle to record for a foreman's conversation,
// given a freshly minted uuid and a private directory this foreman may own.
// Three shapes, strongest first:
//
//	uuid  — the agent takes --session-id (claude, grok)
//	dir   — the agent only has a private session store (omp)
//	""    — neither; foreman recovery cold-starts from durable project state
//
// The caller supplies both candidates rather than this deciding how to build a
// path, so the directory stays the caller's layout concern.
func (p Profile) SessionToken(uuid, dir string) string {
	switch {
	case p.MintsSessionID():
		return uuid
	case p.SessionDir != "":
		return dir
	}
	return ""
}

// NewSessionCommand starts a fresh conversation carrying the durable handle
// SessionToken chose, and ResumeCommand re-opens it. Foremen are autonomous,
// so both shapes include the profile's unattended approval flag.
func (p Profile) NewSessionCommand(session, prompt string) string {
	return p.launchCommand() + p.sessionArgs(session, false) + " " + shellQuote(prompt)
}

func (p Profile) ResumeCommand(session, prompt string) string {
	return p.launchCommand() + p.sessionArgs(session, true) + " " + shellQuote(prompt)
}

// sessionArgs renders the session half of a foreman command line, and its one
// hard rule is that it never emits a flag with nothing after it. An agent with
// no Session used to render `omp  '<prompt>'` — two spaces where a uuid should
// have been — which starts a conversation nobody can ever find again.
func (p Profile) sessionArgs(session string, resume bool) string {
	switch {
	case p.MintsSessionID() && session != "":
		flag := p.Session
		if resume {
			flag = p.Resume
		}
		return " " + flag + " " + session
	case p.SessionDir != "" && session != "":
		// The directory is a path we built, but it still reaches a shell.
		out := " " + p.SessionDir + " " + shellQuote(session)
		if resume && p.Continue != "" {
			out += " " + p.Continue
		}
		return out
	case resume && p.Continue != "":
		return " " + p.Continue
	}
	return ""
}

// SkillRef renders a babysit skill invocation the way THIS agent resolves it —
// `/bbs:autopilot` in Claude Code and grok, `/autopilot` in omp/Cursor's flat skill
// list, and `$bbs:autopilot` in Codex. Every prompt naming a skill must go
// through here; a hard-coded sigil or prefix is the failure that comes up fine
// and then resolves to nothing.
func (p Profile) SkillRef(skill string) string {
	return p.SkillSigil + p.SkillPrefix + skill
}

// shellQuote wraps s in single quotes, ending and reopening the quoted run
// around each embedded single quote — the only escape that is safe inside POSIX
// single quotes, where backslash is literal. It matters because a free-text
// requirement reaches `orca terminal create --command "<this>"` and is parsed
// by a shell: an unquoted apostrophe in "don't break checkout" would truncate
// the requirement at best and split the command line at worst.
func shellQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
}
