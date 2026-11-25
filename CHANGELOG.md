# Changelog

Older releases: [CHANGELOG-ARCHIVE.md](CHANGELOG-ARCHIVE.md).

## 1.141.0

- semantic-decision: the Cloudflare Clef provider is removed — the calling LLM judges in-session and `bbs semantic-decision --kind <kind> --input request.json --llm-answers answers.json` validates and records it in one call. `semantic_decision_provider`/`semantic_decision_model` config keys are gone.
- team: the lead session stays out of work context — integrate conflicts go to one foreground subagent, unexpected wakes get one read, and phase integration QA moves to `team/references/phase-qa.md`; `worker-routing-omp.md` added.
- autopilot: `implement` always runs in a `flash`-tier subagent (one continued across milestones); review-pr runs three subagents from medium up — correctness finder, cleanup finder, one batch verifier that also sweeps.
- setup-project: no longer seeds `CLAUDE_CODE_SUBAGENT_MODEL`/`subagentPromptCacheTtl`; removes the old seeded values.
- marketing-video: a cut with `voice.mode: none` masters to a silent AAC track instead of failing, and all-silent beats count as measured timing.
- skills: new shared `file-size.md` budget (800 lines/file); preamble gains a turns/tool-output cost note; token-economics doc updated.

## 1.140.2

- marketing-video: Micro Fish judge is now optional — without it `mvf next` and doctor run the cold-read pre-test as an `agent-panel` round; Micro Fish doctor rows warn instead of blocking.
- marketing-video: a Micro Fish run that dies on the provider (credits out, auth rejected, model refused) is reported as dead instead of resumed; the run falls back to the agent panel with the reason recorded in `evidence`.

## 1.140.1

- **marketing-video:** channel campaigns now live at `marketing/channels/<channel>/<slug>/` instead of `campaigns/<slug>/`; `mvf` still reads a legacy populated `campaigns/` folder, so existing channels keep working.
- **marketing-video:** delivery output flattens to `08-delivery/<sid>-<canvas>/<platform>/`; flow, status, library and freshness reads accept both layouts.
- **marketing-video:** `viewer.beside` fits 16:9 frames on the keyframe sheet (regression covered).
- Fixed a stale `test_benefit` assertion that no longer matched the TikTok browse instructions shipped in v1.137.0.

## 1.140.0 — 2026-11-19

### Added

- `marketing-video` — channel-scoped campaigns: `mvf init --channel/--series`
  and `channel init` scaffold a channel folder (AGENTS.md, docs/, pack/,
  brand/, metrics/) and campaigns now live under `channels/<C>/campaigns/<slug>`.
- `marketing-video` — brand font files: `campaign.json` `brand.fonts.<role>`
  accepts `{"family", "file", "weight"}` for a font HyperFrames does not
  bundle; the scaffold copies the file and emits its `@font-face`.
- `marketing-video` — campaign `fps` (24–60) renders and exports at that
  rate (`data-fps` on the composition, `-r` and GOP on encode) for looks
  drawn on frames such as anime on threes.
- `marketing-video` — review/production guidance: fix the class of a red
  item across every beat, check a word's time before moving a "late" shot,
  keep shared drawing code under `assets/kit/`, and re-anchor a draft to
  `MV.beat(id).words` after voicing.

## 1.139.1 — 2026-11-18

### Added

- `marketing-video` — animation craft guidance in `references/07-craft.md`:
  how to make drawn characters read as alive (pose-hold-pose on twos/threes
  at the matching `data-fps`, half-shut blinks, cycling talking mouths,
  breath and secondary sway, the squash/smear/shake take, one impact frame
  per shot), plus a stepped-motion frame-rate rule.
- `marketing-video` — production guidance in `references/07-production.md`:
  set line weight in placed px (divide widths by placement scale), give every
  expression a drawn mark beside the face, shift a style through the face
  first, judge motion with stacked side-by-side beats, and keep an error log
  beside the cast code.

