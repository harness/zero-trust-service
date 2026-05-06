package webhook

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"git0.harness.io/l7B_kbSEQD2wjrM7PShm5w/PROD/Harness_Commons/zero-trust-service/types"
	"git0.harness.io/l7B_kbSEQD2wjrM7PShm5w/PROD/Harness_Commons/zero-trust-service/verifier"
)

const (
	defaultTimeout = 5 * time.Second
	maxBodySize    = 1 << 20
)

// Config holds the webhook validator configuration.
type Config struct {
	Name               string            `yaml:"name"`
	URL                string            `yaml:"url"`
	Method             string            `yaml:"method"`
	Timeout            string            `yaml:"timeout"`
	Headers            map[string]string `yaml:"headers"`
	FailOpen           bool              `yaml:"fail_open"`
	AllowedStatusCodes []int             `yaml:"allowed_status_codes"`
}

type webhookResponse struct {
	Allowed  bool                   `json:"allowed"`
	Reason   string                 `json:"reason,omitempty"`
	Metadata map[string]interface{} `json:"metadata,omitempty"`
}

type webhook struct {
	name               string
	method             string
	url                string
	client             *http.Client
	headers            map[string]string
	failOpen           bool
	allowedStatusCodes map[int]struct{}
}

// New creates a new webhook validator from typed config.
func New(cfg Config) (verifier.Interface, error) {
	if cfg.URL == "" {
		return nil, fmt.Errorf("webhook: \"url\" is required")
	}

	name := cfg.Name
	if name == "" {
		name = cfg.URL
	}

	method := http.MethodPost
	if cfg.Method != "" {
		method = strings.ToUpper(cfg.Method)
	}

	timeout := defaultTimeout
	if cfg.Timeout != "" {
		d, err := time.ParseDuration(cfg.Timeout)
		if err != nil {
			return nil, fmt.Errorf("webhook: invalid timeout %q: %w", cfg.Timeout, err)
		}
		timeout = d
	}

	headers := cfg.Headers
	if headers == nil {
		headers = make(map[string]string)
	}

	allowedCodes := make(map[int]struct{})
	for _, code := range cfg.AllowedStatusCodes {
		allowedCodes[code] = struct{}{}
	}

	return &webhook{
		name:               name,
		method:             method,
		url:                cfg.URL,
		client:             &http.Client{Timeout: timeout},
		headers:            headers,
		failOpen:           cfg.FailOpen,
		allowedStatusCodes: allowedCodes,
	}, nil
}

func (w *webhook) Handle(ctx context.Context, request types.VerifyRequest) error {
	body, err := json.Marshal(request)
	if err != nil {
		return fmt.Errorf("webhook %q: failed to marshal request: %w", w.name, err)
	}

	req, err := http.NewRequestWithContext(ctx, w.method, w.url, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("webhook %q: failed to build HTTP request: %w", w.name, err)
	}
	req.Header.Set("Content-Type", "application/json")
	for k, v := range w.headers {
		req.Header.Set(k, v)
	}

	resp, err := w.client.Do(req)
	if err != nil {
		if w.failOpen {
			return nil
		}
		return fmt.Errorf("webhook %q: call failed: %w", w.name, err)
	}
	defer resp.Body.Close()

	if !w.isAllowedStatus(resp.StatusCode) {
		if w.failOpen {
			return nil
		}
		return fmt.Errorf("webhook %q: returned HTTP %d", w.name, resp.StatusCode)
	}

	respBody, err := io.ReadAll(io.LimitReader(resp.Body, maxBodySize))
	if err != nil {
		return fmt.Errorf("webhook %q: failed to read response: %w", w.name, err)
	}

	var result webhookResponse
	if err := json.Unmarshal(respBody, &result); err != nil {
		return fmt.Errorf("webhook %q: invalid response JSON: %w", w.name, err)
	}

	if !result.Allowed {
		msg := "request denied by external validator"
		if result.Reason != "" {
			msg = result.Reason
		}
		return fmt.Errorf("webhook %q: %s", w.name, msg)
	}

	return nil
}

func (w *webhook) isAllowedStatus(code int) bool {
	if len(w.allowedStatusCodes) > 0 {
		_, ok := w.allowedStatusCodes[code]
		return ok
	}
	return code >= 200 && code < 300
}
