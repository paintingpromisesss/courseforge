package course

import (
	"strings"
	"testing"
	"testing/fstest"
)

// validCourse returns an in-memory course tree that exercises every level:
// a course with one track, one topic, and two units — one theory-only, one
// with theory plus a multi-file Go task.
func validCourse() fstest.MapFS {
	f := func(s string) *fstest.MapFile { return &fstest.MapFile{Data: []byte(s)} }
	return fstest.MapFS{
		"go-interview/course.yaml": f(`
schema_version: 1
slug: go-interview
title: Go для собеседований
language: ru
tracks:
  - week-1
`),
		"go-interview/week-1/track.yaml": f(`
slug: week-1
title: "Неделя 1"
topics:
  - slices
`),
		"go-interview/week-1/slices/topic.yaml": f(`
slug: slices
title: Слайсы
units:
  - 01-intro
  - 02-chunk
`),
		// Theory-only unit.
		"go-interview/week-1/slices/01-intro/unit.yaml": f(`
slug: 01-intro
title: Введение
theory: theory.md
`),
		"go-interview/week-1/slices/01-intro/theory.md": f("# Слайсы\n\nТекст."),
		// Unit with theory + one task.
		"go-interview/week-1/slices/02-chunk/unit.yaml": f(`
slug: 02-chunk
title: Chunk
theory: theory.md
tasks:
  - chunk
`),
		"go-interview/week-1/slices/02-chunk/theory.md": f("# Chunk\n"),
		"go-interview/week-1/slices/02-chunk/chunk/task.yaml": f(`
slug: chunk
title: Chunk-функция
statement: statement.md
languages:
  go:
    template: template.go
    solution: solution.go
    tests: solution_test.go
`),
		"go-interview/week-1/slices/02-chunk/chunk/statement.md":        f("Условие."),
		"go-interview/week-1/slices/02-chunk/chunk/go/template.go":      f("package main\n"),
		"go-interview/week-1/slices/02-chunk/chunk/go/solution.go":      f("package main\n"),
		"go-interview/week-1/slices/02-chunk/chunk/go/solution_test.go": f("package main\n"),
	}
}

func TestParseValid(t *testing.T) {
	c, err := Parse(validCourse(), "go-interview")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if c.Slug != "go-interview" || c.SchemaVersion != 1 {
		t.Fatalf("course header wrong: %+v", c)
	}
	if len(c.Tracks) != 1 || c.Tracks[0].Slug != "week-1" {
		t.Fatalf("tracks wrong: %+v", c.Tracks)
	}
	topic := c.Tracks[0].Topics[0]
	if topic.Slug != "slices" || len(topic.Units) != 2 {
		t.Fatalf("topic wrong: %+v", topic)
	}
	// Order must follow the manifest list, not map/alpha order.
	if topic.Units[0].Slug != "01-intro" || topic.Units[1].Slug != "02-chunk" {
		t.Fatalf("unit order wrong: %s, %s", topic.Units[0].Slug, topic.Units[1].Slug)
	}
	if topic.Units[0].Theory != "theory.md" || len(topic.Units[0].Tasks) != 0 {
		t.Fatalf("theory-only unit wrong: %+v", topic.Units[0])
	}
	task := topic.Units[1].Tasks[0]
	if task.Slug != "chunk" || task.Languages["go"].Tests != "solution_test.go" {
		t.Fatalf("task wrong: %+v", task)
	}
}

