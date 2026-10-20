package cmd

import (
	"fmt"
	"os"
	"strings"

	"github.com/2found/2build/internal/identity"
	"github.com/spf13/cobra"
)

// newTicketCmd is the full Go port of the retired bbs-ticket.bash script as
// `bbs ticket`; a `bbs-ticket` argv0 compat alias dispatches to it.
//
// Every subcommand now runs natively: the identity core (resolve/verdicts/
// session/board), the index.json state-accessors (env/get/set-status/set-phase/
// set-parent/add-child/add-relation/remove-relation/set-sibling/set-pointer/
// ensure-size/append-history), the file-only manifest.yaml ops (init/
// get-manifest/set-branch), the git-mutating base-ops family (refresh/surface/
// serve/land), `ensure`, and path/list/reconcile. A byte-identical frozen copy
// of the retired script survives at tests/fixtures/bbs-ticket.reference as the
// differential-harness oracle.
//
// Flag parsing is disabled and each subcommand hand-parses its argv, because
// the bash original hand-parses too and its quirks are part of the contract
// (resolve silently ignores unknown args; set-verdict rejects them with exit 2).
func newTicketCmd() *cobra.Command {
	return &cobra.Command{
		Use:                "ticket",
		Short:              "ticket identity (resolve/board/verdicts/sessions)",
		DisableFlagParsing: true,
		Args:               cobra.ArbitraryArgs,
		RunE: func(_ *cobra.Command, args []string) error {
			if len(args) == 0 {
				ticketUsage()
				os.Exit(2)
			}
			// The bash original hand-parsed argv, so a subcommand ignored an
			// unknown flag and ran anyway: `ticket ensure --help` created a
			// ticket, and `ticket approval --help` published an approval. Help
			// must be a read-only query, decided before any verb dispatches.
			for _, arg := range args[1:] {
				if arg == "-h" || arg == "--help" {
					ticketUsage()
					os.Exit(0)
				}
			}
			switch args[0] {
			case "-h", "--help", "help":
				ticketUsage()
				os.Exit(0)
			case "resolve":
				runResolve(args[1:])
			case "verdict-status":
				runVerdictStatus(args[1:])
			case "set-verdict":
				runSetVerdict(args[1:])
			case "session":
				runSession(args[1:])
			case "board":
				runBoard(args[1:])
			case "dag":
				runDAG(args[1:])
			case "env":
				runTicketEnv()
			case "get":
				runGet(args[1:])
			case "set-status":
				runSetStatus(args[1:])
			case "set-phase":
				runSetPhase(args[1:])
			case "set-parent":
				runSetParent(args[1:])
			case "add-child":
				runAddChild(args[1:])
			case "add-relation":
				runAddRelation(args[1:])
			case "remove-relation":
				runRemoveRelation(args[1:])
			case "assign":
				runAssign(args[1:])
			case "claim":
				runClaim(args[1:])
			case "pause":
				runPause(args[1:])
			case "cancel":
				runCancel(args[1:])
			case "resume":
				runResume()
			case "restore":
				runRestore()
			case "approval":
				runApproval(args[1:])
			case "set-sibling":
				runSetSibling(args[1:])
			case "set-pointer":
				runSetPointer(args[1:])
			case "ensure-size":
				runEnsureSize()
			case "append-history":
				runAppendHistory(args[1:])
			case "init":
				runInit(args[1:])
			case "get-manifest":
				runGetManifest(args[1:])
			case "set-branch":
				runSetBranch(args[1:])
			case "ensure":
				runEnsure(args[1:])
			case "clear":
				runClear(args[1:])
			case "surface":
				runSurface(args[1:])
			case "adopt":
				runAdopt(args[1:])
			case "add-handoff":
				runAddHandoff(args[1:])
			case "set-review":
				runSetReview(args[1:])
			case "set-evidence":
				runSetEvidence(args[1:])
			case "evidence-status":
				runEvidenceStatus(args[1:])
			case "readiness":
				runReadiness(args[1:])
			case "path":
				runPath(args[1:])
			case "list":
				runList(args[1:])
			case "reconcile":
				runReconcile(args[1:])
			case "lint":
				runTicketLint(args[1:])
			case "worktree-remove":
				runWorktreeRemove(args[1:])
			default:
				fmt.Fprintf(os.Stderr, "unknown subcommand: %s\n", args[0])
				os.Exit(2)
			}
			return nil
		},
	}
}

// ticketUsage prints the subcommand listing (tests/fixtures/bbs-ticket.reference:170-237) to stderr.
// help/-h/--help exit 0 after this; empty and unknown subcommands exit 2.
func ticketUsage() {
	fmt.Fprint(os.Stderr, ticketUsageText)
}

