package tasktype

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"git0.harness.io/l7B_kbSEQD2wjrM7PShm5w/PROD/Harness_Commons/zero-trust-service/types"
	"git0.harness.io/l7B_kbSEQD2wjrM7PShm5w/PROD/Harness_Commons/zero-trust-service/verifier"
)

// imageAllowlist validates that all container image names start with
// one of the allowed prefixes.
//
// Supported task types:
//   - INITIALIZATION_PHASE — extracts from imageDetails.name
//   - CI_EXECUTE_STEP      — extracts from stepInfo.image
//
// Config example:
//
//	type: image_allowlist
//	config:
//	  allowed_prefixes:
//	    - "harness/"
//	    - "plugins/"
type imageAllowlist struct {
	allowedPrefixes []string
}

// ImageAllowlist creates an image allowlist validator from config.
func ImageAllowlist(cfg map[string]any) (verifier.Interface, error) {
	raw, ok := cfg["allowed_prefixes"]
	if !ok {
		return nil, fmt.Errorf("image_allowlist: missing 'allowed_prefixes' in config")
	}
	list, ok := raw.([]any)
	if !ok {
		return nil, fmt.Errorf("image_allowlist: 'allowed_prefixes' must be a list")
	}

	var prefixes []string
	for _, v := range list {
		s, ok := v.(string)
		if !ok {
			return nil, fmt.Errorf("image_allowlist: each prefix must be a string, got %T", v)
		}
		prefixes = append(prefixes, s)
	}
	if len(prefixes) == 0 {
		return nil, fmt.Errorf("image_allowlist: allowed_prefixes list is empty")
	}

	return &imageAllowlist{allowedPrefixes: prefixes}, nil
}

// Handle validates all image names found in the parameters.
func (v *imageAllowlist) Handle(_ context.Context, request types.VerifyRequest) error {
	if request.TaskPackage == nil || request.TaskPackage.TaskDetails == nil || len(request.TaskPackage.TaskDetails.Parameters) == 0 {
		return nil
	}

	images := extractImageNames(request.TaskPackage.TaskDetails.Parameters)
	for _, img := range images {
		if !v.isAllowed(img) {
			return fmt.Errorf("image_allowlist: image %q does not match any allowed prefix %v", img, v.allowedPrefixes)
		}
	}
	return nil
}

func (v *imageAllowlist) isAllowed(imageName string) bool {
	for _, prefix := range v.allowedPrefixes {
		if strings.HasPrefix(imageName, prefix) {
			return true
		}
	}
	return false
}

// extractImageNames walks JSON parameters to find container image references.
//
// Supported patterns:
//   - INITIALIZATION_PHASE: imageDetails.name  (e.g. "harness/ci-lite-engine:latest")
//   - CI_EXECUTE_STEP:      stepInfo.image      (e.g. "plugins/policy-evaluator")
func extractImageNames(raw json.RawMessage) []string {
	var data any
	if err := json.Unmarshal(raw, &data); err != nil {
		return nil
	}
	var names []string
	collectImageNames(data, false, &names)
	return names
}

func collectImageNames(v any, insideImageDetails bool, names *[]string) {
	switch val := v.(type) {
	case map[string]any:
		// Pattern 1: imageDetails → { "name": "..." }  (INITIALIZATION_PHASE)
		if inner, ok := val["imageDetails"]; ok {
			collectImageNames(inner, true, names)
		}
		if insideImageDetails {
			if name, ok := val["name"].(string); ok && name != "" {
				*names = append(*names, name)
			}
		}

		// Pattern 2: stepInfo → { "image": "..." }  (CI_EXECUTE_STEP)
		if stepInfo, ok := val["stepInfo"].(map[string]any); ok {
			if img, ok := stepInfo["image"].(string); ok && img != "" {
				*names = append(*names, img)
			}
		}

		for k, child := range val {
			if k == "imageDetails" || k == "stepInfo" {
				continue
			}
			collectImageNames(child, false, names)
		}
	case []any:
		for _, item := range val {
			collectImageNames(item, insideImageDetails, names)
		}
	}
}
