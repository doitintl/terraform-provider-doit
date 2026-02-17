// extract-inline-schemas transforms an OpenAPI 3.0 spec by extracting all inline
// object definitions into named schemas under components/schemas, replacing them
// with $ref pointers. This produces named Go types instead of anonymous structs
// when used with code generators like oapi-codegen.
//
// Usage:
//
//	go run ./tools/extract-inline-schemas -input spec.yml -output processed.yml
package main

import (
	"bytes"
	"flag"
	"fmt"
	"log"
	"os"
	"reflect"
	"sort"
	"strings"
	"unicode"

	"gopkg.in/yaml.v3"
)

func main() {
	inputPath := flag.String("input", "", "Path to the input OpenAPI spec (YAML)")
	outputPath := flag.String("output", "", "Path to write the processed spec (YAML)")
	flag.Parse()

	if *inputPath == "" || *outputPath == "" {
		flag.Usage()
		os.Exit(1)
	}

	data, err := os.ReadFile(*inputPath)
	if err != nil {
		log.Fatalf("reading input: %v", err)
	}

	var root yaml.Node
	if err := yaml.Unmarshal(data, &root); err != nil {
		log.Fatalf("parsing YAML: %v", err)
	}

	// root is a document node; the actual mapping is its first content node.
	if root.Kind != yaml.DocumentNode || len(root.Content) == 0 {
		log.Fatal("unexpected YAML structure: expected document node")
	}
	doc := root.Content[0]

	extractor := &Extractor{
		extracted: make(map[string]*yaml.Node),
	}

	// Phase 1: Extract inline schemas from components/schemas
	schemasNode := findMapValue(doc, "components", "schemas")
	if schemasNode != nil {
		extractor.walkSchemas(schemasNode)
	}

	// Phase 2: Extract inline schemas from paths (response/request bodies)
	pathsNode := findMapValue(doc, "paths")
	if pathsNode != nil {
		extractor.walkPaths(pathsNode)
	}

	// Phase 3: Insert extracted schemas into components/schemas.
	// NOTE: This requires a pre-existing components/schemas section. The tool
	// does not create one automatically. This is acceptable for our spec but
	// limits reuse with specs that lack this section.
	if len(extractor.extracted) > 0 {
		if schemasNode == nil {
			log.Fatal("spec has no components/schemas section to insert into")
		}
		extractor.insertExtractedSchemas(schemasNode)
	}

	var buf bytes.Buffer
	enc := yaml.NewEncoder(&buf)
	enc.SetIndent(2)
	if err := enc.Encode(&root); err != nil {
		log.Fatalf("marshaling YAML: %v", err)
	}
	enc.Close()
	out := buf.Bytes()

	if err := os.WriteFile(*outputPath, out, 0644); err != nil {
		log.Fatalf("writing output: %v", err)
	}

	// Phase 4: Validate functional equivalence
	if err := validateEquivalence(data, out); err != nil {
		// Remove the output file on validation failure to avoid leaving
		// a broken processed spec on disk.
		os.Remove(*outputPath)
		log.Fatalf("equivalence validation failed: %v", err)
	}

	fmt.Printf("Extracted %d inline schemas\n", len(extractor.extracted))
	names := make([]string, 0, len(extractor.extracted))
	for name := range extractor.extracted {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		fmt.Printf("  - %s\n", name)
	}
}

// Extractor holds state for the extraction process.
type Extractor struct {
	// extracted maps generated schema name -> the extracted schema node.
	extracted map[string]*yaml.Node
}

// walkSchemas walks all schemas in the components/schemas mapping node
// and extracts inline object properties.
func (e *Extractor) walkSchemas(schemasNode *yaml.Node) {
	if schemasNode.Kind != yaml.MappingNode {
		return
	}
	for i := 0; i < len(schemasNode.Content)-1; i += 2 {
		schemaName := schemasNode.Content[i].Value
		schemaNode := schemasNode.Content[i+1]
		e.extractFromSchema(schemaNode, schemaName)
	}
}

