package di

import (
	"context"
	"fmt"
	"log"
	"net"
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
	"github.com/paintingpromisesss/courseforge/internal/mcp"
	"github.com/paintingpromisesss/courseforge/internal/tray"
	"github.com/paintingpromisesss/courseforge/internal/web"
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

	// Postgres is opt-in (Settings -> PostgreSQL -> Start). If the user
	// enabled it earlier, bring it back up in the background so initdb/pg_ctl
	// never delay HTTP startup.
	pgDone := make(chan struct{})
	go func() {
		defer close(pgDone)
		if !runner.PostgresEnabled(cfg.DataDir) {
			return
		}
		if err := r.StartPostgres(context.Background(), filepath.Join(cfg.DataDir, "postgres")); err != nil {
			log.Printf("postgres runner disabled: %v", err)
			return
		}
		log.Printf("postgres runner ready")
	}()
	defer func() {
		<-pgDone
		_ = r.StopPostgres()
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

	aiRepo := repo.NewAIConfigRepository(cfg.DataDir)
	aiService := service.NewAIService(aiRepo, logger)
	if err := aiService.Init(context.Background()); err != nil {
		return fmt.Errorf("init ai service: %w", err)
	}

	mcpRepo := repo.NewMCPConfigRepository(cfg.DataDir)

	mcpProvider, err := mcp.NewCourseForgeProvider(cfg.CoursesDir, pr, sr, r)
	if err != nil {
		log.Printf("warning: init mcp provider: %v", err)
	}
	mcpSession, err := mcp.NewFileSessionManager(filepath.Join(cfg.DataDir, "mcp_session.json"))
	if err != nil {
		log.Printf("warning: init mcp session: %v", err)
	}

	var mcpServer *mcp.Server
	if mcpProvider != nil && mcpSession != nil {
		mcpServer, err = mcp.NewServer(mcp.Config{
			Name:        "courseforge",
			Version:     "1.0.0",
			CoursesDir:  cfg.CoursesDir,
			DataDir:     cfg.DataDir,
			DBPath:      cfg.DBPath,
			RunnersJSON: cfg.RunnersJSON,
		}, mcpProvider, mcpSession)
		if err != nil {
			log.Printf("warning: init mcp server: %v", err)
		}
	}

	h := handlers.New(cfg.CoursesDir, cfg.DataDir, courses, catalogs, r, ps, ss, aiService, mcpRepo, mcpServer)

	router, err := api.NewRouter(h, api.RouterOptions{FrontendDir: cfg.FrontendDir, CoursesDir: cfg.CoursesDir, DataDir: cfg.DataDir})
	if err != nil {
		return err
	}

	stopCh := make(chan struct{}, 1)
	srv := &http.Server{Addr: cfg.Addr, Handler: shutdownHandler(router, stopCh)}

	log.Printf("listening on http://%s", displayAddr(cfg.Addr))
	if swaggerEnabled {
		log.Printf("swagger UI: http://%s/swagger/index.html", displayAddr(cfg.Addr))
	}
	if cfg.FrontendDir != "" {
		log.Printf("frontend dir: %s", cfg.FrontendDir)
	} else if web.HasEmbedded() {
		log.Printf("frontend: using embedded assets")
	}

	if cfg.EnableTray {
		go func() {
			<-stopCh
			tray.Quit()
		}()
		return runWithTray(cfg, srv)
	}
	return runHeadless(srv, stopCh)
}

// shutdownHandler serves POST /api/shutdown (used by `courseforge stop`).
// Loopback only, and the custom header forces a CORS preflight, which
// corsMiddleware doesn't allow, so web pages can't trigger it.
func shutdownHandler(next http.Handler, stopCh chan<- struct{}) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/shutdown" {
			next.ServeHTTP(w, r)
			return
		}
		host, _, _ := net.SplitHostPort(r.RemoteAddr)
		if r.Method != http.MethodPost || r.Header.Get("X-Courseforge-Stop") == "" || !net.ParseIP(host).IsLoopback() {
			http.Error(w, "forbidden", http.StatusForbidden)
			return
		}
		select {
		case stopCh <- struct{}{}:
		default:
		}
		w.WriteHeader(http.StatusAccepted)
	})
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
func runHeadless(srv *http.Server, stopCh <-chan struct{}) error {
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
		return shutdown(srv)
	case <-stopCh:
		return shutdown(srv)
	}
	return nil
}

func shutdown(srv *http.Server) error {
	log.Printf("shutting down...")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := srv.Shutdown(ctx); err != nil {
		return fmt.Errorf("shutdown: %w", err)
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
