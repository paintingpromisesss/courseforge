package di

import (
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"

	_ "github.com/paintingpromisesss/courseforge/docs"
	"github.com/paintingpromisesss/courseforge/internal/api"
	"github.com/paintingpromisesss/courseforge/internal/api/handlers"
	"github.com/paintingpromisesss/courseforge/internal/application/service"

	"github.com/paintingpromisesss/courseforge/internal/infrastructure/parser/course"
	"github.com/paintingpromisesss/courseforge/internal/infrastructure/repo"
	"github.com/paintingpromisesss/courseforge/internal/infrastructure/runner"
	"github.com/paintingpromisesss/courseforge/logger"
)

func Run(cfg *Config) error {
	for _, dir := range []string{cfg.DataDir, cfg.CoursesDir, cfg.RunnersDir} {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return fmt.Errorf("create dir %s: %w", dir, err)
		}
	}

	logger := logger.New()

	courses, err := course.LoadAll(cfg.CoursesDir)
	if err != nil {
		return fmt.Errorf("load courses: %w", err)
	}
	log.Printf("loaded %d course(s)", len(courses))

	r := runner.New()
	if err := r.UseFile(cfg.RunnersJSON); err != nil {
		return fmt.Errorf("load runners: %w", err)
	}
	pr := repo.NewFileProgressRepository(cfg.CoursesDir)

	ps := service.NewProgressService(pr, logger)

	sr, err := repo.NewSubmissionRepository(cfg.DBPath)
	if err != nil {
		return fmt.Errorf("open submissions db: %w", err)
	}
	defer sr.Close()

	ss := service.NewSubmissionService(sr, logger)

	h := handlers.New(cfg.CoursesDir, cfg.RunnersDir, courses, r, ps, ss)

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
