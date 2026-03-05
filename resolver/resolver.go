package resolver

import (
	"context"
	"fmt"
	"log"

	"gopkg.in/yaml.v3"
)

// Resolver performs recursive pipeline YAML template resolution.
type Resolver struct {
	store  TemplateStore
	loader ResourceLoader
	Cfg    ResolverConfig
}

// New creates a Resolver with the given store and loader.
func New(store TemplateStore, loader ResourceLoader) *Resolver {
	return &Resolver{
		store:  store,
		loader: loader,
	}
}

// ResolvePipeline takes raw pipeline YAML and returns the fully-resolved
// YAML with all template references expanded.
func (r *Resolver) ResolvePipeline(ctx context.Context, accountID, orgID, projectID, pipelineYAML string) (*ResolvedPipeline, error) {
	doc, err := unmarshalYAMLNode(pipelineYAML)
	if err != nil {
		return nil, fmt.Errorf("parse pipeline yaml: %w", err)
	}

	var templatesUsed []TemplateRef
	pipelineTemplateResolved := false

	pipelineNode := nodeGet(doc, "pipeline")
	if pipelineNode != nil && pipelineNode.Kind == yaml.MappingNode {
		if isTemplateNode(pipelineNode) {
			resolved, refs, err := r.resolveTemplateNode(ctx, accountID, orgID, projectID, pipelineNode, 0)
			if err != nil {
				return nil, fmt.Errorf("resolve pipeline template: %w", err)
			}
			nodeSet(doc, "pipeline", resolved)
			templatesUsed = append(templatesUsed, refs...)
			pipelineTemplateResolved = true
		}
	}

	if !pipelineTemplateResolved {
		resolved, refs, err := r.resolveNode(ctx, accountID, orgID, projectID, doc, 0)
		if err != nil {
			return nil, err
		}
		doc = resolved
		templatesUsed = append(templatesUsed, refs...)
	}

	resolvedYAML, err := marshalYAML(doc)
	if err != nil {
		return nil, fmt.Errorf("serialize resolved yaml: %w", err)
	}

	return &ResolvedPipeline{
		OriginalYAML:  pipelineYAML,
		ResolvedYAML:  resolvedYAML,
		TemplatesUsed: templatesUsed,
	}, nil
}

// LoadAndResolvePipeline fetches the pipeline YAML from SCM and resolves it.
func (r *Resolver) LoadAndResolvePipeline(ctx context.Context, accountID, orgID, projectID, parentUniqueID string, fileRef FileRef) (*ResolvedPipeline, error) {
	if r.loader == nil {
		return nil, ErrNoLoader
	}

	data, err := r.loader.Find(ctx, fileRef.Repo, fileRef.Path, fileRef.Ref)
	if err != nil {
		return nil, fmt.Errorf("load pipeline %s/%s@%s: %w", fileRef.Repo, fileRef.Path, fileRef.Ref, err)
	}

	return r.ResolvePipeline(ctx, accountID, orgID, projectID, string(data))
}

func (r *Resolver) resolveNode(ctx context.Context, accountID, orgID, projectID string, node *yaml.Node, depth int) (*yaml.Node, []TemplateRef, error) {
	if depth > MaxTemplateDepth {
		return nil, nil, ErrMaxDepth
	}
	if node == nil {
		return node, nil, nil
	}
	switch node.Kind {
	case yaml.MappingNode:
		return r.resolveMapNode(ctx, accountID, orgID, projectID, node, depth)
	case yaml.SequenceNode:
		return r.resolveArrayNode(ctx, accountID, orgID, projectID, node, depth)
	default:
		return node, nil, nil
	}
}

func (r *Resolver) resolveMapNode(ctx context.Context, accountID, orgID, projectID string, node *yaml.Node, depth int) (*yaml.Node, []TemplateRef, error) {
	var allRefs []TemplateRef
	for i := 0; i+1 < len(node.Content); i += 2 {
		key := node.Content[i].Value
		value := node.Content[i+1]
		if value.Kind == yaml.MappingNode && isEntityWithTemplate(key, value) {
			resolved, refs, err := r.resolveEntityTemplate(ctx, accountID, orgID, projectID, key, value, depth)
			if err != nil {
				return nil, nil, fmt.Errorf("resolve %s template: %w", key, err)
			}
			node.Content[i+1] = resolved
			allRefs = append(allRefs, refs...)
			continue
		}
		resolved, refs, err := r.resolveNode(ctx, accountID, orgID, projectID, value, depth)
		if err != nil {
			return nil, nil, err
		}
		node.Content[i+1] = resolved
		allRefs = append(allRefs, refs...)
	}
	return node, allRefs, nil
}

func (r *Resolver) resolveArrayNode(ctx context.Context, accountID, orgID, projectID string, node *yaml.Node, depth int) (*yaml.Node, []TemplateRef, error) {
	var allRefs []TemplateRef
	for idx, elem := range node.Content {
		resolved, refs, err := r.resolveNode(ctx, accountID, orgID, projectID, elem, depth)
		if err != nil {
			return nil, nil, err
		}
		node.Content[idx] = resolved
		allRefs = append(allRefs, refs...)
	}
	return node, allRefs, nil
}

