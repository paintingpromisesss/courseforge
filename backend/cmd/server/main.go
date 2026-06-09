// @title CourseForge API
// @version 1.0
// @description Self-hosted programming learning platform
// @host localhost:8080
// @BasePath /api
// @schemes http
package main

import (
	"log"

	"github.com/paintingpromisesss/courseforge/internal/app"
	"github.com/paintingpromisesss/courseforge/internal/config"
)

func main() {
	cfg := config.Load()
	if err := app.Run(cfg); err != nil {
		log.Fatal(err)
	}
}
