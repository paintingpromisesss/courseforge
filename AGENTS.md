# CourseForge — Agent Instructions

## Commands

**Full build** (frontend + backend → `bin/courseforge`):
```bash
./scripts/build.sh          # Windows: .\scripts\build.ps1
./scripts/build.sh --skip-deps   # skip npm ci
```

**Run**:
```bash
./bin/courseforge --port=6770 --courses-dir=./courses --data-dir=./data --frontend-dir=./frontend/dist
```

**MCP Server** (stdio mode for AI agents / IDE):
```bash
./bin/courseforge mcp --courses-dir=./courses --data-dir=./data --transport=stdio
# dev: cd backend && go run ./cmd/server mcp --courses-dir=./courses --data-dir=./data
# smoke test: .\scripts\smoke-test-mcp.ps1
```

**Dev** (run both terminals):
```bash
cd backend && make run       # hot swagger regen + go run via cmd/server (swagger tag)
cd frontend && npm run dev   # Vite HMR, proxies /api → 127.0.0.1:6770
```

**Tests**:
```bash
cd backend && make test      # all Go tests (requires GOCACHE set by Makefile)
cd frontend && npx vitest run
cd frontend && npx vitest run src/lib/buildTree.test.ts   # single file
```

**Swagger** (regenerate before build if handlers changed):
```bash
cd backend && make swagger
```

**Lint**: `cd frontend && npm run lint` (ESLint flat config, no pre-commit hook).

## Build quirks

- Single Go binary serves REST API (chi) + static SPA (`frontend/dist/`), AND the MCP server (`courseforge mcp ...`). There is NO separate `courseforge-mcp` binary.
- `swagger` build tag: enables `/swagger/index.html`. `scripts/build.sh` and `make run` use it; bare `go build` does not.
- Windows binary is built as a GUI-subsystem exe (`-H=windowsgui`) so no console window flashes on double-click. When invoked from a terminal or CLI/MCP, it calls `attachConsoleIfAvailable()` in `cmd/courseforge/console_windows.go` to attach to the parent console and wire stdout/stderr properly.
- Shared `GOCACHE` at `.cache/go-build/` across repo (set by build scripts and Makefile).
- Frontend build = `tsc -b && vite build` (type-check then bundle).

## Architecture

```
backend/                       # Go 1.26, module github.com/paintingpromisesss/courseforge
  cmd/courseforge/             # production CLI (chi + static file server + mcp subcommand)
  cmd/server/                  # dev CLI (adds swagger tag + mcp subcommand)
  internal/di/di.go            # composition root — wires config → courses → runner → repos → services → handlers → router
  internal/api/handlers/       # HTTP handlers (chi router)
  internal/api/dto/            # request/response types
  internal/application/service/ # business logic (progress, submissions)
  internal/infrastructure/     # runner, repos, parser, AI client
  internal/mcp/                # MCP server (stdio & SSE, session manager, tools, resources)
  internal/domain/             # Course, Task, Submission, Progress, MCP
  docs/                        # swagger JSON (generated)

frontend/                      # React 19, TS, Vite 8, Tailwind 3
  src/api/client.ts            # all API calls
  src/pages/                   # CoursesPage, CatalogPage, CoursePage, TaskPage, TheoryPage

scripts/                       # build.sh / build.ps1 / install.sh / install.ps1
courses/                       # course YAML manifests (gitignored — not committed)
data/                          # runtime state (gitignored: SQLite, runners.json, postgres/)
```

## Course format (agent-relevant quirks)

- Hierarchy: **Catalog → Course → Track → Topic → Unit** (theory + tasks).
- Each level has a YAML manifest (`course.yaml`, `track.yaml`, etc.); **slug must equal folder name**.
- Parser walks manifest's explicit child list — it does NOT scan directories.
- Tasks are multilingual: `task.yaml` maps language key → `{template, solution, tests}` files in a per-language subfolder.
- `solution` file is never shipped into user sandbox; only `template` + user code + `tests`.
- `video_url` on unit.yaml; `editorial_url` on task.yaml — both render as embedded players in the UI.

## Runner / execution

- Language drivers defined in `data/runners.json` (or built-in for Go).
- Driver fields: `run_cmd`, `test_cmd`, `ext`, `test_ext`, `init_files`. Placeholders: `{file}`, `{testfile}`, `{dir}`.
- `postgres` driver is special: uses a managed Postgres cluster with `CREATE SCHEMA`/`DROP SCHEMA` per run, not an isolated temp process.
- Two platform implementations: `postgres.go` (Unix, Unix socket) and `postgres_windows.go` (TCP loopback).

## Frontend quirks

- `api/client.ts`: camelCase query params, snake_case JSON bodies (intentional mismatch).
- Frontend tests use `vitest` (not jest).
- TypeScript strict: `noUnusedLocals`, `noUnusedParameters`, `verbatimModuleSyntax`, `moduleDetection: force`.

## Data storage

