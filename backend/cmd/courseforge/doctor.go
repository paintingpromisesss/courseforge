package main

import (
	"context"
	"flag"
	"fmt"
	"os"

	"github.com/jedib0t/go-pretty/v6/table"
	"github.com/jedib0t/go-pretty/v6/text"
	"github.com/paintingpromisesss/courseforge/internal/config"
	"github.com/paintingpromisesss/courseforge/internal/infrastructure/runner"
)

type langCheck struct {
	key   string
	label string
}

// doctorLangs lists the runners checked, in the same order as the README's
// toolchain table.
var doctorLangs = []langCheck{
	{"go", "Go"},
	{"python3", "Python"},
	{"javascript", "Node.js"},
	{"cpp", "C++"},
	{"java", "Java"},
	{"csharp", "C#/.NET"},
	{"postgres", "PostgreSQL"},
}

func runDoctor(args []string) {
	fs := flag.NewFlagSet("doctor", flag.ExitOnError)
	dataDir := fs.String("data-dir", "./data", "directory for app state (where runners.json lives)")
	fs.Parse(args)

	r := runner.New()
	if err := r.UseFile(config.DefaultRunnersJSON(*dataDir)); err != nil {
		fmt.Fprintf(os.Stderr, "courseforge doctor: load runners: %v\n", err)
		os.Exit(1)
	}

	ctx := context.Background()

	fmt.Println()
	t := table.NewWriter()
	t.SetOutputMirror(os.Stdout)
	t.SetStyle(table.StyleRounded)
	t.AppendHeader(table.Row{"Runner", "Status", "Version", "Path", "Note"})
	t.SetColumnConfigs([]table.ColumnConfig{
		{Number: 1, Colors: text.Colors{text.FgCyan, text.Bold}},
	})

	broken := 0
	for _, l := range doctorLangs {
		res := r.Detect(ctx, l.key)
		if res.Status == runner.StatusBroken {
			broken++
		}
		t.AppendRow(table.Row{l.label, statusCell(res.Status), res.Version, res.Path, res.Message})
	}
	t.Render()
	fmt.Println()

	if broken > 0 {
		os.Exit(1)
	}
}

func statusCell(s runner.DetectStatus) string {
	switch s {
	case runner.StatusOK:
		return text.FgGreen.Sprint("ok")
	case runner.StatusBroken:
		return text.FgYellow.Sprint("broken")
	default:
		return text.FgHiBlack.Sprint("missing")
	}
}