// walkPaths walks all paths and their operations, extracting inline schemas
// from response and request bodies.
func (e *Extractor) walkPaths(pathsNode *yaml.Node) {
	if pathsNode.Kind != yaml.MappingNode {
		return
	}
	for i := 0; i < len(pathsNode.Content)-1; i += 2 {
		pathNode := pathsNode.Content[i+1]
		if pathNode.Kind != yaml.MappingNode {
			continue
		}
		// Walk each HTTP method (get, post, put, etc.)
		for j := 0; j < len(pathNode.Content)-1; j += 2 {
			method := pathNode.Content[j].Value
			if !isHTTPMethod(method) {
				continue
			}
			operationNode := pathNode.Content[j+1]
			if operationNode.Kind != yaml.MappingNode {
				continue
			}
			operationID := getScalarValue(operationNode, "operationId")
			if operationID == "" {
				continue
			}
			baseName := toPascalCase(operationID)

			// Walk responses
			responsesNode := getMappingValue(operationNode, "responses")
			if responsesNode != nil {
				e.walkResponses(responsesNode, baseName)
			}

			// Walk requestBody
			requestBodyNode := getMappingValue(operationNode, "requestBody")
			if requestBodyNode != nil {
				e.walkRequestBody(requestBodyNode, baseName)
			}
		}
	}
}

// walkResponses extracts inline schemas from response content.
func (e *Extractor) walkResponses(responsesNode *yaml.Node, operationName string) {
	if responsesNode.Kind != yaml.MappingNode {
		return
	}
	for i := 0; i < len(responsesNode.Content)-1; i += 2 {
		statusCode := responsesNode.Content[i].Value
		responseNode := responsesNode.Content[i+1]

		// Skip $ref responses
		if hasRef(responseNode) {
			continue
		}

		schemaName := operationName + statusCode + "Response"

		contentNode := getMappingValue(responseNode, "content")
		if contentNode == nil {
			continue
		}
		e.walkContentSchemas(contentNode, schemaName)
	}
}

// walkRequestBody extracts inline schemas from request body content.
func (e *Extractor) walkRequestBody(requestBodyNode *yaml.Node, operationName string) {
	if requestBodyNode.Kind != yaml.MappingNode {
		return
	}
	// Skip $ref
	if hasRef(requestBodyNode) {
		return
	}

	schemaName := operationName + "RequestBody"
	contentNode := getMappingValue(requestBodyNode, "content")
	if contentNode == nil {
		return
	}
	e.walkContentSchemas(contentNode, schemaName)
}

// walkContentSchemas walks content type mappings (e.g., application/json)
// and extracts inline schemas found in them.
func (e *Extractor) walkContentSchemas(contentNode *yaml.Node, parentName string) {
	if contentNode.Kind != yaml.MappingNode {
		return
	}
	for i := 0; i < len(contentNode.Content)-1; i += 2 {
		mediaTypeNode := contentNode.Content[i+1]
		if mediaTypeNode.Kind != yaml.MappingNode {
			continue
		}

		schemaNode := getMappingValue(mediaTypeNode, "schema")
		if schemaNode == nil {
			continue
		}

		if isInlineObject(schemaNode) || isComposition(schemaNode) {
			extracted := e.extractInlineObject(schemaNode, parentName)
			// For content-level schemas, use a direct $ref (no allOf wrapper).
			// The description is on the extracted schema itself.
			replaceSchemaValueWithRef(mediaTypeNode, "schema", parentName)
			// Also recurse into the extracted schema for nested inline objects
			e.extractFromSchema(extracted, parentName)
		} else {
			// Even non-inline schemas might have nested inline objects
			e.extractFromSchema(schemaNode, parentName)
		}
	}
}

