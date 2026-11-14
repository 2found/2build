# Changelog

Older releases: [CHANGELOG-ARCHIVE.md](CHANGELOG-ARCHIVE.md).

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
