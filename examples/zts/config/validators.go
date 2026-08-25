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

package config

import "gopkg.in/yaml.v3"

// ValidatorsConfig defines which validators to run.
type ValidatorsConfig struct {
	Global     []ValidatorDef            `yaml:"global"`
	ByTaskType map[string][]ValidatorDef `yaml:"by_task_type"`
	Custom     []ValidatorDef            `yaml:"custom"`
}

// ValidatorDef describes a single validator instance.
type ValidatorDef struct {
	Type    string    `yaml:"type"`
	Enabled *bool     `yaml:"enabled,omitempty"`
	Config  yaml.Node `yaml:"config"`
}

// IsEnabled returns true if the validator is enabled (default: true when omitted).
func (v ValidatorDef) IsEnabled() bool {
	return v.Enabled == nil || *v.Enabled
}
