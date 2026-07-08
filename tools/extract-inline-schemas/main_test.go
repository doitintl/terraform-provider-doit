package main

import (
	"testing"

	"gopkg.in/yaml.v3"
)

// --- toPascalCase ---

func TestToPascalCase(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"customTimeRange", "CustomTimeRange"},
		{"list_allocations", "ListAllocations"},
		{"simple", "Simple"},
		{"already_Pascal", "AlreadyPascal"},
		{"ALL_CAPS", "ALLCAPS"},
		{"with123numbers", "With123numbers"},
		{"a", "A"},
		{"", ""},
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := toPascalCase(tt.input)
			if got != tt.want {
				t.Errorf("toPascalCase(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

// --- isHTTPMethod ---

func TestIsHTTPMethod(t *testing.T) {
	for _, m := range []string{"get", "post", "put", "delete", "patch", "head", "options"} {
		if !isHTTPMethod(m) {
			t.Errorf("isHTTPMethod(%q) = false, want true", m)
		}
	}
	for _, m := range []string{"GET", "foo", "subscribe", ""} {
		if isHTTPMethod(m) {
			t.Errorf("isHTTPMethod(%q) = true, want false", m)
		}
	}
}

// --- YAML helpers ---

func mustParseYAML(t *testing.T, input string) *yaml.Node {
	t.Helper()
	var doc yaml.Node
	if err := yaml.Unmarshal([]byte(input), &doc); err != nil {
		t.Fatalf("failed to parse YAML: %v", err)
	}
	// yaml.Unmarshal wraps in a document node
	if doc.Kind == yaml.DocumentNode && len(doc.Content) > 0 {
		return doc.Content[0]
	}
	return &doc
}

// --- isInlineObject ---

func TestIsInlineObject(t *testing.T) {
	tests := []struct {
		name string
		yaml string
		want bool
	}{
		{
			name: "object with properties",
			yaml: `
type: object
properties:
  name:
    type: string
`,
			want: true,
		},
		{
			name: "object without properties",
			yaml: `
type: object
`,
			want: false,
		},
		{
			name: "object with $ref",
			yaml: `
type: object
properties:
  name:
    type: string
$ref: "#/components/schemas/Foo"
`,
			want: false,
		},
		{
			name: "string type",
			yaml: `
type: string
`,
			want: false,
		},
		{
			name: "no type but has properties",
			yaml: `
properties:
  name:
    type: string
`,
			want: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			node := mustParseYAML(t, tt.yaml)
			got := isInlineObject(node)
			if got != tt.want {
				t.Errorf("isInlineObject() = %v, want %v", got, tt.want)
			}
		})
	}
}

// --- isComposition ---

func TestIsComposition(t *testing.T) {
	tests := []struct {
		name string
		yaml string
		want bool
	}{
		{
			name: "allOf composition",
			yaml: `
allOf:
  - $ref: "#/components/schemas/Foo"
  - type: object
    properties:
      bar:
        type: string
`,
			want: true,
		},
		{
			name: "anyOf composition",
			yaml: `
anyOf:
  - $ref: "#/components/schemas/Foo"
  - $ref: "#/components/schemas/Bar"
`,
			want: true,
		},
		{
			name: "oneOf composition",
			yaml: `
oneOf:
  - $ref: "#/components/schemas/Foo"
`,
			want: true,
		},
		{
			name: "already a ref",
			yaml: `
$ref: "#/components/schemas/Foo"
`,
			want: false,
		},
		{
			name: "plain object",
			yaml: `
type: object
properties:
  name:
    type: string
`,
			want: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			node := mustParseYAML(t, tt.yaml)
			got := isComposition(node)
			if got != tt.want {
				t.Errorf("isComposition() = %v, want %v", got, tt.want)
			}
		})
	}
}

// --- isInlineArray ---

func TestIsInlineArray(t *testing.T) {
	tests := []struct {
		name string
		yaml string
		want bool
	}{
		{
			name: "array with inline object items",
			yaml: `
type: array
items:
  type: object
  properties:
    id:
      type: string
`,
			want: true,
		},
		{
			name: "array with ref items",
			yaml: `
type: array
items:
  $ref: "#/components/schemas/Foo"
`,
			want: false,
		},
		{
			name: "array with string items",
			yaml: `
type: array
items:
  type: string
`,
			want: false,
		},
		{
			name: "not an array",
			yaml: `
type: object
properties:
  name:
    type: string
`,
			want: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			node := mustParseYAML(t, tt.yaml)
			got := isInlineArray(node)
			if got != tt.want {
				t.Errorf("isInlineArray() = %v, want %v", got, tt.want)
			}
		})
	}
}

// --- uniqueName ---

func TestUniqueName(t *testing.T) {
	e := &Extractor{extracted: map[string]*yaml.Node{}}

	// First use gets the base name
	name1 := e.uniqueName("Foo")
	if name1 != "Foo" {
		t.Errorf("first uniqueName = %q, want %q", name1, "Foo")
	}

	// Put something in so the name collides
	e.extracted["Bar"] = &yaml.Node{}

	// Collision appends suffix
	name2 := e.uniqueName("Bar")
	if name2 != "Bar2" {
		t.Errorf("second uniqueName = %q, want %q", name2, "Bar2")
	}

	// Second collision gets Bar3
	e.extracted["Bar2"] = &yaml.Node{}
	name3 := e.uniqueName("Bar")
	if name3 != "Bar3" {
		t.Errorf("third uniqueName = %q, want %q", name3, "Bar3")
	}
}