func isEntityWithTemplate(key string, node *yaml.Node) bool {
	switch key {
	case "step", "stage", "stepGroup":
		return isTemplateNode(node)
	default:
		return false
	}
}

func (r *Resolver) resolveEntityTemplate(ctx context.Context, accountID, orgID, projectID, entityType string, entityNode *yaml.Node, depth int) (*yaml.Node, []TemplateRef, error) {
	var templateRefNode *yaml.Node
	tmpl := nodeGet(entityNode, "template")
	if tmpl != nil && tmpl.Kind == yaml.MappingNode {
		templateRefNode = tmpl
	} else if nodeHas(entityNode, "uses") {
		templateRefNode = entityNode
	}
	if templateRefNode == nil {
		return entityNode, nil, nil
	}

	ref, inputs, err := ParseTemplateRefFromNode(templateRefNode)
	if err != nil {
		return nil, nil, err
	}
	if ref.Scope == ScopeProject {
		ref.OrgIdentifier = orgID
		ref.ProjectIdentifier = projectID
	} else if ref.Scope == ScopeOrg {
		ref.OrgIdentifier = orgID
	}

	log.Printf("[resolver] resolving %s template %s@%s (scope=%s, depth=%d)",
		entityType, ref.Identifier, ref.VersionLabel, ref.Scope, depth)

	entity, err := r.store.GetTemplate(ctx, accountID, *ref)
	if err != nil {
		return nil, nil, err
	}

	spec, _, err := ExtractTemplateSpec(entity.YAML)
	if err != nil {
		return nil, nil, fmt.Errorf("extract spec from template %s@%s: %w", ref.Identifier, ref.VersionLabel, err)
	}

	mergedSpec := MergeTemplateInputs(spec, inputs)
	resolved := nodeDeepClone(entityNode)
	overlayMappingFields(resolved, mergedSpec)

	allRefs := []TemplateRef{*ref}
	finalResolved, childRefs, err := r.resolveMapNode(ctx, accountID, orgID, projectID, resolved, depth+1)
	if err != nil {
		return nil, nil, fmt.Errorf("resolve nested templates in %s@%s: %w", ref.Identifier, ref.VersionLabel, err)
	}
	allRefs = append(allRefs, childRefs...)
	return finalResolved, allRefs, nil
}

func (r *Resolver) resolveTemplateNode(ctx context.Context, accountID, orgID, projectID string, parentNode *yaml.Node, depth int) (*yaml.Node, []TemplateRef, error) {
	if depth > MaxTemplateDepth {
		return nil, nil, ErrMaxDepth
	}

	var templateRefNode *yaml.Node
	tmpl := nodeGet(parentNode, "template")
	if tmpl != nil && tmpl.Kind == yaml.MappingNode {
		templateRefNode = tmpl
	}
	if templateRefNode == nil && nodeHas(parentNode, "uses") {
		templateRefNode = parentNode
	}
	if templateRefNode == nil {
		return r.resolveMapNode(ctx, accountID, orgID, projectID, parentNode, depth)
	}

	ref, inputs, err := ParseTemplateRefFromNode(templateRefNode)
	if err != nil {
		return nil, nil, err
	}
	if ref.Scope == ScopeProject {
		ref.OrgIdentifier = orgID
		ref.ProjectIdentifier = projectID
	} else if ref.Scope == ScopeOrg {
		ref.OrgIdentifier = orgID
	}

	log.Printf("[resolver] resolving template %s@%s (scope=%s, depth=%d)",
		ref.Identifier, ref.VersionLabel, ref.Scope, depth)

	entity, err := r.store.GetTemplate(ctx, accountID, *ref)
	if err != nil {
		return nil, nil, err
	}

	spec, templateType, err := ExtractTemplateSpec(entity.YAML)
	if err != nil {
		return nil, nil, fmt.Errorf("extract spec from template %s@%s: %w", ref.Identifier, ref.VersionLabel, err)
	}

	mergedSpec := MergeTemplateInputs(spec, inputs)
	resolved := buildResolvedNode(parentNode, mergedSpec, templateType)

	allRefs := []TemplateRef{*ref}
	finalResolved, childRefs, err := r.resolveMapNode(ctx, accountID, orgID, projectID, resolved, depth+1)
	if err != nil {
		return nil, nil, fmt.Errorf("resolve nested templates in %s@%s: %w", ref.Identifier, ref.VersionLabel, err)
	}
	allRefs = append(allRefs, childRefs...)
	return finalResolved, allRefs, nil
}

func buildResolvedNode(parentNode, mergedSpec *yaml.Node, templateType string) *yaml.Node {
	resolved := nodeDeepClone(parentNode)
	overlayMappingFields(resolved, mergedSpec)
	return resolved
}

func overlayMappingFields(dst, src *yaml.Node) {
	if dst == nil || src == nil || dst.Kind != yaml.MappingNode || src.Kind != yaml.MappingNode {
		return
	}
	for i := 0; i+1 < len(src.Content); i += 2 {
		key := src.Content[i].Value
		val := src.Content[i+1]
		nodeSet(dst, key, nodeDeepClone(val))
	}
}