### Fixed

- `ui-audit` — adds the missing filesystem-path note for its `../references/`
  links, which OMP resolves as `skill://` URIs and fails on.

## 1.139.0 — 2026-11-18

### Changed

- `ui-audit` — rewritten as a shorter standard: eight principles (the job is
  the unit, side-by-side comparison, backend as truth, measured not opined,
  persona knowledge only, real conditions, subtract first, fix the cause
  where it lives) replace the fixed procedure; evidence-gathering method is
  now the agent's choice. Its three reference files (checklists, helpers,
  report-template) are removed; measures live in the skill body.
- UI bar — three new rows: **J5** a blocker met mid-job (balance, sign-in,
  credential, quota) is fixed in place or as a detour that returns with
  input kept and refreshed state shown; **T5** words are a budget (titles
  ≤ 6 words, buttons ≤ 3 words, verb-first, no how-to sentences); **C6** a
  state change moves only what changed (element boxes compared before and
  after loading/empty/error/expanded/submitting). C1 now also requires the
  same kind of action to behave the same everywhere; P3 defers input-caused
  shifts to C6.
- `design-ui`, `qa` — jobs list the blockers they can hit and walk each fix
  back to the job's end (J5); changed screens are checked for state-change
  stability (C6) and label length (T5).
- `autopilot`, `team` — user-facing UI/UX work runs on the `pro` tier:
  `design-ui` and UI/UX `implement` milestones dispatch on the pro binding
  (`bbs team model` resolves it; Claude Code default `opus` + high effort).
  Claude's pro default moves to `opus` at **high** effort (was medium).

## 1.138.1 — 2026-11-17

### Fixed

- Soot pack: shared references move from `skills/references/` to `shared/`,
  because Soot Forge treats every folder under `skills/` as a skill and
  refused the pack. Relative paths in packed Markdown are rewritten to the
  packed layout, including paths in a skill's auxiliary files that were
  written relative to the skill folder.

## 1.138.0 — 2026-11-17

### Changed

- Soot pack `packs/2build` is now a Soot pack v2: `pack.json` carries only
  `name`, a `{vi, en}` description and keywords, and each skill is
  `skills/<name>/SKILL.md`, named as is (was `2build_<name>`, listed in a v1
  `skills[]` manifest with `api_version`). Shared references ship at
  `skills/references/`, mirroring `.claude/skills/`, so relative links resolve
  unchanged. A Soot runtime that reads only v1 packs refuses it; Soot Forge
  0.21.0 pins this layout.

## 1.137.0 — 2026-11-17

### Changed

- `browse` — plain Chromium (Chrome for Testing, via `agent-browser install`)
  is now the default engine; CloakBrowser stealth is opt-in, requested per
  task or after a site blocks the plain browser. New usage notes: `batch`
  JSON on stdin, `errors`/`console` on pages you built, cache-busting after
  an edit, and one `eval`/`fetch` for many pages.
- `marketing-video` — names the CloakBrowser engine wherever it calls
  `browse` (TikTok is blocked on plain Chromium), and `mvf top-videos` now
  prints `--restore mvf-research` on its `open` step so the research profile
  carries over between runs.

## 1.136.0 — 2026-11-17

### Added

- `marketing-video` — long-form YouTube (15–20 min) is a first-class target. A
  `youtube-long` video is judged as it is chosen: each `mvf judge` subject is
  the title and thumbnail card, the first 90 s word for word, then each chapter
  by its title with its `key_moment` lines verbatim and the rest condensed, so
  a 17-minute script fits one Micro Fish subject. The interview adds `click`
  (would the viewer click the card) and `left_at` (the chapter they would
  leave); the leader is ranked on watch share × click share. A cut over 5
  minutes gets a 1 fps overview brief (six sheets) plus `mvf viewer <proj>
  --chapter K` per-chapter briefs answered in parallel; an unanswered chapter
  WARNs. A beat may set `captions_y` (a frame share, or `[[seconds, share],
  …]` marks) to lift or hide captions part-way through. The direction stage
  splits long-form beats into chapters — at most four agents at once — and the
  production review folds each chapter's verdict back.
