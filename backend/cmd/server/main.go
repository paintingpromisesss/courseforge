// @title CourseForge API
// @version 1.0
// @description Self-hosted programming learning platform
// @host localhost:8080
// @BasePath /api
// @schemes http
package main

import (
	"log"
	"os"

	"github.com/paintingpromisesss/courseforge/internal/config"
	"github.com/paintingpromisesss/courseforge/internal/di"
	"github.com/paintingpromisesss/courseforge/internal/mcp"
)

func main() {
	if len(os.Args) > 1 && (os.Args[1] == "mcp" || os.Args[1] == "--mcp") {
		mcp.RunCLI(os.Args[2:])
		return
	}

	cfg := config.Load()
	if err := di.Run(cfg); err != nil {
		log.Fatal(err)
	}
}
