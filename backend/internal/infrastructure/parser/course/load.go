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

// LoadCatalogOne parses a catalog folder by its directory name inside dir.
// It sets Dir on each child course as dirSlug/courseSlug.
func LoadCatalogOne(dir, dirSlug string) (*domain.Catalog, error) {
	fsys := os.DirFS(dir)
	parser := NewParser(fsys)
	cat, err := parser.ParseCatalog(dirSlug)
	if err != nil {
		return nil, fmt.Errorf("parse catalog %q: %w", dirSlug, err)
	}
	cat.Dir = dirSlug
	for _, c := range cat.Courses {
		c.Dir = dirSlug + "/" + c.Slug
	}
	return cat, nil
}

// LoadCatalogManifest parses only a catalog folder's catalog.yaml (no course
// resolution). Used for reference-style catalogs whose courses live elsewhere.
func LoadCatalogManifest(dir, dirSlug string) (*domain.Catalog, error) {
	fsys := os.DirFS(dir)
	cat, err := NewParser(fsys).ParseCatalogManifest(dirSlug)
	if err != nil {
		return nil, fmt.Errorf("parse catalog %q: %w", dirSlug, err)
	}
	cat.Dir = dirSlug
	return cat, nil
}

// LoadNestedCourses loads any course folders physically nested under a catalog
// directory (the legacy layout). Each course's Dir is set to dirSlug/courseSlug.
func LoadNestedCourses(dir, dirSlug string) ([]*domain.Course, error) {
	fsys := os.DirFS(dir)
	parser := NewParser(fsys)
	subEntries, err := fs.ReadDir(fsys, dirSlug)
	if err != nil {
		return nil, err
	}
	var out []*domain.Course
	for _, se := range subEntries {
		if !se.IsDir() {
			continue
		}
		sub := dirSlug + "/" + se.Name()
		if _, err := fs.Stat(fsys, sub+"/course.yaml"); err != nil {
			continue
		}
		c, err := parser.ParseCourse(sub)
		if err != nil {
			return nil, fmt.Errorf("parse course %q: %w", se.Name(), err)
		}
		c.Dir = dirSlug + "/" + c.Slug
		out = append(out, c)
	}
	return out, nil
}

// ResolveCatalogCourses populates cat.Courses from the global course registry,
// in cat.CourseSlugs order. References to missing courses are skipped.
func ResolveCatalogCourses(cat *domain.Catalog, courses map[string]*domain.Course) {
	cat.Courses = make([]*domain.Course, 0, len(cat.CourseSlugs))
	for _, slug := range cat.CourseSlugs {
		if c := courses[slug]; c != nil {
			cat.Courses = append(cat.Courses, c)
		}
	}
}

// LoadAll scans dir for courses and catalogs and returns all courses and
// catalogs keyed by slug. Catalog membership is by reference: a catalog lists
// course slugs that may live as top-level course folders or (legacy) nested
// inside the catalog folder. Resolution happens in a second pass.
func LoadAll(dir string) (map[string]*domain.Course, map[string]*domain.Catalog, error) {
	fsys := os.DirFS(dir)
	entries, err := fs.ReadDir(fsys, ".")
	if err != nil {
		return nil, nil, fmt.Errorf("read courses dir: %w", err)
	}

	parser := NewParser(fsys)
	courses := make(map[string]*domain.Course)
	catalogs := make(map[string]*domain.Catalog)

	// pass 1: load every course (top-level and legacy nested) + catalog manifests
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}

		if _, err := fs.Stat(fsys, e.Name()+"/catalog.yaml"); err == nil {
			catalog, err := parser.ParseCatalogManifest(e.Name())
			if err != nil {
				return nil, nil, fmt.Errorf("parse catalog %q: %w", e.Name(), err)
			}
			catalog.Dir = e.Name()
			catalogs[catalog.Slug] = catalog
			nested, err := LoadNestedCourses(dir, e.Name())
			if err != nil {
				return nil, nil, err
			}
			for _, c := range nested {
				courses[c.Slug] = c
			}
			continue
		}

		if _, err := fs.Stat(fsys, e.Name()+"/course.yaml"); err == nil {
			c, err := parser.ParseCourse(e.Name())
			if err != nil {
				return nil, nil, fmt.Errorf("parse course %q: %w", e.Name(), err)
			}
			c.Dir = e.Name()
			courses[c.Slug] = c
			continue
		}
	}

	// pass 2: resolve catalog course references against the global registry
	for _, cat := range catalogs {
		ResolveCatalogCourses(cat, courses)
	}
	return courses, catalogs, nil
}
