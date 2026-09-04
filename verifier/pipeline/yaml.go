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

import "gopkg.in/yaml.v3"

// ParsePipeline parses pipeline YAML and returns the root mapping node
// (unwrapping the DocumentNode), or nil if the YAML is invalid or empty.
func ParsePipeline(pipelineYAML string) *yaml.Node {
	var doc yaml.Node
	if err := yaml.Unmarshal([]byte(pipelineYAML), &doc); err != nil {
		return nil
	}
	if doc.Kind != yaml.DocumentNode || len(doc.Content) == 0 {
		return nil
	}
	return doc.Content[0]
}

// FindNodeByFQN walks the pipeline YAML tree following the dot-separated FQN
// (e.g. "pipeline.stages.build.spec.execution.steps.run1") and returns the
// matching node, or nil if not found.
func FindNodeByFQN(root *yaml.Node, fqn string) *yaml.Node {
	if root == nil || fqn == "" {
		return nil
	}
	segments := splitFQN(fqn)
	return walkToNode(root, segments, 0)
}

// GetParentNode returns the parent node of an FQN (walking all segments
// except the last), or nil if the parent path doesn't exist.
func GetParentNode(root *yaml.Node, fqn string) *yaml.Node {
	if root == nil || fqn == "" {
		return nil
	}
	segments := splitFQN(fqn)
	if len(segments) <= 1 {
		return root
	}
	return walkToNode(root, segments[:len(segments)-1], 0)
}

// GetNodeType returns the value of the "type" key on a mapping node, or "".
func GetNodeType(node *yaml.Node) string {
	return GetNodeScalar(node, "type")
}

// GetNodeScalar returns the scalar value for the given key in a mapping node, or "".
func GetNodeScalar(node *yaml.Node, key string) string {
	return nodeGetScalar(node, key)
}

// GetNodeKeys returns the mapping keys of a mapping node.
func GetNodeKeys(node *yaml.Node) []string {
	if node == nil || node.Kind != yaml.MappingNode {
		return nil
	}
	keys := make([]string, 0, len(node.Content)/2)
	for i := 0; i+1 < len(node.Content); i += 2 {
		keys = append(keys, node.Content[i].Value)
	}
	return keys
}

// GetNodeChild returns the child node for the given key in a mapping node, or nil.
func GetNodeChild(node *yaml.Node, key string) *yaml.Node {
	return nodeGet(node, key)
}

// splitFQN splits a dot-separated FQN into segments.
func splitFQN(fqn string) []string {
	var segments []string
	start := 0
	for i := 0; i < len(fqn); i++ {
		if fqn[i] == '.' {
			if i > start {
				segments = append(segments, fqn[start:i])
			}
			start = i + 1
		}
	}
	if start < len(fqn) {
		segments = append(segments, fqn[start:])
	}
	return segments
}

// nodeGet returns the value node for the given key in a MappingNode, or nil.
func nodeGet(node *yaml.Node, key string) *yaml.Node {
	if node == nil || node.Kind != yaml.MappingNode {
		return nil
	}
	for i := 0; i+1 < len(node.Content); i += 2 {
		if node.Content[i].Value == key {
			return node.Content[i+1]
		}
	}
	return nil
}

// nodeGetScalar returns the scalar value for a key, or "".
func nodeGetScalar(node *yaml.Node, key string) string {
	v := nodeGet(node, key)
	if v != nil && v.Kind == yaml.ScalarNode {
		return v.Value
	}
	return ""
}

// walkToNode walks the YAML tree following segments and returns the final node.
func walkToNode(node *yaml.Node, segments []string, idx int) *yaml.Node {
	if idx >= len(segments) {
		return node
	}

	seg := segments[idx]

	switch node.Kind {
	case yaml.MappingNode:
		for i := 0; i+1 < len(node.Content); i += 2 {
			if node.Content[i].Value == seg {
				return walkToNode(node.Content[i+1], segments, idx+1)
			}
		}
		return nil

	case yaml.SequenceNode:
		for _, item := range node.Content {
			if item.Kind != yaml.MappingNode {
				continue
			}
			for _, wrapper := range []string{"step", "stage", "stepGroup"} {
				inner := nodeGet(item, wrapper)
				if inner == nil {
					continue
				}
				if nodeGetScalar(inner, "identifier") == seg {
					return walkToNode(inner, segments, idx+1)
				}
			}
			// Check parallel — transparent pass-through
			if par := nodeGet(item, "parallel"); par != nil {
				if result := walkToNode(par, segments, idx); result != nil {
					return result
				}
			}
			if nodeGetScalar(item, "identifier") == seg {
				return walkToNode(item, segments, idx+1)
			}
		}
		return nil

	default:
		return nil
	}
}