// --- replaceWithRef ---

func TestReplaceWithRef_NoDescription(t *testing.T) {
	node := mustParseYAML(t, `
prop:
  type: object
  properties:
    name:
      type: string
`)
	replaceWithRef(node, "prop", "MySchema")

	// The prop value should now be a $ref
	val := getMappingValue(node, "prop")
	if val == nil {
		t.Fatal("prop value is nil")
	}
	ref := getScalarValue(val, "$ref")
	if ref != "#/components/schemas/MySchema" {
		t.Errorf("$ref = %q, want %q", ref, "#/components/schemas/MySchema")
	}
}

func TestReplaceWithRef_WithDescription(t *testing.T) {
	node := mustParseYAML(t, `
prop:
  description: A nice property
  type: object
  properties:
    name:
      type: string
`)
	replaceWithRef(node, "prop", "MySchema")

	// Should wrap in allOf to preserve description
	val := getMappingValue(node, "prop")
	if val == nil {
		t.Fatal("prop value is nil")
	}
	desc := getScalarValue(val, "description")
	if desc != "A nice property" {
		t.Errorf("description = %q, want %q", desc, "A nice property")
	}
	allOf := getMappingValue(val, "allOf")
	if allOf == nil {
		t.Fatal("allOf is nil — expected allOf wrapper for description")
	}
	if allOf.Kind != yaml.SequenceNode || len(allOf.Content) != 1 {
		t.Fatalf("allOf should be a sequence with 1 element, got kind=%d len=%d", allOf.Kind, len(allOf.Content))
	}
	ref := getScalarValue(allOf.Content[0], "$ref")
	if ref != "#/components/schemas/MySchema" {
		t.Errorf("allOf[0].$ref = %q, want %q", ref, "#/components/schemas/MySchema")
	}
}

// --- replaceSchemaValueWithRef ---

func TestReplaceSchemaValueWithRef(t *testing.T) {
	node := mustParseYAML(t, `
schema:
  description: Request body schema
  type: object
  properties:
    name:
      type: string
`)
	replaceSchemaValueWithRef(node, "schema", "MySchema")

	val := getMappingValue(node, "schema")
	if val == nil {
		t.Fatal("schema value is nil")
	}
	ref := getScalarValue(val, "$ref")
	if ref != "#/components/schemas/MySchema" {
		t.Errorf("$ref = %q, want %q", ref, "#/components/schemas/MySchema")
	}
	// Should NOT have allOf wrapping
	if getMappingValue(val, "allOf") != nil {
		t.Error("replaceSchemaValueWithRef should not wrap in allOf")
	}
	// Should NOT have description (it's on the extracted schema)
	if getScalarValue(val, "description") != "" {
		t.Error("replaceSchemaValueWithRef should not preserve description")
	}
}

// --- extractInlineObject ---

func TestExtractInlineObject(t *testing.T) {
	e := &Extractor{extracted: map[string]*yaml.Node{}}
	node := mustParseYAML(t, `
type: object
properties:
  name:
    type: string
  age:
    type: integer
`)

	extracted := e.extractInlineObject(node, "TestObj")
	if extracted == nil {
		t.Fatal("extracted is nil")
	}
	if _, ok := e.extracted["TestObj"]; !ok {
		t.Fatal("extracted map does not contain TestObj")
	}

	// Extracted should be a deep copy (not the same pointer)
	if extracted == node {
		t.Error("extracted should be a deep copy, not the same node")
	}

	// Verify extracted has the expected structure
	nameType := getScalarValue(getMappingValue(getMappingValue(extracted, "properties"), "name"), "type")
	if nameType != "string" {
		t.Errorf("extracted properties.name.type = %q, want %q", nameType, "string")
	}
}

// --- stripDocFields ---

