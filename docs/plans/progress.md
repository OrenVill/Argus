# Argus v1 mission progress

| Phase | Status | Notes |
| --- | --- | --- |
| Size gate | done | Full v1 mission |
| Brainstorm | done | See implementation design spec |
| Design on integration | done | `efb7195` on `cursor/argus-v1-integration-8966`; PR #1 → integration |
| Implementation plan | done | `docs/superpowers/plans/2026-11-01-argus-v1-implementation.md` |
| Track A normalization | done | Merged to integration @ `ab2302f`; [Track A final review](bc-d91f33af-46f9-5481-a720-7daa4b0bee2c) approve |
| Track B redis-detection | done | Merged to integration @ `0704ba4`; [Track B code review](bc-38c64c8c-d41c-5bfe-a870-8437757cd970) approve |
| Track C runtime | review | PR #4; [Track C runtime implementer](bc-4c662105-9197-5ee3-927d-74a11fa7aa34) done; quality+smoke OK; [Track C code review](bc-c11750f5-a440-57bc-805c-803506bbf1e7) |
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
