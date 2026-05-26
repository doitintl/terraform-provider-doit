package main

import (
	"os"
	"path/filepath"
	"testing"

	"gopkg.in/yaml.v3"
)

// --- loadProviderConfigs ---

func TestLoadProviderConfigs(t *testing.T) {
	dir := t.TempDir()

	datasources := filepath.Join(dir, "datasources.yml")
	resources := filepath.Join(dir, "resources.yml")

	dsContent := `
provider:
  name: test
data_sources:
  user:
    read:
      path: /users/{id}
      method: GET
  users:
    read:
      path: /users
      method: GET
`
	resContent := `
provider:
  name: test
resources:
  user:
    create:
      path: /users
      method: POST
    read:
      path: /users/{id}
      method: GET
    update:
      path: /users/{id}
      method: PATCH
    delete:
      path: /users/{id}
      method: DELETE
`

	if err := os.WriteFile(datasources, []byte(dsContent), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(resources, []byte(resContent), 0600); err != nil {
		t.Fatal(err)
	}

	used, err := loadProviderConfigs(datasources, resources)
	if err != nil {
		t.Fatalf("loadProviderConfigs: %v", err)
	}

	expected := []pathMethod{
		{"/users/{id}", "get"},
		{"/users", "get"},
		{"/users", "post"},
		{"/users/{id}", "patch"},
		{"/users/{id}", "delete"},
	}

	for _, pm := range expected {
		if !used[pm] {
			t.Errorf("expected %s %s to be used", pm.method, pm.path)
		}
	}

	// Should not contain uppercase or unknown paths
	if used[pathMethod{"/users/{id}", "GET"}] {
		t.Error("methods should be lowercased")
	}
}

func TestLoadProviderConfigs_FileNotFound(t *testing.T) {
	_, err := loadProviderConfigs("/nonexistent/datasources.yml", "/nonexistent/resources.yml")
	if err == nil {
		t.Error("expected error for missing file")
	}
}

// --- loadExtraPaths ---

func TestLoadExtraPaths(t *testing.T) {
	dir := t.TempDir()
	extraFile := filepath.Join(dir, "extra_paths.yml")

	content := `
paths:
  - path: /reports/query
    method: POST
  - path: /reports/{id}
    method: GET
`
	if err := os.WriteFile(extraFile, []byte(content), 0600); err != nil {
		t.Fatal(err)
	}

	used := make(map[pathMethod]bool)
	if err := loadExtraPaths(extraFile, used); err != nil {
		t.Fatalf("loadExtraPaths: %v", err)
	}

	if !used[pathMethod{"/reports/query", "post"}] {
		t.Error("expected /reports/query POST to be used")
	}
	if !used[pathMethod{"/reports/{id}", "get"}] {
		t.Error("expected /reports/{id} GET to be used")
	}
}

func TestLoadExtraPaths_FileNotFound(t *testing.T) {
	used := make(map[pathMethod]bool)
	err := loadExtraPaths("/nonexistent/extra.yml", used)
	if err == nil {
		t.Error("expected error for missing file")
	}
}

// --- prunePaths ---

func TestPrunePaths(t *testing.T) {
	spec := `
/users:
  get:
    operationId: listUsers
  post:
    operationId: createUser
/users/{id}:
  get:
    operationId: getUser
  delete:
    operationId: deleteUser
/admin/stats:
  get:
    operationId: getAdminStats
`
	pathsNode := mustParseYAML(t, spec)

	used := map[pathMethod]bool{
		{"/users", "get"}:      true,
		{"/users/{id}", "get"}: true,
	}

	pathsRemoved, methodsPruned := prunePaths(pathsNode, used)

	// /admin/stats should be removed entirely
	if pathsRemoved != 1 {
		t.Errorf("pathsRemoved = %d, want 1", pathsRemoved)
	}

	// POST /users and DELETE /users/{id} should be pruned
	if methodsPruned != 2 {
		t.Errorf("methodsPruned = %d, want 2", methodsPruned)
	}

	// Verify remaining paths
	if pathsNode.Kind != yaml.MappingNode {
		t.Fatal("expected mapping node")
	}

	// Should have 2 paths remaining (4 content nodes: key+value pairs)
	if len(pathsNode.Content) != 4 {
		t.Errorf("remaining path entries = %d, want 4 (2 paths)", len(pathsNode.Content))
	}

	// Check /users only has GET
	usersPath := getMappingValue(pathsNode, "/users")
	if usersPath == nil {
		t.Fatal("/users path missing")
	}
	if len(usersPath.Content) != 2 { // just "get" key + value
		t.Errorf("/users methods = %d content nodes, want 2", len(usersPath.Content))
	}
}

func TestPrunePaths_NoUsed(t *testing.T) {
	spec := `
/users:
  get:
    operationId: listUsers
`
	pathsNode := mustParseYAML(t, spec)
	used := map[pathMethod]bool{}

	pathsRemoved, _ := prunePaths(pathsNode, used)
	if pathsRemoved != 1 {
		t.Errorf("pathsRemoved = %d, want 1", pathsRemoved)
	}
	if len(pathsNode.Content) != 0 {
		t.Errorf("remaining content = %d, want 0", len(pathsNode.Content))
	}
}

func TestPrunePaths_NonHTTPKeys(t *testing.T) {
	// Paths can have non-HTTP keys like "parameters" at the path level
	spec := `
/users:
  parameters:
    - name: org
      in: query
  get:
    operationId: listUsers
`
	pathsNode := mustParseYAML(t, spec)
	used := map[pathMethod]bool{
		{"/users", "get"}: true,
	}

	pathsRemoved, methodsPruned := prunePaths(pathsNode, used)
	if pathsRemoved != 0 {
		t.Errorf("pathsRemoved = %d, want 0", pathsRemoved)
	}
	if methodsPruned != 0 {
		t.Errorf("methodsPruned = %d, want 0", methodsPruned)
	}

	// Both "parameters" and "get" should remain
	usersPath := getMappingValue(pathsNode, "/users")
	if usersPath == nil {
		t.Fatal("/users path missing")
	}
	if len(usersPath.Content) != 4 { // parameters + get
		t.Errorf("/users content nodes = %d, want 4", len(usersPath.Content))
	}
}

// --- collectReachableSchemas ---

func TestCollectReachableSchemas(t *testing.T) {
	paths := mustParseYAML(t, `
/users:
  get:
    responses:
      "200":
        content:
          application/json:
            schema:
              $ref: "#/components/schemas/UserList"
`)

	schemas := mustParseYAML(t, `
UserList:
  type: object
  properties:
    items:
      type: array
      items:
        $ref: "#/components/schemas/User"
User:
  type: object
  properties:
    name:
      type: string
    role:
      $ref: "#/components/schemas/Role"
Role:
  type: object
  properties:
    name:
      type: string
Unused:
  type: object
  properties:
    foo:
      type: string
`)

	reachable := collectReachableSchemas(paths, schemas)

	for _, name := range []string{"UserList", "User", "Role"} {
		if !reachable[name] {
			t.Errorf("expected %q to be reachable", name)
		}
	}
	if reachable["Unused"] {
		t.Error("Unused should not be reachable")
	}
}

func TestCollectReachableSchemas_CircularRef(t *testing.T) {
	paths := mustParseYAML(t, `
/tree:
  get:
    responses:
      "200":
        content:
          application/json:
            schema:
              $ref: "#/components/schemas/TreeNode"
`)

	schemas := mustParseYAML(t, `
TreeNode:
  type: object
  properties:
    children:
      type: array
      items:
        $ref: "#/components/schemas/TreeNode"
`)

	// Should not infinite loop
	reachable := collectReachableSchemas(paths, schemas)
	if !reachable["TreeNode"] {
		t.Error("TreeNode should be reachable")
	}
}

func TestCollectReachableSchemas_AllOfRef(t *testing.T) {
	paths := mustParseYAML(t, `
/items:
  get:
    responses:
      "200":
        content:
          application/json:
            schema:
              allOf:
                - $ref: "#/components/schemas/Base"
                - type: object
                  properties:
                    extra:
                      $ref: "#/components/schemas/Extra"
`)

	schemas := mustParseYAML(t, `
Base:
  type: object
  properties:
    id:
      type: string
Extra:
  type: object
  properties:
    value:
      type: string
Orphan:
  type: object
`)

	reachable := collectReachableSchemas(paths, schemas)

	if !reachable["Base"] {
		t.Error("Base should be reachable via allOf")
	}
	if !reachable["Extra"] {
		t.Error("Extra should be reachable via allOf property")
	}
	if reachable["Orphan"] {
		t.Error("Orphan should not be reachable")
	}
}

func TestCollectReachableSchemas_ResponseRef(t *testing.T) {
	// Simulate a full spec with components/responses that reference schemas
	spec := `
openapi: "3.0.1"
info:
  title: Test
  version: v1
paths:
  /users:
    get:
      responses:
        "200":
          content:
            application/json:
              schema:
                $ref: "#/components/schemas/User"
        "400":
          $ref: "#/components/responses/BadRequest"
components:
  schemas:
    User:
      type: object
      properties:
        name:
          type: string
    Error:
      type: object
      properties:
        message:
          type: string
    Unused:
      type: object
  responses:
    BadRequest:
      description: Bad Request
      content:
        application/json:
          schema:
            $ref: "#/components/schemas/Error"
`
	var root yaml.Node
	if err := yaml.Unmarshal([]byte(spec), &root); err != nil {
		t.Fatal(err)
	}
	doc := root.Content[0]

	pathsNode := findMapValue(doc, "paths")
	schemasNode := findMapValue(doc, "components", "schemas")

	// Set pruneDocRoot so response refs can be resolved
	pruneDocRoot = doc
	defer func() { pruneDocRoot = nil }()

	reachable := collectReachableSchemas(pathsNode, schemasNode)

	if !reachable["User"] {
		t.Error("User should be reachable directly")
	}
	if !reachable["Error"] {
		t.Error("Error should be reachable via components/responses/BadRequest")
	}
	if reachable["Unused"] {
		t.Error("Unused should not be reachable")
	}
}

// --- pruneSchemas ---

func TestPruneSchemas(t *testing.T) {
	schemas := mustParseYAML(t, `
Alpha:
  type: object
Beta:
  type: object
Gamma:
  type: object
`)

	reachable := map[string]bool{
		"Alpha": true,
		"Gamma": true,
	}

	removed := pruneSchemas(schemas, reachable)
	if removed != 1 {
		t.Errorf("removed = %d, want 1", removed)
	}

	// Should have Alpha and Gamma
	if schemas.Kind != yaml.MappingNode {
		t.Fatal("expected mapping node")
	}
	if len(schemas.Content) != 4 { // 2 pairs
		t.Errorf("remaining schema entries = %d content nodes, want 4", len(schemas.Content))
	}

	if getMappingValue(schemas, "Alpha") == nil {
		t.Error("Alpha should remain")
	}
	if getMappingValue(schemas, "Beta") != nil {
		t.Error("Beta should be pruned")
	}
	if getMappingValue(schemas, "Gamma") == nil {
		t.Error("Gamma should remain")
	}
}

func TestPruneSchemas_AllReachable(t *testing.T) {
	schemas := mustParseYAML(t, `
A:
  type: object
B:
  type: object
`)
	reachable := map[string]bool{"A": true, "B": true}
	removed := pruneSchemas(schemas, reachable)
	if removed != 0 {
		t.Errorf("removed = %d, want 0", removed)
	}
}

// --- End-to-end pruning ---

func TestEndToEndPrune(t *testing.T) {
	spec := `
openapi: "3.0.1"
info:
  title: Test API
  version: v1
paths:
  /users:
    get:
      operationId: listUsers
      responses:
        "200":
          description: OK
          content:
            application/json:
              schema:
                type: array
                items:
                  $ref: "#/components/schemas/User"
    post:
      operationId: createUser
      requestBody:
        content:
          application/json:
            schema:
              $ref: "#/components/schemas/CreateUserRequest"
  /admin/stats:
    get:
      operationId: getStats
      responses:
        "200":
          description: OK
          content:
            application/json:
              schema:
                $ref: "#/components/schemas/AdminStats"
components:
  schemas:
    User:
      type: object
      properties:
        name:
          type: string
        role:
          $ref: "#/components/schemas/Role"
    Role:
      type: object
      properties:
        name:
          type: string
    CreateUserRequest:
      type: object
      properties:
        name:
          type: string
    AdminStats:
      type: object
      properties:
        count:
          type: integer
    OrphanSchema:
      type: object
      properties:
        unused:
          type: string
`

	var root yaml.Node
	if err := yaml.Unmarshal([]byte(spec), &root); err != nil {
		t.Fatalf("failed to parse spec: %v", err)
	}
	doc := root.Content[0]

	pathsNode := findMapValue(doc, "paths")
	schemasNode := findMapValue(doc, "components", "schemas")

	// Only keep GET /users
	used := map[pathMethod]bool{
		{"/users", "get"}: true,
	}

	pathsRemoved, methodsPruned := prunePaths(pathsNode, used)
	if pathsRemoved != 1 { // /admin/stats removed
		t.Errorf("pathsRemoved = %d, want 1", pathsRemoved)
	}
	if methodsPruned != 1 { // POST /users pruned
		t.Errorf("methodsPruned = %d, want 1", methodsPruned)
	}

	pruneDocRoot = doc
	defer func() { pruneDocRoot = nil }()

	reachable := collectReachableSchemas(pathsNode, schemasNode)

	// User and Role should be reachable (User -> Role via $ref)
	if !reachable["User"] {
		t.Error("User should be reachable")
	}
	if !reachable["Role"] {
		t.Error("Role should be reachable")
	}
	// CreateUserRequest should NOT be reachable (POST /users was pruned)
	if reachable["CreateUserRequest"] {
		t.Error("CreateUserRequest should not be reachable (POST was pruned)")
	}
	// AdminStats should NOT be reachable (/admin/stats was removed)
	if reachable["AdminStats"] {
		t.Error("AdminStats should not be reachable")
	}
	// OrphanSchema should NOT be reachable
	if reachable["OrphanSchema"] {
		t.Error("OrphanSchema should not be reachable")
	}

	removed := pruneSchemas(schemasNode, reachable)
	if removed != 3 { // CreateUserRequest, AdminStats, OrphanSchema
		t.Errorf("schemas removed = %d, want 3", removed)
	}

	// Verify only User and Role remain
	if getMappingValue(schemasNode, "User") == nil {
		t.Error("User should remain")
	}
	if getMappingValue(schemasNode, "Role") == nil {
		t.Error("Role should remain")
	}
}

// --- collectRefsFromNode ---

func TestCollectRefsFromNode(t *testing.T) {
	node := mustParseYAML(t, `
allOf:
  - $ref: "#/components/schemas/Foo"
  - type: object
    properties:
      bar:
        $ref: "#/components/schemas/Bar"
`)

	var schemaQueue, responseQueue, paramQueue []string
	collectRefsFromNode(node, &schemaQueue, &responseQueue, &paramQueue)

	if len(schemaQueue) != 2 {
		t.Fatalf("schemaQueue length = %d, want 2", len(schemaQueue))
	}

	found := map[string]bool{}
	for _, name := range schemaQueue {
		found[name] = true
	}
	if !found["Foo"] {
		t.Error("expected Foo in schemaQueue")
	}
	if !found["Bar"] {
		t.Error("expected Bar in schemaQueue")
	}
	if len(responseQueue) != 0 {
		t.Errorf("responseQueue should be empty, got %d", len(responseQueue))
	}
}

func TestCollectRefsFromNode_NonSchemaRef(t *testing.T) {
	node := mustParseYAML(t, `
$ref: "#/components/responses/NotFound"
`)

	var schemaQueue, responseQueue, paramQueue []string
	collectRefsFromNode(node, &schemaQueue, &responseQueue, &paramQueue)

	// Response refs should go to responseQueue
	if len(schemaQueue) != 0 {
		t.Errorf("schemaQueue length = %d, want 0", len(schemaQueue))
	}
	if len(responseQueue) != 1 {
		t.Fatalf("responseQueue length = %d, want 1", len(responseQueue))
	}
	if responseQueue[0] != "NotFound" {
		t.Errorf("responseQueue[0] = %q, want %q", responseQueue[0], "NotFound")
	}
}

func TestCollectRefsFromNode_ParameterRef(t *testing.T) {
	node := mustParseYAML(t, `
parameters:
  - $ref: "#/components/parameters/PageToken"
responses:
  "200":
    content:
      application/json:
        schema:
          $ref: "#/components/schemas/Result"
`)

	var schemaQueue, responseQueue, paramQueue []string
	collectRefsFromNode(node, &schemaQueue, &responseQueue, &paramQueue)

	if len(schemaQueue) != 1 {
		t.Fatalf("schemaQueue length = %d, want 1", len(schemaQueue))
	}
	if schemaQueue[0] != "Result" {
		t.Errorf("schemaQueue[0] = %q, want %q", schemaQueue[0], "Result")
	}
	if len(paramQueue) != 1 {
		t.Fatalf("paramQueue length = %d, want 1", len(paramQueue))
	}
	if paramQueue[0] != "PageToken" {
		t.Errorf("paramQueue[0] = %q, want %q", paramQueue[0], "PageToken")
	}
}

func TestCollectRefsFromNode_MixedRefs(t *testing.T) {
	node := mustParseYAML(t, `
"200":
  content:
    application/json:
      schema:
        $ref: "#/components/schemas/User"
"400":
  $ref: "#/components/responses/BadRequest"
"500":
  $ref: "#/components/responses/InternalError"
`)

	var schemaQueue, responseQueue, paramQueue []string
	collectRefsFromNode(node, &schemaQueue, &responseQueue, &paramQueue)

	if len(schemaQueue) != 1 || schemaQueue[0] != "User" {
		t.Errorf("schemaQueue = %v, want [User]", schemaQueue)
	}
	if len(responseQueue) != 2 {
		t.Errorf("responseQueue length = %d, want 2", len(responseQueue))
	}
}
