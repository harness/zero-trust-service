package tasktype

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"git0.harness.io/l7B_kbSEQD2wjrM7PShm5w/PROD/Harness_Commons/zero-trust-service/types"
	"git0.harness.io/l7B_kbSEQD2wjrM7PShm5w/PROD/Harness_Commons/zero-trust-service/verifier"
)

// ImageAllowlistConfig holds the image allowlist verifier configuration.
type ImageAllowlistConfig struct {
	AllowedPrefixes []string `yaml:"allowed_prefixes"`
}

type imageAllowlistVerifier struct {
	allowedPrefixes []string
}

// NewImageAllowlist creates an image allowlist validator from typed config.
func NewImageAllowlist(cfg ImageAllowlistConfig) (verifier.Interface, error) {
	if len(cfg.AllowedPrefixes) == 0 {
		return nil, fmt.Errorf("image_allowlist: allowed_prefixes list is empty")
	}
	return &imageAllowlistVerifier{allowedPrefixes: cfg.AllowedPrefixes}, nil
}

func (v *imageAllowlistVerifier) Handle(_ context.Context, request types.VerifyRequest) error {
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

func (v *imageAllowlistVerifier) isAllowed(imageName string) bool {
	for _, prefix := range v.allowedPrefixes {
		if strings.HasPrefix(imageName, prefix) {
			return true
		}
	}
	return false
}

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
		if inner, ok := val["imageDetails"]; ok {
			collectImageNames(inner, true, names)
		}
		if insideImageDetails {
			if name, ok := val["name"].(string); ok && name != "" {
				*names = append(*names, name)
			}
		}

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