func TestStripDocFields(t *testing.T) {
	t.Run("removes description and example", func(t *testing.T) {
		input := map[string]any{
			"type":        "object",
			"description": "A thing",
			"example":     "foo",
			"properties": map[string]any{
				"name": map[string]any{
					"type":        "string",
					"description": "The name",
				},
			},
		}
		got := stripDocFields(input).(map[string]any)
		if _, ok := got["description"]; ok {
			t.Error("description should be stripped")
		}
		if _, ok := got["example"]; ok {
			t.Error("example should be stripped")
		}
		if _, ok := got["type"]; !ok {
			t.Error("type should remain")
		}
		props := got["properties"].(map[string]any)
		nameProps := props["name"].(map[string]any)
		if _, ok := nameProps["description"]; ok {
			t.Error("nested description should be stripped")
		}
	})

	t.Run("unwraps single-element allOf", func(t *testing.T) {
		// Simulates what replaceWithRef produces after description is stripped:
		// {allOf: [{type: object, ...}]} should unwrap to {type: object, ...}
		input := map[string]any{
			"description": "Will be stripped",
			"allOf": []any{
				map[string]any{
					"type": "object",
					"properties": map[string]any{
						"name": map[string]any{"type": "string"},
					},
				},
			},
		}
		got := stripDocFields(input).(map[string]any)
		// After stripping description, only allOf remains as a single key.
		// The single-element allOf should be unwrapped.
		if _, ok := got["allOf"]; ok {
			t.Error("single-element allOf should be unwrapped")
		}
		if got["type"] != "object" {
			t.Errorf("type = %v, want %q", got["type"], "object")
		}
	})

	t.Run("merges multi-element allOf with object items", func(t *testing.T) {
		input := map[string]any{
			"allOf": []any{
				map[string]any{
					"type": "object",
					"properties": map[string]any{
						"name": map[string]any{"type": "string"},
					},
				},
				map[string]any{
					"type": "object",
					"properties": map[string]any{
						"extra": map[string]any{"type": "integer"},
					},
				},
			},
		}
		got := stripDocFields(input).(map[string]any)
		if _, ok := got["allOf"]; ok {
			t.Error("multi-element allOf with object items should be merged")
		}
		props, _ := got["properties"].(map[string]any)
		if props == nil {
			t.Fatal("merged result should have properties")
		}
		if _, ok := props["name"]; !ok {
			t.Error("merged properties should include 'name'")
		}
		if _, ok := props["extra"]; !ok {
			t.Error("merged properties should include 'extra'")
		}
	})

	t.Run("does not merge multi-element allOf with non-object items", func(t *testing.T) {
		input := map[string]any{
			"allOf": []any{
				map[string]any{"type": "object"},
				"not a map",
			},
		}
		got := stripDocFields(input).(map[string]any)
		if _, ok := got["allOf"]; !ok {
			t.Error("multi-element allOf with non-object items should NOT be merged")
		}
	})

	t.Run("unwraps single-element allOf with sibling keys by merging", func(t *testing.T) {
		input := map[string]any{
			"type": "object",
			"allOf": []any{
				map[string]any{"properties": map[string]any{}},
			},
		}
		got := stripDocFields(input).(map[string]any)
		if _, ok := got["allOf"]; ok {
			t.Error("single-element allOf with siblings should be unwrapped (merged)")
		}
		if got["type"] != "object" {
			t.Error("sibling key 'type' should be preserved after merge")
		}
		if _, ok := got["properties"]; !ok {
			t.Error("inner 'properties' should be present after merge")
		}
	})

	t.Run("unwraps single-element allOf with overlapping properties using union", func(t *testing.T) {
		input := map[string]any{
			"properties": map[string]any{
				"a": map[string]any{"type": "string"},
			},
			"required": []any{"a"},
			"allOf": []any{
				map[string]any{
					"properties": map[string]any{
						"b": map[string]any{"type": "integer"},
					},
					"required": []any{"b"},
				},
			},
		}
		got := stripDocFields(input).(map[string]any)
		if _, ok := got["allOf"]; ok {
			t.Error("single-element allOf should be unwrapped")
		}
		props := got["properties"].(map[string]any)
		if _, ok := props["a"]; !ok {
			t.Error("sibling property 'a' should be preserved")
		}
		if _, ok := props["b"]; !ok {
			t.Error("inner property 'b' should be preserved")
		}
		req := got["required"].([]any)
		seen := map[string]bool{}
		for _, r := range req {
			seen[r.(string)] = true
		}
		if !seen["a"] || !seen["b"] {
			t.Errorf("required should contain both 'a' and 'b', got %v", req)
		}
	})
}

// --- mergeAllOfItems ---

func TestMergeAllOfItems(t *testing.T) {
	t.Run("merges properties from multiple objects", func(t *testing.T) {
		items := []any{
			map[string]any{
				"type": "object",
				"properties": map[string]any{
					"id":   map[string]any{"type": "string"},
					"name": map[string]any{"type": "string"},
				},
				"required": []any{"name"},
			},
			map[string]any{
				"type": "object",
				"properties": map[string]any{
					"owner": map[string]any{"type": "string"},
				},
			},
		}
		merged, ok := mergeAllOfItems(items)
		if !ok {
			t.Fatal("mergeAllOfItems returned false")
		}
		props := merged["properties"].(map[string]any)
		if len(props) != 3 {
			t.Errorf("expected 3 properties, got %d", len(props))
		}
		if _, ok := props["owner"]; !ok {
			t.Error("missing 'owner' property")
		}
		req := merged["required"].([]any)
		if len(req) != 1 || req[0] != "name" {
			t.Errorf("expected required=[name], got %v", req)
		}
	})

	t.Run("returns false for non-map items", func(t *testing.T) {
		items := []any{
			map[string]any{"type": "object"},
			"not a map",
		}
		_, ok := mergeAllOfItems(items)
		if ok {
			t.Error("expected false for non-map item")
		}
	})

	t.Run("later property wins on conflict", func(t *testing.T) {
		items := []any{
			map[string]any{
				"properties": map[string]any{
					"field": map[string]any{"type": "string"},
				},
			},
			map[string]any{
				"properties": map[string]any{
					"field": map[string]any{"type": "integer"},
				},
			},
		}
		merged, ok := mergeAllOfItems(items)
		if !ok {
			t.Fatal("mergeAllOfItems returned false")
		}
		props := merged["properties"].(map[string]any)
		field := props["field"].(map[string]any)
		if field["type"] != "integer" {
			t.Errorf("conflicting property should use last definition, got type=%v", field["type"])
		}
	})

	t.Run("deduplicates required", func(t *testing.T) {
		items := []any{
			map[string]any{"required": []any{"name", "id"}},
			map[string]any{"required": []any{"name", "extra"}},
		}
		merged, ok := mergeAllOfItems(items)
		if !ok {
			t.Fatal("mergeAllOfItems returned false")
		}
		req := merged["required"].([]any)
		if len(req) != 3 {
			t.Errorf("expected 3 unique required fields, got %d: %v", len(req), req)
		}
	})
}