// extractFromSchema recursively extracts inline objects from a schema's
// properties and items.
func (e *Extractor) extractFromSchema(schemaNode *yaml.Node, parentName string) {
	if schemaNode.Kind != yaml.MappingNode {
		return
	}

	// Handle allOf, anyOf, oneOf — walk into each sub-schema
	for _, compositeKey := range []string{"allOf", "anyOf", "oneOf"} {
		compositeNode := getMappingValue(schemaNode, compositeKey)
		if compositeNode != nil && compositeNode.Kind == yaml.SequenceNode {
			for idx, subSchema := range compositeNode.Content {
				subName := fmt.Sprintf("%s%s%d", parentName, toPascalCase(compositeKey), idx)
				e.extractFromSchema(subSchema, subName)
			}
		}
	}

	// Handle properties
	propertiesNode := getMappingValue(schemaNode, "properties")
	if propertiesNode != nil && propertiesNode.Kind == yaml.MappingNode {
		for i := 0; i < len(propertiesNode.Content)-1; i += 2 {
			propName := propertiesNode.Content[i].Value
			propNode := propertiesNode.Content[i+1]
			childName := parentName + toPascalCase(propName)

			if isInlineObject(propNode) {
				extracted := e.extractInlineObject(propNode, childName)
				replaceWithRef(propertiesNode, propName, childName)
				// Recurse into extracted schema
				e.extractFromSchema(extracted, childName)
			} else if isInlineArray(propNode) {
				e.extractFromArrayItems(propNode, childName)
			} else {
				// Recurse for nested structures (e.g., an object that's a $ref
				// but also has additional inline properties via allOf)
				e.extractFromSchema(propNode, childName)
			}
		}
	}

	// Handle items (for array types at schema level)
	e.extractFromArrayItems(schemaNode, parentName)

	// Handle additionalProperties
	additionalNode := getMappingValue(schemaNode, "additionalProperties")
	if additionalNode != nil && additionalNode.Kind == yaml.MappingNode {
		addName := parentName + "Value"
		if isInlineObject(additionalNode) {
			extracted := e.extractInlineObject(additionalNode, addName)
			replaceWithRef(schemaNode, "additionalProperties", addName)
			e.extractFromSchema(extracted, addName)
		} else {
			e.extractFromSchema(additionalNode, addName)
		}
	}
}

// extractFromArrayItems handles inline objects inside array items.
func (e *Extractor) extractFromArrayItems(node *yaml.Node, parentName string) {
	if node.Kind != yaml.MappingNode {
		return
	}
	itemsNode := getMappingValue(node, "items")
	if itemsNode == nil {
		return
	}

	itemName := parentName + "Item"
	if isInlineObject(itemsNode) {
		extracted := e.extractInlineObject(itemsNode, itemName)
		replaceWithRef(node, "items", itemName)
		e.extractFromSchema(extracted, itemName)
	} else {
		e.extractFromSchema(itemsNode, itemName)
	}
}

// extractInlineObject moves an inline object definition to the extracted map
// and returns the extracted node for further processing.
func (e *Extractor) extractInlineObject(node *yaml.Node, name string) *yaml.Node {
	uniqueName := e.uniqueName(name)

	// Deep copy the node so we can modify the original without affecting the copy
	extracted := deepCopyNode(node)
	e.extracted[uniqueName] = extracted

	return extracted
}

// uniqueName generates a unique schema name, appending a numeric suffix if needed.
func (e *Extractor) uniqueName(base string) string {
	if _, exists := e.extracted[base]; !exists {
		return base
	}
	for i := 2; ; i++ {
		candidate := fmt.Sprintf("%s%d", base, i)
		if _, exists := e.extracted[candidate]; !exists {
			return candidate
		}
	}
}

// insertExtractedSchemas inserts all extracted schemas into the components/schemas
// mapping node in sorted order.
func (e *Extractor) insertExtractedSchemas(schemasNode *yaml.Node) {
	// Collect all existing schema names
	existing := make(map[string]bool)
	for i := 0; i < len(schemasNode.Content)-1; i += 2 {
		existing[schemasNode.Content[i].Value] = true
	}

	// Sort extracted names for deterministic output
	names := make([]string, 0, len(e.extracted))
	for name := range e.extracted {
		if existing[name] {
			log.Fatalf("name collision: extracted schema %q conflicts with existing schema", name)
		}
		names = append(names, name)
	}
	sort.Strings(names)

	// Build new content: merge existing + extracted, sorted alphabetically
	type entry struct {
		key   *yaml.Node
		value *yaml.Node
	}
	var entries []entry
	for i := 0; i < len(schemasNode.Content)-1; i += 2 {
		entries = append(entries, entry{schemasNode.Content[i], schemasNode.Content[i+1]})
	}
	for _, name := range names {
		keyNode := &yaml.Node{Kind: yaml.ScalarNode, Value: name, Tag: "!!str"}
		entries = append(entries, entry{keyNode, e.extracted[name]})
	}
	sort.Slice(entries, func(i, j int) bool {
		return entries[i].key.Value < entries[j].key.Value
	})

	schemasNode.Content = nil
	for _, ent := range entries {
		schemasNode.Content = append(schemasNode.Content, ent.key, ent.value)
	}
}

// --- YAML helpers ---

