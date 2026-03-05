package resolver

import (
	"testing"

	"gopkg.in/yaml.v3"
)

func TestParseIdentifierScope(t *testing.T) {
	for _, tt := range []struct {
		in, id string
		sc     Scope
	}{
		{"myTpl", "myTpl", ScopeProject},
		{"org.myTpl", "myTpl", ScopeOrg},
		{"account.myTpl", "myTpl", ScopeAccount},
	} {
		id, sc, _, _ := parseIdentifierScope(tt.in)
		if id != tt.id || sc != tt.sc {
			t.Errorf("parseIdentifierScope(%q)=(%q,%v), want (%q,%v)", tt.in, id, sc, tt.id, tt.sc)
		}
	}
}

func TestParseTemplateRef_V0V1(t *testing.T) {
	ref, _, err := ParseTemplateRefFromNode(mustParse(t, `
templateRef: org.myTpl
versionLabel: v2`))
	if err != nil {
		t.Fatal(err)
	}
	if ref.Identifier != "myTpl" || ref.Scope != ScopeOrg || ref.VersionLabel != "v2" {
		t.Errorf("V0: %+v", ref)
	}
	ref, inputs, err := ParseTemplateRefFromNode(mustParse(t, `
uses: account.shared@v3
with:
  command: echo hi`))
	if err != nil {
		t.Fatal(err)
	}
	if ref.Identifier != "shared" || ref.Scope != ScopeAccount || ref.VersionLabel != "v3" {
		t.Errorf("V1: %+v", ref)
	}
	if inputs == nil || nodeGet(inputs, "command") == nil {
		t.Error("V1: expected with inputs")
	}
}

func TestMergeTemplateInputs_Basic(t *testing.T) {
	spec := mustParse(t, `
type: Http
spec:
  url: <+input>
  method: GET
  headers:
    Content-Type: application/json
timeout: 10m`)
	inputs := mustParse(t, `
spec:
  url: https://example.com
  headers:
    Authorization: Bearer tok`)
	m := nodeToGoValue(MergeTemplateInputs(spec, inputs)).(map[string]any)
	s, _ := toMap(m["spec"])
	if s["url"] != "https://example.com" {
		t.Errorf("url: got %v", s["url"])
	}
	if s["method"] != "GET" {
		t.Errorf("method: got %v", s["method"])
	}
	h, _ := toMap(s["headers"])
	if h["Content-Type"] != "application/json" || h["Authorization"] != "Bearer tok" {
		t.Errorf("headers: got %v", h)
	}
	m2 := nodeToGoValue(MergeTemplateInputs(mustParse(t, `type: X`), nil)).(map[string]any)
	if m2["type"] != "X" {
		t.Error("nil inputs changed spec")
	}
}

func TestMergeArraysByIdentifier_Basic(t *testing.T) {
	spec := mustParse(t, `
variables:
  - name: v1
    type: String
    value: <+input>
  - name: v2
    type: String
    value: fixed`)
	inputs := mustParse(t, `
variables:
  - name: v1
    type: String
    value: overridden`)
	m := nodeToGoValue(MergeTemplateInputs(spec, inputs)).(map[string]any)
	vars, _ := toSlice(m["variables"])
	if len(vars) != 2 {
		t.Fatalf("want 2 vars, got %d", len(vars))
	}
	v0, _ := toMap(vars[0])
	v1, _ := toMap(vars[1])
	if v0["value"] != "overridden" {
		t.Errorf("v1: got %v", v0["value"])
	}
	if v1["value"] != "fixed" {
		t.Errorf("v2: got %v", v1["value"])
	}
}

func TestMergeTemplateInputs_SimpleScalarOverride(t *testing.T) {
	spec := mustParse(t, `url: <+input>
method: GET`)
	inputs := mustParse(t, `url: https://example.com`)
	result := MergeTemplateInputs(spec, inputs)
	urlNode := nodeGet(result, "url")
	if urlNode == nil || urlNode.Value != "https://example.com" {
		t.Errorf("expected url=https://example.com, got %v", urlNode)
	}
	methodNode := nodeGet(result, "method")
	if methodNode == nil || methodNode.Value != "GET" {
		t.Errorf("expected method=GET, got %v", methodNode)
	}
}

func TestMergeTemplateInputs_NestedMaps(t *testing.T) {
	spec := mustParse(t, `spec:
  url: <+input>
  timeout: 10m`)
	inputs := mustParse(t, `spec:
  url: https://example.com`)
	result := MergeTemplateInputs(spec, inputs)
	specNode := nodeGet(result, "spec")
	if specNode == nil {
		t.Fatal("spec node not found")
	}
	urlNode := nodeGet(specNode, "url")
	if urlNode == nil || urlNode.Value != "https://example.com" {
		t.Errorf("expected url=https://example.com, got %v", urlNode)
	}
	timeoutNode := nodeGet(specNode, "timeout")
	if timeoutNode == nil || timeoutNode.Value != "10m" {
		t.Errorf("expected timeout=10m, got %v", timeoutNode)
	}
}