// --- wrapRefSiblings ---

func TestWrapRefSiblings(t *testing.T) {
	t.Run("wraps $ref with schema keyword siblings", func(t *testing.T) {
		node := mustParseYAML(t, `
root:
  $ref: "#/components/schemas/Foo"
  nullable: true
`)
		root := getMappingValue(node, "root")
		wrapRefSiblings(root)
		if getMappingValue(root, "allOf") == nil {
			t.Error("$ref with nullable should be wrapped in allOf")
		}
		if getMappingValue(root, "$ref") != nil {
			t.Error("$ref should be moved inside allOf")
		}
	})

	t.Run("skips $ref with only description sibling", func(t *testing.T) {
		node := mustParseYAML(t, `
root:
  $ref: "#/components/responses/NotFound"
  description: "Not found response"
`)
		root := getMappingValue(node, "root")
		wrapRefSiblings(root)
		if getMappingValue(root, "allOf") != nil {
			t.Error("$ref with only description sibling should NOT be wrapped")
		}
		if getScalarValue(root, "$ref") != "#/components/responses/NotFound" {
			t.Error("$ref should remain untouched")
		}
	})

	t.Run("skips $ref with only summary sibling", func(t *testing.T) {
		node := mustParseYAML(t, `
root:
  $ref: "#/components/schemas/Bar"
  summary: "A bar"
`)
		root := getMappingValue(node, "root")
		wrapRefSiblings(root)
		if getMappingValue(root, "allOf") != nil {
			t.Error("$ref with only summary sibling should NOT be wrapped")
		}
	})

	t.Run("skips $ref with summary and description siblings only", func(t *testing.T) {
		node := mustParseYAML(t, `
root:
  $ref: "#/components/parameters/PageToken"
  summary: "Pagination token"
  description: "The token returned from the previous page"
`)
		root := getMappingValue(node, "root")
		wrapRefSiblings(root)
		if getMappingValue(root, "allOf") != nil {
			t.Error("$ref with only summary+description siblings should NOT be wrapped")
		}
	})

	t.Run("wraps $ref with description and schema keyword siblings", func(t *testing.T) {
		node := mustParseYAML(t, `
root:
  $ref: "#/components/schemas/Foo"
  description: "A foo"
  nullable: true
`)
		root := getMappingValue(node, "root")
		wrapRefSiblings(root)
		if getMappingValue(root, "allOf") == nil {
			t.Error("$ref with description+nullable should be wrapped (nullable is a schema keyword)")
		}
	})
}

// --- flattenAllOfSchemas ---

