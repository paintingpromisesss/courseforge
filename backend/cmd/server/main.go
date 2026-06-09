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
	"os"

	_ "github.com/paintingpromisesss/courseforge/docs"
	"github.com/paintingpromisesss/courseforge/internal/api"
	"github.com/paintingpromisesss/courseforge/internal/config"
	"github.com/paintingpromisesss/courseforge/internal/course"
	"github.com/paintingpromisesss/courseforge/internal/progress"
	"github.com/paintingpromisesss/courseforge/internal/runner"
	"github.com/paintingpromisesss/courseforge/internal/submission"
)

func main() {
	cfg := config.Load()

	for _, dir := range []string{cfg.DataDir, cfg.CoursesDir, cfg.RunnersDir} {
		if err := os.MkdirAll(dir, 0755); err != nil {
			log.Fatalf("create dir %s: %v", dir, err)
		}
	}

	courses, err := course.LoadAll(cfg.CoursesDir)
	if err != nil {
		log.Fatalf("load courses: %v", err)
	}
	log.Printf("loaded %d course(s)", len(courses))

	r := runner.New()
	if err := r.UseFile(cfg.RunnersJSON); err != nil {
		log.Fatalf("load runners: %v", err)
	}
	ps := progress.NewStore(cfg.CoursesDir)

	ss, err := submission.New(cfg.DBPath)
	if err != nil {
		log.Fatalf("open submissions db: %v", err)
	}
	defer ss.Close()

	h := api.New(cfg.CoursesDir, cfg.RunnersDir, courses, r, ps, ss)

	log.Printf("listening on %s", cfg.Addr)
	log.Printf("swagger UI: http://localhost%s/swagger/index.html", cfg.Addr)
	if err := http.ListenAndServe(cfg.Addr, api.NewRouter(h)); err != nil {
		log.Fatal(err)
	}
}
