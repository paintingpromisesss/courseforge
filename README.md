<div align="center">

<img src="frontend/public/favicon.svg" width="72" height="72" alt="CourseForge logo" />

# CourseForge

**Self-hosted platform for interactive programming courses**

Local course files · In-browser code execution · Automated test grading · Progress tracking

[![Go](https://img.shields.io/badge/Go-1.26-00ADD8?logo=go&logoColor=white)](https://go.dev)
[![React](https://img.shields.io/badge/React-19-61DAFB?logo=react&logoColor=black)](https://react.dev)
[![TypeScript](https://img.shields.io/badge/TypeScript-6.0-3178C6?logo=typescript&logoColor=white)](https://www.typescriptlang.org)
[![Vite](https://img.shields.io/badge/Vite-8-646CFF?logo=vite&logoColor=white)](https://vite.dev)
[![Tailwind CSS](https://img.shields.io/badge/Tailwind_CSS-3-06B6D4?logo=tailwindcss&logoColor=white)](https://tailwindcss.com)
[![Platform](https://img.shields.io/badge/platform-Windows%20%7C%20Linux%20%7C%20macOS-8251EE)](#quick-start)

</div>

---

CourseForge reads courses from local files, serves a web UI on `localhost`,
and lets you solve programming tasks right in the browser — with code
execution, automated test grading, and progress tracking. A single Go binary
serves both the REST API and the SPA frontend: no Docker, no external
services, no cloud.

## Features

- **Multi-language tasks** — Go, Python, JavaScript, C++, Java, C#,
  PostgreSQL. Each task can ship a template, a reference solution, and tests
  for several languages.
- **Isolated code execution** — every run executes in its own temporary
  directory; PostgreSQL runs get a dedicated schema in a managed cluster
  (`CREATE SCHEMA` / `DROP SCHEMA` per run).
- **Automated grading** — submissions run against the course's own tests on
  the server (`go test`, `pytest`, `mocha`/TAP, GoogleTest, JUnit, NUnit,
  pgTAP); the output is parsed and persisted, and a task is marked complete
  only when 100 % of its tests pass.
- **Theory lessons** — Markdown with syntax highlighting, images, and video;
  the "mark as read" state counts toward course progress.
- **Progress tracking** — progress strip and counters on course cards, `N/M`
  counters in the course tree, a "Completed" badge, a "Continue learning"
  banner on the home page, and one-click course progress reset.
- **Course catalogs** — group courses into catalogs, create and edit groups
  from the UI, import courses from archives.
- **Code editor** — CodeMirror 6 with per-language syntax highlighting,
  draft autosave, and reset-to-template.
- **Configurable runners** — run and test commands for every language are
  editable in Settings (`data/runners.json`); installed toolchains are
  detected with status and version.
- **Submission history** — every submission is stored in SQLite along with
  its output and result.
- **Dark and light themes**, Swagger UI for the API (in dev builds).

## Quick Start

### Windows

```powershell
.\scripts\build.ps1
.\bin\courseforge.exe --port=8080 --courses-dir=.\courses --data-dir=.\data --frontend-dir=.\frontend\dist
```

### Linux / macOS / WSL

```bash
./scripts/build.sh
./bin/courseforge --port=8080 --courses-dir=./courses --data-dir=./data --frontend-dir=./frontend/dist
```

Open [http://localhost:8080](http://localhost:8080).

### Requirements

| Component | Version | Purpose |
|---|---|---|
| Go | 1.26+ | building and running the backend |
| Node.js + npm | 20+ | building the frontend |
| Language toolchains | — | optional: only for the languages you actually run (`go`, `python`, `node`, `g++`, `javac`, `dotnet`, `psql`/`initdb`/`pg_prove`) |

A missing toolchain doesn't break the platform — the corresponding runner is
simply reported as unavailable in Settings.

## Installation

```powershell
.\scripts\install.ps1   # Windows
```

```bash
./scripts/install.sh    # Linux / macOS
```

The script copies the binary into a user bin directory. The frontend build
stays in `frontend/dist` — pass `--frontend-dir` when running an installed
binary outside the repository.

### CLI Flags

| Flag | Default | Description |
|---|---|---|
| `--host` | `127.0.0.1` | address to bind the server to |
| `--port` | `8080` | HTTP server port |
| `--courses-dir` | `./courses` | directory with course files |
| `--data-dir` | `./data` | app state (SQLite, runners, PostgreSQL cluster) |
| `--db-path` | `{data-dir}/…` | path to the submissions SQLite database |
| `--frontend-dir` | auto-detected | directory with built SPA assets (`frontend/dist`) |

## Architecture

```
┌───────────────────────────── single Go binary ───────────────────────────┐
│                                                                          │
│  REST API (chi)          Core subsystems               Static SPA        │
│  /api/courses            • Runner — code execution     React + Vite      │
│  /api/progress           • PostgresManager — cluster   TanStack Query    │
│  /api/submissions        • Progress (JSON per course)  Tailwind CSS      │
│  /api/runners            • Submissions (SQLite)        CodeMirror 6      │
│  /swagger (dev)                                                          │
└──────────────────────────────────────────────────────────────────────────┘
```

**Backend** (`backend/`) — layered structure: `internal/api` (handlers +
DTOs), `internal/application/service` (business logic),
`internal/infrastructure` (runners, repositories, course parser),
`internal/domain` (domain types), `internal/di` (composition root).

**Frontend** (`frontend/`) — React 19 + React Router + TanStack Query +
Tailwind; all API calls live in `src/api/client.ts`.

**Runner** — language drivers are described as commands with `{file}` /
`{testfile}` / `{dir}` placeholders and are editable from the UI. The
`postgres` driver is special: instead of an isolated per-run process it uses
a single managed PostgreSQL cluster with schema-per-run isolation (a private
Unix socket on Unix, loopback TCP on Windows).

## Course Format

Hierarchy: **Catalog → Course → Track → Topic → Unit** (theory and/or
tasks). Every level is described by a YAML manifest; the slug must equal the
folder name.

```
courses/
└── my-course/
    ├── course.yaml
    ├── progress.json            # created automatically
    └── basics/                  # track
        ├── track.yaml
        └── intro/                # topic
            ├── topic.yaml
            └── 01-hello/         # unit
                ├── unit.yaml
                ├── theory.md
                └── hello-task/   # task
                    ├── task.yaml
                    ├── statement.md
                    └── go/
                        ├── template.go
                        ├── solution.go
                        └── solution_test.go
```

Tasks are multilingual: `task.yaml` maps a language key to its
`{template, solution, tests}` files in a per-language subfolder. See
[docs/course-format.md](docs/course-format.md) for details.

## Development

```bash
# backend: hot swagger + go run (run make swagger after changing handlers)
cd backend && make swagger && make run

# frontend: Vite HMR, proxies /api to :8080
cd frontend && npm install && npm run dev

# tests
cd backend && make test
cd frontend && npx vitest run
```

Swagger UI is available at `/swagger/index.html` in builds with the
`swagger` tag (`make run`, `scripts/build.sh`).

## Data Storage

| What | Where |
|---|---|
| Course progress | `progress.json` next to the course files |
| Submissions | SQLite in `--data-dir` |
| Runner configuration | `data/runners.json` |
| PostgreSQL cluster | `data/postgres/` (created on first run) |

---

<div align="center">
<sub>CourseForge — forge your knowledge locally. ⚒️</sub>
</div>