func TestFlattenAllOfSchemas(t *testing.T) {
	t.Run("ref plus inline (AlertListItem pattern)", func(t *testing.T) {
		schemasNode := mustParseYAML(t, `
Alert:
  required:
    - name
  type: object
  properties:
    id:
      type: string
    name:
      type: string
    config:
      $ref: "#/components/schemas/AlertConfig"
AlertListItem:
  description: Alert with owner
  allOf:
    - $ref: "#/components/schemas/Alert"
    - type: object
      properties:
        owner:
          type: string
          description: The owner
AlertConfig:
  type: object
  properties:
    value:
      type: number
`)
		flattenAllOfSchemas(schemasNode)

		listItem := getMappingValue(schemasNode, "AlertListItem")
		if listItem == nil {
			t.Fatal("AlertListItem not found")
		}

		// Should be type: object now
		if getScalarValue(listItem, "type") != "object" {
			t.Errorf("type = %q, want %q", getScalarValue(listItem, "type"), "object")
		}

		// Should NOT have allOf anymore
		if getMappingValue(listItem, "allOf") != nil {
			t.Error("allOf should have been removed")
		}

		// Description preserved
		if getScalarValue(listItem, "description") != "Alert with owner" {
			t.Errorf("description = %q, want %q", getScalarValue(listItem, "description"), "Alert with owner")
		}

		// Required merged from Alert
		reqNode := getMappingValue(listItem, "required")
		if reqNode == nil || reqNode.Kind != yaml.SequenceNode {
			t.Fatal("required should be a sequence")
		}
		if len(reqNode.Content) != 1 || reqNode.Content[0].Value != "name" {
			t.Errorf("required = %v, want [name]", reqNode.Content)
		}

		// Properties should include all Alert props + owner
		propsNode := getMappingValue(listItem, "properties")
		if propsNode == nil {
			t.Fatal("properties should exist")
		}
		for _, expected := range []string{"id", "name", "config", "owner"} {
			if getMappingValue(propsNode, expected) == nil {
				t.Errorf("missing property %q", expected)
			}
		}

		// config should still be a $ref
		configProp := getMappingValue(propsNode, "config")
		if getScalarValue(configProp, "$ref") != "#/components/schemas/AlertConfig" {
			t.Error("config property should preserve $ref")
		}

		// owner should have description
		ownerProp := getMappingValue(propsNode, "owner")
		if getScalarValue(ownerProp, "description") != "The owner" {
			t.Errorf("owner description = %q, want %q", getScalarValue(ownerProp, "description"), "The owner")
		}
	})

	t.Run("multiple refs", func(t *testing.T) {
		schemasNode := mustParseYAML(t, `
Base:
  type: object
  required:
    - id
  properties:
    id:
      type: string
Extra:
  type: object
  required:
    - label
  properties:
    label:
      type: string
Combined:
  allOf:
    - $ref: "#/components/schemas/Base"
    - $ref: "#/components/schemas/Extra"
`)
		flattenAllOfSchemas(schemasNode)

		combined := getMappingValue(schemasNode, "Combined")
		if getMappingValue(combined, "allOf") != nil {
			t.Error("allOf should have been removed")
		}
		propsNode := getMappingValue(combined, "properties")
		if getMappingValue(propsNode, "id") == nil {
			t.Error("missing 'id' from Base")
		}
		if getMappingValue(propsNode, "label") == nil {
			t.Error("missing 'label' from Extra")
		}

		reqNode := getMappingValue(combined, "required")
		if reqNode == nil || len(reqNode.Content) != 2 {
			t.Fatalf("expected 2 required fields, got %v", reqNode)
		}
	})

	t.Run("single element allOf is left alone", func(t *testing.T) {
		schemasNode := mustParseYAML(t, `
Base:
  type: object
  properties:
    id:
      type: string
Wrapper:
  allOf:
    - $ref: "#/components/schemas/Base"
`)
		flattenAllOfSchemas(schemasNode)

		wrapper := getMappingValue(schemasNode, "Wrapper")
		if getMappingValue(wrapper, "allOf") == nil {
			t.Error("single-element allOf should NOT be flattened (handled by codegen)")
		}
	})

	t.Run("property conflict uses last definition", func(t *testing.T) {
		schemasNode := mustParseYAML(t, `
A:
  type: object
  properties:
    field:
      type: string
      description: original
B:
  allOf:
    - $ref: "#/components/schemas/A"
    - type: object
      properties:
        field:
          type: integer
          description: override
`)
		flattenAllOfSchemas(schemasNode)

		b := getMappingValue(schemasNode, "B")
		fieldProp := getMappingValue(getMappingValue(b, "properties"), "field")
		if getScalarValue(fieldProp, "type") != "integer" {
			t.Error("conflicting property should use last definition")
		}
		if getScalarValue(fieldProp, "description") != "override" {
			t.Error("conflicting property description should use last definition")
		}
	})
}

func TestFlattenInlineAllOf(t *testing.T) {
	t.Run("inline property allOf (CloudflowConnection pattern)", func(t *testing.T) {
		schemasNode := mustParseYAML(t, `
GCPConfigRequest:
  type: object
  properties:
    projectId:
      type: string
    level:
      type: string
CloudflowConnection:
  type: object
  properties:
    name:
      type: string
    gcpConfig:
      allOf:
        - $ref: "#/components/schemas/GCPConfigRequest"
        - type: object
          properties:
            status:
              type: string
            deploymentCommand:
              type: string
`)
		flattenAllOfSchemas(schemasNode)

		conn := getMappingValue(schemasNode, "CloudflowConnection")
		gcpConfig := getMappingValue(getMappingValue(conn, "properties"), "gcpConfig")
		if gcpConfig == nil {
			t.Fatal("gcpConfig not found")
		}
		if getMappingValue(gcpConfig, "allOf") != nil {
			t.Error("allOf should have been flattened")
		}
		if getScalarValue(gcpConfig, "type") != "object" {
			t.Error("flattened gcpConfig should have type: object")
		}
		props := getMappingValue(gcpConfig, "properties")
		if props == nil {
			t.Fatal("properties not found on flattened gcpConfig")
		}
		for _, name := range []string{"projectId", "level", "status", "deploymentCommand"} {
			if getMappingValue(props, name) == nil {
				t.Errorf("expected property %q in flattened gcpConfig", name)
			}
		}
	})

	t.Run("nested inline allOf", func(t *testing.T) {
		schemasNode := mustParseYAML(t, `
Inner:
  type: object
  properties:
    x:
      type: string
Outer:
  type: object
  properties:
    wrapper:
      type: object
      properties:
        nested:
          allOf:
            - $ref: "#/components/schemas/Inner"
            - type: object
              properties:
                y:
                  type: integer
`)
		flattenAllOfSchemas(schemasNode)

		outer := getMappingValue(schemasNode, "Outer")
		wrapper := getMappingValue(getMappingValue(outer, "properties"), "wrapper")
		nested := getMappingValue(getMappingValue(wrapper, "properties"), "nested")
		if nested == nil {
			t.Fatal("nested not found")
		}
		if getMappingValue(nested, "allOf") != nil {
			t.Error("nested allOf should have been flattened")
		}
		props := getMappingValue(nested, "properties")
		if getMappingValue(props, "x") == nil {
			t.Error("expected property x from $ref")
		}
		if getMappingValue(props, "y") == nil {
			t.Error("expected property y from inline extension")
		}
	})

	t.Run("sibling key preservation", func(t *testing.T) {
		schemasNode := mustParseYAML(t, `
Base:
  type: object
  properties:
    a:
      type: string
Parent:
  type: object
  properties:
    child:
      description: "Keep this description"
      allOf:
        - $ref: "#/components/schemas/Base"
        - type: object
          properties:
            b:
              type: integer
`)
		flattenAllOfSchemas(schemasNode)

		parent := getMappingValue(schemasNode, "Parent")
		child := getMappingValue(getMappingValue(parent, "properties"), "child")
		if child == nil {
			t.Fatal("child not found")
		}
		desc := getScalarValue(child, "description")
		if desc != "Keep this description" {
			t.Errorf("expected parent description to be preserved, got %q", desc)
		}
		if getMappingValue(child, "allOf") != nil {
			t.Error("allOf should have been flattened")
		}
		props := getMappingValue(child, "properties")
		if getMappingValue(props, "a") == nil {
			t.Error("expected property a from $ref")
		}
		if getMappingValue(props, "b") == nil {
			t.Error("expected property b from inline extension")
		}
	})
}

