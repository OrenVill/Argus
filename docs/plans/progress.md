# Argus v1 mission progress

| Phase | Status | Notes |
| --- | --- | --- |
| Size gate | done | Full v1 mission |
| Brainstorm | done | See implementation design spec |
| Design on integration | done | `efb7195` on `cursor/argus-v1-integration-8966`; PR #1 → integration |
| Implementation plan | done | `docs/superpowers/plans/2026-11-01-argus-v1-implementation.md` |
| Track A normalization | done | Merged to integration @ `ab2302f`; [Track A final review](bc-d91f33af-46f9-5481-a720-7daa4b0bee2c) approve |
| Track B redis-detection | in_progress | [Track B redis implementer](bc-71e029ed-21b6-580d-a0af-2a5a7f62dcf0) |
| Track C runtime | pending | blocked on B merge |
| project-reviewer | pending | on integration after C; then PR integration → `main` |

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
