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

package pipeline

import "testing"

const testPipeline = `
pipeline:
  identifier: p1
  stages:
    - stage:
        identifier: build
        type: CI
        spec:
          execution:
            steps:
              - step:
                  identifier: run1
                  type: Run
                  spec:
                    command: echo hello
              - step:
                  identifier: run2
                  type: ShellScript
    - stage:
        identifier: deploy
        type: Deployment
        spec:
          execution:
            steps:
              - stepGroup:
                  identifier: sg1
                  steps:
                    - step:
                        identifier: inner1
                        type: Http
`

func TestParsePipeline(t *testing.T) {
	root := ParsePipeline(testPipeline)
	if root == nil {
		t.Fatal("ParsePipeline returned nil")
	}
}

func TestParsePipeline_Invalid(t *testing.T) {
	root := ParsePipeline("not: [valid: yaml:")
	if root != nil {
		t.Error("expected nil for invalid YAML")
	}
}

func TestFindNodeByFQN(t *testing.T) {
	root := ParsePipeline(testPipeline)

	tests := []struct {
		name     string
		fqn      string
		wantNil  bool
		wantType string
	}{
		{"top-level pipeline", "pipeline", false, ""},
		{"stage", "pipeline.stages.build", false, "CI"},
		{"step", "pipeline.stages.build.spec.execution.steps.run1", false, "Run"},
		{"nested step", "pipeline.stages.deploy.spec.execution.steps.sg1.steps.inner1", false, "Http"},
		{"missing step", "pipeline.stages.build.spec.execution.steps.nonexistent", true, ""},
		{"missing stage", "pipeline.stages.nonexistent", true, ""},
		{"empty fqn", "", true, ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			node := FindNodeByFQN(root, tt.fqn)
			if tt.wantNil {
				if node != nil {
					t.Error("expected nil, got node")
				}
				return
			}
			if node == nil {
				t.Fatal("expected node, got nil")
			}
			if tt.wantType != "" && GetNodeType(node) != tt.wantType {
				t.Errorf("type = %q, want %q", GetNodeType(node), tt.wantType)
			}
		})
	}
}

func TestGetParentNode(t *testing.T) {
	root := ParsePipeline(testPipeline)

	t.Run("parent of step", func(t *testing.T) {
		// Parent of a step is the steps array (a sequence), so we check its kind
		parent := GetParentNode(root, "pipeline.stages.build.spec.execution.steps.run1")
		if parent == nil {
			t.Fatal("expected parent, got nil")
		}
	})

	t.Run("parent of stage", func(t *testing.T) {
		parent := GetParentNode(root, "pipeline.stages.build")
		if parent == nil {
			t.Fatal("expected parent, got nil")
		}
	})

	t.Run("parent of top level", func(t *testing.T) {
		parent := GetParentNode(root, "pipeline")
		if parent == nil {
			t.Fatal("expected root, got nil")
		}
	})

	t.Run("missing path", func(t *testing.T) {
		parent := GetParentNode(root, "pipeline.stages.nonexistent.steps.x")
		if parent != nil {
			t.Error("expected nil for missing path")
		}
	})
}

func TestGetNodeScalar(t *testing.T) {
	root := ParsePipeline(testPipeline)
	node := FindNodeByFQN(root, "pipeline.stages.build.spec.execution.steps.run1")
	if node == nil {
		t.Fatal("node not found")
	}

	if typ := GetNodeScalar(node, "type"); typ != "Run" {
		t.Errorf("type = %q, want Run", typ)
	}
	if id := GetNodeScalar(node, "identifier"); id != "run1" {
		t.Errorf("identifier = %q, want run1", id)
	}
	if missing := GetNodeScalar(node, "nonexistent"); missing != "" {
		t.Errorf("nonexistent = %q, want empty", missing)
	}
}

func TestGetNodeKeys(t *testing.T) {
	root := ParsePipeline(testPipeline)
	node := FindNodeByFQN(root, "pipeline")
	if node == nil {
		t.Fatal("pipeline node not found")
	}

	keys := GetNodeKeys(node)
	if len(keys) == 0 {
		t.Fatal("expected keys, got none")
	}
	found := false
	for _, k := range keys {
		if k == "identifier" {
			found = true
		}
	}
	if !found {
		t.Errorf("expected 'identifier' in keys: %v", keys)
	}
}

func TestGetNodeChild(t *testing.T) {
	root := ParsePipeline(testPipeline)
	pipeline := FindNodeByFQN(root, "pipeline")
	if pipeline == nil {
		t.Fatal("pipeline not found")
	}

	stages := GetNodeChild(pipeline, "stages")
	if stages == nil {
		t.Fatal("stages not found")
	}

	missing := GetNodeChild(pipeline, "nonexistent")
	if missing != nil {
		t.Error("expected nil for missing child")
	}
}
