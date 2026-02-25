package custom

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
	maxBodySize    = 1 << 20 // 1 MB
)

// webhookResponse is the expected JSON response from the external endpoint.
// Matches the ZtsVerificationResponse structure: allowed/reason/metadata.
type webhookResponse struct {
	Allowed  bool                   `json:"allowed"`
	Reason   string                 `json:"reason,omitempty"`
	Metadata map[string]interface{} `json:"metadata,omitempty"`
}

// webhook calls an external HTTP endpoint with the full verify request
// and expects a pass/fail JSON response.
type webhook struct {
	name               string
	method             string
	url                string
	client             *http.Client
	headers            map[string]string
	failOpen           bool
	allowedStatusCodes map[int]struct{}
}

// Webhook creates a new webhook validator from config.
//
// Expected config keys:
//
//	name:                 string  — friendly name for logging / metrics
//	method:               string  — HTTP method, e.g. "POST", "PUT" (default: POST)
//	url:                  string  — the endpoint to call (required)
//	timeout:              string  — HTTP timeout, e.g. "5s" (default: 5s)
//	headers:              map     — extra HTTP headers to send
//	fail_open:            bool    — if true, allow the request when the webhook is unreachable (default: false)
//	allowed_status_codes: []int   — HTTP status codes treated as success (default: 200-299)
func Webhook(cfg map[string]any) (verifier.Interface, error) {
	rawURL, _ := cfg["url"].(string)
	if rawURL == "" {
		return nil, fmt.Errorf("webhook: \"url\" is required")
	}

	name, _ := cfg["name"].(string)
	if name == "" {
		name = rawURL
	}

	method := http.MethodPost
	if raw, ok := cfg["method"].(string); ok && raw != "" {
		method = strings.ToUpper(raw)
	}

	timeout := defaultTimeout
	if raw, ok := cfg["timeout"].(string); ok && raw != "" {
		d, err := time.ParseDuration(raw)
		if err != nil {
			return nil, fmt.Errorf("webhook: invalid timeout %q: %w", raw, err)
		}
		timeout = d
	}

	headers := make(map[string]string)
	if raw, ok := cfg["headers"].(map[string]any); ok {
		for k, v := range raw {
			headers[k] = fmt.Sprintf("%v", v)
		}
	}

	failOpen, _ := cfg["fail_open"].(bool)

	allowedCodes := make(map[int]struct{})
	if raw, ok := cfg["allowed_status_codes"].([]any); ok {
		for _, v := range raw {
			if code, ok := toInt(v); ok {
				allowedCodes[code] = struct{}{}
			}
		}
	}

	return &webhook{
		name:               name,
		method:             method,
		url:                rawURL,
		client:             &http.Client{Timeout: timeout},
		headers:            headers,
		failOpen:           failOpen,
		allowedStatusCodes: allowedCodes,
	}, nil
}

// Handle sends the full VerifyRequest to the configured URL and interprets the response.
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
			return nil // webhook unreachable, fail-open allows the request
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

// isAllowedStatus returns true if the HTTP status code is acceptable.
// When allowed_status_codes is configured, only those codes pass.
// Otherwise the default 200-299 range is used.
func (w *webhook) isAllowedStatus(code int) bool {
	if len(w.allowedStatusCodes) > 0 {
		_, ok := w.allowedStatusCodes[code]
		return ok
	}
	return code >= 200 && code < 300
}

// toInt converts a YAML-decoded numeric value to int.
func toInt(v any) (int, bool) {
	switch n := v.(type) {
	case int:
		return n, true
	case float64:
		return int(n), true
	}
	return 0, false
}
