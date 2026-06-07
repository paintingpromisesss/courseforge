package main

import (
	"fmt"
	"io/fs"
	"os"

	"github.com/jedib0t/go-pretty/v6/table"
	"github.com/jedib0t/go-pretty/v6/text"
	"github.com/paintingpromisesss/courseforge/internal/course"
)

func main() {
	if len(os.Args) < 3 {
		fmt.Fprintln(os.Stderr, "usage: parse-check <courses-dir> <slug>")
		os.Exit(2)
	}
	dir, slug := os.Args[1], os.Args[2]
	fsys := os.DirFS(dir)

	if isCatalog(fsys, slug) {
		p := course.NewParser(fsys)
		cat, err := p.ParseCatalog(slug)
		if err != nil {
			fmt.Fprintln(os.Stderr, text.FgRed.Sprint("PARSE ERRORS:"))
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		printCatalog(cat)
	} else {
		c, err := course.Parse(fsys, slug)
		if err != nil {
			fmt.Fprintln(os.Stderr, text.FgRed.Sprint("PARSE ERRORS:"))
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		printCourse(c)
	}
}

func isCatalog(fsys fs.FS, slug string) bool {
	f, err := fsys.Open(slug + "/catalog.yaml")
	if err != nil {
		return false
	}
	f.Close()
	return true
}

// courseStats holds aggregated counts for one course.
type courseStats struct {
	tracks int
	topics int
	units  int
	theory int
	tasks  int
}

func calcStats(c *course.Course) courseStats {
	s := courseStats{tracks: len(c.Tracks)}
	for _, tr := range c.Tracks {
		s.topics += len(tr.Topics)
		for _, tp := range tr.Topics {
			s.units += len(tp.Units)
			for _, u := range tp.Units {
				s.tasks += len(u.Tasks)
				if u.Theory != "" {
					s.theory++
				}
			}
		}
	}
	return s
}

func printCatalog(cat *course.Catalog) {
	fmt.Println()
	fmt.Printf("  %s  %s\n",
		text.Bold.Sprint(cat.Title),
		text.FgHiBlack.Sprintf("(catalog: %s, %d courses)", cat.Slug, len(cat.Courses)),
	)
	fmt.Println()

	t := table.NewWriter()
	t.SetOutputMirror(os.Stdout)
	t.SetStyle(table.StyleRounded)
	t.AppendHeader(table.Row{"Course", "Tracks", "Topics", "Units", "Theory", "Tasks"})
	t.SetColumnConfigs([]table.ColumnConfig{
		{Number: 1, Colors: text.Colors{text.FgCyan, text.Bold}},
		{Number: 2, Align: text.AlignRight},
		{Number: 3, Align: text.AlignRight},
		{Number: 4, Align: text.AlignRight},
		{Number: 5, Align: text.AlignRight},
		{Number: 6, Align: text.AlignRight},
	})

	for _, c := range cat.Courses {
		s := calcStats(c)
		t.AppendRow(table.Row{c.Title, s.tracks, s.topics, s.units, s.theory, s.tasks})
	}

	t.Render()
	fmt.Println()
}

func printCourse(c *course.Course) {
	fmt.Println()
	fmt.Printf("  %s  %s\n",
		text.Bold.Sprint(c.Title),
		text.FgHiBlack.Sprintf("(%s, schema v%d)", c.Slug, c.SchemaVersion),
	)
	fmt.Println()

	t := table.NewWriter()
	t.SetOutputMirror(os.Stdout)
	t.SetStyle(table.StyleRounded)
	t.Style().Options.SeparateRows = false

	t.AppendHeader(table.Row{"Track", "Topic", "Units", "Theory", "Tasks"})
	t.SetColumnConfigs([]table.ColumnConfig{
		{Number: 1, Colors: text.Colors{text.FgCyan, text.Bold}},
		{Number: 2, Colors: text.Colors{text.FgYellow}},
		{Number: 3, Align: text.AlignRight},
		{Number: 4, Align: text.AlignRight},
		{Number: 5, Align: text.AlignRight},
	})

	for _, tr := range c.Tracks {
		for i, tp := range tr.Topics {
			theory, tasks := 0, 0
			for _, u := range tp.Units {
				tasks += len(u.Tasks)
				if u.Theory != "" {
					theory++
				}
			}
			trackCell := ""
			if i == 0 {
				trackCell = tr.Title
			}
			t.AppendRow(table.Row{trackCell, tp.Title, len(tp.Units), theory, tasks})
		}
		t.AppendSeparator()
	}

	t.Render()
	fmt.Println()
}
