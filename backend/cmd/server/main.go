// @title CourseForge API
// @version 1.0
// @description Self-hosted programming learning platform
// @host localhost:8080
// @BasePath /api
// @schemes http
package main

import (
	"log"

	"github.com/paintingpromisesss/courseforge/internal/di"
)

func main() {
	cfg := di.Load()
	if err := di.Run(cfg); err != nil {
		log.Fatal(err)
	}
}
