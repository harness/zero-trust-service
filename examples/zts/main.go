package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	zts "git0.harness.io/l7B_kbSEQD2wjrM7PShm5w/PROD/Harness_Commons/zero-trust-service"
	auditfile "git0.harness.io/l7B_kbSEQD2wjrM7PShm5w/PROD/Harness_Commons/zero-trust-service/audit/file"
	"git0.harness.io/l7B_kbSEQD2wjrM7PShm5w/PROD/Harness_Commons/zero-trust-service/examples/zts/config"
	prommetrics "git0.harness.io/l7B_kbSEQD2wjrM7PShm5w/PROD/Harness_Commons/zero-trust-service/metrics/prometheus"
	"git0.harness.io/l7B_kbSEQD2wjrM7PShm5w/PROD/Harness_Commons/zero-trust-service/resolver"
	resolverscm "git0.harness.io/l7B_kbSEQD2wjrM7PShm5w/PROD/Harness_Commons/zero-trust-service/resolver/scm"
	"git0.harness.io/l7B_kbSEQD2wjrM7PShm5w/PROD/Harness_Commons/zero-trust-service/validators"
	"git0.harness.io/l7B_kbSEQD2wjrM7PShm5w/PROD/Harness_Commons/zero-trust-service/verifier"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
)

func main() {
	configPath := flag.String("config", "config.yaml", "path to config.yaml")
	flag.Parse()

	cfg, err := config.Load(*configPath)
	if err != nil {
		log.Fatalf("failed to load config from %s: %v", *configPath, err)
	}

	m := prommetrics.New()
	chain, err := validators.BuildFromConfig(cfg.Validators, m)
	if err != nil {
		log.Fatalf("failed to build validators from config: %v", err)
	}

	var chainParts []verifier.Interface
	if cfg.Resolver.Enabled {
		cfg.Resolver.Defaults()
		clients, err := resolverscm.NewClients(cfg.Resolver.SCM)
		if err != nil {
			log.Fatalf("failed to create SCM clients: %v", err)
		}
		multiLoader := resolverscm.NewMultiLoader(clients)

		mappings, err := config.LoadTemplateMappings(cfg.Resolver.Templates.TemplateMappingsFile)
		if err != nil {
			log.Fatalf("failed to load template mappings: %v", err)
		}
		store := resolverscm.NewTemplateStore(multiLoader, cfg.Resolver, mappings)

		var loader resolver.ResourceLoader
		dp := cfg.Resolver.Templates.DefaultProvider
		if dp != "" {
			loader, err = multiLoader.Loader(dp)
			if err != nil {
				log.Fatalf("default provider %q: %v", dp, err)
			}
		} else {
			loader = multiLoader
		}

		r := resolver.New(store, loader)

		resolverOpts := []verifier.ResolverMiddlewareOption{
			verifier.WithRepoQualifier(func(repo string) string {
				return cfg.Resolver.QualifyRepo(dp, repo)
			}),
		}
		if cfg.Resolver.OutputDir != "" {
			resolverOpts = append(resolverOpts, verifier.WithOutputDir(cfg.Resolver.OutputDir))
		}
		rm := verifier.NewResolverMiddleware(r, m, resolverOpts...)
		chainParts = append(chainParts, rm)
		log.Printf("pipeline resolver enabled")
	}

	chainParts = append(chainParts, chain)
	fullChain := verifier.Chain(chainParts...)

	// ZTS core server options
	opts := []zts.Option{
		zts.WithPort(cfg.Port),
		zts.WithMetrics(m),
		zts.WithVerifyHandler(verifier.ToHandler(fullChain)),
	}

	// Audit writer (file-backed)
	var aw *auditfile.Writer
	if cfg.Audit.Enabled {
		acfg := auditfile.Config{
			Dir:        cfg.Audit.Dir,
			MaxAgeDays: cfg.Audit.MaxAgeDays,
		}
		aw, err = auditfile.NewWriter(acfg)
		if err != nil {
			log.Fatalf("failed to create audit writer: %v", err)
		}
		opts = append(opts, zts.WithAuditWriter(aw))
		log.Printf("audit enabled, writing to %s", acfg.Dir)
	}

	server := zts.NewServer(opts...)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Start audit cleanup goroutine
	if aw != nil {
		go aw.Start(ctx)
	}

	// Admin server (metrics, healthz, audit routes)
	adminMux := chi.NewRouter()
	adminMux.Use(middleware.Recoverer)
	prommetrics.NewHandler().RegisterRoutes(adminMux)
	adminMux.Get("/healthz", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/plain")
		io.WriteString(w, "OK")
	})
	if aw != nil {
		reader := auditfile.NewReader(cfg.Audit.Dir)
		auditfile.NewHandler(reader).RegisterRoutes(adminMux)
	}

	adminServer := &http.Server{
		Addr:              fmt.Sprintf(":%d", cfg.AdminPort),
		Handler:           adminMux,
		ReadHeaderTimeout: 2 * time.Second,
		ReadTimeout:       5 * time.Second,
		WriteTimeout:      5 * time.Second,
		IdleTimeout:       30 * time.Second,
	}

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)

	go func() {
		<-sigCh
		log.Println("shutting down...")
		cancel()
	}()

	// Start admin server
	go func() {
		log.Printf("admin server listening on %s", adminServer.Addr)
		if err := adminServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("admin server: %v", err)
		}
	}()

	log.Printf("ZTS server starting (config=%s)", *configPath)
	if err := server.Run(ctx); err != nil {
		log.Fatalf("server error: %v", err)
	}

	// Graceful shutdown of admin server
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer shutdownCancel()
	adminServer.Shutdown(shutdownCtx)

	if aw != nil {
		aw.Close()
	}
}