// --- validateEquivalence with flattened allOf ---

func TestValidateEquivalence_FlattenedAllOf(t *testing.T) {
	original := `
openapi: "3.0.1"
info:
  title: Test
  version: v1
paths:
  /items:
    get:
      operationId: listItems
      responses:
        "200":
          description: OK
          content:
            application/json:
              schema:
                $ref: "#/components/schemas/ItemList"
components:
  schemas:
    Item:
      type: object
      required:
        - name
      properties:
        id:
          type: string
        name:
          type: string
    ItemListEntry:
      description: Item in a list
      allOf:
        - $ref: "#/components/schemas/Item"
        - type: object
          properties:
            owner:
              type: string
    ItemList:
      type: object
      properties:
        items:
          type: array
          items:
            $ref: "#/components/schemas/ItemListEntry"
`
	// Processed: ItemListEntry flattened, allOf removed
	processed := `
openapi: "3.0.1"
info:
  title: Test
  version: v1
paths:
  /items:
    get:
      operationId: listItems
      responses:
        "200":
          description: OK
          content:
            application/json:
              schema:
                $ref: "#/components/schemas/ItemList"
components:
  schemas:
    Item:
      type: object
      required:
        - name
      properties:
        id:
          type: string
        name:
          type: string
    ItemListEntry:
      description: Item in a list
      type: object
      required:
        - name
      properties:
        id:
          type: string
        name:
          type: string
        owner:
          type: string
    ItemList:
      type: object
      properties:
        items:
          type: array
          items:
            $ref: "#/components/schemas/ItemListEntry"
`
	if err := validateEquivalence([]byte(original), []byte(processed)); err != nil {
		t.Errorf("flattened allOf should be equivalent: %v", err)
	}
}

// --- resolveRefs ---

func TestResolveRefs(t *testing.T) {
	schemas := map[string]any{
		"Pet": map[string]any{
			"type": "object",
			"properties": map[string]any{
				"name": map[string]any{"type": "string"},
			},
		},
		"Dog": map[string]any{
			"allOf": []any{
				map[string]any{"$ref": "#/components/schemas/Pet"},
				map[string]any{
					"type": "object",
					"properties": map[string]any{
						"breed": map[string]any{"type": "string"},
					},
				},
			},
		},
	}

	t.Run("resolves simple ref", func(t *testing.T) {
		input := map[string]any{
			"$ref": "#/components/schemas/Pet",
		}
		got := resolveRefs(input, schemas).(map[string]any)
		if got["type"] != "object" {
			t.Errorf("resolved type = %v, want %q", got["type"], "object")
		}
	})

	t.Run("resolves nested ref in allOf", func(t *testing.T) {
		input := map[string]any{
			"$ref": "#/components/schemas/Dog",
		}
		got := resolveRefs(input, schemas).(map[string]any)
		allOf := got["allOf"].([]any)
		if len(allOf) != 2 {
			t.Fatalf("allOf length = %d, want 2", len(allOf))
		}
		// First element should be resolved Pet
		pet := allOf[0].(map[string]any)
		if pet["type"] != "object" {
			t.Errorf("allOf[0].type = %v, want %q", pet["type"], "object")
		}
	})

	t.Run("handles circular refs", func(t *testing.T) {
		circular := map[string]any{
			"Self": map[string]any{
				"type": "object",
				"properties": map[string]any{
					"child": map[string]any{
						"$ref": "#/components/schemas/Self",
					},
				},
			},
		}
		input := map[string]any{
			"$ref": "#/components/schemas/Self",
		}
		// Should not stack overflow
		got := resolveRefs(input, circular).(map[string]any)
		if got["type"] != "object" {
			t.Errorf("resolved type = %v, want %q", got["type"], "object")
		}
		// The circular child should remain as a $ref
		props := got["properties"].(map[string]any)
		child := props["child"].(map[string]any)
		if _, ok := child["$ref"]; !ok {
			t.Error("circular ref should remain as $ref")
		}
	})

	t.Run("leaves non-schema refs as-is", func(t *testing.T) {
		input := map[string]any{
			"$ref": "#/components/responses/NotFound",
		}
		got := resolveRefs(input, schemas).(map[string]any)
		if got["$ref"] != "#/components/responses/NotFound" {
			t.Error("non-schema ref should be left as-is")
		}
	})

	t.Run("merges $ref with sibling properties using union", func(t *testing.T) {
		schemas := map[string]any{
			"Base": map[string]any{
				"type": "object",
				"properties": map[string]any{
					"a": map[string]any{"type": "string"},
					"b": map[string]any{"type": "string"},
				},
				"required": []any{"a"},
			},
		}
		input := map[string]any{
			"$ref": "#/components/schemas/Base",
			"properties": map[string]any{
				"c": map[string]any{"type": "integer"},
			},
			"required": []any{"b", "c"},
		}
		got := resolveRefs(input, schemas).(map[string]any)
		props := got["properties"].(map[string]any)
		for _, name := range []string{"a", "b", "c"} {
			if _, ok := props[name]; !ok {
				t.Errorf("expected property %q in merged result", name)
			}
		}
		req := got["required"].([]any)
		seen := map[string]bool{}
		for _, r := range req {
			seen[r.(string)] = true
		}
		for _, name := range []string{"a", "b", "c"} {
			if !seen[name] {
				t.Errorf("expected %q in required, got %v", name, req)
			}
		}
		if len(req) != 3 {
			t.Errorf("required should have 3 entries (deduped), got %d", len(req))
		}
	})
}

