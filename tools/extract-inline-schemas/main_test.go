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

	t.Run("does not unwrap multi-element allOf", func(t *testing.T) {
		input := map[string]any{
			"allOf": []any{
				map[string]any{"type": "object"},
				map[string]any{"type": "string"},
			},
		}
		got := stripDocFields(input).(map[string]any)
		if _, ok := got["allOf"]; !ok {
			t.Error("multi-element allOf should NOT be unwrapped")
		}
	})

	t.Run("does not unwrap allOf with other sibling keys", func(t *testing.T) {
		input := map[string]any{
			"type": "object",
			"allOf": []any{
				map[string]any{"properties": map[string]any{}},
			},
		}
		got := stripDocFields(input).(map[string]any)
		if _, ok := got["allOf"]; !ok {
			t.Error("allOf with sibling type should NOT be unwrapped")
		}
	})
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
