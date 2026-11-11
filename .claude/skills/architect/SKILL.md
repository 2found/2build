---
name: architect
description: Map a system's current architecture and design its next generation with diagrams: verified as-is design, headroom, safety gaps, and a staged scaling roadmap (edge, database, cache, jobs, replicas, services) where each stage waits for a measured trigger.
disable-model-invocation: true
---
# architect
System-level design over a product horizon. `plan-draft` plans one change and
`team` builds a project; this skill decides what the system should become and
in what order, and hands each stage to them. It writes documents and
diagrams only: never code, infrastructure, configuration or data.

Follow [preamble](../references/preamble.md) and the
[Auto-Decision Framework](../references/auto-decision-framework.md). Shared
refs are filesystem paths beside this skill's directory, so read them by path, not as `skill://`.
Detail: [roadmap](references/roadmap.md) (tracks, triggers, seams) ·
[diagrams](references/diagrams.md) (set, homes, render check) ·
[next-gen template](references/next-gen.md).

## Modes
- `current` — verify the as-is architecture, update its diagrams, audit it.
  Steps 1–3.
- `next` — design the next horizon and roadmap. Runs `current` first when the
  current owners were not verified in the last 30 days.
- No argument — both.

Two levels never mix: **current** lives in the project's existing
architecture owners ([project docs](../setup-project/references/project-docs.md)),
true to code, config and production, nothing speculative; **next** lives in
the [next-gen document](references/next-gen.md).

## Principles
1. **Simplest that fits**: measured load plus the next horizon with headroom,
   fewest moving parts. Managed and boring wins when operating it costs more
   than its bill.
2. **Ready to grow means seams, not components**: keep the seam now, add the
   component when its trigger fires.
3. **Every component earns its place**: the trigger that justified it, owner,
   failure mode, monthly cost, how to remove it.
4. **Reversible first**: configuration before code before topology.
5. **Attribute before you architect**: a load-driven stage needs measured
   evidence that waste is removed (a [perf-audit](../perf-audit/SKILL.md)
   report or equal).
6. **Split along data ownership**: one writer per entity; a boundary cutting
   across tables two modules write is a distributed monolith.

## Inputs and assumptions
- **System**: `AGENTS.md` and the architecture, caching and deployment docs it
  links; code entrypoints, modules, data access; deploy and infrastructure
  config; read-only running state. Docs are claims, code and config are
  evidence. Two docs that disagree: draw what the newest code proves, tag the
  element `(unverified)` when code cannot settle it, and open a finding with
  the command that would.
- **Load**: perf-audit reports and the
  [performance profile](../perf-audit/references/profile.md), else the few
  numbers the headroom table needs, else `unmeasured`.
- **Cost**: bills or provider pricing pages with the date read; never an
  invented price.
- **Horizon**, first source that has one: PRD → roadmap milestone → open
  tickets → the owner's stated target → a labelled baseline of ×2 and ×5
  current peak.
- Owner-only facts (growth, budget ceiling, availability target, residency,
  team size) are never a blocking question: infer, tag
  `assumed (<source>, <date>)`, and list each under "Your decisions" with its
  default and the cost if the default is wrong.

## Flow
Mirror the steps into the task list.

1. **Inventory** what runs: components, stores, caches (each with its owner),
   queues, scheduled jobs, external services, entry path (DNS, CDN), and
   where each runs (host, size, region, managed or self-hosted). Mark every
   [roadmap](references/roadmap.md) stage of the three tracks `in place`,
   `partial` or `absent` with evidence.
2. **Current diagrams** ([diagrams](references/diagrams.md)), each in its home.
   In current owners correct only facts verified wrong and the diagrams;
   trimming, moving or other drift is a finding with its target owner, as in
   [project docs § Audit](../setup-project/references/project-docs.md#audit-the-owners).
3. **Audit.** Findings carry evidence, severity (`critical` outage or
   data-loss risk now · `high` breaks at the next horizon · `medium` taxes
   every change · `low`) and the smallest fix:
   - **Safety floor** gaps (always `critical`).
   - **Headroom** per component: peak, limit, headroom ×, what saturates
     first.
   - **Single points of failure** and what fails together, against the
     availability target.
   - **Coupling**: tables written by several modules, synchronous chains,
     cycles, shared mutable state.
   - **Scaling cliffs**: in-process state, local disk, singleton jobs without
     a lock, unbounded tables or queries, fan-out per request.
   - **Cost**: spend per component against what it serves.
   - **Operability**: rows of
     [agent-operable](../references/agent-operable.md) the system does not
     meet (UI-only operator actions, writes with no preview or retry safety,
     no health or needs-attention read path), `high` when an agent is due to
     run the product within the horizon.
4. **Horizon**: name it and its drivers, each new feature's load shape
   (read-heavy, write-heavy, CPU-bound jobs, real time, large files) and
   multiplier; project the headroom table and order the limits crossed.
5. **Next-gen design**: the minimal change set that clears those limits and
   carries the new features. Diagram it with changes marked, say what stays
   the same, record each costly-to-reverse decision.
6. **Roadmap**: the stages the horizon or a finding needs, in
   [roadmap](references/roadmap.md) order, each with a trigger in the
   `<signal> <op> <threshold> in <n> of <m> windows` form, prerequisites,
   change, what to watch, cost, rollback and how to verify; the seams to keep
   now; a "not yet" list. A trigger whose signal is not collected adds it to
   the profile as `TODO(2build):`.
7. **Verify** before reporting: every diagram rendered and looked at; every
   current fact has a file, command or console source; numbers carry date and
   source; facts that could not be read are in "Verify next".

## Output
The owner summary comes first, in the owner's language
([next-gen template](references/next-gen.md) § For the owner).
```text
STATUS: DONE | DONE_WITH_CONCERNS | NEEDS_CONTEXT | BLOCKED
VERDICT: ARCHITECTED(<current|next|both>, findings=<n>, stages=<n>)
DOCS: <updated current owners; next-gen doc; rendered diagram paths>
SUMMARY: <biggest risk, first limit to break and when, first stage and its trigger>
NEXT: <owner decisions; /bbs:autopilot "<seam or critical fix>"; /bbs:team "<stage>" for a phased stage; /bbs:perf-audit when load is unmeasured>
```
An unrendered diagram is a concern, not a deliverable. A next-gen design
without one measured or sourced load number is `DONE_WITH_CONCERNS`, every
projection tagged as an assumption.