| What | Where |
|---|---|
| Course progress | `progress.json` alongside course files |
| Submissions | SQLite (`--data-dir/courseforge_submissions.db` or `courseforge.db`) |
| Runner config | `data/runners.json` |
| Postgres cluster | `data/postgres/` (auto-created on first run) |
| Active MCP session | `mcp_session.json` inside `--data-dir` |
| MCP server settings | `mcp_server_config.json` inside `--data-dir` |

## MCP integration

- Unified binary: `courseforge mcp [flags]` runs the MCP server directly.
- Transports supported: `stdio` (default for CLI / IDE agents) and `sse` (mounted at `/api/mcp/sse` when running the web server, or standalone `--transport=sse`).
- Active task synchronization:
  - When user views a task in the UI, `TaskPage.tsx` debounces 3 seconds and calls `PUT /api/mcp/active-task`.
  - The web server writes the active task context to `mcp_session.json`.
  - `FileSessionManager.GetActiveTask()` checks `os.Stat(filePath).ModTime()` on every invocation, automatically hot-reloading changes on the fly without restarting the MCP stdio process.
- MCP client config convention for agents:
  ```json
  "courseforge": {
    "command": "F:\\Proga\\courseforge\\bin\\courseforge.exe",
    "args": [
      "mcp",
      "--courses-dir=F:\\Proga\\courseforge\\courses",
      "--data-dir=F:\\Proga\\courseforge\\data"
    ]
  }
  ```

## CI / CD

- `.github/workflows/ci.yml`: runs on push/merge to `main`, PRs to `main`, and manual dispatch. Tests backend (`go test ./internal/...`), dev server build (`go build ./cmd/server`), frontend tests (`vitest run`), and frontend bundle build (`tsc -b && vite build`).
- `.github/workflows/release.yml`: runs on tag push (`v*`) and manual dispatch. Matrix build across 6 OS targets and release publishing.
- No pre-commit hooks, no Husky. Lint and test manually before pushing.


## MCP Tools & Execution Rules

### `sequential-thinking` — structured reasoning

**Trigger**: multi-step backend changes that touch domain → service → handler → DTO chain; any bug fix with unclear root cause; non-trivial refactors spanning ≥2 layers.
**Do not use**: single-file edits, command lookup, reading config files, adding a field to one DTO.
**Tokenomics**: keep each `thought` to one sentence. Set `totalThoughts` conservatively (3–5 for most tasks). Never pass full source code into thoughts — reference file:line instead.
**Project tie-in**: Go backend is layered (`di` → `handler` → `service` → `repo`/`runner`). Before editing, trace through all layers with sequential thinking to avoid half-fixes (e.g., changing a handler without updating the DTO or domain type).

### `memory_*` — knowledge graph

**Trigger**: cross-cutting facts that span multiple files (e.g., "where are API response conventions defined?", "what does the postgres driver do on Windows?"). Use `memory_search_nodes` before reading files if the answer might be cached.
**Do not use**: transient state (current git status, one-off file paths), single-file facts, anything that expires after the session.
**Tokenomics**: store facts as short, declarative sentences. One observation per call unless they belong to the same entity. Never dump entire file contents as observations.
**Project tie-in**: Use to remember architecture decisions (e.g., intentional camelCase/snake_case mismatch in `api/client.ts`, postgres platform split) so future sessions don't "fix" things that are deliberately asymmetric.

### `git_*` — version control

**Trigger**: after every set of edits that change tracked files — `git diff` → stage selectively → commit with conventional message. Before branching for a feature, `git status` to verify clean state.
**Do not use**: on untracked files (`courses/`, `data/` are gitignored). Don't commit generated files (`frontend/dist/`, `bin/`, `.cache/`).
**Tokenomics**: only pass `git diff` output to the commit message generator — never the full file content.
**Project tie-in**: Go module path is `github.com/paintingpromisesss/courseforge`. If changing handler+DTO together, stage and commit them as one logical change. `swagger/` docs generated in `backend/docs/` — regenerate (`make swagger`) and commit alongside handler changes.

### `fetch_fetch` — URL content retrieval

**Trigger**: need to read a remote URL's content (documentation, API specs, external references in course `editorial_url` / `video_url`).
**Do not use**: for local file reading — use `read` instead. Never fabricate URLs.
**Tokenomics**: use `raw=false` (default) to get simplified markdown. Only set `raw=true` when you need HTML structure. Use `max_length` to avoid pulling entire pages.
**Project tie-in**: Course content references (`video_url`, `editorial_url`) point to YouTube, archive.org, or direct video files — these are embedded players, not fetched by the agent during development.

### `duckduckgo_*` — web search

**Trigger**: need current information about external dependencies (Go library versions, Vite plugin compatibility, Tailwind features) that isn't obvious from `go.mod` / `package.json`.
**Do not use**: for repo-local questions answerable by reading files. For checking Go package docs — `go doc` or `fetch` the pkg.go.dev URL directly is faster.
**Tokenomics**: use specific queries with package names and versions. One search batch, max 5 results.