// findMapValue navigates a chain of mapping keys (e.g., "components", "schemas")
// and returns the final value node.
func findMapValue(node *yaml.Node, keys ...string) *yaml.Node {
	current := node
	for _, key := range keys {
		if current.Kind != yaml.MappingNode {
			return nil
		}
		found := false
		for i := 0; i < len(current.Content)-1; i += 2 {
			if current.Content[i].Value == key {
				current = current.Content[i+1]
				found = true
				break
			}
		}
		if !found {
			return nil
		}
	}
	return current
}

// getMappingValue gets a value from a mapping node by key.
func getMappingValue(node *yaml.Node, key string) *yaml.Node {
	if node.Kind != yaml.MappingNode {
		return nil
	}
	for i := 0; i < len(node.Content)-1; i += 2 {
		if node.Content[i].Value == key {
			return node.Content[i+1]
		}
	}
	return nil
}

// getScalarValue gets a scalar string value from a mapping node by key.
func getScalarValue(node *yaml.Node, key string) string {
	v := getMappingValue(node, key)
	if v != nil && v.Kind == yaml.ScalarNode {
		return v.Value
	}
	return ""
}

// hasRef checks if a mapping node contains a $ref key.
func hasRef(node *yaml.Node) bool {
	if node.Kind != yaml.MappingNode {
		return false
	}
	for i := 0; i < len(node.Content)-1; i += 2 {
		if node.Content[i].Value == "$ref" {
			return true
		}
	}
	return false
}

// isInlineObject checks if a node is an inline object definition:
// a mapping node with type: object and properties, but no $ref.
func isInlineObject(node *yaml.Node) bool {
	if node.Kind != yaml.MappingNode {
		return false
	}
	if hasRef(node) {
		return false
	}
	hasProperties := getMappingValue(node, "properties") != nil
	typeVal := getScalarValue(node, "type")

	// An object with properties — explicit type: object or implicit (properties present without type)
	if hasProperties && (typeVal == "object" || typeVal == "") {
		return true
	}
	return false
}

// isInlineArray checks if a node is an array type with inline object items.
func isInlineArray(node *yaml.Node) bool {
	if node.Kind != yaml.MappingNode {
		return false
	}
	typeVal := getScalarValue(node, "type")
	if typeVal != "array" {
		return false
	}
	itemsNode := getMappingValue(node, "items")
	if itemsNode == nil {
		return false
	}
	return isInlineObject(itemsNode)
}

// isComposition checks if a node is an allOf/anyOf/oneOf composition
// (which code generators also turn into anonymous structs).
func isComposition(node *yaml.Node) bool {
	if node.Kind != yaml.MappingNode {
		return false
	}
	if hasRef(node) {
		return false
	}
	for _, key := range []string{"allOf", "anyOf", "oneOf"} {
		if getMappingValue(node, key) != nil {
			return true
		}
	}
	return false
}

// replaceWithRef replaces a property value in a mapping node with a $ref.
// It preserves the description from the original inline definition by wrapping
// the $ref in an allOf.
//
// Limitation: Only the "description" field is preserved from the usage site.
// Other property-level metadata (e.g., "example", custom extensions like
// "x-terraform-*") present on the original inline object will be dropped.
// This is currently acceptable because our spec does not use such metadata
// at inline usage sites, and the equivalence validator would catch any
// structural loss. If the spec evolves to include such metadata, this
// function should be updated to preserve all non-structural sibling fields.
func replaceWithRef(parentMapping *yaml.Node, propertyKey string, schemaName string) {
	if parentMapping.Kind != yaml.MappingNode {
		return
	}
	for i := 0; i < len(parentMapping.Content)-1; i += 2 {
		if parentMapping.Content[i].Value == propertyKey {
			oldNode := parentMapping.Content[i+1]

			// Preserve description from the inline definition
			description := getScalarValue(oldNode, "description")

			// Build new $ref node
			refNode := &yaml.Node{
				Kind: yaml.MappingNode,
				Content: []*yaml.Node{
					{Kind: yaml.ScalarNode, Value: "$ref", Tag: "!!str"},
					{Kind: yaml.ScalarNode, Value: fmt.Sprintf("#/components/schemas/%s", schemaName), Tag: "!!str"},
				},
			}

			// If description exists, wrap in allOf to preserve it
			if description != "" {
				parentMapping.Content[i+1] = &yaml.Node{
					Kind: yaml.MappingNode,
					Content: []*yaml.Node{
						{Kind: yaml.ScalarNode, Value: "description", Tag: "!!str"},
						{Kind: yaml.ScalarNode, Value: description, Tag: "!!str", Style: oldNode.Style},
						{Kind: yaml.ScalarNode, Value: "allOf", Tag: "!!str"},
						{Kind: yaml.SequenceNode, Content: []*yaml.Node{refNode}},
					},
				}
			} else {
				parentMapping.Content[i+1] = refNode
			}
			return
		}
	}
}

