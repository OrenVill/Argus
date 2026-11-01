# Argus v1 mission progress

| Phase | Status | Notes |
| --- | --- | --- |
| Size gate | done | Full v1 mission |
| Brainstorm | done | See implementation design spec |
| Design on integration | done | `efb7195` on `cursor/argus-v1-integration-8966`; PR #1 → integration |
| Implementation plan | done | `docs/superpowers/plans/2026-11-01-argus-v1-implementation.md` |
| Track A normalization | done | Merged to integration @ `ab2302f`; [Track A final review](bc-d91f33af-46f9-5481-a720-7daa4b0bee2c) approve |
| Track B redis-detection | done | Merged to integration @ `0704ba4`; [Track B code review](bc-38c64c8c-d41c-5bfe-a870-8437757cd970) approve |
| Track C runtime | done | Merged @ `0c5013c`; [Track C code review](bc-c11750f5-a440-57bc-805c-803506bbf1e7) approve |
| Release must-fix | done | Round 2 [Release must-fix round 2](bc-497b73c3-d8c9-5b77-b1d7-276702dc82a4) merged @ `c8e12ff`; PR #6 |
| project-reviewer | final gate | [Opus final release gate](bc-ef015357-0846-5a38-aa37-493e1b7bd6aa) |

**Integration branch:** `cursor/argus-v1-integration-8966` — **all** mission PRs (design + tracks A/B/C) merge here first.

**Merge policy:** serialized **A → B → C** onto **integration** (never directly to `main`). One **release PR** `cursor/argus-v1-integration-8966` → `main` after `make quality:pr` on integration and mission `project-reviewer`.

**Release PR:** open only when integration contains full v1; do not merge tracks to `main` individually.

## Cloud agent models (orchestrator)

| Role | Model slug |
| --- | --- |
| Track implementers (A/B/C), debugger | `composer-2.5-fast` |
| Per-track `code-reviewer` (once per PR) | `cursor-grok-4.6-high` |
| Mission close `project-reviewer` (on integration, before release PR) | `claude-opus-5-5-high` |

**Last updated:** 2026-11-01 (model policy)
