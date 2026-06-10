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
	return CourseItem{
		Slug:        c.Slug,
		Title:       c.Title,
		Description: c.Description,
		Language:    c.Language,
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