// --- End-to-end extraction ---

func TestEndToEndExtraction(t *testing.T) {
	spec := `
openapi: "3.0.1"
info:
  title: Test API
  version: v1
paths:
  /items:
    get:
      operationId: listItems
      responses:
        "200":
          description: Success
          content:
            application/json:
              schema:
                type: object
                properties:
                  items:
                    type: array
                    items:
                      $ref: "#/components/schemas/Item"
                  pageToken:
                    type: string
    post:
      operationId: createItem
      requestBody:
        content:
          application/json:
            schema:
              type: object
              properties:
                name:
                  type: string
                tags:
                  type: array
                  items:
                    type: object
                    properties:
                      key:
                        type: string
                      value:
                        type: string
      responses:
        "201":
          description: Created
          content:
            application/json:
              schema:
                type: object
                properties:
                  id:
                    type: string
                  name:
                    type: string
  /items/{id}:
    get:
      operationId: getItem
      responses:
        "200":
          description: Success
          content:
            application/json:
              schema:
                allOf:
                  - $ref: "#/components/schemas/Item"
                  - type: object
                    properties:
                      extraField:
                        type: string
components:
  schemas:
    Item:
      type: object
      properties:
        id:
          type: string
        name:
          type: string
        config:
          description: Item configuration
          type: object
          properties:
            setting:
              type: string
            nested:
              type: object
              properties:
                deep:
                  type: boolean
`

	var root yaml.Node
	if err := yaml.Unmarshal([]byte(spec), &root); err != nil {
		t.Fatalf("failed to parse spec: %v", err)
	}

	doc := root.Content[0]
	schemasNode := findMapValue(doc, "components", "schemas")
	pathsNode := findMapValue(doc, "paths")

	extractor := &Extractor{extracted: map[string]*yaml.Node{}}
	extractor.walkSchemas(schemasNode)
	extractor.walkPaths(pathsNode)

	// Verify expected extractions
	expectedNames := []string{
		"ItemConfig",                    // config property of Item
		"ItemConfigNested",              // nested property of ItemConfig
		"ListItems200Response",          // GET /items response
		"CreateItemRequestBody",         // POST /items request body
		"CreateItemRequestBodyTagsItem", // inline object in tags array
		"CreateItem201Response",         // POST /items 201 response
		"GetItem200Response",            // GET /items/{id} allOf response
	}

	for _, name := range expectedNames {
		if _, ok := extractor.extracted[name]; !ok {
			t.Errorf("expected extracted schema %q not found", name)
		}
	}

	// Verify total count
	if len(extractor.extracted) != len(expectedNames) {
		t.Errorf("extracted %d schemas, want %d", len(extractor.extracted), len(expectedNames))
		t.Log("Extracted schemas:")
		for name := range extractor.extracted {
			t.Logf("  - %s", name)
		}
	}

	// Verify ItemConfig has correct structure
	itemConfig := extractor.extracted["ItemConfig"]
	if itemConfig == nil {
		t.Fatal("ItemConfig not found")
	}
	settingType := getScalarValue(
		getMappingValue(getMappingValue(itemConfig, "properties"), "setting"),
		"type",
	)
	if settingType != "string" {
		t.Errorf("ItemConfig.properties.setting.type = %q, want %q", settingType, "string")
	}

	// Verify nested extraction happened — ItemConfigNested should exist
	nested := extractor.extracted["ItemConfigNested"]
	if nested == nil {
		t.Fatal("ItemConfigNested not found")
	}
	deepType := getScalarValue(
		getMappingValue(getMappingValue(nested, "properties"), "deep"),
		"type",
	)
	if deepType != "boolean" {
		t.Errorf("ItemConfigNested.properties.deep.type = %q, want %q", deepType, "boolean")
	}
}