- `mvf judge` on a rate-limit stop now resumes where it left off —
  `mvf judge resume --round rN` — and `progress.log` records why each session
  failed.

### Changed

- `marketing-video` — YouTube hard limits follow account state: verified
  uploads allow 12 h / 256 GB; the 15-minute unverified cap moved from a hard
  limit to a WARN (`unverified_max_s`).
- Long cuts are read at 12 fps instead of 1 fps for viewer briefs: a 17-minute
  30 fps master otherwise held ~450 MB in memory.
- `mvf export` and `mvf lint` copy and check long-form files (chapter briefs,
  captions marks) alongside the existing script and timeline.

## 1.135.0 — 2026-11-17

### Added

- `marketing-video` — the library, what one video leaves for the next. Campaign
  folders were the only memory, so deleting one took its facts, captures,
  formats and every script with it. The library
  (`<repo>/.babysit/marketing-video/library/`, `$MVF_LIBRARY` moves it) keeps the
  product pack, each subject's material, saved formats, the production code and
  one history line per angle and script. `mvf library` lists it, `mvf library
  save` files a campaign into it, and `mvf library check <campaigns dir>` says
  whether those folders can be deleted. `mvf init --subject` starts from the
  product pack and that subject's material; `mvf export` saves into the library;
  `mvf approve` records the scripts, so `mvf overlap` and the research stage see
  campaigns whose folder is gone.
- `mvf judge --parallel` caps Micro Fish sessions at once; a run stopped by a
  provider rate limit resumes 2 at a time, and `progress.log` says why each
  session failed.

### Changed

- `marketing-video` formats now live in the library (`library/formats/<name>/`),
  so deleting the campaigns keeps them. A format saved before this release in
  `<campaigns>/_formats/<name>/` is still read, and `mvf library save` copies it
  forward.

### Fixed

- `bbs team watch` no longer wakes a held team lead that has no open batch. A
  lead waiting on a human spent a full reconciliation turn at every status
  interval to rediscover the same wait; the default nudge now names the exit —
  nothing changed and nothing running means the team is waiting on a human, so
  run `bbs team hold`, say so and end the turn. The nudge is a plain line
  carrying the skill prompt as its fallback instead of a skill invocation that
  re-reads the whole skill.
- `bbs team hold release` re-arms the watcher, which may have exited while every
  record it saw was held (an empty watch set is its terminal condition).

## 1.134.0 — 2026-11-17

### Changed

- `marketing-video` — subagent budget and parallelism rules from live campaign
  findings: gathering and capture agents run on the mid-tier model with call
  caps, product captures happen at script time instead of inside production,
  scene agents build in parallel (max four) against a frozen key frame and
  shared drawing code, and long production runs hand off via `HANDOFF.md`
  past ~150k tokens.
- `mvf next` — asks for the key frame even when every scene placeholder is
  already filled, so a cut built ahead of its look still settles that look
  beside the references.
