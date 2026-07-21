# Course Format

CourseForge reads courses from plain files on disk — YAML manifests plus
Markdown content. This is the canonical, git-friendly source of truth; there
is no database record of course structure.

## Hierarchy

```
Catalog (optional)  →  Course  →  Track  →  Topic  →  Unit
```

- **Catalog** — an optional group of related courses (e.g. all
  `algo-interview-*` courses). Described by `catalog.yaml` in the group
  folder; child courses live in subfolders. A standalone course with no
  catalog lives directly under `courses/` and has no `catalog.yaml`.
- **Track** — a week or a major module. A single-track course is just a
  course with one track.
- **Topic** — a section within a track.
- **Unit** — the unit of learning. A **container, not a leaf**: it holds
  optional theory and/or a list of tasks (rendered as "Theory" / "Tasks"
  tabs in the UI).

Every container level is described by its own manifest and **explicitly**
lists its children, in order. Order is never inferred from folder names.

## Conventions

- **`slug` must equal the folder name** at every level. The parser walks the
  manifest's child list and opens the matching subfolder directly — it does
  not scan the directory.
- `slug` is also duplicated **inside** the manifest as a self-contained,
  checked field; the parser verifies it against the folder name and fails
  with an error on mismatch.
- **Fixed manifest filenames:** `course.yaml`, `track.yaml`, `topic.yaml`,
  `unit.yaml`, `task.yaml`. The level's type is determined by which manifest
  is present.
- `schema_version` is set **only** in `course.yaml` and applies to the whole
  course.
- Encoding is **UTF-8 without BOM**.
- An invalid course fails with a specific error (which file, which field),
  never silently.

## Directory Layout

```
courses/
  algo-interview/                   # catalog folder (slug == folder name)
    catalog.yaml
    core/                           # nested course
      course.yaml
      ...
    arrays-and-matrices/
      course.yaml
      ...
  go-interview/                     # standalone course, no catalog
    course.yaml
    week-1/
      track.yaml
      slices/
        topic.yaml
        02-chunk/
          unit.yaml
          theory.md
          assets/
            chunking.png            # co-located, referenced as ![](assets/chunking.png)
          chunk/                    # task folder (slug == folder name)
            task.yaml
            statement.md
            go/
              template.go
              solution.go
              solution_test.go
          stack/
            task.yaml
            ...
```

## Manifests

### `catalog.yaml`

Optional manifest for a group of courses. Its presence marks the folder as a
catalog; its absence means a standalone course.

```yaml
slug: algo-interview
title: Algo Interview
description: Preparation for algorithmic technical interview rounds.
courses:               # ordered list of child course slugs
  - core
  - arrays-and-matrices
  - binary-search
```

- `slug` must equal the catalog folder name.
- `courses` is an explicit, ordered list; the order drives UI navigation.
- Child courses live in subfolders of the catalog and have a normal
  `course.yaml`.

### `course.yaml`

```yaml
schema_version: 1
slug: go-interview
title: Go for Interviews
description: |
  Preparation for Go technical interview rounds.
language: en                 # natural language of the content
tracks:                      # track order — list of slugs
  - week-1
  - week-2
```

### `track.yaml`

```yaml
slug: week-1
title: "Week 1. Basics"
description: Slices, pointers, strings, defer.
topics:
  - slices
  - pointers
```

### `topic.yaml`

```yaml
slug: slices
title: Slices
description: Internals, length vs capacity, common pitfalls.
units:
  - 01-slices-theory
  - 02-chunk
```

### `unit.yaml`

A unit is optional **one** theory lesson plus an optional **list** of tasks.
A unit must have at least one of the two, or the parser fails.

```yaml
slug: 02-chunk
title: Slices — Basic Operations
theory: theory.md            # optional; theory content for the unit
video_url: https://youtu.be/dQw4w9WgXcQ  # optional; intro video above the theory
tasks:                       # optional; ordered list of task slugs
  - chunk
  - stack
  - remove-inplace
```

- **Theory** is a single Markdown file. Media goes in the unit's `assets/`,
  referenced with relative links. YouTube / archive.org / video file links
  render as an embedded player.
- **`video_url`** — optional intro video link, rendered as an embedded player
  above `theory.md`'s content. Supports the same formats as a task's
  `editorial_url` (YouTube, archive.org, `.mp4`/`.webm`/`.ogg`).
- **Tasks** — an ordered list; the UI renders one "Tasks" tab with list
  navigation. Each task is a subfolder with its own `task.yaml`.

#### Rule: theory and its tasks belong in the same unit

Tasks that belong to a topic must live in the same unit as its theory — not
in separate units. A new unit is only created when a **new**, independent
topic with its own theory begins.

```
# Correct: one topic — one unit
units:
  - 01-strings        # theory: theory.md, tasks: [reverse, append]
  - 02-runes          # theory: theory.md, tasks: [palindrome]

# Incorrect: theory and tasks split across units
units:
  - 01-strings        # theory only
  - 02-reverse         # task only
  - 03-append          # task only
  - 04-runes           # theory only
  - 05-palindrome       # task only
```

### `task.yaml`

A task is multilingual: one shared statement plus a per-language set of
files.

```yaml
slug: chunk
title: Chunk Function
statement: statement.md          # statement, shared across all languages
editorial_url: https://youtu.be/dQw4w9WgXcQ  # optional; video walkthrough
languages:                       # map: language key → file set
  go:
    template: template.go        # starting code shown in the user's editor
    solution: solution.go        # reference solution — for offline test validation on import
    tests: solution_test.go      # test file that grades a submission
  # python:
  #   template: template.py
  #   solution: solution.py
  #   tests: test_solution.py
limits:                          # optional; overrides sandbox defaults
  timeout_sec: 10
  memory_mb: 256
```

- `editorial_url` — optional link to a video walkthrough of the task.
  Supports YouTube (`youtube.com/watch?v=…`, `youtu.be/…`), Internet Archive
  (`archive.org/details/…`), and direct video files (`.mp4`, `.webm`,
  `.ogg`). When set, a "Video Walkthrough" tab appears in the UI.
- `statement.md` is one per task; the (language-specific) function signature
  is visible from `template`, not from the statement.
- `languages` — the key selects the runner/sandbox for that language. Adding
  a language means adding a key and a folder, without touching anything
  else.
- File names (`template`, `solution`, `tests`) are **relative to the
  language's subfolder** — e.g. `go/` for the `go` key. Do not prefix the
  value with the language folder: `template: template.go` is correct,
  `template: go/template.go` is wrong.
- File names are set by the manifest; conventions differ per language
  (`*_test.go`, `test_*.py`, etc.) and are not hardcoded by the parser.

## Solution Grading

- The user edits the **whole file**, starting from `template`.
- In the sandbox: the user's file plus `tests` are placed side by side and
  the language's runner executes (`go test` for Go, etc.). `solution` is
  never shipped into the user's sandbox run.
- `solution` is used separately — on course import, the platform can run
  `tests` against `solution` and refuse to publish a task whose tests don't
  pass against its own reference solution.
- Sandbox limits: global defaults live in the platform config; an optional
  `limits` block in `task.yaml` overrides them for heavier tasks.
- The sandbox itself (isolation mechanism, process/container setup) is a
  separate subsystem — the course format only specifies the language and
  file names.

### Progress

A unit's progress is solved tasks / total tasks (as a percentage). Theory
does not count toward this ratio — it has its own separate "read" state. A
task's progress is addressed by `(unit_slug, task_slug)`, which stays stable
across reordering.
