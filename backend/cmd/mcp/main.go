package main

import (
	"os"

	"github.com/paintingpromisesss/courseforge/internal/mcp"
)

func main() {
	mcp.RunCLI(os.Args[1:])
}