// replaceSchemaValueWithRef replaces a property value in a mapping node with
// a plain $ref. Unlike replaceWithRef, it does NOT wrap in allOf — the
// description is expected to live on the extracted schema itself.
// Used for content-level schemas (response/request bodies).
func replaceSchemaValueWithRef(parentMapping *yaml.Node, propertyKey string, schemaName string) {
	if parentMapping.Kind != yaml.MappingNode {
		return
	}
	for i := 0; i < len(parentMapping.Content)-1; i += 2 {
		if parentMapping.Content[i].Value == propertyKey {
			parentMapping.Content[i+1] = &yaml.Node{
				Kind: yaml.MappingNode,
				Content: []*yaml.Node{
					{Kind: yaml.ScalarNode, Value: "$ref", Tag: "!!str"},
					{Kind: yaml.ScalarNode, Value: fmt.Sprintf("#/components/schemas/%s", schemaName), Tag: "!!str"},
				},
			}
			return
		}
	}
}

// deepCopyNode creates a deep copy of a yaml.Node tree.
func deepCopyNode(node *yaml.Node) *yaml.Node {
	if node == nil {
		return nil
	}
	cp := &yaml.Node{
		Kind:        node.Kind,
		Style:       node.Style,
		Tag:         node.Tag,
		Value:       node.Value,
		Anchor:      node.Anchor,
		HeadComment: node.HeadComment,
		LineComment: node.LineComment,
		FootComment: node.FootComment,
	}
	if len(node.Content) > 0 {
		cp.Content = make([]*yaml.Node, len(node.Content))
		for i, child := range node.Content {
			cp.Content[i] = deepCopyNode(child)
		}
	}
	return cp
}

// toPascalCase converts a string like "customTimeRange" or "list_allocations"
// to "CustomTimeRange" or "ListAllocations".
func toPascalCase(s string) string {
	var result strings.Builder
	capitalizeNext := true
	for _, r := range s {
		if r == '_' || r == '-' || r == ' ' {
			capitalizeNext = true
			continue
		}
		if capitalizeNext {
			result.WriteRune(unicode.ToUpper(r))
			capitalizeNext = false
		} else {
			result.WriteRune(r)
		}
	}
	return result.String()
}

// isHTTPMethod returns true if the string is an HTTP method.
func isHTTPMethod(s string) bool {
	switch s {
	case "get", "post", "put", "patch", "delete", "head", "options", "trace":
		return true
	}
	return false
}

// --- Equivalence validation ---

// validateEquivalence verifies that the processed spec is functionally
// equivalent to the original. It parses both into generic maps, resolves
// all $ref pointers, strips documentation-only fields (description, example),
// and deep-compares the paths sections.
func validateEquivalence(originalData, processedData []byte) error {
	var original, processed map[string]any
	if err := yaml.Unmarshal(originalData, &original); err != nil {
		return fmt.Errorf("re-parsing original: %w", err)
	}
	if err := yaml.Unmarshal(processedData, &processed); err != nil {
		return fmt.Errorf("re-parsing processed: %w", err)
	}

	// Extract schemas maps for ref resolution
	origSchemas := extractSchemasMap(original)
	procSchemas := extractSchemasMap(processed)

	// Resolve all $refs in paths
	origPaths, _ := original["paths"].(map[string]any)
	procPaths, _ := processed["paths"].(map[string]any)

	if origPaths == nil || procPaths == nil {
		return nil // nothing to compare
	}

	resolvedOrig := stripDocFields(resolveRefs(origPaths, origSchemas))
	resolvedProc := stripDocFields(resolveRefs(procPaths, procSchemas))

	if !reflect.DeepEqual(resolvedOrig, resolvedProc) {
		// Find the first differing path for a useful error message
		for path, origOp := range resolvedOrig.(map[string]any) {
			procOp, ok := resolvedProc.(map[string]any)[path]
			if !ok {
				return fmt.Errorf("path %q missing from processed spec", path)
			}
			if !reflect.DeepEqual(origOp, procOp) {
				return fmt.Errorf("path %q differs after resolution", path)
			}
		}
		// Check for extra paths in processed
		for path := range resolvedProc.(map[string]any) {
			if _, ok := resolvedOrig.(map[string]any)[path]; !ok {
				return fmt.Errorf("extra path %q in processed spec", path)
			}
		}
		return fmt.Errorf("paths differ after resolution (cause not identified)")
	}

	// Also compare components/schemas after resolution.
	// The processed spec has MORE schemas (the extracted ones), so only
	// verify that all original schemas are present and structurally equivalent.
	resolvedOrigSchemas := stripDocFields(resolveRefs(origSchemas, origSchemas))
	resolvedProcSchemas := stripDocFields(resolveRefs(procSchemas, procSchemas))

	origMap, _ := resolvedOrigSchemas.(map[string]any)
	procMap, _ := resolvedProcSchemas.(map[string]any)
	for name, origSchema := range origMap {
		procSchema, ok := procMap[name]
		if !ok {
			return fmt.Errorf("schema %q missing from processed spec", name)
		}
		if !reflect.DeepEqual(origSchema, procSchema) {
			return fmt.Errorf("schema %q differs after resolution", name)
		}
	}

	return nil
}

