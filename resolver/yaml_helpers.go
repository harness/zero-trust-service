package resolver

import (
	"fmt"
	"strings"

	"gopkg.in/yaml.v3"
)

func nodeGet(n *yaml.Node, key string) *yaml.Node {
	if n == nil || n.Kind != yaml.MappingNode {
		return nil
	}
	for i := 0; i+1 < len(n.Content); i += 2 {
		if n.Content[i].Value == key {
			return n.Content[i+1]
		}
	}
	return nil
}

func nodeSet(n *yaml.Node, key string, val *yaml.Node) {
	if n == nil || n.Kind != yaml.MappingNode {
		return
	}
	for i := 0; i+1 < len(n.Content); i += 2 {
		if n.Content[i].Value == key {
			n.Content[i+1] = val
			return
		}
	}
	n.Content = append(n.Content,
		&yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: key},
		val,
	)
}

func nodeHas(n *yaml.Node, key string) bool {
	return nodeGet(n, key) != nil
}

func nodeKeys(n *yaml.Node) []string {
	if n == nil || n.Kind != yaml.MappingNode {
		return nil
	}
	keys := make([]string, 0, len(n.Content)/2)
	for i := 0; i+1 < len(n.Content); i += 2 {
		keys = append(keys, n.Content[i].Value)
	}
	return keys
}

func nodeDeepClone(n *yaml.Node) *yaml.Node {
	if n == nil {
		return nil
	}
	clone := &yaml.Node{
		Kind:        n.Kind,
		Tag:         n.Tag,
		Value:       n.Value,
		Style:       n.Style,
		HeadComment: n.HeadComment,
		LineComment: n.LineComment,
		FootComment: n.FootComment,
		Anchor:      n.Anchor,
	}
	if len(n.Content) > 0 {
		clone.Content = make([]*yaml.Node, len(n.Content))
		for i, c := range n.Content {
			clone.Content[i] = nodeDeepClone(c)
		}
	}
	return clone
}

func nodeScalar(val string) *yaml.Node {
	return &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: val}
}

func nodeToGoValue(n *yaml.Node) any {
	if n == nil {
		return nil
	}
	switch n.Kind {
	case yaml.DocumentNode:
		if len(n.Content) > 0 {
			return nodeToGoValue(n.Content[0])
		}
		return nil
	case yaml.MappingNode:
		m := make(map[string]any, len(n.Content)/2)
		for i := 0; i+1 < len(n.Content); i += 2 {
			m[n.Content[i].Value] = nodeToGoValue(n.Content[i+1])
		}
		return m
	case yaml.SequenceNode:
		s := make([]any, len(n.Content))
		for i, c := range n.Content {
			s[i] = nodeToGoValue(c)
		}
		return s
	case yaml.ScalarNode:
		var v any
		if err := n.Decode(&v); err != nil {
			return n.Value
		}
		return v
	case yaml.AliasNode:
		return nodeToGoValue(n.Alias)
	default:
		return n.Value
	}
}

func unmarshalYAMLNode(data string) (*yaml.Node, error) {
	var doc yaml.Node
	if err := yaml.Unmarshal([]byte(data), &doc); err != nil {
		return nil, fmt.Errorf("%w: %v", ErrInvalidYAML, err)
	}
	if doc.Kind == yaml.DocumentNode && len(doc.Content) > 0 {
		return doc.Content[0], nil
	}
	return &doc, nil
}

func marshalYAML(node *yaml.Node) (string, error) {
	doc := node
	if node.Kind != yaml.DocumentNode {
		doc = &yaml.Node{
			Kind:    yaml.DocumentNode,
			Content: []*yaml.Node{node},
		}
	}
	var buf strings.Builder
	enc := yaml.NewEncoder(&buf)
	enc.SetIndent(2)
	if err := enc.Encode(doc); err != nil {
		return "", fmt.Errorf("marshal yaml: %w", err)
	}
	if err := enc.Close(); err != nil {
		return "", fmt.Errorf("marshal yaml: %w", err)
	}
	return buf.String(), nil
}
