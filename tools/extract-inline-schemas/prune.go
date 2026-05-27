package main

import (
	"fmt"
	"os"
	"strings"

	"gopkg.in/yaml.v3"
)

// pathMethod identifies a specific API endpoint by its path and HTTP method.
type pathMethod struct {
	path   string
	method string // lowercase: "get", "post", etc.
}

// providerConfig represents the structure of datasources.yml or resources.yml.
type providerConfig struct {
	DataSources map[string]map[string]struct {
		Path   string `yaml:"path"`
		Method string `yaml:"method"`
	} `yaml:"data_sources"`
	Resources map[string]map[string]struct {
		Path   string `yaml:"path"`
		Method string `yaml:"method"`
	} `yaml:"resources"`
}

// extraPathsConfig represents the structure of the extra-paths YAML file.
// It is a simple list of path+method pairs.
type extraPathsConfig struct {
	Paths []struct {
		Path   string `yaml:"path"`
		Method string `yaml:"method"`
	} `yaml:"paths"`
}

// loadProviderConfigs parses the provider datasources and resources YAML config
// files and returns the set of path+method pairs that the provider uses.
func loadProviderConfigs(datasourcesPath, resourcesPath string) (map[pathMethod]bool, error) {
	used := make(map[pathMethod]bool)

	for _, filePath := range []string{datasourcesPath, resourcesPath} {
		data, err := os.ReadFile(filePath)
		if err != nil {
			return nil, fmt.Errorf("reading %s: %w", filePath, err)
		}

		var cfg providerConfig
		if err := yaml.Unmarshal(data, &cfg); err != nil {
			return nil, fmt.Errorf("parsing %s: %w", filePath, err)
		}

		// Process data_sources
		for _, ops := range cfg.DataSources {
			for _, details := range ops {
				if details.Path != "" && details.Method != "" {
					used[pathMethod{
						path:   details.Path,
						method: strings.ToLower(details.Method),
					}] = true
				}
			}
		}

		// Process resources
		for _, ops := range cfg.Resources {
			for _, details := range ops {
				if details.Path != "" && details.Method != "" {
					used[pathMethod{
						path:   details.Path,
						method: strings.ToLower(details.Method),
					}] = true
				}
			}
		}
	}

	return used, nil
}

// loadExtraPaths parses the extra-paths YAML file and adds its entries to the
// used set. This supports manually-written resources that don't have full
// entries in datasources.yml or resources.yml.
func loadExtraPaths(extraPathsFile string, used map[pathMethod]bool) error {
	data, err := os.ReadFile(extraPathsFile)
	if err != nil {
		return fmt.Errorf("reading %s: %w", extraPathsFile, err)
	}

	var cfg extraPathsConfig
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return fmt.Errorf("parsing %s: %w", extraPathsFile, err)
	}

	for _, entry := range cfg.Paths {
		if entry.Path != "" && entry.Method != "" {
			used[pathMethod{
				path:   entry.Path,
				method: strings.ToLower(entry.Method),
			}] = true
		}
	}

	return nil
}

// prunePaths removes unused paths and HTTP methods from the paths mapping node.
// It prunes at the method level: if a path has some methods used and others not,
// only the unused methods are removed. If no methods remain, the entire path
// entry is removed.
// Returns the number of paths removed entirely and methods pruned.
func prunePaths(pathsNode *yaml.Node, used map[pathMethod]bool) (pathsRemoved, methodsPruned int) {
	if pathsNode.Kind != yaml.MappingNode {
		return 0, 0
	}

	var newContent []*yaml.Node
	for i := 0; i < len(pathsNode.Content)-1; i += 2 {
		pathKey := pathsNode.Content[i]
		pathValue := pathsNode.Content[i+1]
		pathStr := pathKey.Value

		if pathValue.Kind != yaml.MappingNode {
			newContent = append(newContent, pathKey, pathValue)
			continue
		}

		// Check if any method on this path is used
		anyUsed := false
		for j := 0; j < len(pathValue.Content)-1; j += 2 {
			method := pathValue.Content[j].Value
			if isHTTPMethod(method) && used[pathMethod{path: pathStr, method: method}] {
				anyUsed = true
				break
			}
		}

		if !anyUsed {
			pathsRemoved++
			continue
		}

		// Prune individual methods
		var methodContent []*yaml.Node
		for j := 0; j < len(pathValue.Content)-1; j += 2 {
			methodKey := pathValue.Content[j]
			methodValue := pathValue.Content[j+1]
			method := methodKey.Value

			if isHTTPMethod(method) && !used[pathMethod{path: pathStr, method: method}] {
				methodsPruned++
				continue
			}
			methodContent = append(methodContent, methodKey, methodValue)
		}

		pathValue.Content = methodContent
		newContent = append(newContent, pathKey, pathValue)
	}

	pathsNode.Content = newContent
	return pathsRemoved, methodsPruned
}

