// @title CourseForge API
// @version 1.0
// @description Self-hosted programming learning platform
// @host localhost:8080
// @BasePath /api
// @schemes http
package main

import (
	"log"
	"net/http"

	_ "github.com/paintingpromisesss/courseforge/docs"
	"github.com/paintingpromisesss/courseforge/internal/api"
	"github.com/paintingpromisesss/courseforge/internal/config"
	"github.com/paintingpromisesss/courseforge/internal/course"
	"github.com/paintingpromisesss/courseforge/internal/progress"
	"github.com/paintingpromisesss/courseforge/internal/runner"
)

func main() {
	cfg := config.Load()

	courses, err := course.LoadAll(cfg.CoursesDir)
	if err != nil {
		log.Fatalf("load courses: %v", err)
	}
	log.Printf("loaded %d course(s)", len(courses))

	r := runner.New()
	if err := r.UseFile(cfg.CoursesDir + "/runners.json"); err != nil {
		log.Fatalf("load runners: %v", err)
	}
	ps := progress.NewStore(cfg.CoursesDir)
	h := api.New(cfg.CoursesDir, courses, r, ps)

	log.Printf("listening on %s", cfg.Addr)
	log.Printf("swagger UI: http://localhost%s/swagger/index.html", cfg.Addr)
	if err := http.ListenAndServe(cfg.Addr, api.NewRouter(h)); err != nil {
		log.Fatal(err)
	}
}