- Pretest audience sizing — a `quick` run now draws 4-8 personas in total
  (personas run on the user's own AI tokens); `standard`/`deep` keep 48/96
  per video.
- `mvf viewer` review — stop accepting fixes once a round scores no higher
  than the last with no P0 open; write what keeps the cut from the next step.
- `mvf doctor` — checks `agent-browser` is runnable and reports its path for
  gathering briefs; `browse` skill reinstalls the link once when it fails.

## 1.133.0 — 2026-11-14

### Added

- `bbs changelog`: `check`, `add --notes <file>` and `rotate --keep N`
  subcommands for the growing changelog problem — `add` inserts a `## <version>`
  section below the title without rewriting the file, `rotate` moves older
  sections into `CHANGELOG-ARCHIVE.md`, and `check` reports size and rotation
  suggestions. Older releases of this repo now live in `CHANGELOG-ARCHIVE.md`
  (shipped inside the CLI archives).
- `release` skill now owns the changelog mechanics: notes file beside the
  release record doubles as the GitHub `--notes-file`, rotation is suggested
  (asked once on the developer channel, never blocking), and profile/code drift
  found during a release is corrected in the release commit.

### Changed

- `harness-audit` and `marketing-video` skills: reference fixes and updated
  campaign trends template.
- Ticket layout reference documents the release record location under
  `<git-common-dir>/bbs-release/`.

## 1.132.0 — 2026-11-14

### Added

- **`autopilot` new workflow**: in a checkout with no project,
  `/bbs:autopilot "<idea>"` (or no idea) interviews the owner, writes
  `docs/REQUIREMENT.md`, builds a prototype with one product checkpoint, runs
  `architect new`, scaffolds services with `bbs bootstrap`, sets up local
  infrastructure and a deploy config that provisions nothing, runs
  `setup-project` and `harness-audit`, passes review and QA gates, writes
  `docs/BACKLOG.md` of paste-ready autopilot lines and makes one skeleton
  commit (the only commit autopilot makes, and only on a repository it started
  from empty). Verdict `PROJECT_READY`.
- `architect`: a `new` mode for a product with no code yet.
- New shared reference `references/owner-language.md`: the standard for every
  stop a person reads; `autopilot`, `perf-audit`, `architect`, `setup-project`
  and `release` follow it.
- `marketing-video`: `mvf art` WARNs on more than two generated full-frame
  plate/background jobs (A12); a run of generated-plate scenes is a P0 in
  review.

### Changed

- `marketing-video`: every format is built in code (HTML, SVG, CSS, canvas,
  GSAP) around the real product's captures; generated art only supports a
  coded scene (texture, far layer, portrait, prop, thumbnail, reference). The
  story/pov formats and the anime preset draw the place and cast in code;
  `SCENE_GUIDE` and production notes no longer defer to the hyperframes skill.
- `perf-audit`: the report opens with a one-screen owner part (what people feel
  in seconds, why, numbered recommendations with cost, risk and the command to
  paste); routes, percentiles and ids move below a divider.
- `builder` ticks the cited backlog item and names the next one.
- `autopilot` verification: `limitations` is only for required coverage that
  could not run.
- README (five languages), docs, CLAUDE.md and llms.txt lead with the new
  entry.

## 1.131.0 — 2026-11-14

### Added

- **Phase sessions** (`bbs team task` / `team` skill): a phase that is one
  ticket now runs as a `--kind phase` Task — one Dispatch builds the ticket,
  records its work via the new `bbs team task built <task>` verb (the same
  readiness gate and work snapshot `task done` applies, without settling the
  Dispatch), then integrates, lands, reviews and QAs the phase in the same
  warm session. `task done` for a phase session is refused until the phase is
  landed and verified. See `team/references/phases.md` "Phase session".
- `scripts/foreman-cache-report.py`: recognizes Team-era transcripts (worker
  launch prompts, Orca workspace pool, `/bbs:team` invocations), groups
  workers by Task kind including `phase`, and counts the `team watch`
  keep-warm alongside the legacy `foreman watch`.

### Changed

- `autopilot`: the design approval is one in-session question — `design-ui`
  always runs as a subagent and the plan carries the UI spec + prototype
  before approval; no `/goal` block to paste for the design review. README
  (all five languages) and CLAUDE.md describe the new flow.
- `team` phases: merge a `none`-release phase into the next unless it earns
  its own boundary (named reasons only); fix review/QA/product-review
  findings in one batch per integrate-and-land round instead of one finding
  at a time; phase QA runs through the skill's QA executor subagent.

## 1.130.0 — 2026-11-13

### Added

- New shared reference `references/ui-bar.md` (and pack copy
  `packs/2build/shared/ui-bar.md`): the shared UI bar with row ids
  (jobs J1–…, ownership O1–…, components C1–…, data D1–…) that
  `ui-audit` scores against and `design-ui` designs to.
- `generate-image --limits` (image-gen): reports this machine's drawable
  pixel budget, side range and step as JSON, so callers (mvf art) size
  jobs from it. Streaming weight loads keep runs near 3.5 GiB up to a
  measured 1088×1920 on 16 GB; timeout raised to 3000 s.

### Changed

- `ui-audit` restructured around the UI bar: a product scope (the Map —
  side-by-side duplication, unfinishable jobs, complexity, text density,
  component/loading drift, UI speed, backend ground truth) joins the
  journey walk; scopes are `product` / `flow <id>` / `inventory` /
  `re-audit`; engine-truthful targets (WebKit audits in WebKit);
  production-shaped data (D1–D3) is required. Checklists, helpers,
  live-safety and the report template follow.
- `design-ui` adds "Fit the product before drawing": jobs → path first,
  one owner per fact/action, subtract in the same design, lifecycle
  states from code, named shared components; every changed screen shown
  loaded/loading/empty/failed/partly-failed on production-shaped data.
- `marketing-video`: context discipline for long campaigns (fresh agents
  for gathering, one agent per scene after your key frame, production in
  a fresh context via `mvf next`); judge rewrite (persona society +
  expert panel), art/sound/mix/flow checks upgraded, `mvf init --root`,
  hook may show the product working in the first seconds,
  picture-fills-the-frame rule; SCENE_GUIDE and pretest references
  updated.
- `qa`, `implement` and `references/handoff-contracts.md`: small fixes;
  pack `packs/2build` mirrors all of the above.

## 1.129.0 — 2026-11-12

### Added

- `bbs` help now scopes to the subcommand you mistyped (`usage_for`):
  `bbs autopilot <bad>`, `bbs team task <bad>` and `bbs ticket <bad>` print
  that command's usage instead of the top-level wall.
- New autopilot reference `references/team-assignment.md`: the contract a
  Team Dispatch assigns — plan → approval → implement → verification in one
  session, `XS`/`S` sizing shortcuts, resume and follow-up recovery.
- New shared reference `references/agent-operable.md`: defines when a
  release changed an agent-operable surface (action, status, public fact,
  API) and the tools/knowledge/eval handover bar for Soot.

### Changed

- `humanizer-vi` restructured: the skill body keeps invocation and staging;
  pattern details moved to `references/format-leftovers-and-reader.md`,
  `references/rhythm-and-inflation.md` and
  `references/staging-and-translationese.md` so the prompt stays small.
- `team` handover condensed into `references/delivery.md`; plan-approval
  points at the project-contract design checkpoint; ticket, phases,
  kickoff, execution, model-routing and product-review references updated
  for the team-assignment and agent-operable contracts.
- `autopilot` SKILL trimmed to its mode router; `release` SKILL gains the
  operating handover stage (never blocks; gaps are concerns); `qa`,
  `ui-audit`, `design-ui`, `implement`, `plan-draft` and `marketing-video`
  get the agent-operable / assignment hooks and small fixes
  (`marketing-video` flow and material handling).
- Pack `packs/2build` mirrors all of the above (25 skills; shared
  `agent-operable.md` added).

## 1.128.0 — 2026-11-10

### Added

- New `humanizer-vi` skill: the Vietnamese edition of blader/humanizer
  (MIT, adapted). It keeps the staging, inflation, formatting and leftover
  patterns, adds a translationese group (English idiom calques,
  `việc`/`sự`/`một cách` nominalization, `là` before adjectives, `được ...
  bởi` passives, `của bạn`/`hãy`), Vietnamese watch lists and examples, a
  register guide and EN → VI translation rules. Packed without the `bbs`
  prerequisite, since it never calls the CLI.

## 1.127.2 — 2026-11-10

### Added

- `marketing-video` hit-stop and impact vocabulary: a new `freeze` effect
  holds the frame on contact (shaking in real time, catching up to the
  voice), `ghosts` trails after-images on a dash, `poses` gain
  `snap: true` (hold the last pose, jump on the key), and rigs get
  `steps` for hand-drawn stepped poses; `07-craft.md` documents the blow/
  dash/impact lessons. Adapted from pixelfx (MIT); no code copied.

## 1.127.1 — 2026-11-09

### Added

- New guide `docs/performance-and-architecture.md` covering `perf-audit`
  and `architect`, linked from `docs/skills.md` and the READMEs.

### Changed

- Plugin manifests, `package.json`, the Homebrew formula and the READMEs
  now point at `2found.dev` for homepage/docs/blog instead of the GitHub
  repo; release `DOWNLOADS.md` adds a "Guides on 2found.dev" link.
- `RELEASE.md` spells out the post-release website refresh commands
  (`docs:sync:build` revision pin, `downloads:sync`, build + check) and
  the catalog/facts requirements for new docs pages and skills.

## 1.127.0 — 2026-11-09

### Added

- New `perf-audit` skill: audits one feature or the whole system's
  performance for a non-engineer owner — what users feel, causes proven
  from production signals (route templates from request logs, burst-second
  and 429-retry signatures, read-only SQL snapshot), fixes ranked easiest
  and safest first, each shipped as a ready `autopilot` requirement with
  before/after acceptance; `verify <report>` re-measures a fix.
- New `architect` skill: maps a system's verified as-is architecture and
  designs its next generation — diagrams, headroom, safety gaps and a
  staged scaling roadmap (edge, database, cache, jobs, replicas, services)
  where each stage waits for a measured trigger; writes
  `docs/architecture-next.md`, never code or config.
- Handoff contracts: `perf-audit` reports `AUDITED(<verdict>, counts)` /
  `VERIFIED(fixed/still/regressed)`; `architect` reports
  `ARCHITECTED(<scope>, findings, stages)`.
- `tests/test_perf_audit_scripts.sh` covers the log aggregator's route
  templating, id redaction and log shapes.

### Changed

- `ui-audit` reworked around end-to-end journeys (PASS/FRICTION/BLOCKED):
  backend ground truth gathered first and checked against every screen,
  dead ends, discoverability and non-happy paths; live/production audits
  get a read-only safety contract; detail split into `references/`
  (checklists, live-safety, helpers, report-template).
- `autopilot` routes performance work to `perf-audit`: a requirement
  citing its report takes the report as baseline and `verify <report>`
  as acceptance.
- `setup-project` documents that `docs/architecture-next.md` and the
  `perf-audit` profile (`docs/PERFORMANCE.md`) link from the system
  document's Sources; they are not architecture owners.
- `marketing-video` adds a craft stage and cold-viewer review: new mvflib
  modules (craft, picture, formats, fresh, viewer) and `07-craft.md` cut
  standards; the benefit is now the owner's optional call, ideas ranked by
  what the audience watched; stage 7 is reviewed by a cold viewer who
  never read the plan.

## 1.126.2 — 2026-11-08

### Changed

- `marketing-video` OmniVoice voice stage drops the ASR dropped/added-word
  judge and its re-voice retry loop: each cue is voiced once and Whisper is
  used only to time the words for captions; `--rehear` re-times the latest
  take instead of re-listening to it. The `retries` engine option,
  `mvf voice-pick --force`, and the manifest `asr`/`asr_problems` fields are
  removed — listen to every line.

## 1.126.1 — 2026-11-08

### Changed

- `marketing-video` review now contracts "judge once, fix once, ship": after
  the panel judges a cut, fix the findings and export without a second
  `mvf judge video` round; re-judge only on owner request or a
  script-level change.
