package resolver

import (
	"fmt"
	"strings"

	"gopkg.in/yaml.v3"
)

// MergeTemplateInputs deep-merges templateInputs onto the template spec.
func MergeTemplateInputs(spec, inputs *yaml.Node) *yaml.Node {
	if inputs == nil || inputs.Kind != yaml.MappingNode {
		return spec
	}
	if spec == nil || spec.Kind != yaml.MappingNode {
		return inputs
	}
	result := nodeDeepClone(spec)
	for i := 0; i+1 < len(inputs.Content); i += 2 {
		key := inputs.Content[i].Value
		inputVal := inputs.Content[i+1]
		specVal := nodeGet(result, key)
		if specVal == nil {
			nodeSet(result, key, nodeDeepClone(inputVal))
			continue
		}
		if specVal.Kind == yaml.MappingNode && inputVal.Kind == yaml.MappingNode {
			nodeSet(result, key, MergeTemplateInputs(specVal, inputVal))
			continue
		}
		if specVal.Kind == yaml.SequenceNode && inputVal.Kind == yaml.SequenceNode {
			nodeSet(result, key, mergeArraysByIdentifier(specVal, inputVal))
			continue
		}
		nodeSet(result, key, nodeDeepClone(inputVal))
	}
	return result
}

func mergeArraysByIdentifier(specSeq, inputSeq *yaml.Node) *yaml.Node {
	type indexEntry struct {
		wrapperKey string
		innerNode  *yaml.Node
		elemIdx    int
		used       bool
	}
	inputIndex := make(map[string]*indexEntry)
	for idx, elem := range inputSeq.Content {
		wrapperKey, innerNode := unwrapArrayElement(elem)
		if wrapperKey == "" {
			continue
		}
		id := getElementIdentifier(innerNode)
		if id != "" {
			inputIndex[wrapperKey+"/"+id] = &indexEntry{wrapperKey: wrapperKey, innerNode: innerNode, elemIdx: idx}
		}
	}
	if len(inputIndex) == 0 {
		return nodeDeepClone(inputSeq)
	}
	result := &yaml.Node{Kind: yaml.SequenceNode, Tag: "!!seq"}
	for _, elem := range specSeq.Content {
		wrapperKey, specInner := unwrapArrayElement(elem)
		if wrapperKey == "" {
			result.Content = append(result.Content, nodeDeepClone(elem))
			continue
		}
		id := getElementIdentifier(specInner)
		key := wrapperKey + "/" + id
		if id != "" {
			if entry, ok := inputIndex[key]; ok {
				merged := MergeTemplateInputs(specInner, entry.innerNode)
				if wrapperKey == "_direct" {
					result.Content = append(result.Content, merged)
				} else {
					wrapper := &yaml.Node{Kind: yaml.MappingNode, Tag: "!!map"}
					wrapper.Content = append(wrapper.Content, nodeScalar(wrapperKey), merged)
					result.Content = append(result.Content, wrapper)
				}
				entry.used = true
				continue
			}
		}
		result.Content = append(result.Content, nodeDeepClone(elem))
	}
	for _, elem := range inputSeq.Content {
		wrapperKey, innerNode := unwrapArrayElement(elem)
		if wrapperKey == "" {
			continue
		}
		id := getElementIdentifier(innerNode)
		key := wrapperKey + "/" + id
		if entry, ok := inputIndex[key]; ok && !entry.used {
			result.Content = append(result.Content, nodeDeepClone(elem))
		}
	}
	return result
}

func unwrapArrayElement(elem *yaml.Node) (string, *yaml.Node) {
	if elem == nil || elem.Kind != yaml.MappingNode {
		return "", nil
	}
	for _, wk := range []string{"stage", "step", "stepGroup", "parallel"} {
		inner := nodeGet(elem, wk)
		if inner != nil && inner.Kind == yaml.MappingNode {
			return wk, inner
		}
	}
	if nodeHas(elem, "identifier") || nodeHas(elem, "name") {
		return "_direct", elem
	}
	return "", nil
}

