package dto

import (
	"sort"

	"github.com/paintingpromisesss/courseforge/internal/domain"
)

type CourseItem struct {
	Slug        string `json:"slug"`
	Title       string `json:"title"`
	Description string `json:"description"`
	Language    string `json:"language"`
	CatalogSlug string `json:"catalog_slug,omitempty"`
	TheoryCount int    `json:"theory_count"`
	TaskCount   int    `json:"task_count"`
}

type CatalogItem struct {
	Slug        string       `json:"slug"`
	Title       string       `json:"title"`
	Description string       `json:"description"`
	Courses     []CourseItem `json:"courses"`
}

type CourseDetail struct {
	Slug        string      `json:"slug"`
	Title       string      `json:"title"`
	Description string      `json:"description"`
	Language    string      `json:"language"`
	Tracks      []TrackItem `json:"tracks"`
}

type TrackItem struct {
	Slug        string      `json:"slug"`
	Title       string      `json:"title"`
	Description string      `json:"description"`
	Topics      []TopicItem `json:"topics"`
}

type TopicItem struct {
	Slug        string     `json:"slug"`
	Title       string     `json:"title"`
	Description string     `json:"description"`
	Units       []UnitItem `json:"units"`
}

type UnitItem struct {
	Slug      string     `json:"slug"`
	Title     string     `json:"title"`
	HasTheory bool       `json:"has_theory"`
	Tasks     []TaskItem `json:"tasks"`
}

type TaskItem struct {
	Slug      string   `json:"slug"`
	Title     string   `json:"title"`
	Languages []string `json:"languages"`
}

func ToCourseItem(c *domain.Course) CourseItem {
	theory, tasks := 0, 0
	for _, t := range c.Tracks {
		for _, p := range t.Topics {
			for _, u := range p.Units {
				if u.Theory != "" {
					theory++
				}
				tasks += len(u.Tasks)
			}
		}
	}
	return CourseItem{
		Slug:        c.Slug,
		Title:       c.Title,
		Description: c.Description,
		Language:    c.Language,
		TheoryCount: theory,
		TaskCount:   tasks,
	}
}

func ToCatalogItem(cat *domain.Catalog) CatalogItem {
	courses := make([]CourseItem, 0, len(cat.Courses))
	for _, c := range cat.Courses {
		item := ToCourseItem(c)
		item.CatalogSlug = cat.Slug
		courses = append(courses, item)
	}
	sort.Slice(courses, func(i, j int) bool { return courses[i].Slug < courses[j].Slug })
	return CatalogItem{
		Slug:        cat.Slug,
		Title:       cat.Title,
		Description: cat.Description,
		Courses:     courses,
	}
}

func ToCourseDetail(c *domain.Course) CourseDetail {
	tracks := make([]TrackItem, len(c.Tracks))
	for i, t := range c.Tracks {
		topics := make([]TopicItem, len(t.Topics))
		for j, p := range t.Topics {
			units := make([]UnitItem, len(p.Units))
			for k, u := range p.Units {
				tasks := make([]TaskItem, len(u.Tasks))
				for l, task := range u.Tasks {
					langs := make([]string, 0, len(task.Languages))
					for lang := range task.Languages {
						langs = append(langs, lang)
					}
					sort.Strings(langs)
					tasks[l] = TaskItem{Slug: task.Slug, Title: task.Title, Languages: langs}
				}
				units[k] = UnitItem{Slug: u.Slug, Title: u.Title, HasTheory: u.Theory != "", Tasks: tasks}
			}
			topics[j] = TopicItem{Slug: p.Slug, Title: p.Title, Description: p.Description, Units: units}
		}
		tracks[i] = TrackItem{Slug: t.Slug, Title: t.Title, Description: t.Description, Topics: topics}
	}
	return CourseDetail{Slug: c.Slug, Title: c.Title, Description: c.Description, Language: c.Language, Tracks: tracks}
}
