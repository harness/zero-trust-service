package zts

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"time"

	"git0.harness.io/l7B_kbSEQD2wjrM7PShm5w/PROD/Harness_Commons/zero-trust-service/audit"
	"git0.harness.io/l7B_kbSEQD2wjrM7PShm5w/PROD/Harness_Commons/zero-trust-service/metrics"
	"git0.harness.io/l7B_kbSEQD2wjrM7PShm5w/PROD/Harness_Commons/zero-trust-service/types"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

type Server struct {
	router     *chi.Mux
	httpServer *http.Server

	verifyHandler types.VerifyHandler
	metrics       *metrics.Metrics

	auditWriter  *audit.Writer
	auditHandler *audit.Handler
}

func NewServer(opts ...Option) *Server {
	options := resolveOptions(opts...)

	s := &Server{
		verifyHandler: options.verifyHandler,
		metrics:       options.metrics,
		auditWriter:   options.auditWriter,
		auditHandler:  options.auditHandler,
	}

	// Setup routes
	r := s.setupRouter()

	s.httpServer = &http.Server{
		Addr:    fmt.Sprintf(":%d", options.Port),
		Handler: r,
		// TODO: expose via config ...
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       10 * time.Second,
		WriteTimeout:      10 * time.Second,
		IdleTimeout:       60 * time.Second,
	}

	return s
}

func (s *Server) setupRouter() *chi.Mux {
	r := chi.NewRouter()

	// Basic middleware
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)
	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)

	// Prometheus metrics endpoint
	r.Handle("/metrics", promhttp.Handler())

	// register endpoints
	r.Route("/api", func(r chi.Router) {
		r.Post("/verify", s.handleVerify)

		// Audit query endpoints (only when audit is enabled)
		if s.auditHandler != nil {
			s.auditHandler.RegisterRoutes(r)
		}
	})

	return r
}

// Run runs the HTTP server and background audit cleanup.
func (s *Server) Run(ctx context.Context) error {
	// Start audit cleanup goroutine if audit is enabled
	if s.auditWriter != nil {
		go s.runAuditCleanup(ctx)
	}

	errCh := make(chan error, 1)

	go func() {
		if err := s.httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			errCh <- err
		}
	}()

	select {
	case err := <-errCh:
		return err
	case <-ctx.Done():
		return s.shutdown(ctx)
	}
}

// shutdown gracefully shuts down the server and closes audit writer.
func (s *Server) shutdown(ctx context.Context) error {
	if s.auditWriter != nil {
		if err := s.auditWriter.Close(); err != nil {
			log.Printf("audit: close writer: %v", err)
		}
	}
	ctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 30*time.Second)
	defer cancel()
	return s.httpServer.Shutdown(ctx)
}

// runAuditCleanup runs periodic cleanup of old audit files.
func (s *Server) runAuditCleanup(ctx context.Context) {
	ticker := time.NewTicker(24 * time.Hour)
	defer ticker.Stop()

	// Run once at startup
	s.auditWriter.RunCleanup()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			s.auditWriter.RunCleanup()
		}
	}
}
