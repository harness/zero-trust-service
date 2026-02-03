package zts

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
)

type Server struct {
	router     *chi.Mux
	httpServer *http.Server

	verifyHandler VerifyHandler
}

func NewServer(opts ...Option) *Server {
	options := resolveOptions(opts...)

	s := &Server{
		verifyHandler: options.verifyHandler,
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

	// register endpoints
	r.Route("/api", func(r chi.Router) {
		r.Post("/verify", s.handleVerify)
	})

	return r
}

// Run runs the HTTP server
func (s *Server) Run(ctx context.Context) error {
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

// shutdown gracefully shuts down the server
func (s *Server) shutdown(ctx context.Context) error {
	ctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 30*time.Second)
	defer cancel()
	return s.httpServer.Shutdown(ctx)
}
