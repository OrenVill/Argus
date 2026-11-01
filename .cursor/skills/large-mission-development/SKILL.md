---
name: large-mission-development
description: Use when starting a large NexVill mission — multi-track architecture/refactor, data-layer cutover, worker extraction, protocol/hot-path epic, or when the user says mission / architecture initiative / cut over / extract worker.
---

# Large mission development

Orchestrator for **big** NexVill work (cutovers, extractions, multi-track epics). It wires Superpowers and existing NexVill subagent rules — it does not replace them.

**Iron rule:** For work that passes the size gate, do not implement until brainstorm → approved spec → written plan. Violating the letter of this is violating the spirit.

Spec: `docs/superpowers/specs/2026-09-23-large-mission-development-design.md`

## When to use

**In:** multi-track or cross-package architecture; risky cutovers; needs `mission-planner` and ≤3 parallel implementer tracks.

**Out:** single-PR feature/fix/chore; release/promote → release-and-deploy; gitops-only → gitops-operator; housekeeping scan → housekeeping; greenfield GitOps → bootstrapping-argocd-gitops.

Failed size gate → normal Development Workflow.

## Phase checklist

1. **Size gate** — Large mission? If not, exit this skill.
2. **Brainstorm** — **REQUIRED SUB-SKILL:** Superpowers brainstorming. No code until design is approved.
3. **Spec** — `docs/superpowers/specs/YYYY-MM-DD-<topic>-design.md`, commit, user reviews.
4. **Plan** — **REQUIRED SUB-SKILL:** Superpowers writing-plans → `docs/superpowers/plans/...`. Follow Planning-Preference-for-Parallel-Execution.
5. **Decompose** — Dispatch `mission-planner` (tracks, file ownership, merge order; max 3 parallel).
6. **Execute waves** — Per track: synced `main` → branch → implementer (`backend-implementer` / `web-implementer` / `spa-implementer`) → `code-reviewer` → `verifier` (`npm run quality:pr`). Failures → `debugger`, not on `main`. Orchestrator merges **serialized**; updates `docs/plans/progress.md`. Mechanics: `.cursor/rules/Parallel-Subagent-Workflow.mdc`, `.cursor/rules/Subagent-Delegation.mdc`.
7. **Cross-repo** — `gitops-operator` / release skill only when the mission plan requires it (usually after related app work is on `main`).
8. **Close** — All tracks on `main`: dispatch `project-reviewer` **once**. Update `docs/superpowers/plans/2026-09-20-architecture-initiatives-index.md` or progress when applicable.

## Hard rules

- Sync `main` with `origin/main` before branching; stop on conflicts and ask the user.
- Never implement on `main`.
- Max 3 parallel implementer tracks.
- `quality:pr` before PR; do not open if it fails.
- `product-updates.json` only when user-facing.
- Cloud agents: never `BYPASS_AUTH` (see cloud-agent-verify skill).

## Red flags — stop and restart at brainstorm

| Excuse | Reality |
|--------|---------|
| "We'll document after" | Large missions without spec/plan waste merge time. |
| "One PR is enough" | If it needs parallel tracks, it is not one PR. |
| "Skip project-reviewer" | Holistic gaps show up only after integration. |
| "Merge tracks in parallel" | Merges to `main` stay serialized. |

## Common mistakes

- Treating a routine bugfix as a mission.
- Inlining parallel-subagent steps here instead of reading the rules.
- Debugging on `main` after a failed track.