func TestUnwrapArrayElement(t *testing.T) {
	tests := []struct {
		name, yaml, wantKey string
		wantFound           bool
	}{
		{"stage wrapper", "stage:\n  identifier: s1\n  name: Stage 1", "stage", true},
		{"step wrapper", "step:\n  identifier: step1\n  type: ShellScript", "step", true},
		{"direct with id", "identifier: var1\nname: Variable 1", "_direct", true},
		{"no wrapper", "someKey: someValue", "", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			node := mustParse(t, tt.yaml)
			key, inner := unwrapArrayElement(node)
			if key != tt.wantKey {
				t.Errorf("key=%q, want %q", key, tt.wantKey)
			}
			if (inner != nil) != tt.wantFound {
				t.Errorf("found=%v, want %v", inner != nil, tt.wantFound)
			}
		})
	}
}

func TestGetElementIdentifier(t *testing.T) {
	tests := []struct {
		name, yaml, want string
	}{
		{"identifier field", "identifier: elem1\nname: Element 1", "elem1"},
		{"name only", "name: elem1\ntype: String", "elem1"},
		{"identifier wins", "identifier: id1\nname: name1", "id1"},
		{"none", "type: String\nvalue: test", ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := getElementIdentifier(mustParse(t, tt.yaml))
			if got != tt.want {
				t.Errorf("got %q, want %q", got, tt.want)
			}
		})
	}
}

func TestParseTemplateRefFromNode_V0(t *testing.T) {
	tests := []struct {
		name      string
		yaml      string
		wantID    string
		wantScope Scope
		wantInput bool
	}{
		{"basic", "templateRef: myTemplate\nversionLabel: v1", "myTemplate", ScopeProject, false},
		{"org", "templateRef: org.myTemplate\nversionLabel: v1", "myTemplate", ScopeOrg, false},
		{"account", "templateRef: account.myTemplate\nversionLabel: v1", "myTemplate", ScopeAccount, false},
		{"with inputs", "templateRef: myTemplate\nversionLabel: v1\ntemplateInputs:\n  spec:\n    url: x", "myTemplate", ScopeProject, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ref, inputs, err := ParseTemplateRefFromNode(mustParse(t, tt.yaml))
			if err != nil {
				t.Fatal(err)
			}
			if ref.Identifier != tt.wantID || ref.Scope != tt.wantScope {
				t.Errorf("got id=%q scope=%v", ref.Identifier, ref.Scope)
			}
			if (inputs != nil) != tt.wantInput {
				t.Errorf("inputs=%v, want %v", inputs != nil, tt.wantInput)
			}
		})
	}
}

func TestParseTemplateRefFromNode_V1(t *testing.T) {
	tests := []struct {
		name, yaml, wantID, wantVer string
		wantInput                    bool
	}{
		{"no version", "uses: myTemplate", "myTemplate", "", false},
		{"with version", "uses: myTemplate@v1", "myTemplate", "v1", false},
		{"with inputs", "uses: myTemplate@v1\nwith:\n  url: x", "myTemplate", "v1", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ref, inputs, err := ParseTemplateRefFromNode(mustParse(t, tt.yaml))
			if err != nil {
				t.Fatal(err)
			}
			if ref.Identifier != tt.wantID || ref.VersionLabel != tt.wantVer {
				t.Errorf("got id=%q ver=%q", ref.Identifier, ref.VersionLabel)
			}
			if (inputs != nil) != tt.wantInput {
				t.Errorf("inputs=%v, want %v", inputs != nil, tt.wantInput)
			}
		})
	}
}

func TestNodeHelpers(t *testing.T) {
	t.Run("get and set", func(t *testing.T) {
		node := &yaml.Node{Kind: yaml.MappingNode}
		nodeSet(node, "key1", nodeScalar("value1"))
		if r := nodeGet(node, "key1"); r == nil || r.Value != "value1" {
			t.Errorf("got %v", r)
		}
		if nodeGet(node, "none") != nil {
			t.Error("expected nil")
		}
		nodeSet(node, "key1", nodeScalar("value2"))
		if r := nodeGet(node, "key1"); r == nil || r.Value != "value2" {
			t.Errorf("got %v", r)
		}
	})
	t.Run("has", func(t *testing.T) {
		node := &yaml.Node{Kind: yaml.MappingNode}
		nodeSet(node, "k", nodeScalar("v"))
		if !nodeHas(node, "k") || nodeHas(node, "x") {
			t.Error("unexpected")
		}
	})
	t.Run("keys", func(t *testing.T) {
		node := &yaml.Node{Kind: yaml.MappingNode}
		nodeSet(node, "a", nodeScalar("1"))
		nodeSet(node, "b", nodeScalar("2"))
		keys := nodeKeys(node)
		if len(keys) != 2 || keys[0] != "a" || keys[1] != "b" {
			t.Errorf("got %v", keys)
		}
	})
	t.Run("deep clone isolation", func(t *testing.T) {
		orig := &yaml.Node{Kind: yaml.MappingNode}
		nodeSet(orig, "k", nodeScalar("v"))
		clone := nodeDeepClone(orig)
		nodeSet(clone, "k", nodeScalar("x"))
		if nodeGet(orig, "k").Value != "v" {
			t.Error("clone affected original")
		}
	})
}

func TestNodeToGoValue(t *testing.T) {
	for _, tt := range []struct {
		name, yaml string
	}{
		{"map", "key1: value1\nkey2: value2"},
		{"nested", "outer:\n  inner:\n    key: value"},
		{"array", "- item1\n- item2"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			if nodeToGoValue(mustParse(t, tt.yaml)) == nil {
				t.Fatal("nil")
			}
		})
	}
}