func getElementIdentifier(n *yaml.Node) string {
	if n == nil || n.Kind != yaml.MappingNode {
		return ""
	}
	if id := nodeGet(n, "identifier"); id != nil && id.Value != "" {
		return id.Value
	}
	if name := nodeGet(n, "name"); name != nil && name.Value != "" {
		return name.Value
	}
	return ""
}

// ExtractTemplateSpec extracts the "spec" node and type from a template YAML string.
func ExtractTemplateSpec(templateYAML string) (*yaml.Node, string, error) {
	doc, err := unmarshalYAMLNode(templateYAML)
	if err != nil {
		return nil, "", err
	}
	templateNode := nodeGet(doc, "template")
	if templateNode == nil {
		return doc, "", nil
	}
	if templateNode.Kind != yaml.MappingNode {
		return nil, "", fmt.Errorf("%w: template node is not a map", ErrInvalidYAML)
	}
	templateType := ""
	if t := nodeGet(templateNode, "type"); t != nil {
		templateType = t.Value
	}
	specNode := nodeGet(templateNode, "spec")
	if specNode == nil {
		return nil, templateType, fmt.Errorf("%w: template has no spec", ErrInvalidYAML)
	}
	if specNode.Kind != yaml.MappingNode {
		return nil, templateType, fmt.Errorf("%w: template.spec is not a map", ErrInvalidYAML)
	}
	return specNode, templateType, nil
}

// ParseTemplateRefFromNode extracts a TemplateRef and optional inputs from a template reference node.
func ParseTemplateRefFromNode(node *yaml.Node) (*TemplateRef, *yaml.Node, error) {
	if node == nil || node.Kind != yaml.MappingNode {
		return nil, nil, fmt.Errorf("%w: node is nil or not a mapping", ErrInvalidTemplateRef)
	}
	if templateRefNode := nodeGet(node, "templateRef"); templateRefNode != nil {
		refStr := templateRefNode.Value
		if refStr == "" {
			return nil, nil, fmt.Errorf("%w: empty templateRef", ErrInvalidTemplateRef)
		}
		ref := &TemplateRef{}
		ref.Identifier, ref.Scope, ref.OrgIdentifier, ref.ProjectIdentifier = parseIdentifierScope(refStr)
		if vl := nodeGet(node, "versionLabel"); vl != nil {
			ref.VersionLabel = vl.Value
		}
		if gb := nodeGet(node, "gitBranch"); gb != nil {
			ref.GitBranch = gb.Value
		}
		return ref, nodeGet(node, "templateInputs"), nil
	}
	if usesNode := nodeGet(node, "uses"); usesNode != nil {
		usesStr := usesNode.Value
		if usesStr == "" {
			return nil, nil, fmt.Errorf("%w: empty uses", ErrInvalidTemplateRef)
		}
		ref := &TemplateRef{}
		parts := strings.SplitN(usesStr, "@", 2)
		ref.Identifier, ref.Scope, ref.OrgIdentifier, ref.ProjectIdentifier = parseIdentifierScope(parts[0])
		if len(parts) > 1 {
			ref.VersionLabel = parts[1]
		}
		return ref, nodeGet(node, "with"), nil
	}
	return nil, nil, fmt.Errorf("%w: no templateRef or uses found", ErrInvalidTemplateRef)
}

func parseIdentifierScope(identifier string) (string, Scope, string, string) {
	if strings.HasPrefix(identifier, "account.") {
		return strings.TrimPrefix(identifier, "account."), ScopeAccount, "", ""
	}
	if strings.HasPrefix(identifier, "org.") {
		return strings.TrimPrefix(identifier, "org."), ScopeOrg, "", ""
	}
	return identifier, ScopeProject, "", ""
}

func isTemplateNode(node *yaml.Node) bool {
	if node == nil || node.Kind != yaml.MappingNode {
		return false
	}
	tmpl := nodeGet(node, "template")
	if tmpl != nil && tmpl.Kind == yaml.MappingNode && nodeHas(tmpl, "templateRef") {
		return true
	}
	return nodeHas(node, "uses")
}
