package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"

	zts "git0.harness.io/l7B_kbSEQD2wjrM7PShm5w/PROD/Harness_Commons/zero-trust-service"
	"git0.harness.io/l7B_kbSEQD2wjrM7PShm5w/PROD/Harness_Commons/zero-trust-service/verifier"
	middleware "git0.harness.io/l7B_kbSEQD2wjrM7PShm5w/PROD/Harness_Commons/zero-trust-service/verifier"
)

func main() {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	const port = 4210
	server := zts.NewServer(
		zts.WithPort(port),
		zts.WithVerifyHandler(verifier.ToHandler(
			verifier.Chain(
				middlewareLogId(),
				middleware.From(middlewareIdNotEmpty),
				&middleWareIdIsEvenLength{},
			),
		)),
	)

	// Setup signal handling
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)

	go func() {
		<-sigCh
		log.Println("Received shutdown signal, stopping server...")
		cancel()
	}()

	log.Printf("Server starting on :%d\n", port)
	if err := server.Run(ctx); err != nil {
		log.Fatalf("Server error: %v", err)
	}

	log.Println("Server stopped gracefully")
}

// middlewareIdNotEmpty is an example of a raw function that defines a middleware
func middlewareIdNotEmpty(request zts.VerifyRequest) error {
	if request.TaskID == "" {
		return fmt.Errorf("task_id is empty")
	}
	return nil
}

// middlewareLogId is an example of a function returning a middleware.
func middlewareLogId() middleware.Interface {
	return middleware.From(func(request zts.VerifyRequest) error {
		log.Printf("received task_id: %q", request.TaskID)
		return nil
	})
}

// middleWareIdIsEvenLength is an example of a struct that implements the middleware.Interface
type middleWareIdIsEvenLength struct {
}

func (m *middleWareIdIsEvenLength) Handle(request zts.VerifyRequest) error {
	if len(request.TaskID)%2 != 0 {
		return fmt.Errorf("task_id is not even length")
	}
	return nil
}
