package course

import (
	"fmt"
	"io/fs"
	"os"

	"github.com/paintingpromisesss/courseforge/internal/domain"
)

// LoadOne parses a single course by its directory name inside dir.
func LoadOne(dir, slug string) (*domain.Course, error) {
	fsys := os.DirFS(dir)
	parser := NewParser(fsys)
	c, err := parser.ParseCourse(slug)
	if err != nil {
		return nil, fmt.Errorf("parse course %q: %w", slug, err)
	}
	c.Dir = slug
	return c, nil
}

// LoadAll scans dir for courses and catalogs and returns all courses keyed by slug.
func LoadAll(dir string) (map[string]*domain.Course, error) {
	fsys := os.DirFS(dir)
	entries, err := fs.ReadDir(fsys, ".")
	if err != nil {
		return nil, fmt.Errorf("read courses dir: %w", err)
	}

	parser := NewParser(fsys)
	courses := make(map[string]*domain.Course)

	for _, e := range entries {
		if !e.IsDir() {
			continue
		}

		// catalog.yaml → parse catalog, load all its courses
		if _, err := fs.Stat(fsys, e.Name()+"/catalog.yaml"); err == nil {
			catalog, err := parser.ParseCatalog(e.Name())
			if err != nil {
				return nil, fmt.Errorf("parse catalog %q: %w", e.Name(), err)
			}
			for _, c := range catalog.Courses {
				c.Dir = e.Name() + "/" + c.Slug
				courses[c.Slug] = c
			}
			continue
		}

		// course.yaml → parse single course
		if _, err := fs.Stat(fsys, e.Name()+"/course.yaml"); err == nil {
			c, err := parser.ParseCourse(e.Name())
			if err != nil {
				return nil, fmt.Errorf("parse course %q: %w", e.Name(), err)
			}
			c.Dir = e.Name()
			courses[c.Slug] = c
			continue
		}
	}
	return courses, nil
}
