package resolver

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

type mockTemplateStore struct {
	templates map[string]*TemplateEntity
}

func newMockStore() *mockTemplateStore {
	return &mockTemplateStore{templates: make(map[string]*TemplateEntity)}
}

func (m *mockTemplateStore) add(id, ver, y string) {
	m.templates[id+"@"+ver] = &TemplateEntity{Identifier: id, VersionLabel: ver, YAML: y}
}

func (m *mockTemplateStore) GetTemplate(_ context.Context, _ string, ref TemplateRef) (*TemplateEntity, error) {
	if e, ok := m.templates[ref.Identifier+"@"+ref.VersionLabel]; ok {
		return e, nil
	}
	return nil, fmt.Errorf("%w: %s@%s", ErrTemplateNotFound, ref.Identifier, ref.VersionLabel)
}

func toMap(v any) (map[string]any, bool)  { m, ok := v.(map[string]any); return m, ok }
func toSlice(v any) ([]any, bool)         { s, ok := v.([]any); return s, ok }

func mustParse(t *testing.T, s string) *yaml.Node {
	t.Helper()
	n, err := unmarshalYAMLNode(s)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	return n
}

func mustResolve(t *testing.T, store *mockTemplateStore, py string) *ResolvedPipeline {
	t.Helper()
	res, err := New(store, nil).ResolvePipeline(context.Background(), "a", "o", "p", py)
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}
	return res
}

func digStage(t *testing.T, y string, idx int) map[string]any {
	t.Helper()
	var m map[string]any
	yaml.Unmarshal([]byte(y), &m)
	p, _ := toMap(m["pipeline"])
	ss, _ := toSlice(p["stages"])
	sw, _ := toMap(ss[idx])
	s, _ := toMap(sw["stage"])
	return s
}

func digStep(t *testing.T, stg map[string]any, idx int) map[string]any {
	t.Helper()
	sp, _ := toMap(stg["spec"])
	ex, _ := toMap(sp["execution"])
	steps, _ := toSlice(ex["steps"])
	sw, _ := toMap(steps[idx])
	s, _ := toMap(sw["step"])
	return s
}

func TestResolvePipeline_NoTemplates(t *testing.T) {
	res := mustResolve(t, newMockStore(), `
pipeline:
  name: s
  identifier: s
  stages:
    - stage:
        name: b
        identifier: b
        type: CI
        spec:
          execution:
            steps:
              - step:
                  type: Run
                  name: e
                  identifier: e
                  spec:
                    command: echo hello`)
	if len(res.TemplatesUsed) != 0 {
		t.Errorf("want 0, got %d", len(res.TemplatesUsed))
	}
}

func TestResolvePipeline_StepTemplate(t *testing.T) {
	s := newMockStore()
	s.add("httpTpl", "v1", `
template:
  identifier: httpTpl
  versionLabel: v1
  type: Step
  spec:
    type: Http
    spec:
      url: <+input>
      method: GET`)
	res := mustResolve(t, s, `
pipeline:
  name: t
  identifier: t
  stages:
    - stage:
        name: s
        identifier: s
        type: CI
        spec:
          execution:
            steps:
              - step:
                  identifier: h
                  name: h
                  template:
                    templateRef: httpTpl
                    versionLabel: v1
                    templateInputs:
                      spec:
                        url: https://example.com`)
	if len(res.TemplatesUsed) != 1 {
		t.Errorf("want 1 template, got %d", len(res.TemplatesUsed))
	}
	step := digStep(t, digStage(t, res.ResolvedYAML, 0), 0)
	sp, _ := toMap(step["spec"])
	if sp["url"] != "https://example.com" {
		t.Errorf("url: got %v", sp["url"])
	}
}

func TestResolvePipeline_Nested(t *testing.T) {
	store := newMockStore()
	store.add("shellTpl", "v1", `
template:
  identifier: shellTpl
  versionLabel: v1
  type: Step
  spec:
    type: ShellScript
    spec:
      shell: Bash
    timeout: 10m`)
	store.add("buildStage", "v1", `
template:
  identifier: buildStage
  versionLabel: v1
  type: Stage
  spec:
    type: Custom
    spec:
      execution:
        steps:
          - step:
              identifier: sh
              name: sh
              template:
                templateRef: shellTpl
                versionLabel: v1`)
	res := mustResolve(t, store, `
pipeline:
  name: n
  identifier: n
  stages:
    - stage:
        name: b
        identifier: b
        template:
          templateRef: buildStage
          versionLabel: v1`)
	if len(res.TemplatesUsed) != 2 {
		t.Errorf("want 2 templates, got %d", len(res.TemplatesUsed))
	}
	stage := digStage(t, res.ResolvedYAML, 0)
	if stage["type"] != "Custom" {
		t.Errorf("want Custom, got %v", stage["type"])
	}
}

func TestResolvePipeline_MaxDepth(t *testing.T) {
	store := newMockStore()
	store.add("rec", "v1", `
template:
  identifier: rec
  versionLabel: v1
  type: Step
  spec:
    type: Custom
    spec:
      steps:
        - step:
            identifier: i
            name: i
            template:
              templateRef: rec
              versionLabel: v1`)
	r := New(store, nil)
	_, err := r.ResolvePipeline(context.Background(), "a", "o", "p", `
pipeline:
  name: d
  identifier: d
  stages:
    - stage:
        name: t
        identifier: t
        type: Custom
        spec:
          execution:
            steps:
              - step:
                  identifier: s
                  name: s
                  template:
                    templateRef: rec
                    versionLabel: v1`)
	if err == nil || !strings.Contains(err.Error(), "max template recursion depth") {
		t.Fatalf("want max depth error, got: %v", err)
	}
}

func TestResolvePipeline_FullE2E(t *testing.T) {
	s := newMockStore()
	s.add("sTpl", "v1", `
template:
  identifier: sTpl
  versionLabel: v1
  type: Step
  spec:
    type: ShellScript
    spec:
      shell: Bash`)
	s.add("stgTpl", "v1", `
template:
  identifier: stgTpl
  versionLabel: v1
  type: Stage
  spec:
    type: Deployment
    spec:
      deploymentType: Kubernetes
    variables:
      - name: v1
        type: String
        value: <+input>`)
	s.add("pTpl", "v1", `
template:
  identifier: pTpl
  versionLabel: v1
  type: Pipeline
  spec:
    stages:
      - stage:
          name: sh
          identifier: sh
          type: Custom
          spec:
            execution:
              steps:
                - step:
                    identifier: s1
                    name: s1
                    template:
                      templateRef: sTpl
                      versionLabel: v1
      - stage:
          name: cd
          identifier: cd
          template:
            templateRef: stgTpl
            versionLabel: v1`)
	res := mustResolve(t, s, `
pipeline:
  name: e
  identifier: e
  template:
    templateRef: pTpl
    versionLabel: v1
    templateInputs:
      stages:
        - stage:
            identifier: cd
            template:
              templateInputs:
                type: Deployment
                variables:
                  - name: v1
                    type: String
                    value: done`)
	if len(res.TemplatesUsed) != 3 {
		t.Errorf("want 3, got %d", len(res.TemplatesUsed))
	}
	step := digStep(t, digStage(t, res.ResolvedYAML, 0), 0)
	if step["type"] != "ShellScript" {
		t.Errorf("want ShellScript, got %v", step["type"])
	}
	cd := digStage(t, res.ResolvedYAML, 1)
	if cd["type"] != "Deployment" {
		t.Errorf("want Deployment, got %v", cd["type"])
	}
}
