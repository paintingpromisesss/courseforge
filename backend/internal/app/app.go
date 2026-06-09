package app

import (
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"

	_ "github.com/paintingpromisesss/courseforge/docs"
	"github.com/paintingpromisesss/courseforge/internal/api"
	"github.com/paintingpromisesss/courseforge/internal/config"
	"github.com/paintingpromisesss/courseforge/internal/course"
	"github.com/paintingpromisesss/courseforge/internal/progress"
	"github.com/paintingpromisesss/courseforge/internal/runner"
	"github.com/paintingpromisesss/courseforge/internal/submission"
)

func Run(cfg *config.Config) error {
	for _, dir := range []string{cfg.DataDir, cfg.CoursesDir, cfg.RunnersDir} {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return fmt.Errorf("create dir %s: %w", dir, err)
		}
	}

	courses, err := course.LoadAll(cfg.CoursesDir)
	if err != nil {
		return fmt.Errorf("load courses: %w", err)
	}
	log.Printf("loaded %d course(s)", len(courses))

	r := runner.New()
	if err := r.UseFile(cfg.RunnersJSON); err != nil {
		return fmt.Errorf("load runners: %w", err)
	}
	ps := progress.NewStore(cfg.CoursesDir)

	ss, err := submission.New(cfg.DBPath)
	if err != nil {
		return fmt.Errorf("open submissions db: %w", err)
	}
	defer ss.Close()

	h := api.New(cfg.CoursesDir, cfg.RunnersDir, courses, r, ps, ss)

	router, err := api.NewRouter(h, api.RouterOptions{FrontendDir: cfg.FrontendDir})
	if err != nil {
		return err
	}

	log.Printf("listening on http://%s", displayAddr(cfg.Addr))
	log.Printf("swagger UI: http://%s/swagger/index.html", displayAddr(cfg.Addr))
	if cfg.FrontendDir != "" {
		log.Printf("frontend dir: %s", cfg.FrontendDir)
	}

	return http.ListenAndServe(cfg.Addr, router)
}

func displayAddr(addr string) string {
	switch {
	case strings.HasPrefix(addr, ":"):
		return "localhost" + addr
	case strings.HasPrefix(addr, "0.0.0.0:"):
		return "localhost:" + strings.TrimPrefix(addr, "0.0.0.0:")
	default:
		return addr
	}
}