func TestParseErrors(t *testing.T) {
	tests := []struct {
		name    string
		mutate  func(fstest.MapFS)
		wantSub string // substring expected in the error
	}{
		{
			name: "slug mismatch with folder",
			mutate: func(m fstest.MapFS) {
				m["go-interview/week-1/track.yaml"].Data = []byte("slug: wrong\ntitle: t\ntopics:\n  - slices\n")
			},
			wantSub: "must equal folder name",
		},
		{
			name: "unsupported schema version",
			mutate: func(m fstest.MapFS) {
				m["go-interview/course.yaml"].Data = []byte("schema_version: 99\nslug: go-interview\ntitle: t\ntracks:\n  - week-1\n")
			},
			wantSub: "schema_version",
		},
		{
			name: "empty unit",
			mutate: func(m fstest.MapFS) {
				m["go-interview/week-1/slices/01-intro/unit.yaml"].Data = []byte("slug: 01-intro\ntitle: t\n")
			},
			wantSub: "unit is empty",
		},
		{
			name: "theory file missing",
			mutate: func(m fstest.MapFS) {
				delete(m, "go-interview/week-1/slices/01-intro/theory.md")
			},
			wantSub: "file not found",
		},
		{
			name: "task template missing on disk",
			mutate: func(m fstest.MapFS) {
				delete(m, "go-interview/week-1/slices/02-chunk/chunk/go/template.go")
			},
			wantSub: "file not found",
		},
		{
			name: "task without languages",
			mutate: func(m fstest.MapFS) {
				m["go-interview/week-1/slices/02-chunk/chunk/task.yaml"].Data =
					[]byte("slug: chunk\ntitle: t\nstatement: statement.md\n")
			},
			wantSub: "at least one language",
		},
		{
			name: "unknown manifest field",
			mutate: func(m fstest.MapFS) {
				m["go-interview/week-1/slices/topic.yaml"].Data =
					[]byte("slug: slices\ntitle: t\nunts:\n  - 01-intro\n")
			},
			wantSub: "invalid YAML",
		},
		{
			name: "missing child folder",
			mutate: func(m fstest.MapFS) {
				delete(m, "go-interview/week-1/slices/02-chunk/unit.yaml")
			},
			wantSub: "cannot read manifest",
		},
		{
			name: "orphan unit folder not listed in topic",
			mutate: func(m fstest.MapFS) {
				// A complete unit on disk that topic.yaml does not reference.
				m["go-interview/week-1/slices/99-orphan/unit.yaml"] =
					&fstest.MapFile{Data: []byte("slug: 99-orphan\ntitle: t\ntheory: theory.md\n")}
				m["go-interview/week-1/slices/99-orphan/theory.md"] =
					&fstest.MapFile{Data: []byte("# x\n")}
			},
			wantSub: "exists on disk but is not listed",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := validCourse()
			tt.mutate(m)
			_, err := Parse(m, "go-interview")
			if err == nil {
				t.Fatalf("expected error containing %q, got nil", tt.wantSub)
			}
			if !strings.Contains(err.Error(), tt.wantSub) {
				t.Fatalf("error %q does not contain %q", err.Error(), tt.wantSub)
			}
		})
	}
}

func TestParserParseEntityFromDisk(t *testing.T) {
	p := NewParser(validCourse())

	// Each method parses (and thereby validates) its own subtree on disk.
	tr, err := p.ParseTrack("go-interview/week-1")
	if err != nil || tr.Slug != "week-1" {
		t.Errorf("ParseTrack: got %+v, err %v", tr, err)
	}
	tp, err := p.ParseTopic("go-interview/week-1/slices")
	if err != nil || tp.Slug != "slices" {
		t.Errorf("ParseTopic: got %+v, err %v", tp, err)
	}
	u, err := p.ParseUnit("go-interview/week-1/slices/02-chunk")
	if err != nil || u.Slug != "02-chunk" {
		t.Errorf("ParseUnit: got %+v, err %v", u, err)
	}
	tk, err := p.ParseTask("go-interview/week-1/slices/02-chunk/chunk")
	if err != nil || tk.Slug != "chunk" {
		t.Errorf("ParseTask: got %+v, err %v", tk, err)
	}
}

