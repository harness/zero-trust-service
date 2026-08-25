// Copyright 2026 Harness, Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

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
	"git0.harness.io/l7B_kbSEQD2wjrM7PShm5w/PROD/Harness_Commons/zero-trust-service/examples/zts/auditreader"
	"git0.harness.io/l7B_kbSEQD2wjrM7PShm5w/PROD/Harness_Commons/zero-trust-service/examples/zts/config"
	prommetrics "git0.harness.io/l7B_kbSEQD2wjrM7PShm5w/PROD/Harness_Commons/zero-trust-service/metrics/prometheus"
	outputmw "git0.harness.io/l7B_kbSEQD2wjrM7PShm5w/PROD/Harness_Commons/zero-trust-service/middleware/output"
	verifymw "git0.harness.io/l7B_kbSEQD2wjrM7PShm5w/PROD/Harness_Commons/zero-trust-service/middleware/verify"
	"git0.harness.io/l7B_kbSEQD2wjrM7PShm5w/PROD/Harness_Commons/zero-trust-service/resolver"
	resolverscm "git0.harness.io/l7B_kbSEQD2wjrM7PShm5w/PROD/Harness_Commons/zero-trust-service/resolver/scm"
	"git0.harness.io/l7B_kbSEQD2wjrM7PShm5w/PROD/Harness_Commons/zero-trust-service/verifier"
	"git0.harness.io/l7B_kbSEQD2wjrM7PShm5w/PROD/Harness_Commons/zero-trust-service/verifier/instrumented"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

func main() {
	configPath := flag.String("config", "config.yaml", "path to config.yaml")
	flag.Parse()

	cfg, err := config.Load(*configPath)
	if err != nil {
		log.Fatalf("failed to load config from %s: %v", *configPath, err)
	}

	m := prommetrics.New(
		prommetrics.WithBuckets("zts_verify_request_duration_seconds",
			[]float64{0.0005, 0.001, 0.0025, 0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1}),
		prommetrics.WithBuckets("zts_verifier_duration_seconds",
			[]float64{0.0001, 0.0005, 0.001, 0.0025, 0.005, 0.01, 0.025, 0.05, 0.1}),
		prommetrics.WithBuckets("zts_resolver_duration_seconds",
			[]float64{0.05, 0.1, 0.25, 0.5, 1, 2.5, 5, 10, 30}),
	)
	reg := DefaultRegistry()

	wrap := func(name string, v verifier.Interface) verifier.Interface {
		return instrumented.Wrap(name, v, m)
	}
	chain, err := BuildChain(cfg.Validators, m, reg.Resolve, wrap)
	if err != nil {
		log.Fatalf("failed to build validators from config: %v", err)
	}

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

		resolverOpts := []verifier.ResolverOption{
			verifier.WithRepoQualifier(func(repo string) string {
				return cfg.Resolver.QualifyRepo(dp, repo)
			}),
		}
		if cfg.Resolver.OutputDir != "" {
			resolverOpts = append(resolverOpts, verifier.WithOutputDir(cfg.Resolver.OutputDir))
		}
		rm := verifier.NewResolver(r, m, resolverOpts...)
		chain = rm.Wrap(chain)
		log.Printf("pipeline resolver enabled")
	}

	// Verify middleware stack — outermost first.
	verifyMW := []zts.VerifyMiddleware{
		verifymw.Logging(),
		verifymw.Metrics(m),
		verifymw.MissingMetadata(m),
	}
	outputMW := []zts.OutputMiddleware{
		outputmw.Logging(),
		outputmw.Metrics(m),
	}

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
		verifyMW = append(verifyMW, verifymw.Audit(aw))
		outputMW = append(outputMW, outputmw.Audit(aw))
		log.Printf("audit enabled, writing to %s", acfg.Dir)
	}

	server := zts.NewServer(
		zts.WithPort(cfg.Port),
		zts.WithVerifyHandler(verifier.ToHandler(chain)),
		zts.WithVerifyMiddleware(verifyMW...),
		zts.WithOutputMiddleware(outputMW...),
	)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	if aw != nil {
		go aw.Start(ctx)
	}

	adminMux := chi.NewRouter()
	adminMux.Use(middleware.Recoverer)
	adminMux.Handle("/metrics", promhttp.Handler())
	adminMux.Get("/healthz", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/plain")
		_, _ = io.WriteString(w, "OK")
	})
	if aw != nil {
		reader := auditfile.NewReader(cfg.Audit.Dir)
		auditreader.NewHandler(reader).RegisterRoutes(adminMux)
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

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer shutdownCancel()
	if err := adminServer.Shutdown(shutdownCtx); err != nil {
		log.Printf("admin server shutdown: %v", err)
	}

	if aw != nil {
		if err := aw.Close(); err != nil {
			log.Printf("audit writer close: %v", err)
		}
	}
}
