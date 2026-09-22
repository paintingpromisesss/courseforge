package dto

import (
	"sort"
	"strconv"
	"strings"

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
	DoneCount   int    `json:"done_count"`
	TheoryDone  int    `json:"theory_done_count"`
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

type VideoSource struct {
	Src  string `json:"src"`
	Size int    `json:"size,omitempty"`
}

type UnitItem struct {
	Slug         string        `json:"slug"`
	Title        string        `json:"title"`
	HasTheory    bool          `json:"has_theory"`
	VideoURL     string        `json:"video_url,omitempty"`
	VideoSources []VideoSource `json:"video_sources,omitempty"`
	Tasks        []TaskItem    `json:"tasks"`
}

type TaskItem struct {
	Slug         string        `json:"slug"`
	Title        string        `json:"title"`
	Difficulty   int           `json:"difficulty,omitempty"`
	Tags         []string      `json:"tags,omitempty"`
	Languages    []string      `json:"languages"`
	EditorialURL string        `json:"editorial_url,omitempty"`
	VideoURL     string        `json:"video_url,omitempty"`
	VideoSources []VideoSource `json:"video_sources,omitempty"`
}

func parseQuality(k string) int {
	k = strings.TrimSpace(strings.TrimSuffix(strings.TrimSuffix(strings.ToLower(k), "p"), "р"))
	val, err := strconv.Atoi(k)
	if err != nil {
		return 0
	}
	return val
}

func BuildVideoSources(primaryURL string, videos map[string]string) ([]VideoSource, string) {
	if len(videos) == 0 {
		if primaryURL == "" {
			return nil, ""
		}
		return []VideoSource{{Src: primaryURL, Size: 1080}}, primaryURL
	}

	sources := make([]VideoSource, 0, len(videos))
	for k, src := range videos {
		src = strings.TrimSpace(src)
		if src == "" {
			continue
		}
		size := parseQuality(k)
		sources = append(sources, VideoSource{
			Src:  src,
			Size: size,
		})
	}

	if primaryURL != "" {
		found := false
		for _, s := range sources {
			if s.Src == primaryURL {
				found = true
				break
			}
		}
		if !found {
			sources = append(sources, VideoSource{Src: primaryURL, Size: 1080})
		}
	}

	sort.Slice(sources, func(i, j int) bool {
		return sources[i].Size > sources[j].Size
	})

	mainURL := primaryURL
	if mainURL == "" && len(sources) > 0 {
		mainURL = sources[0].Src
	}

	return sources, mainURL
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

					taskPrimaryURL := task.VideoURL
					if taskPrimaryURL == "" {
						taskPrimaryURL = task.EditorialURL
					}
					taskVideos := task.Videos
					if len(taskVideos) == 0 {
						taskVideos = task.EditorialVideos
					}
					taskSources, taskVideoURL := BuildVideoSources(taskPrimaryURL, taskVideos)

					tasks[l] = TaskItem{
						Slug:         task.Slug,
						Title:        task.Title,
						Difficulty:   task.Difficulty,
						Tags:         task.Tags,
						Languages:    langs,
						EditorialURL: taskVideoURL,
						VideoURL:     taskVideoURL,
						VideoSources: taskSources,
					}
				}
				unitSources, unitVideoURL := BuildVideoSources(u.VideoURL, u.Videos)
				units[k] = UnitItem{
					Slug:         u.Slug,
					Title:        u.Title,
					HasTheory:    u.Theory != "",
					VideoURL:     unitVideoURL,
					VideoSources: unitSources,
					Tasks:        tasks,
				}
			}
			topics[j] = TopicItem{Slug: p.Slug, Title: p.Title, Description: p.Description, Units: units}
		}
		tracks[i] = TrackItem{Slug: t.Slug, Title: t.Title, Description: t.Description, Topics: topics}
	}
	return CourseDetail{Slug: c.Slug, Title: c.Title, Description: c.Description, Language: c.Language, Tracks: tracks}
}