// validCatalog returns an in-memory catalog with two courses: the existing
// go-interview course plus a minimal second course.
func validCatalog() fstest.MapFS {
	m := validCourse() // contributes go-interview/
	f := func(s string) *fstest.MapFile { return &fstest.MapFile{Data: []byte(s)} }
	m["algo-interview/catalog.yaml"] = f(`
slug: algo-interview
title: Алго-интервью
description: Алгоритмы.
courses:
  - go-interview
  - extra
`)
	// Move go-interview under the catalog.
	for key, v := range validCourse() {
		m["algo-interview/"+key] = v
	}
	// Second minimal course.
	m["algo-interview/extra/course.yaml"] = f(`
schema_version: 1
slug: extra
title: Extra
language: ru
tracks:
  - t1
`)
	m["algo-interview/extra/t1/track.yaml"] = f(`
slug: t1
title: Track 1
topics:
  - p1
`)
	m["algo-interview/extra/t1/p1/topic.yaml"] = f(`
slug: p1
title: Topic 1
units:
  - u1
`)
	m["algo-interview/extra/t1/p1/u1/unit.yaml"] = f(`
slug: u1
title: Unit 1
theory: theory.md
`)
	m["algo-interview/extra/t1/p1/u1/theory.md"] = f("# Extra\n")
	return m
}

func TestParseCatalogValid(t *testing.T) {
	p := NewParser(validCatalog())
	cat, err := p.ParseCatalog("algo-interview")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if cat.Slug != "algo-interview" || cat.Title != "Алго-интервью" {
		t.Fatalf("catalog header wrong: %+v", cat)
	}
	if len(cat.Courses) != 2 {
		t.Fatalf("expected 2 courses, got %d", len(cat.Courses))
	}
	// Order must follow catalog.yaml, not map/alpha order.
	if cat.Courses[0].Slug != "go-interview" || cat.Courses[1].Slug != "extra" {
		t.Fatalf("course order wrong: %s, %s", cat.Courses[0].Slug, cat.Courses[1].Slug)
	}
	// Courses are fully parsed.
	if len(cat.Courses[0].Tracks) != 1 {
		t.Fatalf("first course not fully parsed: %+v", cat.Courses[0])
	}
}

func TestParseCatalogErrors(t *testing.T) {
	tests := []struct {
		name    string
		mutate  func(fstest.MapFS)
		wantSub string
	}{
		{
			name: "slug mismatch with folder",
			mutate: func(m fstest.MapFS) {
				m["algo-interview/catalog.yaml"].Data = []byte("slug: wrong\ntitle: t\ncourses:\n  - go-interview\n")
			},
			wantSub: "must equal folder name",
		},
		{
			name: "missing title",
			mutate: func(m fstest.MapFS) {
				m["algo-interview/catalog.yaml"].Data = []byte("slug: algo-interview\ncourses:\n  - go-interview\n")
			},
			wantSub: "title",
		},
		{
			name: "empty courses list",
			mutate: func(m fstest.MapFS) {
				m["algo-interview/catalog.yaml"].Data = []byte("slug: algo-interview\ntitle: t\n")
			},
			wantSub: "at least one course",
		},
		{
			name: "course parse error propagates",
			mutate: func(m fstest.MapFS) {
				// Break one of the courses inside the catalog.
				m["algo-interview/go-interview/course.yaml"].Data = []byte("schema_version: 99\nslug: go-interview\ntitle: t\ntracks:\n  - week-1\n")
			},
			wantSub: "schema_version",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := validCatalog()
			tt.mutate(m)
			_, err := NewParser(m).ParseCatalog("algo-interview")
			if err == nil {
				t.Fatalf("expected error containing %q, got nil", tt.wantSub)
			}
			if !strings.Contains(err.Error(), tt.wantSub) {
				t.Fatalf("error %q does not contain %q", err.Error(), tt.wantSub)
			}
		})
	}
}

func TestParserParseUnitCatchesDiskError(t *testing.T) {
	m := validCourse()
	delete(m, "go-interview/week-1/slices/02-chunk/chunk/go/template.go")

	// A caller that only cares about validity ignores the parsed value.
	_, err := NewParser(m).ParseUnit("go-interview/week-1/slices/02-chunk")
	if err == nil || !strings.Contains(err.Error(), "file not found") {
		t.Fatalf("expected file-not-found error, got: %v", err)
	}
}
