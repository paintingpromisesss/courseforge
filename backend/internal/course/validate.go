package course

import (
	"errors"
	"io/fs"
	"path"
)

const schemaVersion = 1

// validateSlug enforces the slug == folder name invariant.
func validateSlug(manifest, folder, declared string) error {
	if declared == "" {
		return errField(manifest, "slug", "is required")
	}
	if declared != folder {
		return errField(manifest, "slug",
			"must equal folder name "+quote(folder)+", got "+quote(declared))
	}
	return nil
}

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

// validateUnitContent checks the unit is non-empty and the theory file exists.
func validateUnitContent(fsys fs.FS, dir, manifest string, u *Unit) []error {
	var errs []error
	if u.Theory == "" && len(u.TaskSlugs) == 0 {
		errs = append(errs, errAt(manifest, "unit is empty: declare theory, tasks, or both"))
	}
	if u.Theory != "" {
		if !fileExists(fsys, path.Join(dir, u.Theory)) {
			errs = append(errs, errField(manifest, "theory",
				"file not found: "+quote(u.Theory)))
		}
	}
	return errs
}

// validateTask checks slug, statement, and that all declared language files exist.
func validateTask(fsys fs.FS, dir, manifest, folder string, tk *Task) []error {
	var errs []error
	if err := validateSlug(manifest, folder, tk.Slug); err != nil {
		errs = append(errs, err)
	}
	if tk.Title == "" {
		errs = append(errs, errField(manifest, "title", "is required"))
	}

	if tk.Statement == "" {
		errs = append(errs, errField(manifest, "statement", "is required"))
	} else if !fileExists(fsys, path.Join(dir, tk.Statement)) {
		errs = append(errs, errField(manifest, "statement",
			"file not found: "+quote(tk.Statement)))
	}

	if len(tk.Languages) == 0 {
		errs = append(errs, errField(manifest, "languages", "must define at least one language"))
		return errs
	}

	for lang, l := range tk.Languages {
		field := "languages." + lang
		if l.Template == "" {
			errs = append(errs, errField(manifest, field+".template", "is required"))
		} else if !fileExists(fsys, path.Join(dir, lang, l.Template)) {
			errs = append(errs, errField(manifest, field+".template",
				"file not found: "+quote(path.Join(lang, l.Template))))
		}
		// Solution and tests are optional, but if named they must exist.
		if l.Solution != "" && !fileExists(fsys, path.Join(dir, lang, l.Solution)) {
			errs = append(errs, errField(manifest, field+".solution",
				"file not found: "+quote(path.Join(lang, l.Solution))))
		}
		if l.Tests != "" && !fileExists(fsys, path.Join(dir, lang, l.Tests)) {
			errs = append(errs, errField(manifest, field+".tests",
				"file not found: "+quote(path.Join(lang, l.Tests))))
		}
	}
	return errs
}

func fileExists(fsys fs.FS, name string) bool {
	info, err := fs.Stat(fsys, name)
	return err == nil && !info.IsDir()
}

// validateNoOrphanDirs errors on any subdirectory not in declared (or extra allowed names).
func validateNoOrphanDirs(fsys fs.FS, dir, manifest string, declared []string, extra ...string) []error {
	entries, err := fs.ReadDir(fsys, dir)
	if err != nil {
		return []error{errAt(dir, "cannot read directory: "+err.Error())}
	}

	known := make(map[string]bool, len(declared)+len(extra))
	for _, s := range declared {
		known[s] = true
	}
	for _, s := range extra {
		known[s] = true
	}

	var errs []error
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		if !known[e.Name()] {
			errs = append(errs, errAt(manifest,
				"folder "+quote(e.Name())+" exists on disk but is not listed"))
		}
	}
	return errs
}

// Validate methods check in-memory entities structurally; they do not verify that referenced files exist.

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