// collectReachableSchemas performs a BFS traversal starting from the
// remaining paths' request/response bodies, parameters, and any schemas they
// reference via $ref. It also follows $ref chains through components/responses
// and components/parameters using the provided docRoot to locate sibling
// component sections. Returns the set of schema names that are transitively
// reachable.
func collectReachableSchemas(docRoot, pathsNode, schemasNode *yaml.Node) map[string]bool {
	reachable := make(map[string]bool)
	var schemaQueue []string
	var responseQueue []string
	var paramQueue []string

	// Collect initial refs from paths
	if pathsNode != nil && pathsNode.Kind == yaml.MappingNode {
		collectRefsFromNode(pathsNode, &schemaQueue, &responseQueue, &paramQueue)
	}

	// Track visited responses/params to avoid infinite loops
	visitedResponses := make(map[string]bool)
	visitedParams := make(map[string]bool)

	// BFS: resolve transitive refs through schemas, responses, and parameters
	for len(schemaQueue) > 0 || len(responseQueue) > 0 || len(paramQueue) > 0 {
		// Process schema refs
		if len(schemaQueue) > 0 {
			name := schemaQueue[0]
			schemaQueue = schemaQueue[1:]

			if reachable[name] {
				continue
			}
			reachable[name] = true

			// Find this schema in components/schemas and collect its refs
			if schemasNode != nil {
				schemaNode := getMappingValue(schemasNode, name)
				if schemaNode != nil {
					collectRefsFromNode(schemaNode, &schemaQueue, &responseQueue, &paramQueue)
				}
			}
			continue
		}

		// Process response refs — resolve through components/responses
		if len(responseQueue) > 0 {
			name := responseQueue[0]
			responseQueue = responseQueue[1:]

			if visitedResponses[name] {
				continue
			}
			visitedResponses[name] = true

			if docRoot != nil {
				responsesNode := findMapValue(docRoot, "components", "responses")
				if responsesNode != nil {
					responseNode := getMappingValue(responsesNode, name)
					if responseNode != nil {
						collectRefsFromNode(responseNode, &schemaQueue, &responseQueue, &paramQueue)
					}
				}
			}
			continue
		}

		// Process parameter refs — resolve through components/parameters
		if len(paramQueue) > 0 {
			name := paramQueue[0]
			paramQueue = paramQueue[1:]

			if visitedParams[name] {
				continue
			}
			visitedParams[name] = true

			if docRoot != nil {
				paramsNode := findMapValue(docRoot, "components", "parameters")
				if paramsNode != nil {
					paramNode := getMappingValue(paramsNode, name)
					if paramNode != nil {
						collectRefsFromNode(paramNode, &schemaQueue, &responseQueue, &paramQueue)
					}
				}
			}
			continue
		}
	}

	return reachable
}

// collectRefsFromNode recursively walks a YAML node tree and appends any
// referenced names to the appropriate queue based on their $ref prefix:
//   - "#/components/schemas/Foo"     → schemaQueue
//   - "#/components/responses/400"   → responseQueue
//   - "#/components/parameters/Bar"  → paramQueue
func collectRefsFromNode(node *yaml.Node, schemaQueue, responseQueue, paramQueue *[]string) {
	if node == nil {
		return
	}

	switch node.Kind {
	case yaml.MappingNode:
		for i := 0; i < len(node.Content)-1; i += 2 {
			key := node.Content[i].Value
			value := node.Content[i+1]

			if key == "$ref" && value.Kind == yaml.ScalarNode {
				ref := value.Value
				switch {
				case strings.HasPrefix(ref, "#/components/schemas/"):
					name := ref[len("#/components/schemas/"):]
					*schemaQueue = append(*schemaQueue, name)
				case strings.HasPrefix(ref, "#/components/responses/"):
					name := ref[len("#/components/responses/"):]
					*responseQueue = append(*responseQueue, name)
				case strings.HasPrefix(ref, "#/components/parameters/"):
					name := ref[len("#/components/parameters/"):]
					*paramQueue = append(*paramQueue, name)
				}
			} else {
				collectRefsFromNode(value, schemaQueue, responseQueue, paramQueue)
			}
		}
	case yaml.SequenceNode:
		for _, child := range node.Content {
			collectRefsFromNode(child, schemaQueue, responseQueue, paramQueue)
		}
	}
}

// pruneSchemas removes schemas from components/schemas that are not in the
// reachable set. Returns the number of schemas removed.
func pruneSchemas(schemasNode *yaml.Node, reachable map[string]bool) int {
	if schemasNode.Kind != yaml.MappingNode {
		return 0
	}

	removed := 0
	var newContent []*yaml.Node
	for i := 0; i < len(schemasNode.Content)-1; i += 2 {
		name := schemasNode.Content[i].Value
		if reachable[name] {
			newContent = append(newContent, schemasNode.Content[i], schemasNode.Content[i+1])
		} else {
			removed++
		}
	}

	schemasNode.Content = newContent
	return removed
}

// collectReachableResponses scans the retained paths and returns the set of
// response names referenced via $ref "#/components/responses/...".
func collectReachableResponses(pathsNode *yaml.Node) map[string]bool {
	reachable := make(map[string]bool)
	if pathsNode == nil {
		return reachable
	}

	var schemaQueue, responseQueue, paramQueue []string
	collectRefsFromNode(pathsNode, &schemaQueue, &responseQueue, &paramQueue)

	for _, name := range responseQueue {
		reachable[name] = true
	}
	return reachable
}

// pruneResponses removes response entries from components/responses that are
// not in the reachable set. Returns the number of responses removed.
func pruneResponses(responsesNode *yaml.Node, reachable map[string]bool) int {
	if responsesNode.Kind != yaml.MappingNode {
		return 0
	}

	removed := 0
	var newContent []*yaml.Node
	for i := 0; i < len(responsesNode.Content)-1; i += 2 {
		name := responsesNode.Content[i].Value
		if reachable[name] {
			newContent = append(newContent, responsesNode.Content[i], responsesNode.Content[i+1])
		} else {
			removed++
		}
	}

	responsesNode.Content = newContent
	return removed
}
