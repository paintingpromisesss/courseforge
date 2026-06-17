package course

import (
	"bytes"
	"errors"
	"io/fs"
	"path"

	"github.com/paintingpromisesss/courseforge/internal/domain"
	"gopkg.in/yaml.v3"
)

var utf8BOM = []byte{0xEF, 0xBB, 0xBF}

// Parser reads course trees from an fs.FS. Construct with NewParser.
type Parser struct {
	fsys fs.FS
}

// NewParser returns a Parser backed by fsys (use os.DirFS for real FS, fstest.MapFS in tests).
func NewParser(fsys fs.FS) *Parser {
	return &Parser{fsys: fsys}
}

// Parse is shorthand for NewParser(fsys).ParseCourse(root).
func Parse(fsys fs.FS, root string) (*domain.Course, error) {
	return NewParser(fsys).ParseCourse(root)
}

// ParseCourse reads and validates the course tree rooted at root.
// Errors are *Error values joined into a single error.
//
// The parse methods below do the IO and recursion (fsys comes from the
// receiver); the pure check helpers live in validate.go and take fsys explicitly.
func (p *Parser) ParseCourse(root string) (*domain.Course, error) {
	c := &domain.Course{}
	manifest := path.Join(root, "course.yaml")
	if err := p.readManifest(manifest, c); err != nil {
		return nil, err
	}

	var errs []error
	if err := validateCourse(c); err != nil {
		errs = append(errs, err)
	}
	errs = append(errs, validateNoOrphanDirs(p.fsys, root, manifest, c.TrackSlugs)...)

	for _, ts := range c.TrackSlugs {
		t, err := p.parseTrack(path.Join(root, ts), ts)
		if err != nil {
			errs = append(errs, err)
			continue
		}
		c.Tracks = append(c.Tracks, t)
	}

	if len(errs) > 0 {
		return nil, errors.Join(errs...)
	}
	return c, nil
}

func (p *Parser) parseTrack(dir, slug string) (*domain.Track, error) {
	t := &domain.Track{}
	manifest := path.Join(dir, "track.yaml")
	if err := p.readManifest(manifest, t); err != nil {
		return nil, err
	}

	var errs []error
	if err := validateSlug(manifest, slug, t.Slug); err != nil {
		errs = append(errs, err)
	}
	if t.Title == "" {
		errs = append(errs, errField(manifest, "title", "is required"))
	}
	if len(t.TopicSlugs) == 0 {
		errs = append(errs, errField(manifest, "topics", "must list at least one topic"))
	}
	errs = append(errs, validateNoOrphanDirs(p.fsys, dir, manifest, t.TopicSlugs)...)

	for _, ts := range t.TopicSlugs {
		tp, err := p.parseTopic(path.Join(dir, ts), ts)
		if err != nil {
			errs = append(errs, err)
			continue
		}
		t.Topics = append(t.Topics, tp)
	}

	if len(errs) > 0 {
		return nil, errors.Join(errs...)
	}
	return t, nil
}

func (p *Parser) parseTopic(dir, slug string) (*domain.Topic, error) {
	tp := &domain.Topic{}
	manifest := path.Join(dir, "topic.yaml")
	if err := p.readManifest(manifest, tp); err != nil {
		return nil, err
	}

	var errs []error
	if err := validateSlug(manifest, slug, tp.Slug); err != nil {
		errs = append(errs, err)
	}
	if tp.Title == "" {
		errs = append(errs, errField(manifest, "title", "is required"))
	}
	if len(tp.UnitSlugs) == 0 {
		errs = append(errs, errField(manifest, "units", "must list at least one unit"))
	}
	errs = append(errs, validateNoOrphanDirs(p.fsys, dir, manifest, tp.UnitSlugs)...)

	for _, us := range tp.UnitSlugs {
		u, err := p.parseUnit(path.Join(dir, us), us)
		if err != nil {
			errs = append(errs, err)
			continue
		}
		tp.Units = append(tp.Units, u)
	}

	if len(errs) > 0 {
		return nil, errors.Join(errs...)
	}
	return tp, nil
}

