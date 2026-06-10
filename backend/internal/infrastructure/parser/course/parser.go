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
func (p *Parser) ParseCourse(root string) (*domain.Course, error) {
	fsys := p.fsys
	c := &domain.Course{}
	if err := readManifest(fsys, path.Join(root, "course.yaml"), c); err != nil {
		return nil, err
	}

	var errs []error
	collect := func(err error) {
		if err != nil {
			errs = append(errs, err)
		}
	}

	collect(validateCourse(c))
	for _, err := range validateNoOrphanDirs(fsys, root, path.Join(root, "course.yaml"), c.TrackSlugs) {
		collect(err)
	}

	for _, ts := range c.TrackSlugs {
		t, err := parseTrack(fsys, path.Join(root, ts), ts)
		if err != nil {
			collect(err)
			continue
		}
		c.Tracks = append(c.Tracks, t)
	}

	if len(errs) > 0 {
		return nil, errors.Join(errs...)
	}
	return c, nil
}

func parseTrack(fsys fs.FS, dir, slug string) (*domain.Track, error) {
	t := &domain.Track{}
	manifest := path.Join(dir, "track.yaml")
	if err := readManifest(fsys, manifest, t); err != nil {
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
	errs = append(errs, validateNoOrphanDirs(fsys, dir, manifest, t.TopicSlugs)...)

	for _, ps := range t.TopicSlugs {
		p, err := parseTopic(fsys, path.Join(dir, ps), ps)
		if err != nil {
			errs = append(errs, err)
			continue
		}
		t.Topics = append(t.Topics, p)
	}

	if len(errs) > 0 {
		return nil, errors.Join(errs...)
	}
	return t, nil
}

func parseTopic(fsys fs.FS, dir, slug string) (*domain.Topic, error) {
	tp := &domain.Topic{}
	manifest := path.Join(dir, "topic.yaml")
	if err := readManifest(fsys, manifest, tp); err != nil {
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
	errs = append(errs, validateNoOrphanDirs(fsys, dir, manifest, tp.UnitSlugs)...)

	for _, us := range tp.UnitSlugs {
		u, err := parseUnit(fsys, path.Join(dir, us), us)
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

func parseUnit(fsys fs.FS, dir, slug string) (*domain.Unit, error) {
	u := &domain.Unit{}
	manifest := path.Join(dir, "unit.yaml")
	if err := readManifest(fsys, manifest, u); err != nil {
		return nil, err
	}

	var errs []error
	if err := validateSlug(manifest, slug, u.Slug); err != nil {
		errs = append(errs, err)
	}
	if u.Title == "" {
		errs = append(errs, errField(manifest, "title", "is required"))
	}

	errs = append(errs, validateUnitContent(fsys, dir, manifest, u)...)
	// A unit's subfolders are its task folders plus the optional assets folder.
	errs = append(errs, validateNoOrphanDirs(fsys, dir, manifest, u.TaskSlugs, "assets")...)

	for _, ks := range u.TaskSlugs {
		tk, err := parseTask(fsys, path.Join(dir, ks), ks)
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

func parseTask(fsys fs.FS, dir, slug string) (*domain.Task, error) {
	tk := &domain.Task{}
	manifest := path.Join(dir, "task.yaml")
	if err := readManifest(fsys, manifest, tk); err != nil {
		return nil, err
	}

	if errs := validateTask(fsys, dir, manifest, slug, tk); len(errs) > 0 {
		return nil, errors.Join(errs...)
	}
	return tk, nil
}

// readManifest decodes YAML into dst; unknown fields are rejected.
func readManifest(fsys fs.FS, name string, dst any) error {
	data, err := fs.ReadFile(fsys, name)
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
	fsys := p.fsys
	c := &domain.Catalog{}
	manifest := path.Join(root, "catalog.yaml")
	if err := readManifest(fsys, manifest, c); err != nil {
		return nil, err
	}

	var errs []error
	collect := func(err error) {
		if err != nil {
			errs = append(errs, err)
		}
	}

	if c.Slug == "" {
		collect(errField(manifest, "slug", "is required"))
	} else if slug := path.Base(root); c.Slug != slug {
		collect(errField(manifest, "slug",
			"must equal folder name "+quote(slug)+", got "+quote(c.Slug)))
	}
	if c.Title == "" {
		collect(errField(manifest, "title", "is required"))
	}
	if len(c.CourseSlugs) == 0 {
		collect(errField(manifest, "courses", "must list at least one course"))
	}

	for _, cs := range c.CourseSlugs {
		course, err := p.ParseCourse(path.Join(root, cs))
		if err != nil {
			collect(err)
			continue
		}
		c.Courses = append(c.Courses, course)
	}

	if len(errs) > 0 {
		return nil, errors.Join(errs...)
	}
	return c, nil
}

func (p *Parser) ParseTrack(dir string) (*domain.Track, error) {
	return parseTrack(p.fsys, dir, path.Base(dir))
}

func (p *Parser) ParseTopic(dir string) (*domain.Topic, error) {
	return parseTopic(p.fsys, dir, path.Base(dir))
}

func (p *Parser) ParseUnit(dir string) (*domain.Unit, error) {
	return parseUnit(p.fsys, dir, path.Base(dir))
}

func (p *Parser) ParseTask(dir string) (*domain.Task, error) {
	return parseTask(p.fsys, dir, path.Base(dir))
}