// extractSchemasMap extracts the components/schemas map from a parsed spec.
func extractSchemasMap(spec map[string]any) map[string]any {
	components, _ := spec["components"].(map[string]any)
	if components == nil {
		return nil
	}
	schemas, _ := components["schemas"].(map[string]any)
	return schemas
}

// resolveRefs recursively resolves all $ref pointers in a value tree.
// It replaces each {"$ref": "#/components/schemas/Foo"} with the actual
// schema definition, then recursively resolves the result.
// Uses a visited set to prevent infinite recursion on circular references.
func resolveRefs(v any, schemas map[string]any) any {
	return resolveRefsWithVisited(v, schemas, make(map[string]bool))
}

func resolveRefsWithVisited(v any, schemas map[string]any, visited map[string]bool) any {
	switch val := v.(type) {
	case map[string]any:
		// Check if this is a $ref
		if ref, ok := val["$ref"].(string); ok && len(val) == 1 {
			const prefix = "#/components/schemas/"
			if strings.HasPrefix(ref, prefix) {
				name := ref[len(prefix):]
				if visited[name] {
					// Circular reference — return as-is to avoid infinite loop
					return val
				}
				if schema, ok := schemas[name]; ok {
					newVisited := make(map[string]bool, len(visited)+1)
					for k, v := range visited {
						newVisited[k] = v
					}
					newVisited[name] = true
					return resolveRefsWithVisited(schema, schemas, newVisited)
				}
			}
			// Non-schema $ref (e.g., responses, parameters) — leave as-is
			return val
		}

		result := make(map[string]any, len(val))
		for k, v := range val {
			result[k] = resolveRefsWithVisited(v, schemas, visited)
		}
		return result
	case []any:
		result := make([]any, len(val))
		for i, item := range val {
			result[i] = resolveRefsWithVisited(item, schemas, visited)
		}
		return result
	default:
		return v
	}
}

// stripDocFields recursively removes documentation-only fields (description,
// example) from a value tree and unwraps single-element allOf/anyOf/oneOf
// compositions. This normalizes structures for equivalence comparison,
// accounting for the tool's use of allOf wrappers to preserve property-level
// descriptions alongside $ref pointers.
func stripDocFields(v any) any {
	switch val := v.(type) {
	case map[string]any:
		result := make(map[string]any, len(val))
		for k, v := range val {
			if k == "description" || k == "example" {
				continue
			}
			result[k] = stripDocFields(v)
		}
		// Unwrap single-element allOf/anyOf/oneOf when it's the only remaining key.
		// This handles the pattern: {description: "...", allOf: [{$ref: ...}]}
		// After stripping description: {allOf: [{resolved_schema}]}
		// Which should be equivalent to just {resolved_schema}.
		for _, compKey := range []string{"allOf", "anyOf", "oneOf"} {
			if items, ok := result[compKey].([]any); ok && len(items) == 1 && len(result) == 1 {
				if inner, ok := items[0].(map[string]any); ok {
					return inner
				}
			}
		}
		return result
	case []any:
		result := make([]any, len(val))
		for i, item := range val {
			result[i] = stripDocFields(item)
		}
		return result
	default:
		return v
	}
}
