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
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"

	zts "git0.harness.io/l7B_kbSEQD2wjrM7PShm5w/PROD/Harness_Commons/zero-trust-service"
	"git0.harness.io/l7B_kbSEQD2wjrM7PShm5w/PROD/Harness_Commons/zero-trust-service/types"
	"git0.harness.io/l7B_kbSEQD2wjrM7PShm5w/PROD/Harness_Commons/zero-trust-service/verifier"
)

// This example shows how to create a ZTS server with hardcoded validators.
// For production use, see cmd/server/ which loads validators from config.yaml.
func main() {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	const port = 4210
	server := zts.NewServer(
		zts.WithPort(port),
		zts.WithVerifyHandler(verifier.ToHandler(
			verifier.Chain(
				middlewareLogId(),
				verifier.From(middlewareIdNotEmpty),
				&middleWareIdIsEvenLength{},
			),
		)),
	)

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)

	go func() {
		<-sigCh
		log.Println("Received shutdown signal, stopping server...")
		cancel()
	}()

	log.Printf("Example server starting on :%d\n", port)
	if err := server.Run(ctx); err != nil {
		log.Fatalf("Server error: %v", err)
	}

	log.Println("Server stopped gracefully")
}

// middlewareIdNotEmpty is an example of a raw function that defines a middleware
func middlewareIdNotEmpty(_ context.Context, request types.VerifyRequest) error {
	if request.TaskPackage == nil || request.TaskPackage.TaskID == "" {
		return fmt.Errorf("task_id is empty")
	}
	return nil
}

// middlewareLogId is an example of a function returning a middleware.
func middlewareLogId() verifier.Interface {
	return verifier.From(func(_ context.Context, request types.VerifyRequest) error {
		taskID := ""
		if request.TaskPackage != nil {
			taskID = request.TaskID()
		}
		log.Printf("received task_id: %q", taskID)
		return nil
	})
}

// middleWareIdIsEvenLength is an example of a struct that implements the verifier.Interface
type middleWareIdIsEvenLength struct{}

func (m *middleWareIdIsEvenLength) Handle(_ context.Context, request types.VerifyRequest) error {
	if request.TaskPackage == nil || len(request.TaskID())%2 != 0 {
		return fmt.Errorf("task_id is not even length")
	}
	return nil
}