// --- validateEquivalence ---

func TestValidateEquivalence_Identical(t *testing.T) {
	spec := `
openapi: "3.0.1"
info:
  title: Test
  version: v1
paths:
  /test:
    get:
      responses:
        "200":
          description: OK
          content:
            application/json:
              schema:
                $ref: "#/components/schemas/Foo"
components:
  schemas:
    Foo:
      type: object
      properties:
        name:
          type: string
`
	data := []byte(spec)
	if err := validateEquivalence(data, data); err != nil {
		t.Errorf("identical specs should be equivalent: %v", err)
	}
}

func TestValidateEquivalence_StructurallyEquivalent(t *testing.T) {
	original := `
openapi: "3.0.1"
info:
  title: Test
  version: v1
paths:
  /test:
    get:
      responses:
        "200":
          description: OK
          content:
            application/json:
              schema:
                type: object
                properties:
                  name:
                    type: string
components:
  schemas: {}
`
	// Processed: inline schema extracted to GetTest200Response
	processed := `
openapi: "3.0.1"
info:
  title: Test
  version: v1
paths:
  /test:
    get:
      responses:
        "200":
          description: OK
          content:
            application/json:
              schema:
                $ref: "#/components/schemas/GetTest200Response"
components:
  schemas:
    GetTest200Response:
      type: object
      properties:
        name:
          type: string
`
	if err := validateEquivalence([]byte(original), []byte(processed)); err != nil {
		t.Errorf("structurally equivalent specs should pass: %v", err)
	}
}

func TestValidateEquivalence_DescriptionPreservedViaAllOf(t *testing.T) {
	original := `
openapi: "3.0.1"
info:
  title: Test
  version: v1
paths:
  /test:
    get:
      responses:
        "200":
          description: OK
          content:
            application/json:
              schema:
                $ref: "#/components/schemas/Parent"
components:
  schemas:
    Parent:
      type: object
      properties:
        child:
          description: A child property
          type: object
          properties:
            name:
              type: string
`
	// Processed: child extracted with allOf wrapper for description
	processed := `
openapi: "3.0.1"
info:
  title: Test
  version: v1
paths:
  /test:
    get:
      responses:
        "200":
          description: OK
          content:
            application/json:
              schema:
                $ref: "#/components/schemas/Parent"
components:
  schemas:
    Parent:
      type: object
      properties:
        child:
          description: A child property
          allOf:
            - $ref: "#/components/schemas/ParentChild"
    ParentChild:
      description: A child property
      type: object
      properties:
        name:
          type: string
`
	if err := validateEquivalence([]byte(original), []byte(processed)); err != nil {
		t.Errorf("description-preserved allOf should pass validation: %v", err)
	}
}

func TestValidateEquivalence_StructuralDifference(t *testing.T) {
	original := `
openapi: "3.0.1"
info:
  title: Test
  version: v1
paths:
  /test:
    get:
      responses:
        "200":
          description: OK
          content:
            application/json:
              schema:
                type: object
                properties:
                  name:
                    type: string
components:
  schemas: {}
`
	// Processed with wrong type
	processed := `
openapi: "3.0.1"
info:
  title: Test
  version: v1
paths:
  /test:
    get:
      responses:
        "200":
          description: OK
          content:
            application/json:
              schema:
                $ref: "#/components/schemas/GetTest200Response"
components:
  schemas:
    GetTest200Response:
      type: object
      properties:
        name:
          type: integer
`
	err := validateEquivalence([]byte(original), []byte(processed))
	if err == nil {
		t.Error("structurally different specs should fail validation")
	}
}

// --- deepCopyNode ---

func TestDeepCopyNode(t *testing.T) {
	node := mustParseYAML(t, `
type: object
properties:
  name:
    type: string
`)
	cp := deepCopyNode(node)
	if cp == node {
		t.Error("deep copy should return a new node")
	}

	// Modify the copy and verify original is unchanged
	cp.Content[1].Value = "array" // change type value
	origType := getScalarValue(node, "type")
	if origType != "object" {
		t.Errorf("original type changed to %q after modifying copy", origType)
	}
}

// --- insertExtractedSchemas ---

func TestInsertExtractedSchemas(t *testing.T) {
	schemasNode := mustParseYAML(t, `
Zebra:
  type: object
Apple:
  type: object
`)

	e := &Extractor{extracted: map[string]*yaml.Node{}}
	e.extracted["Middle"] = mustParseYAML(t, `
type: object
properties:
  id:
    type: string
`)

	e.insertExtractedSchemas(schemasNode)

	// Verify the new schema was inserted in sorted order
	// Expected order: Apple, Middle, Zebra
	if len(schemasNode.Content) != 6 { // 3 key-value pairs = 6 nodes
		t.Fatalf("expected 6 content nodes (3 schemas), got %d", len(schemasNode.Content))
	}
	keys := []string{
		schemasNode.Content[0].Value,
		schemasNode.Content[2].Value,
		schemasNode.Content[4].Value,
	}
	expected := []string{"Apple", "Middle", "Zebra"}
	for i, k := range keys {
		if k != expected[i] {
			t.Errorf("key[%d] = %q, want %q", i, k, expected[i])
		}
	}
}
