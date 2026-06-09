package api

import "github.com/paintingpromisesss/courseforge/internal/course"

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
