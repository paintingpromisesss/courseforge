package api

import (
	"net/http"
	"os"
	"sort"

	"github.com/paintingpromisesss/courseforge/internal/course"
)

func findTrack(c *course.Course, slug string) *course.Track {
	for _, t := range c.Tracks {
		if t.Slug == slug {
			return t
		}
	}
	return nil
}

func findTopic(t *course.Track, slug string) *course.Topic {
	for _, p := range t.Topics {
		if p.Slug == slug {
			return p
		}
	}
	return nil
}

func findUnit(p *course.Topic, slug string) *course.Unit {
	for _, u := range p.Units {
		if u.Slug == slug {
			return u
		}
	}
	return nil
}

func findTask(u *course.Unit, slug string) *course.Task {
	for _, t := range u.Tasks {
		if t.Slug == slug {
			return t
		}
	}
	return nil
}

func serveTextFile(w http.ResponseWriter, path string) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			http.Error(w, "file not found", http.StatusNotFound)
		} else {
			http.Error(w, "failed to read file", http.StatusInternalServerError)
		}
		return
	}
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	_, _ = w.Write(data)
}

func toCourseDetail(c *course.Course) CourseDetail {
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
