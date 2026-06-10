package domain

import (
	"errors"
	"fmt"
	"strconv"
)

const schemaVersion = 1

type validationError struct {
	Path  string
	Field string
	Msg   string
}

func (e *validationError) Error() string {
	switch {
	case e.Path != "" && e.Field != "":
		return fmt.Sprintf("%s: field %q: %s", e.Path, e.Field, e.Msg)
	case e.Path != "":
		return fmt.Sprintf("%s: %s", e.Path, e.Msg)
	default:
		return e.Msg
	}
}

func errAt(path, msg string) error {
	return &validationError{Path: path, Msg: msg}
}

func errField(path, field, msg string) error {
	return &validationError{Path: path, Field: field, Msg: msg}
}

func quote(s string) string { return strconv.Quote(s) }

func itoa(n int) string { return strconv.Itoa(n) }

func validateCourse(c *Course) error {
	manifest := "course.yaml"
	if c.SchemaVersion != schemaVersion {
		return errField(manifest, "schema_version",
			"unsupported version; this parser supports "+itoa(schemaVersion))
	}
	if c.Slug == "" {
		return errField(manifest, "slug", "is required")
	}
	if c.Title == "" {
		return errField(manifest, "title", "is required")
	}
	if len(c.TrackSlugs) == 0 {
		return errField(manifest, "tracks", "must list at least one track")
	}
	return nil
}

func (cat *Catalog) Validate() error {
	const manifest = "catalog.yaml"
	var errs []error
	if cat.Slug == "" {
		errs = append(errs, errField(manifest, "slug", "is required"))
	}
	if cat.Title == "" {
		errs = append(errs, errField(manifest, "title", "is required"))
	}
	if len(cat.CourseSlugs) == 0 {
		errs = append(errs, errField(manifest, "courses", "must list at least one course"))
	}
	for _, c := range cat.Courses {
		if err := c.Validate(); err != nil {
			errs = append(errs, err)
		}
	}
	return join(errs)
}

func (c *Course) Validate() error {
	var errs []error
	if err := validateCourse(c); err != nil {
		errs = append(errs, err)
	}
	for _, t := range c.Tracks {
		if err := t.Validate(); err != nil {
			errs = append(errs, err)
		}
	}
	return join(errs)
}

func (t *Track) Validate() error {
	const manifest = "track.yaml"
	var errs []error
	if t.Slug == "" {
		errs = append(errs, errField(manifest, "slug", "is required"))
	}
	if t.Title == "" {
		errs = append(errs, errField(manifest, "title", "is required"))
	}
	if len(t.TopicSlugs) == 0 {
		errs = append(errs, errField(manifest, "topics", "must list at least one topic"))
	}
	for _, tp := range t.Topics {
		if err := tp.Validate(); err != nil {
			errs = append(errs, err)
		}
	}
	return join(errs)
}

func (tp *Topic) Validate() error {
	const manifest = "topic.yaml"
	var errs []error
	if tp.Slug == "" {
		errs = append(errs, errField(manifest, "slug", "is required"))
	}
	if tp.Title == "" {
		errs = append(errs, errField(manifest, "title", "is required"))
	}
	if len(tp.UnitSlugs) == 0 {
		errs = append(errs, errField(manifest, "units", "must list at least one unit"))
	}
	for _, u := range tp.Units {
		if err := u.Validate(); err != nil {
			errs = append(errs, err)
		}
	}
	return join(errs)
}

func (u *Unit) Validate() error {
	const manifest = "unit.yaml"
	var errs []error
	if u.Slug == "" {
		errs = append(errs, errField(manifest, "slug", "is required"))
	}
	if u.Title == "" {
		errs = append(errs, errField(manifest, "title", "is required"))
	}
	if u.Theory == "" && len(u.TaskSlugs) == 0 {
		errs = append(errs, errAt(manifest, "unit is empty: declare theory, tasks, or both"))
	}
	for _, tk := range u.Tasks {
		if err := tk.Validate(); err != nil {
			errs = append(errs, err)
		}
	}
	return join(errs)
}

func (tk *Task) Validate() error {
	const manifest = "task.yaml"
	var errs []error
	if tk.Slug == "" {
		errs = append(errs, errField(manifest, "slug", "is required"))
	}
	if tk.Title == "" {
		errs = append(errs, errField(manifest, "title", "is required"))
	}
	if tk.Statement == "" {
		errs = append(errs, errField(manifest, "statement", "is required"))
	}
	if len(tk.Languages) == 0 {
		errs = append(errs, errField(manifest, "languages", "must define at least one language"))
	}
	for lang, l := range tk.Languages {
		if l.Template == "" {
			errs = append(errs, errField(manifest, "languages."+lang+".template", "is required"))
		}
	}
	return join(errs)
}

func join(errs []error) error {
	if len(errs) == 0 {
		return nil
	}
	return errors.Join(errs...)
}
