# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Commands

**Full build** (frontend + backend → `bin/courseforge`):
```bash
./scripts/build.sh          # or --skip-deps to skip npm ci
```

**Run** (after build):
```bash
./bin/courseforge --port=8080 --courses-dir=./courses --data-dir=./data --frontend-dir=./frontend/dist
```

**Backend dev** (hot swagger regen + `go run`, no binary):
```bash
cd backend && make run       # requires make swagger first
cd backend && make test      # run all Go tests
cd backend && go test ./internal/infrastructure/runner/...  # single package
```

**Frontend dev** (Vite HMR, proxies `/api` to backend):
```bash
cd frontend && npm run dev
```

**Regenerate swagger docs** (required before building if handlers changed):
```bash
cd backend && make swagger
```

## Architecture

Single Go binary serves both the REST API and the React SPA (`frontend/dist/`).

**Backend layout** (`backend/`):
- `cmd/courseforge/` — CLI entry point; `cmd/server/` — used by `make run` (swagger tag)
- `internal/di/` — wires everything together (config → courses → runner → repos → services → handlers → router)
- `internal/api/handlers/` — HTTP handlers (chi router); `internal/api/dto/` — request/response types
- `internal/application/service/` — business logic (progress, submissions)
- `internal/infrastructure/repo/` — SQLite (submissions) + file-based (progress JSON per course)
- `internal/infrastructure/runner/` — executes user code in temp dirs; drivers configured in `data/runners.json`
- `internal/domain/` — core types (Course, Task, Submission, Progress)

**Frontend layout** (`frontend/src/`):
- React + React Router + TanStack Query + Tailwind
- Route tree: `/` → CoursesPage, `/catalogs/:slug` → CatalogPage, `/courses/:slug` → CoursePage (outlet) → TaskPage / TheoryPage
- `api/client.ts` — all API calls; camelCase query params, snake_case JSON bodies (intentional — see memory)

**Course format** (`courses/`):
- Hierarchy: Catalog → Course → Track → Topic → Unit (theory + tasks)
- Each level has a YAML manifest (`course.yaml`, `track.yaml`, etc.); slug must equal folder name
- Tasks are multilingual: `task.yaml` maps language keys to `{template, solution, tests}` files in a per-language subfolder
- Progress stored per-course as `progress.json` alongside course files; submissions in SQLite

**Runner** (`internal/infrastructure/runner/runner.go`):
- `LangDriver` defines `run_cmd`, `test_cmd`, `ext`, `test_ext`, `init_files` for a language
- Go driver is built-in; others loaded from `data/runners.json`
- Task runs = user code + test file in isolated temp dir; solution file never included

**Build flags**: `swagger` build tag enables Swagger UI at `/swagger/index.html` (used by `make run` and `scripts/build.sh`; disabled in production-style builds).

---

Behavioral guidelines to reduce common LLM coding mistakes. Merge with project-specific instructions as needed.

**Tradeoff:** These guidelines bias toward caution over speed. For trivial tasks, use judgment.

## 1. Think Before Coding

**Don't assume. Don't hide confusion. Surface tradeoffs.**

Before implementing:
- State your assumptions explicitly. If uncertain, ask.
- If multiple interpretations exist, present them - don't pick silently.
- If a simpler approach exists, say so. Push back when warranted.
- If something is unclear, stop. Name what's confusing. Ask.

## 2. Simplicity First

**Minimum code that solves the problem. Nothing speculative.**

- No features beyond what was asked.
- No abstractions for single-use code.
- No "flexibility" or "configurability" that wasn't requested.
- No error handling for impossible scenarios.
- If you write 200 lines and it could be 50, rewrite it.

Ask yourself: "Would a senior engineer say this is overcomplicated?" If yes, simplify.

## 3. Surgical Changes

**Touch only what you must. Clean up only your own mess.**

When editing existing code:
- Don't "improve" adjacent code, comments, or formatting.
- Don't refactor things that aren't broken.
- Match existing style, even if you'd do it differently.
- If you notice unrelated dead code, mention it - don't delete it.

When your changes create orphans:
- Remove imports/variables/functions that YOUR changes made unused.
- Don't remove pre-existing dead code unless asked.

The test: Every changed line should trace directly to the user's request.

## 4. Goal-Driven Execution

**Define success criteria. Loop until verified.**

Transform tasks into verifiable goals:
- "Add validation" → "Write tests for invalid inputs, then make them pass"
- "Fix the bug" → "Write a test that reproduces it, then make it pass"
- "Refactor X" → "Ensure tests pass before and after"

For multi-step tasks, state a brief plan:
```
1. [Step] → verify: [check]
2. [Step] → verify: [check]
3. [Step] → verify: [check]
```

Strong success criteria let you loop independently. Weak criteria ("make it work") require constant clarification.

---

**These guidelines are working if:** fewer unnecessary changes in diffs, fewer rewrites due to overcomplication, and clarifying questions come before implementation rather than after mistakes.
