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
	"maps"
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
	datasourcesPath := flag.String("datasources", "", "Path to the provider datasources config (YAML) — enables pruning")
	resourcesPath := flag.String("resources", "", "Path to the provider resources config (YAML) — enables pruning")
	extraPathsFile := flag.String("extra-paths", "", "Path to a YAML file listing additional path+method pairs to retain when pruning")
	flag.Parse()

	if *inputPath == "" || *outputPath == "" {
		flag.Usage()
		os.Exit(1)
	}

	pruning := *datasourcesPath != "" && *resourcesPath != ""
	if (*datasourcesPath != "") != (*resourcesPath != "") {
		log.Fatal("-datasources and -resources must be specified together")
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

	// Phase 3.5: Wrap $ref nodes with sibling keywords (e.g., nullable) into allOf.
	// OAS 3.1 allows $ref with siblings, but codegen tools expect 3.0-style allOf.
	// This must run before flattenAllOfSchemas so both named and inline patterns
	// are normalized.
	wrapRefSiblings(doc)

	// Phase 3.6: Flatten multi-element allOf compositions in components/schemas.
	// Schemas like {allOf: [$ref: Base, {properties: {extra}}]} are merged into
	// a single {type: object, properties: {all Base props + extra}}. This allows
	// downstream code generators (tfplugingen-openapi) that don't support allOf
	// composition to process them.
	schemasNode = findMapValue(doc, "components", "schemas")
	if schemasNode != nil {
		flattenAllOfSchemas(schemasNode)
	}

	// Phase 4: Validate functional equivalence of the extraction.
	// This runs BEFORE pruning so that inline-schema extraction regressions
	// are always caught, regardless of whether pruning is enabled.
	{
		var buf bytes.Buffer
		enc := yaml.NewEncoder(&buf)
		enc.SetIndent(2)
		if err := enc.Encode(&root); err != nil {
			log.Fatalf("marshaling YAML for validation: %v", err)
		}
		if err := enc.Close(); err != nil {
			log.Fatalf("closing YAML encoder for validation: %v", err)
		}
		if err := validateEquivalence(data, buf.Bytes()); err != nil {
			log.Fatalf("equivalence validation failed: %v", err)
		}
	}

	// Phase 5: Prune unused paths and unreachable schemas (optional).
	// When -datasources and -resources are provided, only paths/methods
	// referenced by the provider configs (plus any -extra-paths) are retained.
	// Schemas not transitively reachable from the remaining paths are removed.
	if pruning {
		// Re-read schemasNode after insertion, since insertExtractedSchemas
		// may have modified it.
		schemasNode = findMapValue(doc, "components", "schemas")

		used, err := loadProviderConfigs(*datasourcesPath, *resourcesPath)
		if err != nil {
			log.Fatalf("loading provider configs: %v", err)
		}

		if *extraPathsFile != "" {
			if err := loadExtraPaths(*extraPathsFile, used); err != nil {
				log.Fatalf("loading extra paths: %v", err)
			}
		}

		pathsRemoved, methodsPruned := prunePaths(pathsNode, used)
		fmt.Printf("Pruned %d paths and %d methods\n", pathsRemoved, methodsPruned)

		if schemasNode != nil {
			reachable := collectReachableSchemas(doc, pathsNode, schemasNode)
			schemasRemoved := pruneSchemas(schemasNode, reachable)
			fmt.Printf("Pruned %d unreachable schemas (kept %d)\n", schemasRemoved, len(reachable))

			// Also prune components/responses that are no longer referenced
			// from any retained path, to avoid dangling $ref errors.
			responsesNode := findMapValue(doc, "components", "responses")
			if responsesNode != nil {
				reachableResponses := collectReachableResponses(pathsNode)
				responsesRemoved := pruneResponses(responsesNode, reachableResponses)
				if responsesRemoved > 0 {
					fmt.Printf("Pruned %d unreachable responses\n", responsesRemoved)
				}
			}
		}
	}

	var buf bytes.Buffer
	enc := yaml.NewEncoder(&buf)
	enc.SetIndent(2)
	if err := enc.Encode(&root); err != nil {
		log.Fatalf("marshaling YAML: %v", err)
	}
	if err := enc.Close(); err != nil {
		log.Fatalf("closing YAML encoder: %v", err)
	}
	out := buf.Bytes()

	if err := os.WriteFile(*outputPath, out, 0600); err != nil {
		log.Fatalf("writing output: %v", err)
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

// getScalarNode gets a scalar node from a mapping node by key.
// Unlike getScalarValue, this returns the full node so callers can access
// metadata like Style (e.g., folded >- or literal | block styles).
func getScalarNode(node *yaml.Node, key string) *yaml.Node {
	v := getMappingValue(node, key)
	if v != nil && v.Kind == yaml.ScalarNode {
		return v
	}
	return nil
}

// removeMappingKey removes a key-value pair from a mapping node by key name.
func removeMappingKey(node *yaml.Node, key string) {
	for k := 0; k < len(node.Content)-1; k += 2 {
		if node.Content[k].Value == key {
			node.Content = append(node.Content[:k], node.Content[k+2:]...)
			return
		}
	}
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

// wrapRefSiblings recursively walks the YAML tree and wraps any $ref node
// that has sibling keywords (e.g., description, nullable) in a single-member
// allOf. This converts valid OAS 3.1 patterns into 3.0-compatible structures.
//
// Before:  {$ref: "#/.../Foo", description: "...", nullable: true}.
// After:   {allOf: [{$ref: "#/.../Foo"}], description: "...", nullable: true}.
func wrapRefSiblings(node *yaml.Node) {
	if node == nil {
		return
	}
	switch node.Kind {
	case yaml.MappingNode:
		refIdx := -1
		for i := 0; i < len(node.Content)-1; i += 2 {
			if node.Content[i].Value == "$ref" {
				refIdx = i
				break
			}
		}
		if refIdx >= 0 && len(node.Content) > 2 {
			refKeyNode := node.Content[refIdx]
			refValNode := node.Content[refIdx+1]

			innerRef := &yaml.Node{
				Kind:    yaml.MappingNode,
				Content: []*yaml.Node{refKeyNode, refValNode},
			}

			var siblings []*yaml.Node
			for i := 0; i < len(node.Content)-1; i += 2 {
				if i != refIdx {
					siblings = append(siblings, node.Content[i], node.Content[i+1])
				}
			}

			node.Content = append(
				[]*yaml.Node{
					{Kind: yaml.ScalarNode, Value: "allOf", Tag: "!!str"},
					{Kind: yaml.SequenceNode, Content: []*yaml.Node{innerRef}},
				},
				siblings...,
			)
		}
		for i := 1; i < len(node.Content); i += 2 {
			wrapRefSiblings(node.Content[i])
		}
	case yaml.SequenceNode:
		for _, child := range node.Content {
			wrapRefSiblings(child)
		}
	case yaml.DocumentNode:
		for _, child := range node.Content {
			wrapRefSiblings(child)
		}
	}
}

// flattenAllOfSchemas walks all named schemas in components/schemas and flattens
// any schema that uses allOf with 2+ sub-schemas into a single flat object.
// Each $ref sub-schema is resolved by looking it up in the same schemasNode.
// Properties from later sub-schemas override earlier ones on conflict (with a
// log warning). Required arrays are merged (union, deduplicated).
func flattenAllOfSchemas(schemasNode *yaml.Node) {
	if schemasNode.Kind != yaml.MappingNode {
		return
	}
	for i := 0; i < len(schemasNode.Content)-1; i += 2 {
		schemaName := schemasNode.Content[i].Value
		schemaNode := schemasNode.Content[i+1]
		if schemaNode.Kind != yaml.MappingNode {
			continue
		}
		allOfNode := getMappingValue(schemaNode, "allOf")
		if allOfNode == nil || allOfNode.Kind != yaml.SequenceNode || len(allOfNode.Content) < 2 {
			continue
		}

		merged := flattenAllOf(allOfNode, schemasNode, schemaName)
		if merged == nil {
			continue
		}

		// Preserve parent-level fields (description, example) from outside the allOf.
		parentDesc := getScalarNode(schemaNode, "description")
		parentExample := getMappingValue(schemaNode, "example")

		// Replace the schema node content with the merged result.
		schemaNode.Content = merged.Content

		// Re-add parent description, overriding any inherited from sub-schemas.
		if parentDesc != nil && parentDesc.Value != "" {
			removeMappingKey(schemaNode, "description")
			schemaNode.Content = append(
				[]*yaml.Node{
					{Kind: yaml.ScalarNode, Value: "description", Tag: "!!str"},
					{Kind: yaml.ScalarNode, Value: parentDesc.Value, Tag: "!!str", Style: parentDesc.Style},
				},
				schemaNode.Content...,
			)
		}

		// Re-add parent example, overriding any inherited from sub-schemas.
		if parentExample != nil {
			removeMappingKey(schemaNode, "example")
			schemaNode.Content = append(schemaNode.Content,
				&yaml.Node{Kind: yaml.ScalarNode, Value: "example", Tag: "!!str"},
				deepCopyNode(parentExample),
			)
		}

		fmt.Printf("Flattened allOf in schema %s\n", schemaName)
	}

	for i := 0; i < len(schemasNode.Content)-1; i += 2 {
		schemaName := schemasNode.Content[i].Value
		schemaNode := schemasNode.Content[i+1]
		flattenInlineAllOf(schemaNode, schemasNode, schemaName)
	}
}

// flattenInlineAllOf recursively walks a schema node's properties and
// flattens any inline multi-element allOf found in property values.
func flattenInlineAllOf(node *yaml.Node, schemasNode *yaml.Node, path string) {
	if node == nil || node.Kind != yaml.MappingNode {
		return
	}
	propsNode := getMappingValue(node, "properties")
	if propsNode == nil || propsNode.Kind != yaml.MappingNode {
		return
	}
	for i := 0; i < len(propsNode.Content)-1; i += 2 {
		propName := propsNode.Content[i].Value
		propVal := propsNode.Content[i+1]
		if propVal.Kind != yaml.MappingNode {
			continue
		}

		allOfNode := getMappingValue(propVal, "allOf")
		if allOfNode != nil && allOfNode.Kind == yaml.SequenceNode && len(allOfNode.Content) >= 2 {
			propPath := path + "." + propName
			merged := flattenAllOf(allOfNode, schemasNode, propPath)
			if merged != nil {
				parentDesc := getScalarNode(propVal, "description")
				parentExample := getMappingValue(propVal, "example")

				propVal.Content = merged.Content

				if parentDesc != nil && parentDesc.Value != "" {
					removeMappingKey(propVal, "description")
					propVal.Content = append(
						[]*yaml.Node{
							{Kind: yaml.ScalarNode, Value: "description", Tag: "!!str"},
							{Kind: yaml.ScalarNode, Value: parentDesc.Value, Tag: "!!str", Style: parentDesc.Style},
						},
						propVal.Content...,
					)
				}

				if parentExample != nil {
					removeMappingKey(propVal, "example")
					propVal.Content = append(propVal.Content,
						&yaml.Node{Kind: yaml.ScalarNode, Value: "example", Tag: "!!str"},
						deepCopyNode(parentExample),
					)
				}

				fmt.Printf("Flattened inline allOf in %s\n", propPath)
			}
		}

		flattenInlineAllOf(propsNode.Content[i+1], schemasNode, path+"."+propName)
	}
}

// flattenAllOf merges all sub-schemas of an allOf sequence node into a single
// flat object mapping node. Returns nil if any sub-schema cannot be resolved.
func flattenAllOf(allOfNode *yaml.Node, schemasNode *yaml.Node, contextName string) *yaml.Node {
	// Collect resolved sub-schemas.
	var resolved []*yaml.Node
	for _, sub := range allOfNode.Content {
		r := resolveSubSchema(sub, schemasNode)
		if r == nil {
			log.Printf("WARNING: cannot resolve allOf sub-schema in %s, skipping flatten", contextName)
			return nil
		}
		resolved = append(resolved, r)
	}

	// Build the merged result.
	result := &yaml.Node{Kind: yaml.MappingNode}

	// Set type: object
	result.Content = append(result.Content,
		&yaml.Node{Kind: yaml.ScalarNode, Value: "type", Tag: "!!str"},
		&yaml.Node{Kind: yaml.ScalarNode, Value: "object", Tag: "!!str"},
	)

	// Merge required arrays.
	requiredSet := make(map[string]bool)
	var requiredOrder []string
	for _, r := range resolved {
		reqNode := getMappingValue(r, "required")
		if reqNode == nil || reqNode.Kind != yaml.SequenceNode {
			continue
		}
		for _, item := range reqNode.Content {
			if !requiredSet[item.Value] {
				requiredSet[item.Value] = true
				requiredOrder = append(requiredOrder, item.Value)
			}
		}
	}
	if len(requiredOrder) > 0 {
		reqSeq := &yaml.Node{Kind: yaml.SequenceNode}
		for _, name := range requiredOrder {
			reqSeq.Content = append(reqSeq.Content,
				&yaml.Node{Kind: yaml.ScalarNode, Value: name, Tag: "!!str"},
			)
		}
		result.Content = append(result.Content,
			&yaml.Node{Kind: yaml.ScalarNode, Value: "required", Tag: "!!str"},
			reqSeq,
		)
	}

	// Merge properties. Later sub-schemas win on conflict.
	mergedProps := &yaml.Node{Kind: yaml.MappingNode}
	propIndex := make(map[string]int) // property name -> index in mergedProps.Content
	for _, r := range resolved {
		propsNode := getMappingValue(r, "properties")
		if propsNode == nil || propsNode.Kind != yaml.MappingNode {
			continue
		}
		for j := 0; j < len(propsNode.Content)-1; j += 2 {
			propKey := propsNode.Content[j].Value
			propVal := propsNode.Content[j+1]
			if idx, exists := propIndex[propKey]; exists {
				log.Printf("WARNING: property %q in %s defined in multiple allOf sub-schemas, using last definition", propKey, contextName)
				mergedProps.Content[idx+1] = deepCopyNode(propVal)
			} else {
				propIndex[propKey] = len(mergedProps.Content)
				mergedProps.Content = append(mergedProps.Content,
					deepCopyNode(propsNode.Content[j]),
					deepCopyNode(propVal),
				)
			}
		}
	}
	if len(mergedProps.Content) > 0 {
		result.Content = append(result.Content,
			&yaml.Node{Kind: yaml.ScalarNode, Value: "properties", Tag: "!!str"},
			mergedProps,
		)
	}

	// Copy over any other top-level keys from sub-schemas that aren't
	// type/required/properties (e.g., description, example on sub-schemas).
	for _, r := range resolved {
		if r.Kind != yaml.MappingNode {
			continue
		}
		for j := 0; j < len(r.Content)-1; j += 2 {
			key := r.Content[j].Value
			if key == "type" || key == "required" || key == "properties" {
				continue
			}
			if getMappingValue(result, key) == nil {
				result.Content = append(result.Content,
					deepCopyNode(r.Content[j]),
					deepCopyNode(r.Content[j+1]),
				)
			}
		}
	}

	return result
}

// resolveSubSchema resolves a single allOf sub-schema. If it's a $ref, it
// looks up the target in schemasNode and returns a deep copy. If it's an
// inline object, it returns a deep copy directly.
func resolveSubSchema(node *yaml.Node, schemasNode *yaml.Node) *yaml.Node {
	if node.Kind != yaml.MappingNode {
		return nil
	}
	refVal := getScalarValue(node, "$ref")
	if refVal != "" {
		const prefix = "#/components/schemas/"
		if !strings.HasPrefix(refVal, prefix) {
			return nil
		}
		name := refVal[len(prefix):]
		for i := 0; i < len(schemasNode.Content)-1; i += 2 {
			if schemasNode.Content[i].Value == name {
				return deepCopyNode(schemasNode.Content[i+1])
			}
		}
		return nil
	}
	return deepCopyNode(node)
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

			// Preserve description from the inline definition.
			// We capture the full scalar node (not just the value) to preserve
			// the original Style (e.g., folded >- or literal | block styles).
			descNode := getScalarNode(oldNode, "description")

			// Build new $ref node
			refNode := &yaml.Node{
				Kind: yaml.MappingNode,
				Content: []*yaml.Node{
					{Kind: yaml.ScalarNode, Value: "$ref", Tag: "!!str"},
					{Kind: yaml.ScalarNode, Value: fmt.Sprintf("#/components/schemas/%s", schemaName), Tag: "!!str"},
				},
			}

			// If description exists, wrap in allOf to preserve it
			if descNode != nil && descNode.Value != "" {
				parentMapping.Content[i+1] = &yaml.Node{
					Kind: yaml.MappingNode,
					Content: []*yaml.Node{
						{Kind: yaml.ScalarNode, Value: "description", Tag: "!!str"},
						{Kind: yaml.ScalarNode, Value: descNode.Value, Tag: "!!str", Style: descNode.Style},
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
		origMap, ok1 := resolvedOrig.(map[string]any)
		procMap, ok2 := resolvedProc.(map[string]any)
		if !ok1 || !ok2 {
			return fmt.Errorf("paths differ after resolution (unexpected type)")
		}
		for path, origOp := range origMap {
			procOp, ok := procMap[path]
			if !ok {
				return fmt.Errorf("path %q missing from processed spec", path)
			}
			if !reflect.DeepEqual(origOp, procOp) {
				return fmt.Errorf("path %q differs after resolution", path)
			}
		}
		// Check for extra paths in processed
		for path := range procMap {
			if _, ok := origMap[path]; !ok {
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
		// Check if this is a $ref (with or without sibling keywords).
		// OAS 3.1 allows $ref with siblings; we resolve the $ref and
		// merge the non-$ref siblings into the result so that equivalence
		// checking works correctly after wrapRefSiblings transforms them.
		if ref, ok := val["$ref"].(string); ok {
			const prefix = "#/components/schemas/"
			if strings.HasPrefix(ref, prefix) {
				name := ref[len(prefix):]
				if visited[name] {
					return val
				}
				if schema, ok := schemas[name]; ok {
					newVisited := make(map[string]bool, len(visited)+1)
					maps.Copy(newVisited, visited)
					newVisited[name] = true
					resolved := resolveRefsWithVisited(schema, schemas, newVisited)
					if len(val) > 1 {
						if resolvedMap, ok := resolved.(map[string]any); ok {
							merged := make(map[string]any, len(resolvedMap)+len(val))
							for k, v := range resolvedMap {
								merged[k] = v
							}
							for k, v := range val {
								if k != "$ref" {
									merged[k] = resolveRefsWithVisited(v, schemas, newVisited)
								}
							}
							return merged
						}
					}
					return resolved
				}
			}
			if len(val) == 1 {
				return val
			}
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
		// Unwrap single-element allOf/anyOf/oneOf.
		// Case 1: {allOf: [{resolved_schema}]} → {resolved_schema}
		// Case 2: {allOf: [{resolved_schema}], nullable: true} → {resolved_schema..., nullable: true}
		// Case 2 handles patterns from wrapRefSiblings where $ref+siblings
		// becomes allOf+siblings, which must resolve equivalently.
		for _, compKey := range []string{"allOf", "anyOf", "oneOf"} {
			items, ok := result[compKey].([]any)
			if !ok || len(items) != 1 {
				continue
			}
			inner, ok := items[0].(map[string]any)
			if !ok {
				continue
			}
			if len(result) == 1 {
				return inner
			}
			merged := make(map[string]any, len(inner)+len(result))
			for k, v := range inner {
				merged[k] = v
			}
			for k, v := range result {
				if k != compKey {
					merged[k] = v
				}
			}
			return merged
		}
		// Merge multi-element allOf: when all elements are objects, merge their
		// properties and required arrays into a single flat object.
		// This ensures the validator treats the original allOf as equivalent to
		// the flattened version produced by flattenAllOfSchemas.
		if items, ok := result["allOf"].([]any); ok && len(items) >= 2 {
			if merged, ok := mergeAllOfItems(items); ok {
				return merged
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

// mergeAllOfItems merges multiple allOf sub-schema maps into a single flat
// object map. Returns (merged, true) if all items are object maps, or
// (nil, false) if any item is not a map.
func mergeAllOfItems(items []any) (map[string]any, bool) {
	merged := make(map[string]any)
	for _, item := range items {
		obj, ok := item.(map[string]any)
		if !ok {
			return nil, false
		}
		for k, v := range obj {
			switch k {
			case "properties":
				existing, _ := merged["properties"].(map[string]any)
				if existing == nil {
					existing = make(map[string]any)
				}
				if newProps, ok := v.(map[string]any); ok {
					maps.Copy(existing, newProps)
				}
				merged["properties"] = existing
			case "required":
				existing, _ := merged["required"].([]any)
				if newReq, ok := v.([]any); ok {
					seen := make(map[string]bool, len(existing))
					for _, r := range existing {
						if s, ok := r.(string); ok {
							seen[s] = true
						}
					}
					for _, r := range newReq {
						if s, ok := r.(string); ok && !seen[s] {
							existing = append(existing, r)
							seen[s] = true
						}
					}
				}
				merged["required"] = existing
			default:
				merged[k] = v
			}
		}
	}
	return merged, true
}
