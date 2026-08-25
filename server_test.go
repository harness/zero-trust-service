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

package zts

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestNewServer_DefaultRoutes(t *testing.T) {
	s := NewServer()

	req := httptest.NewRequest("POST", "/api/verify", nil)
	w := httptest.NewRecorder()
	s.httpServer.Handler.ServeHTTP(w, req)

	if w.Code == http.StatusNotFound {
		t.Fatal("/api/verify route not registered")
	}
}

func TestNewServer_OutputRoute(t *testing.T) {
	s := NewServer()

	req := httptest.NewRequest("POST", "/api/output", nil)
	w := httptest.NewRecorder()
	s.httpServer.Handler.ServeHTTP(w, req)

	if w.Code == http.StatusNotFound {
		t.Fatal("/api/output route not registered")
	}
}

func TestServer_Run_Shutdown(t *testing.T) {
	s := NewServer(WithPort(19876))

	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()

	_ = s.Run(ctx)
}

