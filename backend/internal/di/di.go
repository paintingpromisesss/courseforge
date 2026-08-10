package di

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/paintingpromisesss/courseforge/internal/api"
	"github.com/paintingpromisesss/courseforge/internal/api/handlers"
	"github.com/paintingpromisesss/courseforge/internal/application/service"
	"github.com/paintingpromisesss/courseforge/internal/config"

	"github.com/paintingpromisesss/courseforge/internal/infrastructure/parser/course"
	"github.com/paintingpromisesss/courseforge/internal/infrastructure/repo"
	"github.com/paintingpromisesss/courseforge/internal/infrastructure/runner"
	"github.com/paintingpromisesss/courseforge/internal/tray"
	"github.com/paintingpromisesss/courseforge/logger"
)

func Run(cfg *config.Config) error {
	for _, dir := range []string{cfg.DataDir, cfg.CoursesDir} {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return fmt.Errorf("create dir %s: %w", dir, err)
		}
	}

	logger := logger.New()

	courses, catalogs, err := course.LoadAll(cfg.CoursesDir)
	if err != nil {
		return fmt.Errorf("load courses: %w", err)
	}
	log.Printf("loaded %d course(s) in %d catalog(s)", len(courses), len(catalogs))

	r := runner.New()
	if err := r.UseFile(cfg.RunnersJSON); err != nil {
		return fmt.Errorf("load runners: %w", err)
	}

	// Start the Postgres cluster in the background so initdb/pg_ctl never
	// delay HTTP startup; postgres runs that arrive before it's ready get a
	// clear "cluster is not running" error from the runner.
	pgMgr := runner.NewPostgresManager(filepath.Join(cfg.DataDir, "postgres"))
	pgDone := make(chan struct{})
	var pgStarted bool
	go func() {
		defer close(pgDone)
		if err := pgMgr.Start(context.Background()); err != nil {
			log.Printf("postgres runner disabled: %v", err)
			return
		}
		r.ConfigurePostgres(pgMgr.Host(), pgMgr.Port())
		pgStarted = true
		log.Printf("postgres runner ready on %s:%d", pgMgr.Host(), pgMgr.Port())
	}()
	defer func() {
		<-pgDone
		if pgStarted {
			_ = pgMgr.Stop()
		}
	}()

	pr := repo.NewFileProgressRepository(cfg.CoursesDir)

	ps := service.NewProgressService(pr, logger)

	db, err := repo.NewDB(cfg.DBPath)
	if err != nil {
		return fmt.Errorf("open submissions db: %w", err)
	}
	sr := repo.NewSubmissionRepository(db)
	defer sr.Close()

	ss := service.NewSubmissionService(sr, logger)

	h := handlers.New(cfg.CoursesDir, courses, catalogs, r, ps, ss)

	router, err := api.NewRouter(h, api.RouterOptions{FrontendDir: cfg.FrontendDir})
	if err != nil {
		return err
	}

	srv := &http.Server{Addr: cfg.Addr, Handler: router}

	log.Printf("listening on http://%s", displayAddr(cfg.Addr))
	if swaggerEnabled {
		log.Printf("swagger UI: http://%s/swagger/index.html", displayAddr(cfg.Addr))
	}
	if cfg.FrontendDir != "" {
		log.Printf("frontend dir: %s", cfg.FrontendDir)
	}

	if cfg.EnableTray {
		return runWithTray(cfg, srv)
	}
	return runHeadless(srv)
}

// runWithTray delegates the application lifecycle to the system tray icon.
// systray.Run blocks the main goroutine; the HTTP server starts inside onReady.
func runWithTray(cfg *config.Config, srv *http.Server) error {
	var serverErr error

	tray.Run(displayAddr(cfg.Addr), func() {
		// onServerStart — launch HTTP in a background goroutine.
		go func() {
			if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
				serverErr = err
				log.Printf("server error: %v", err)
				tray.Quit()
			}
		}()
	}, func() {
		// onQuit — graceful HTTP shutdown.
		log.Printf("shutting down...")
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := srv.Shutdown(ctx); err != nil {
			log.Printf("shutdown error: %v", err)
		}
	})

	return serverErr
}

// runHeadless keeps the original signal-based lifecycle (no tray icon).
func runHeadless(srv *http.Server) error {
	errCh := make(chan error, 1)
	go func() { errCh <- srv.ListenAndServe() }()

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)

	select {
	case err := <-errCh:
		if err != nil && err != http.ErrServerClosed {
			return err
		}
	case <-stop:
		log.Printf("shutting down...")
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := srv.Shutdown(ctx); err != nil {
			return fmt.Errorf("shutdown: %w", err)
		}
	}
	return nil
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