func (p *Parser) parseUnit(dir, slug string) (*domain.Unit, error) {
	u := &domain.Unit{}
	manifest := path.Join(dir, "unit.yaml")
	if err := p.readManifest(manifest, u); err != nil {
		return nil, err
	}

	var errs []error
	if err := validateSlug(manifest, slug, u.Slug); err != nil {
		errs = append(errs, err)
	}
	if u.Title == "" {
		errs = append(errs, errField(manifest, "title", "is required"))
	}

	errs = append(errs, validateUnitContent(p.fsys, dir, manifest, u)...)
	// A unit's subfolders are its task folders plus the optional assets folder.
	errs = append(errs, validateNoOrphanDirs(p.fsys, dir, manifest, u.TaskSlugs, "assets")...)

	for _, ks := range u.TaskSlugs {
		tk, err := p.parseTask(path.Join(dir, ks), ks)
		if err != nil {
			errs = append(errs, err)
			continue
		}
		u.Tasks = append(u.Tasks, tk)
	}

	if len(errs) > 0 {
		return nil, errors.Join(errs...)
	}
	return u, nil
}

func (p *Parser) parseTask(dir, slug string) (*domain.Task, error) {
	tk := &domain.Task{}
	manifest := path.Join(dir, "task.yaml")
	if err := p.readManifest(manifest, tk); err != nil {
		return nil, err
	}

	if errs := validateTask(p.fsys, dir, manifest, slug, tk); len(errs) > 0 {
		return nil, errors.Join(errs...)
	}
	return tk, nil
}

// readManifest decodes YAML into dst; unknown fields are rejected.
func (p *Parser) readManifest(name string, dst any) error {
	data, err := fs.ReadFile(p.fsys, name)
	if err != nil {
		return errAt(name, "cannot read manifest: "+err.Error())
	}
	data = bytes.TrimPrefix(data, utf8BOM)

	dec := yaml.NewDecoder(bytes.NewReader(data))
	dec.KnownFields(true)
	if err := dec.Decode(dst); err != nil {
		return errAt(name, "invalid YAML: "+err.Error())
	}
	return nil
}

// ParseCatalog reads and validates the catalog at root, including all listed courses.
func (p *Parser) ParseCatalog(root string) (*domain.Catalog, error) {
	c := &domain.Catalog{}
	manifest := path.Join(root, "catalog.yaml")
	if err := p.readManifest(manifest, c); err != nil {
		return nil, err
	}

	errs := validateCatalogMeta(manifest, root, c)
	if len(c.CourseSlugs) == 0 {
		errs = append(errs, errField(manifest, "courses", "must list at least one course"))
	}

	for _, cs := range c.CourseSlugs {
		course, err := p.ParseCourse(path.Join(root, cs))
		if err != nil {
			errs = append(errs, err)
			continue
		}
		c.Courses = append(c.Courses, course)
	}

	if len(errs) > 0 {
		return nil, errors.Join(errs...)
	}
	return c, nil
}

// ParseCatalogManifest reads and validates only the catalog.yaml at root,
// without resolving its course references. Reference-style catalogs list course
// slugs that live elsewhere on disk, so courses are resolved later against the
// global registry. An empty course list is allowed (a freshly created group).
func (p *Parser) ParseCatalogManifest(root string) (*domain.Catalog, error) {
	c := &domain.Catalog{}
	manifest := path.Join(root, "catalog.yaml")
	if err := p.readManifest(manifest, c); err != nil {
		return nil, err
	}

	errs := validateCatalogMeta(manifest, root, c)
	if len(errs) > 0 {
		return nil, errors.Join(errs...)
	}
	return c, nil
}

func (p *Parser) ParseTrack(dir string) (*domain.Track, error) {
	return p.parseTrack(dir, path.Base(dir))
}

func (p *Parser) ParseTopic(dir string) (*domain.Topic, error) {
	return p.parseTopic(dir, path.Base(dir))
}

func (p *Parser) ParseUnit(dir string) (*domain.Unit, error) {
	return p.parseUnit(dir, path.Base(dir))
}

func (p *Parser) ParseTask(dir string) (*domain.Task, error) {
	return p.parseTask(dir, path.Base(dir))
}
