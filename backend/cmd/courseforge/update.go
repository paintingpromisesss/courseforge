package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/paintingpromisesss/courseforge/internal/updater"
)

func runUpdate(args []string) {
	fs := flag.NewFlagSet("update", flag.ExitOnError)
	tag := fs.String("tag", "latest", "release tag to install")
	force := fs.Bool("force", false, "reinstall even if already up to date")
	fs.Parse(args)

	u := updater.New(version, "")
	if err := u.UpdateCLI(os.Stdout, os.Stderr, *tag, *force); err != nil {
		fmt.Fprintf(os.Stderr, "courseforge update: %v\n", err)
		os.Exit(1)
	}
}