const ticketUsageText = `usage: bbs-ticket <subcommand> [args...]

Subcommands:
  ensure            idempotent: no-op when a ticket resolves, else create + init
                    mode trunk|worktree (default trunk): trunk = no cut;
                    worktree = cut .babysit/worktrees/<ticket>_<slug> from
                    local base (--from-ref REF) and print WORKTREE=<path>
  clear --all       move every active ticket record under ~/.babysit/projects
                    to ~/.babysit/trash; leaves config, sessions, analytics,
                    branches, and worktrees untouched
  init              initialize index.json + manifest.yaml for the current ticket
  resolve [--explain]   print ticket id (env → manifest cwd-match → branch);
                        exit 0 resolved, 1 no resolution, 2 conflict-BLOCKED
  get-manifest <ticket> emit manifest.yaml as JSON
  set-branch <ticket> <repo> <branch>
                        rewrite manifest.yaml — update branch on the named repo row
  get <path>        print a field (dotted path)
  set-status <s>    set ticket status
  set-phase <s>     set current owning skill
  set-parent <t>    set child-side parent link
  add-child <t>     append child id to current ticket's children
  add-relation <type> <target>
                    append blocks|blocked_by|related or set duplicate_of
  remove-relation <type> <target>
                    remove blocks|blocked_by|related entry or matching duplicate_of
  assign <foreman-id>|--none    set the owning foreman (assignee)
  claim <foreman-id>             atomically claim an unowned project; same owner is idempotent
  pause [--note M]  human override: stop dispatch, keep status (reversible)
  cancel [--note M] human override: drop from dispatch, keep status (reversible)
  resume            clear a pause      restore   clear a cancel
  approval publish [--kind K] [--note M]   open a plan/prototype decision on the dashboard
  approval status                          print {none|pending|approved|redirected|dropped}
  approval resolve --action approve|redirect|drop [--note M]
  approval await [--interval S] [--reminder-min M]
                    block until the human decides; prints the outcome. No timeout —
                    it reminds once, it never guesses.
  set-sibling --role R --repo REPO --ticket T
  set-pointer <key> <value>
  ensure-size                   resolve ticket_size (XS|S|M|L); estimate from diff if unset
  add-handoff --skill S --status STATUS [--body MD | --body-file FILE]
  set-verdict --skill S [--body MD | --body-file FILE]
  verdict-status --skill S       print {none|DONE|DONE_WITH_CONCERNS|BLOCKED|NEEDS_CONTEXT}
  set-review  --skill S [--body MD | --body-file FILE]
  set-evidence --kind K (--json STR | --json-file FILE)   K ∈ verification|risk-gate|adversarial
  evidence-status --kind K       print {none|valid|malformed}
  readiness --json --action push|land|pr|review
                                 read-only typed gate freshness for the exact action
  append-history --event E [--actor A] [--extra-json JSON]

Only for runs inside a ticket worktree (a foreman batch). Workers never
commit; Foreman seals, integrates and lands one commit per phase:

  adopt <ticket>...              merge sealed tickets' snapshots into this
                                 worktree as uncommitted changes; exit 5 and
                                 CONFLICT=<file> lines on conflicts
  surface <verb>                 lease on a shared test resource:
      acquire [--ticket ID] [--ttl-min N]
                                 exclusive QA-session lease; stale (> ttl) is stolen
      release [--ticket ID] [--force]
                                 drop the lease; --force overrides ownership
      status                     lease owner, age, ttl

  board [--all] [--pr]           read-only ticket board: status, branch, qa/
                                 review verdicts, session, PR, surface lease
  dag [<ticket>] [--mermaid|--json]
                                 print a project DAG: a parent's declared
                                 children, layered into waves by
                                 relations.blocked_by/blocks. Bare = the ticket
                                 in scope when one resolves, else every DAG in
                                 the project; --mermaid emits a mermaid block
                                 for a session that renders it.
                                 Read-only.
  path <kind> [selectors] --read|--write   resolve a ticket file path (canonical → legacy)
  list <kind> [selectors]                  list ticket files of a kind
  reconcile [--ticket <id> | --all] [--dry-run] [--quiet]
                    advance index.json.status from observable filesystem state
  lint --mode discovery|enforce [--path DIR] [--manifest FILE]
                    flag inline $TH/… constructions inside bash fenced blocks
  worktree-remove <path>       git worktree remove with bounded retry — survives
                               transient NTFS open handles during close-out
  session <list|attach|end>      inspect/rehydrate ~/.babysit/sessions/
  env               print SLUG / BRANCH / TICKET / BABYSIT_PROJECT_HOME
                    (+ TICKET_HOME / INDEX when a ticket is in scope) for eval
`

// ─── shared helpers ──────────────────────────────────────────────────────

// needTicket mirrors bash need_ticket() (tests/fixtures/bbs-ticket.reference:262-266).
func needTicket(env identity.Env) {
	if env.Ticket == "" {
		fmt.Fprintf(os.Stderr, retarget("bbs-ticket: no ticket in scope (branch='%s'; set BBS_TICKET to override)\n"), env.Branch)
		os.Exit(2)
	}
}

// safePathComponent mirrors bash _safe_path_component() (tests/fixtures/bbs-ticket.reference:690-708)
// with _PATH_KIND=verdict: traversal exits 3, other validation exits 2.
func safePathComponent(kind, label, value string) string {
	if value == "" {
		fmt.Fprintf(os.Stderr, retarget("bbs-ticket path: %s: --%s is empty (try: bbs-ticket path)\n"), kind, label)
		os.Exit(2)
	}
	if strings.Contains(value, "/") || strings.Contains(value, "..") || strings.HasPrefix(value, "/") {
		fmt.Fprintf(os.Stderr, retarget("bbs-ticket path: %s: --%s '%s' rejected (path traversal)\n"), kind, label, value)
		os.Exit(3)
	}
	cleaned := keepChars(value, "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789._-")
	if cleaned != value {
		fmt.Fprintf(os.Stderr, retarget("bbs-ticket path: %s: --%s '%s' contains forbidden characters (allowed: a-zA-Z0-9._-)\n"), kind, label, value)
		os.Exit(2)
	}
	return value
}

func keepChars(s, allowed string) string {
	var b strings.Builder
	for _, r := range s {
		if strings.ContainsRune(allowed, r) {
			b.WriteRune(r)
		}
	}
	return b.String()
}
