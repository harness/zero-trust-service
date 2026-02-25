package main

import (
	"context"
	"flag"
	"log"
	"os"
	"os/signal"
	"syscall"

	zts "git0.harness.io/l7B_kbSEQD2wjrM7PShm5w/PROD/Harness_Commons/zero-trust-service"
	"git0.harness.io/l7B_kbSEQD2wjrM7PShm5w/PROD/Harness_Commons/zero-trust-service/audit"
	"git0.harness.io/l7B_kbSEQD2wjrM7PShm5w/PROD/Harness_Commons/zero-trust-service/config"
	"git0.harness.io/l7B_kbSEQD2wjrM7PShm5w/PROD/Harness_Commons/zero-trust-service/metrics"
	"git0.harness.io/l7B_kbSEQD2wjrM7PShm5w/PROD/Harness_Commons/zero-trust-service/validators"
	"git0.harness.io/l7B_kbSEQD2wjrM7PShm5w/PROD/Harness_Commons/zero-trust-service/verifier"
)

func main() {
	configPath := flag.String("config", "config.yaml", "path to config.yaml")
	flag.Parse()

	// Load CLI config (YAML)
	cfg, err := config.Load(*configPath)
	if err != nil {
		log.Fatalf("failed to load config from %s: %v", *configPath, err)
	}

	// Convert CLI config → library options
	m := metrics.New()
	chain, err := validators.BuildFromConfig(cfg.Validators, m)
	if err != nil {
		log.Fatalf("failed to build validators from config: %v", err)
	}

	opts := []zts.Option{
		zts.WithPort(cfg.Port),
		zts.WithMetrics(m),
		zts.WithVerifyHandler(verifier.ToHandler(chain)),
	}

	// Set up audit if enabled
	if cfg.Audit.Enabled {
		w, err := audit.NewWriter(cfg.Audit)
		if err != nil {
			log.Fatalf("failed to create audit writer: %v", err)
		}
		reader := audit.NewReader(cfg.Audit.Dir)
		opts = append(opts,
			zts.WithAuditWriter(w),
			zts.WithAuditHandler(audit.NewHandler(reader)),
		)
		log.Printf("audit enabled, writing to %s", cfg.Audit.Dir)
	}

	server := zts.NewServer(opts...)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)

	go func() {
		<-sigCh
		log.Println("shutting down...")
		cancel()
	}()

	log.Printf("ZTS server starting (config=%s)", *configPath)
	if err := server.Run(ctx); err != nil {
		log.Fatalf("server error: %v", err)
	}
}
