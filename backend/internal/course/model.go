package course

// Catalog groups related courses; identified by catalog.yaml in the folder.
type Catalog struct {
	Slug        string   `yaml:"slug"`
	Title       string   `yaml:"title"`
	Description string   `yaml:"description"`
	CourseSlugs []string `yaml:"courses"`

	Courses []*Course `yaml:"-"` // populated in CourseSlugs order
}

// Course is the root of a course tree.
type Course struct {
	SchemaVersion int      `yaml:"schema_version"`
	Slug          string   `yaml:"slug"`
	Title         string   `yaml:"title"`
	Description   string   `yaml:"description"`
	Language      string   `yaml:"language"`
	TrackSlugs    []string `yaml:"tracks"`

	Dir    string   `yaml:"-"` // path relative to coursesDir, set by LoadAll
	Tracks []*Track `yaml:"-"` // populated in TrackSlugs order
}

// Track is a week or major module.
type Track struct {
	Slug        string   `yaml:"slug"`
	Title       string   `yaml:"title"`
	Description string   `yaml:"description"`
	TopicSlugs  []string `yaml:"topics"`

	Topics []*Topic `yaml:"-"`
}

// Topic is a section within a track.
type Topic struct {
	Slug        string   `yaml:"slug"`
	Title       string   `yaml:"title"`
	Description string   `yaml:"description"`
	UnitSlugs   []string `yaml:"units"`

	Units []*Unit `yaml:"-"`
}

// Unit is a theory document and/or an ordered list of tasks; at least one must be set.
type Unit struct {
	Slug      string   `yaml:"slug"`
	Title     string   `yaml:"title"`
	Theory    string   `yaml:"theory"` // path relative to unit folder, or ""
	TaskSlugs []string `yaml:"tasks"`

	Tasks []*Task `yaml:"-"`
}

// Task is a coding exercise: one shared statement and per-language file sets.
type Task struct {
	Slug      string              `yaml:"slug"`
	Title     string              `yaml:"title"`
	Statement string              `yaml:"statement"` // path relative to task folder
	Languages map[string]Language `yaml:"languages"`
	Limits    *Limits             `yaml:"limits"`
}

// Language holds files for one language; paths are relative to the language subfolder (e.g. go/).
// Solution and Tests are optional.
type Language struct {
	Template string `yaml:"template"`
	Solution string `yaml:"solution"`
	Tests    string `yaml:"tests"`
}

// Limits overrides global sandbox defaults for a heavy task.
type Limits struct {
	TimeoutSec int `yaml:"timeout_sec"`
	MemoryMB   int `yaml:"memory_mb"`
}
